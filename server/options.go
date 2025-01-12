package server

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/changlongH/srpc/codec"
)

type AccessHandle func(ctx *SkynetContext, sname, method string, cost time.Duration, err error)

type Options struct {
	PayloadCodec codec.PayloadCodec
	AccessHdle   AccessHandle
}

type Option func(*Options)

func WithPayloadCodec(c codec.PayloadCodec) Option {
	return func(o *Options) {
		o.PayloadCodec = c
	}
}

func defaultAccessHandle(ctx *SkynetContext, sname string, cmd string, cost time.Duration, err error) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("access %s.%s %dms count:%d", sname, cmd, cost.Milliseconds(), ctx.numCall))
	if err != nil {
		builder.WriteString(fmt.Sprintf(" err=%s", err.Error()))
	}
	log.Println(builder.String())
}

func WithAccessLog(hdl AccessHandle) Option {
	return func(o *Options) {
		if hdl == nil {
			hdl = defaultAccessHandle
		}
		o.AccessHdle = hdl
	}
}
