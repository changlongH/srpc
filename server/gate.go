package server

import (
	"context"
	"encoding/binary"
	"log"
	"time"

	"github.com/cloudwego/netpoll"
)

const (
	headerSize = 2
)

var _ netpoll.OnPrepare = prepare
var _ netpoll.OnConnect = connect
var _ netpoll.OnRequest = handle

type connkey struct{}

var ctxkey connkey

func prepare(conn netpoll.Connection) context.Context {
	agent := NewGateAgent(conn)
	ctx := context.WithValue(context.Background(), ctxkey, agent)
	return ctx
}

func connect(ctx context.Context, conn netpoll.Connection) context.Context {
	log.Printf("new connect from %s\n", conn.RemoteAddr().String())
	agent := ctx.Value(ctxkey).(*GateAgent)
	go agent.Start()
	return ctx
}

func handle(ctx context.Context, conn netpoll.Connection) error {
	reader := conn.Reader()
	p, err := reader.ReadBinary(headerSize)
	if err != nil {
		return err
	}
	// gate header BigEndian
	size := int(binary.BigEndian.Uint16(p))
	r, err := reader.Slice(size)
	if err != nil {
		return err
	}

	agent := ctx.Value(ctxkey).(*GateAgent)
	agent.Reader <- r
	return nil
}

// Open open address with netpoll.Option
// opts disable WithOnPrepare and WithOnConnect
func Open(address string, ops ...netpoll.Option) (netpoll.EventLoop, error) {
	listener, err := netpoll.CreateListener("tcp", address)
	if err != nil {
		return nil, err
	}

	ops = append(ops,
		netpoll.WithOnPrepare(prepare),
		netpoll.WithOnConnect(connect),
		//netpoll.WithReadTimeout(time.Second),
	)

	eventLoop, err := netpoll.NewEventLoop(handle, ops...)
	if err != nil {
		return nil, err
	}

	err = eventLoop.Serve(listener)
	return eventLoop, err
}

func Shutdown(eventLoop netpoll.EventLoop, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	eventLoop.Shutdown(ctx)
}
