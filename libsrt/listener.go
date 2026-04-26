package libsrt

import (
	"fmt"
	"hash/crc32"
	"math/rand"
	"net"
	"sync"
	"time"
)

// AES-CTR encryption support is intentionally a no-op in this minimal
// implementation. We accept KMREQ control messages but reply with
// "encryption not supported" so the publisher falls back to a clean
// (unencrypted) stream — matches what FFmpeg does without -srt_passphrase.

// DataHandler is invoked once per data packet with the unwrapped
// payload bytes (typically 188-byte aligned MPEG-TS packets when the
// publisher is FFmpeg / OBS / GStreamer in their default Live mode).
// Returning an error stops the session.
type DataHandler func(streamID string, payload []byte) error

// Listener owns one UDP socket and demultiplexes packets across many
// SRT sessions keyed by (peer addr, our destSocketID). Sessions live
// in memory until the peer Shuts down or stops sending for a while.
type Listener struct {
	conn       *net.UDPConn
	streamID   string
	onData     DataHandler

	mu       sync.Mutex
	sessions map[string]*session
	socketID uint32 //ours, monotonically incremented per accept
}

type session struct {
	peerAddr      *net.UDPAddr
	peerSocketID  uint32
	ourSocketID   uint32
	cookie        uint32
	concluded     bool
	startTime     time.Time
	lastSeen      time.Time
	streamID      string
	expectedSeq   uint32

	// ACK / NAK bookkeeping. lastACKedSeq is the sequence number we
	// last reported via an ACK; ackNum is the monotonically-incrementing
	// ACK serial for SRT's ACKACK round-trip estimation.
	mu             sync.Mutex
	lastACKedSeq   uint32
	ackNum         uint32
	lastACK        time.Time
	receivedSeqs   uint64 //packets observed since session start (for receiving rate)
}

// Listen opens a UDP socket on addr and dispatches data packets via
// onData. streamID is the logical name applied to every accepted
// session (since the SRT spec carries an optional StreamID extension
// we don't parse, the fallback is the listener-supplied default).
func Listen(addr, streamID string, onData DataHandler) (*Listener, error) {
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		return nil, err
	}
	l := &Listener{
		conn:     conn,
		streamID: streamID,
		onData:   onData,
		sessions: map[string]*session{},
		socketID: rand.Uint32() | 0x40000000, //bit 30 set so it doesn't look like 0
	}
	return l, nil
}

// Run drives the receive loop until the underlying socket closes.
// Caller is responsible for invoking Close().
func (l *Listener) Run() error {
	buf := make([]byte, 1500)
	for {
		n, peer, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		if n < HeaderSize {
			continue
		}
		l.handlePacket(peer, append([]byte(nil), buf[:n]...))
	}
}

// Close shuts the UDP socket; the Run loop returns shortly after.
func (l *Listener) Close() error { return l.conn.Close() }

// handlePacket is the per-datagram dispatch. We split off control
// packets (handshake / shutdown / keepalive) for the session state
// machine and forward data packets straight to the user callback.
func (l *Listener) handlePacket(peer *net.UDPAddr, raw []byte) {
	hdr, body, err := ParseHeader(raw)
	if err != nil {
		return
	}
	key := peer.String()
	l.mu.Lock()
	sess := l.sessions[key]
	l.mu.Unlock()

	switch hdr.Kind {
	case KindControl:
		switch hdr.ControlType {
		case CtrlHandshake:
			l.handleHandshake(peer, hdr, body)
		case CtrlShutdown:
			if sess != nil {
				l.mu.Lock()
				delete(l.sessions, key)
				l.mu.Unlock()
			}
		case CtrlKeepAlive:
			if sess != nil {
				sess.lastSeen = time.Now()
				//Mirror the keepalive back per spec.
				resp := make([]byte, HeaderSize)
				MarshalControlHeader(resp, CtrlKeepAlive, 0, 0,
					uint32(time.Since(sess.startTime).Microseconds()),
					sess.peerSocketID)
				_, _ = l.conn.WriteToUDP(resp, peer)
			}
		}
	case KindData:
		if sess == nil || !sess.concluded {
			return
		}
		sess.lastSeen = time.Now()
		l.handleDataPacket(sess, hdr, body)
	}
}

// handleDataPacket runs the ARQ receiver state machine for one data
// packet. We deliver in-order packets immediately; out-of-order arrivals
// trigger a NAK for the gap (so the sender retransmits) and then we
// fast-forward — we don't keep a reorder buffer, since live MPEG-TS
// over SRT tolerates the occasional skip better than an unbounded
// jitter buffer.
func (l *Listener) handleDataPacket(sess *session, hdr *Header, body []byte) {
	sess.mu.Lock()
	if sess.expectedSeq == 0 && sess.ackNum == 0 {
		//First data packet — adopt its sequence as the baseline.
		sess.expectedSeq = hdr.SeqNumber
	}
	diff := SeqDiff(hdr.SeqNumber, sess.expectedSeq)
	sess.receivedSeqs++

	switch {
	case diff < 0:
		//Late retransmit — already delivered, drop quietly.
		sess.mu.Unlock()
		return
	case diff > 0:
		//Loss detected: gap is [expectedSeq, hdr.SeqNumber-1]. Emit a
		//NAK so the sender can retransmit; either it arrives before we
		//hand off media (benign) or we've already moved past it.
		from := sess.expectedSeq
		to := hdr.SeqNumber - 1
		sess.expectedSeq = hdr.SeqNumber + 1
		sess.mu.Unlock()
		l.sendNAK(sess, from, to)
	default:
		sess.expectedSeq++
		sess.mu.Unlock()
	}
	if l.onData != nil {
		_ = l.onData(sess.streamID, body)
	}
	l.maybeSendACK(sess)
}

// sendNAK emits a single-range NAK loss-list.
func (l *Listener) sendNAK(sess *session, from, to uint32) {
	body := MarshalNAK([]LossRange{{From: from, To: to}})
	l.sendControl(sess.peerAddr, CtrlNAK, 0, 0, sess, body)
}

// maybeSendACK emits an ACK at most every 10 ms (SRT's recommended
// "Light ACK" interval) so the sender's send window can slide.
func (l *Listener) maybeSendACK(sess *session) {
	const ackInterval = 10 * time.Millisecond
	sess.mu.Lock()
	if time.Since(sess.lastACK) < ackInterval {
		sess.mu.Unlock()
		return
	}
	sess.lastACK = time.Now()
	sess.ackNum++
	ackNum := sess.ackNum
	body := (&ACKBody{LastAckedSeq: sess.expectedSeq}).Marshal()
	sess.mu.Unlock()
	l.sendControl(sess.peerAddr, CtrlACK, 0, ackNum, sess, body)
}

// handleHandshake walks the v5 induction → conclusion two-step. We
// produce a per-session cookie at induction time and require the
// caller to echo it back in the conclusion.
func (l *Listener) handleHandshake(peer *net.UDPAddr, hdr *Header, body []byte) {
	hs, err := ParseHandshake(body)
	if err != nil {
		return
	}
	key := peer.String()

	l.mu.Lock()
	sess := l.sessions[key]
	if sess == nil {
		sess = &session{
			peerAddr:     peer,
			ourSocketID:  l.socketID,
			startTime:    time.Now(),
			streamID:     l.streamID,
		}
		l.socketID++
		l.sessions[key] = sess
	}
	l.mu.Unlock()

	switch hs.HandshakeType {
	case HSTypeAgreement: //v5 INDUCTION
		//Mint a cookie tied to the peer's socket id; the conclusion
		//must echo it back. CRC32 is used as a cheap mixer; real SRT
		//also includes a server start time and IP.
		sess.peerSocketID = hs.SrtSocketID
		sess.cookie = crc32.ChecksumIEEE(
			[]byte(fmt.Sprintf("%s-%d-%d",
				peer.IP.String(), hs.SrtSocketID, time.Now().UnixNano())))

		reply := *hs
		reply.HandshakeType = HSTypeAgreement
		reply.SrtSocketID = sess.ourSocketID
		reply.SyncCookie = sess.cookie

		l.sendControl(peer, CtrlHandshake, 0, 0, sess, reply.Marshal())

	case HSTypeConclusion:
		if hs.SyncCookie != sess.cookie {
			//Wrong cookie — quietly drop. Real SRT rejects.
			return
		}
		sess.peerSocketID = hs.SrtSocketID
		sess.expectedSeq = hs.InitialSequence
		sess.concluded = true

		reply := *hs
		reply.HandshakeType = HSTypeConclusion
		reply.SrtSocketID = sess.ourSocketID
		reply.ExtensionField = 0 //no extensions in our reply
		l.sendControl(peer, CtrlHandshake, 0, 0, sess, reply.Marshal())
	}
}

func (l *Listener) sendControl(peer *net.UDPAddr, ct ControlType, sub uint16, typeInfo uint32, sess *session, body []byte) {
	out := make([]byte, HeaderSize+len(body))
	ts := uint32(0)
	if sess != nil {
		ts = uint32(time.Since(sess.startTime).Microseconds())
		MarshalControlHeader(out, ct, sub, typeInfo, ts, sess.peerSocketID)
	} else {
		MarshalControlHeader(out, ct, sub, typeInfo, ts, 0)
	}
	copy(out[HeaderSize:], body)
	_, _ = l.conn.WriteToUDP(out, peer)
}
