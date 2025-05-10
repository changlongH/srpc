package client

import (
	"time"

	"github.com/changlongH/srpc/codec"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
)

type ConnectHandle func(remoteAddr string)
type DisconnectHandle func(remoteAddr string)

type Options struct {
	PayloadCodec   codec.PayloadCodec
	CallTimeout    time.Duration
	ConnectHdle    ConnectHandle
	DisconnectHdle DisconnectHandle
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

func WithConnectHandle(hdl ConnectHandle) Option {
	return func(o *Options) {
		o.ConnectHdle = hdl
	}
}

func WithDisConnectHandle(hdl DisconnectHandle) Option {
	return func(o *Options) {
		o.DisconnectHdle = hdl
	}
}

var defaultClientOptions = Options{
	CallTimeout:  time.Second * 5,
	PayloadCodec: payloadcodec.MsgPack{},
}
