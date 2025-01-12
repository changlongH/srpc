package client

import (
	"time"

	"github.com/changlongH/srpc/codec"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
)

type Options struct {
	Timeout      time.Duration
	PayloadCodec codec.PayloadCodec
}

type Option func(*Options)

func WithPayloadCodec(argsCodec codec.PayloadCodec) Option {
	return func(o *Options) {
		o.PayloadCodec = argsCodec
	}
}

func WithTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.Timeout = t
	}
}

var defaultClientOptions = Options{
	Timeout:      time.Second * 5,
	PayloadCodec: payloadcodec.MsgPack{},
}
