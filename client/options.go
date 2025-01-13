package client

import (
	"time"

	"github.com/changlongH/srpc/codec"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
)

type Options struct {
	PayloadCodec codec.PayloadCodec
	CallTimeout  time.Duration
}

type Option func(*Options)

func WithPayloadCodec(argsCodec codec.PayloadCodec) Option {
	return func(o *Options) {
		o.PayloadCodec = argsCodec
	}
}

func WithCallTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.CallTimeout = t
	}
}

var defaultClientOptions = Options{
	CallTimeout:  time.Second * 5,
	PayloadCodec: payloadcodec.MsgPack{},
}
