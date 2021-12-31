package libhls

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libaac"
	"github.com/SmartBrave/GGmpeg/libavc"
	"github.com/SmartBrave/GGmpeg/libflv"
	"github.com/SmartBrave/GGmpeg/libmpeg"
)

type HLS_MODE uint8

//rtmp->hls 转码有两种方式，懒汉式和饿汉式
// 饿汉式：流推上来后立即开始转为 hls，不管有没有人拉 hls 流。推流断掉后才停止转码
// 懒汉式：第一个人拉 hls 时才开始转码，没人拉时就停掉，即使推流还没停
//
// 如果需要做录制，需要使用饿汉式，以保证录制到全量流
// 如果只是为了 hls 实时拉流，就可以使用懒汉式，没人拉 hls 时不用转码，节省性能
const (
	NONE        HLS_MODE = iota
	IMMEDIATELY          //default
	DELAY
)

type HLS struct {
	Version int //3
	// M3u8    *M3U8
	Pat         *libmpeg.PAT
	TsClipStart bool
	Cc          map[uint16]uint8 //key:pid
	Ah          *libaac.AACHeader
	AvcParser   *libavc.Parser
	audioCache  *audioCache //copy from livego
	align       *align
}

func NewHls() *HLS {
	return &HLS{
		Version: 3,
		//iso13818-1.pdf: Table 2-3 PID table
		//https://en.wikipedia.org/wiki/MPEG_transport_stream#Packet_identifier_(PID)
		// PIDTable: map[uint16]libmpeg.PSI{
		// 0x0000: &libmpeg.PAT{},
		//0x0001: libmpeg.NewCAT,
		//0x0002: libmpeg.NewTSDT,
		//0x0003: libmpeg.NewIPMP,
		// },
		TsClipStart: true,
		Cc:          map[uint16]uint8{},
		Ah:          &libaac.AACHeader{},
		AvcParser: &libavc.Parser{
			Pps: bytes.NewBuffer(make([]byte, libavc.MaxSpsPpsLen)),
		},
		audioCache: newAudioCache(),
		align:      &align{},
	}
}

//start to generate ts file and store to disk.
func (hls *HLS) Start(gopReader *broadcast.BroadcastReader) (err error) {
	tsFile, err := os.Create("./data/test.ts")
	if err != nil {
		return err
	}
	defer tsFile.Close()
	writer := easyio.NewEasyWriter(tsFile)

outer:
	for {
		p, alive := gopReader.Read()
		if !alive {
			fmt.Println("the publisher had been exit")
			break
		}

		tag := p.(libflv.Tag)
		fmt.Printf("11111111111111111111 len(data):%d, tag:%+v\n", len(tag.Data()), tag)
		if hls.Pat == nil {
			hls.Pat = &libmpeg.PAT{
				TableID:                0x00,
				SectionSyntaxIndicator: 0x01,
				SectionLength:          0x0d,
				TransportStreamID:      0x01,
				VersionNumber:          0x00,
				CurrentNextIndicator:   0x01,
				SectionNumber:          0x00,
				LastSectionNumber:      0x00,
				PMTs: map[uint16]*libmpeg.PMT{
					libmpeg.PMT_PID: &libmpeg.PMT{
						TableID:                0x02,
						SectionSyntaxIndicator: 0x01,
						SectionLength:          0x17,
						ProgramNumber:          0x01,
						VersionNumber:          0x00,
						CurrentNextIndicator:   0x01,
						SectionNumber:          0x00,
						LastSectionNumber:      0x00,
						PCR_PID:                libmpeg.VIDEO_PID,
						ProgramInfoLength:      0x00,
						Streams: map[uint16]*libmpeg.PES{
							//libmpeg.AUDIO_PID: &libmpeg.PES{
							//	StreamID: 0xc0,
							//	// StreamID:              0x1b,
							//	PacketStartCodePrefix: 0x000001,
							//}, //audio
							libmpeg.VIDEO_PID: &libmpeg.PES{
								StreamID: 0xe0,
								// StreamID:              0x0f,
								PacketStartCodePrefix: 0x000001,
							}, //video
						},
					},
				},
			}
		}

		var pes *libmpeg.PES
		var pid uint16
		videoFrameKey := false
		var pa *libflv.AudioTag
		var pv *libflv.VideoTag

		switch tag.GetTagInfo().TagType {
		case libflv.AUDIO_TAG:
			continue
			pa, _ = p.(*libflv.AudioTag)
			switch pa.SoundFormat { //TODO: support MP3...
			case libflv.FLV_AUDIO_AAC:
				if pa.AACPacketType == libflv.AAC_SEQUENCE_HEADER {
					err = hls.Ah.Parse(pa.Data())
					if err != nil {
						fmt.Printf("parse aac header error:%+v\n", err)
					}
					continue
				}

				pid = libmpeg.AUDIO_PID
				pes = hls.Pat.PMTs[libmpeg.PMT_PID].Streams[libmpeg.AUDIO_PID]

				pes.DTS = uint64(tag.GetTagInfo().TimeStamp * 90)
				pes.PESPacketLength = uint16(len(tag.Data())) + 5 + 3
				pes.PTS_DTSFlag = 0x02
				pes.PESHeaderDataLength = 0x05
				pes.Index = 0

				aacHeader := hls.Ah.Adts(pa.Data())
				pes.Data = append(aacHeader, pes.Data...) //XXX performance better?

				rate := 44100
				if hls.Ah.SampleRate < uint8(len(libaac.AACRates)-1) {
					rate = libaac.AACRates[hls.Ah.SampleRate]
					hls.align.align(&pes.DTS, uint32(90000*1024/rate))
					pes.PTS = pes.DTS
				}

				hls.audioCache.Cache(pes.Data, pes.PTS)
				if hls.audioCache.CacheNum() < cache_max_frames {
					continue
				}
				_, pes.PTS, pes.Data = hls.audioCache.GetFrame()

			case libflv.FLV_AUDIO_OPUS:
			default:
				continue
			}
		case libflv.VIDEO_TAG:
			pv, _ = p.(*libflv.VideoTag)
			switch pv.CodecID { //TODO: support VVC/AV1...
			case libflv.FLV_VIDEO_AVC:
				//TODO: AnnexB is supported, AVCC is not!
				if pv.FrameType == libflv.KEY_FRAME && pv.AVCPacketType == libflv.AVC_SEQUENCE_HEADER {
					err = hls.AvcParser.ParseSpecificInfo(pv.Data())
					if err != nil {
						fmt.Printf("parse avc header error:%+v\n", err)
					}
					continue
				}

				if pv.FrameType == libflv.KEY_FRAME {
					videoFrameKey = true
				}

				pid = libmpeg.VIDEO_PID
				pes = hls.Pat.PMTs[libmpeg.PMT_PID].Streams[libmpeg.VIDEO_PID]

				pes.DTS = uint64(tag.GetTagInfo().TimeStamp * 90)
				pes.PESPacketLength = uint16(len(tag.Data())) + 5 + 3
				pes.PTS_DTSFlag = 0x02
				pes.PESHeaderDataLength = 0x05
				pes.Index = 0

				if hls.AvcParser.IsNaluHeader(pv.Data()) {
					pes.Data = pv.Data()
				} else {
					buf := bytes.NewBuffer([]byte{})
					err = hls.AvcParser.GetAnnexbH264(pv.Data(), easyio.NewEasyWriter(buf))
					if err != nil {
						fmt.Printf("AvcParser.GetAnnexbH264 error:%+v\n", err)
						continue
					}
					pes.Data, err = io.ReadAll(buf)
					if err != nil {
						fmt.Printf("io.ReadAll error:%+v\n", err)
						continue
					}
				}

				if pv.Cts != 0 {
					//TODO: pes.PESHeaderDataLength = tag.GetTagInfo().DataSize + 0x0a + 3
					pes.PTS_DTSFlag = 0x03
					pes.PTS = pes.DTS + uint64(pv.Cts*90)
					pes.PESPacketLength += 5
				}
			case libflv.FLV_VIDEO_HEVC:
			default:
				continue
			}
		default:
			continue
		}

		if hls.TsClipStart {
			_, err1 := libmpeg.NewTs(libmpeg.PAT_PID, hls.Cc, true).Mux(hls.Pat, false, 0, writer)
			_, err2 := libmpeg.NewTs(libmpeg.PMT_PID, hls.Cc, true).Mux(hls.Pat.PMTs[libmpeg.PMT_PID], false, 0, writer)
			if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
				fmt.Printf("ts.Mux error:%v\n", err)
				continue
			}
			hls.TsClipStart = false
		}

		fmt.Printf("3333333333333333333333333333333333 len(data):%d, pes packet length:%d\n", len(pes.Data), pes.PESPacketLength)
		firstTS := true
		for {
			finish, err := libmpeg.NewTs(pid, hls.Cc, firstTS).Mux(pes, videoFrameKey && firstTS, pes.DTS, writer)
			if err != nil {
				fmt.Printf("ts.Mux pes error:%+v", err)
				continue outer
			}
			firstTS = false
			if finish {
				break
			}
		}
	}
	return nil
}
