package libhls

import (
	"fmt"

	"github.com/SmartBrave/Athena/broadcast"
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
	M3u8    *M3U8
	TsList  *libmpeg.Ts
}

func NewHls() *HLS {
	return &HLS{}
}

//start to generate ts file and store to disk.
func (hls *HLS) Start(gopReader *broadcast.BroadcastReader) (err error) {
	for {
		p, alive := gopReader.Read()
		if !alive {
			fmt.Println("the publisher had been exit")
			break
		}

		//PAT
		//PMT
		// pes := mpeg.NewPES(p.(libflv.Tag))
	}
	return nil
}
