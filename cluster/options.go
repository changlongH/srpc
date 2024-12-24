package cluster

import (
	"encoding/json"
	"time"
)

type ClientOptions struct {
	Timeout   time.Duration
	ArgsCodec ArgsCodec
}

type ClientOption func(*ClientOptions)

// WithArgsCodec 指定args序列化方式 json/msgpack/other
func WithArgsCodec(argsCodec ArgsCodec) ClientOption {
	return func(o *ClientOptions) {
		o.ArgsCodec = argsCodec
	}
}

func WithTimeout(t time.Duration) ClientOption {
	return func(o *ClientOptions) {
		o.Timeout = t
	}
}

var defaultClientOptions = ClientOptions{
	Timeout:   time.Second * 5,
	ArgsCodec: ArgsCodecJson{},
}

type ArgsCodecJson struct{}

func (c ArgsCodecJson) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (c ArgsCodecJson) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

type ArgsCodecMsgPack struct{}

func (c ArgsCodecMsgPack) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (c ArgsCodecMsgPack) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
