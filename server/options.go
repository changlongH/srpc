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
	PayloadCodec    codec.PayloadCodec
	AccessHdle      AccessHandle
	SyncDisptch     bool
	MonitorInterval time.Duration
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

// Default Message dispatch mod is async. The order of messages is not guaranteed.
// SyncDispatch guarenteed order of messages. But be careful of endlessloop
func WithSyncDispatch() Option {
	return func(o *Options) {
		o.SyncDisptch = true
		o.MonitorInterval = 5 * time.Second
	}
}

// Synchronous mode monitor message maybe in endlessloop interval. Default 5 time.Seconds. must greate than 1 time.Seconds
func WithMonitorInterval(interval time.Duration) Option {
	return func(o *Options) {
		if interval.Seconds() < 1 {
			interval = 1 * time.Second
		}
		o.MonitorInterval = interval
	}
}
