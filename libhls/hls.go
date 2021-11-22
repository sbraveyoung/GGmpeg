package libhls

import (
	"fmt"
	"os"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
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
		Cc:          make(map[uint16]uint8),
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

		if hls.Pat == nil {
			//TODO
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
					0x1000: &libmpeg.PMT{
						TableID:                0x02,
						SectionSyntaxIndicator: 0x01,
						SectionLength:          0x17,
						ProgramNumber:          0x01,
						VersionNumber:          0x00,
						CurrentNextIndicator:   0x01,
						SectionNumber:          0x00,
						LastSectionNumber:      0x00,
						PCR_PID:                0x0100,
						ProgramInfoLength:      0x00,
						Streams:                map[uint16]*libmpeg.PES{
							// 0x0100: &libmpeg.PES{}, //audio
							// 0x0101: &libmpeg.PES{}, //video
						},
					},
				},
			}
			// hls.PIDTable[0x0000] = pat
			// hls.PIDTable[0x1000] = pat.PMTs[0x1000]
		}

		tag := p.(libflv.Tag)
		var pes *libmpeg.PES
		var pid uint16
		frameKey := false
		// var codec uint8
		switch tag.GetTagInfo().TagType {
		case libflv.AUDIO_TAG:
			pa, _ := p.(*libflv.AudioTag)
			switch pa.SoundFormat {
			case libflv.FLV_AUDIO_AAC, libflv.FLV_AUDIO_OPUS: //TODO: support MP3...
				// codec = pa.SoundFormat

				pes = &libmpeg.PES{
					PacketStartCodePrefix: 0x000001,
					StreamID:              0xc0,
					//TODO PESPacketLength:       tag.GetTagInfo().DataSize + 0x05 + 3,
					PTS_DTSFlag:         0x02,
					PESHeaderDataLength: 0x05,
					PTS:                 uint64(tag.GetTagInfo().TimeStamp * 90),
				}
				pid = 0x0100
			default:
				continue
			}
		case libflv.VIDEO_TAG:
			pv, _ := p.(*libflv.VideoTag)
			switch pv.CodecID {
			case libflv.FLV_VIDEO_AVC, libflv.FLV_VIDEO_HEVC: //TODO: support VVC/AV1...
				// codec = pv.CodecID

				pes = &libmpeg.PES{
					PacketStartCodePrefix: 0x000001,
					StreamID:              0xe0,
					//TODO: PESPacketLength:       tag.GetTagInfo().DataSize + 0x05 + 3,
					PTS_DTSFlag:         0x02,
					PESHeaderDataLength: 0x05,
					PTS:                 uint64(tag.GetTagInfo().TimeStamp * 90),
					DTS:                 uint64(tag.GetTagInfo().TimeStamp * 90),
				}
				if pv.Cts != 0 {
					//TODO: pes.PESHeaderDataLength = tag.GetTagInfo().DataSize + 0x0a + 3
					pes.PTS_DTSFlag = 0x03
					pes.PTS = pes.DTS + uint64(pv.Cts*90)
				}
				pid = 0x0101

				if pv.FrameType == libflv.KEY_FRAME {
					frameKey = true
				}

			default:
				continue
			}
		case libflv.SCRIPT_DATA_TAG:
			continue
		default:
			continue
		}
		// _ = codec
		hls.Pat.PMTs[0x1000].Streams[pid] = pes

		if hls.TsClipStart {
			_, err1 := libmpeg.NewTs(0x0000, hls.Cc).Mux(hls.Pat, false, 0, writer)
			_, err2 := libmpeg.NewTs(0x1000, hls.Cc).Mux(hls.Pat.PMTs[0x1000], false, 0, writer)
			if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
				fmt.Printf("ts.Mux error:%v\n", err)
				continue
			}
			hls.TsClipStart = false
		}

		for {
			finish, err := libmpeg.NewTs(pid, hls.Cc).Mux(pes, frameKey, pes.DTS, writer)
			if err != nil {
				fmt.Printf("ts.Mux pes error:%+v", err)
				continue outer
			}
			if finish {
				break
			}
		}
	}
	return nil
}
