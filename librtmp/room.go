package librtmp

import (
	"fmt"
	"sync"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/sbraveyoung/GGmpeg/libflv"
)

type Room struct {
	RoomID    string
	Publisher *RTMP
	GOP       *broadcast.Broadcast

	// Cached sequence headers. Populated by the publisher the first
	// time it emits them and replayed to every new subscriber before
	// GOP playback begins so mid-join viewers can initialise their
	// decoders (an FLV viewer needs AVCDecoderConfigurationRecord +
	// AudioSpecificConfig + onMetaData up front).
	mu          sync.RWMutex
	videoSeqHdr *libflv.VideoTag
	audioSeqHdr *libflv.AudioTag
	metaTag     *libflv.MetaTag
	closed      bool
}

//NOTE: the room must be created by publisher
func NewRoom(rtmp *RTMP, roomID string) *Room {
	r := &Room{
		RoomID:    roomID,
		Publisher: rtmp,
		GOP:       broadcast.NewBroadcast(3),
	}
	return r
}

func (room *Room) setVideoSequenceHeader(tag *libflv.VideoTag) {
	room.mu.Lock()
	room.videoSeqHdr = tag
	room.mu.Unlock()
}

func (room *Room) setAudioSequenceHeader(tag *libflv.AudioTag) {
	room.mu.Lock()
	room.audioSeqHdr = tag
	room.mu.Unlock()
}

func (room *Room) setMeta(tag *libflv.MetaTag) {
	room.mu.Lock()
	room.metaTag = tag
	room.mu.Unlock()
}

func (room *Room) snapshotHeaders() (meta *libflv.MetaTag, video *libflv.VideoTag, audio *libflv.AudioTag) {
	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.metaTag, room.videoSeqHdr, room.audioSeqHdr
}

// Close releases the broadcast so every subscriber wakes up with
// alive=false. Safe to call multiple times.
func (room *Room) Close() {
	room.mu.Lock()
	if room.closed {
		room.mu.Unlock()
		return
	}
	room.closed = true
	room.mu.Unlock()
	//DisAlive() wakes every BroadcastReader with alive=false so the
	//RTMP/FLV/HLS join goroutines exit cleanly.
	room.GOP.DisAlive()
}

//player join the room
func (room *Room) RTMPJoin(rtmp *RTMP) {
	go func() {
		//Backfill sequence headers so a player joining mid-GOP has the
		//decoder configuration before the first video tag arrives.
		meta, videoHdr, audioHdr := room.snapshotHeaders()
		if meta != nil {
			mb := MessageBase{
				rtmp:          rtmp,
				messageTime:   meta.GetTagInfo().TimeStamp,
				messageLength: meta.GetTagInfo().DataSize,
				messageType:   MessageType(meta.GetTagInfo().TagType),
			}
			_ = NewDataMessage(mb, meta).Send()
		}
		if videoHdr != nil {
			mb := MessageBase{
				rtmp:          rtmp,
				messageTime:   videoHdr.GetTagInfo().TimeStamp,
				messageLength: videoHdr.GetTagInfo().DataSize,
				messageType:   MessageType(videoHdr.GetTagInfo().TagType),
			}
			_ = NewVideoMessage(mb, videoHdr).Send()
		}
		if audioHdr != nil {
			mb := MessageBase{
				rtmp:          rtmp,
				messageTime:   audioHdr.GetTagInfo().TimeStamp,
				messageLength: audioHdr.GetTagInfo().DataSize,
				messageType:   MessageType(audioHdr.GetTagInfo().TagType),
			}
			_ = NewAudioMessage(mb, audioHdr).Send()
		}

		var err error
		gopReader := broadcast.NewBroadcastReader(room.GOP)
		for {
			p, alive := gopReader.Read()
			if !alive {
				fmt.Println("the publisher had been exit.")
				break
			}
			tag := p.(libflv.Tag)
			fmt.Printf("read packet from gop, now:%v, tag:%+v\n", time.Now(), tag)
			mb := MessageBase{
				rtmp:            rtmp,
				messageTime:     tag.GetTagInfo().TimeStamp,
				messageLength:   tag.GetTagInfo().DataSize,
				messageType:     MessageType(tag.GetTagInfo().TagType),
				messageStreamID: 0,
			}
			if audioTag, oka := tag.(*libflv.AudioTag); oka {
				err = NewAudioMessage(mb, audioTag).Send()
			} else if videoTag, okv := tag.(*libflv.VideoTag); okv {
				err = NewVideoMessage(mb, videoTag).Send()
			} else if dataTag, okd := tag.(*libflv.MetaTag); okd {
				err = NewDataMessage(mb, dataTag).Send()
			}
			if err != nil {
				//A write error on a player socket generally means the
				//player disconnected. Stop pushing — the outer handler
				//will notice when the TCP read loop sees EOF.
				fmt.Println("send data error:", err)
				return
			}
		}
	}()
}

func (room *Room) FLVJoin(writer easyio.EasyWriter) {
	//FLV header + PreviousTagSize0 (always zero).
	if err := writer.WriteFull([]byte{0x46, 0x4c, 0x56, 0x01, 0x05, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00}); err != nil {
		return
	}

	//Backfill cached sequence headers for mid-GOP joiners.
	meta, videoHdr, audioHdr := room.snapshotHeaders()
	if meta != nil {
		if err := writer.WriteFull(libflv.FLVWrite(meta)); err != nil {
			return
		}
	}
	if videoHdr != nil {
		if err := writer.WriteFull(libflv.FLVWrite(videoHdr)); err != nil {
			return
		}
	}
	if audioHdr != nil {
		if err := writer.WriteFull(libflv.FLVWrite(audioHdr)); err != nil {
			return
		}
	}

	gopReader := broadcast.NewBroadcastReader(room.GOP)
	for {
		p, alive := gopReader.Read()
		if !alive {
			fmt.Println("the publisher had been exit.")
			return
		}
		if err := writer.WriteFull(libflv.FLVWrite(p.(libflv.Tag))); err != nil {
			//Client disconnected; bail out of the loop.
			return
		}
	}
}
