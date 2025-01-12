package server

import (
	"context"
	"reflect"
	"sync"
)

type (
	methodType struct {
		sync.Mutex // protects counters
		method     reflect.Method
		ArgType    reflect.Type
		ReplyType  reflect.Type
		errIndex   int
		hasReply   bool
		numCalls   uint
	}

	SkynetContext struct {
		context.Context
		numCall uint
	}
)

func (m *methodType) GetCallsNum() uint {
	m.Lock()
	defer m.Unlock()
	return m.numCalls
}

func NewSkynetContext(ctx context.Context) *SkynetContext {
	skynetCtx := &SkynetContext{Context: ctx}
	return skynetCtx
}

/*
func (ctx *SkynetContext) GetStages() *TraceStage {
	v := ctx.Value(ctxStages{})
	if v == nil {
		return nil
	}
	return v.(*TraceStage)
}

func (ctx *SkynetContext) AddTrace(msg string, args ...any) *SkynetContext {
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	ctx.Context = context.WithValue(ctx, ctxStages{}, msg)
	return ctx
}

func (trace *TraceStage) String() string {
	if trace.Key == "" {
		return ""
	}
	return fmt.Sprintf("|stage=%s %v cost=%s", trace.Key, trace.Fields, trace.Cost)
}
*/
