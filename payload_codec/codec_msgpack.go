package payloadcodec

import "github.com/vmihailenco/msgpack"

type MsgPack struct{}

func (c MsgPack) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

func (c MsgPack) Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

func (c MsgPack) IsNull(data []byte) bool {
	return len(data) == 0
}

func (c MsgPack) Name() string {
	return "msgpack"
}
