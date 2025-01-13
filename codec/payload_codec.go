package codec

import payloadcodec "github.com/changlongH/srpc/payload_codec"

var payloadCodecs = map[string]PayloadCodec{
	"json":    payloadcodec.Json{},
	"msgpack": payloadcodec.MsgPack{},
	"text":    payloadcodec.Text{},
}

type PayloadCodec interface {
	Marshal(msg any) ([]byte, error)
	Unmarshal(in []byte, msg any) error
	IsNull(in []byte) bool
	Name() string
}

// default registered json/msgpack
// GetPayloadCodec gets desired payload codec from name.
func GetPayloadCodec(name string) (PayloadCodec, bool) {
	pc, ok := payloadCodecs[name]
	return pc, ok
}

// PutPayloadCode puts the desired payload codec to message.
func PutPayloadCode(name string, v PayloadCodec) {
	payloadCodecs[name] = v
}
