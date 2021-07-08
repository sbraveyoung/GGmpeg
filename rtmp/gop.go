package rtmp

import (
	"github.com/SmartBrave/GGmpeg/flv"
)

type GOP struct {
	array []flv.Tag
}

// func NewGOP(gopLength int) (gop *GOP) {
// return &GOP{
// Audios: ring_buffer.NewRingBuffer(1024).Block().Build(),
// Videos: ring_buffer.NewRingBuffer(1024).Block().Build(),
// }
// }

// func (gop *GOP) GetKeyFrame() (tag flv.Tag) {
// if len(gop.Videos) == 0 {
// return nil
// }
// return gop.Videos[0]
// }

// func (gop *GOP) GetAudioKeyFrame() (tag flv.Tag) {
// if len(gop.Audios) == 0 {
// return nil
// }
// return gop.Audios[0]
// }
