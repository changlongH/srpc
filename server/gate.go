package server

import (
	"context"
	"encoding/binary"
	"log"
	"time"

	"github.com/cloudwego/netpoll"
)

type Gate struct {
	listener  netpoll.Listener
	eventLoop netpoll.EventLoop
}
type connkey struct{}

const (
	headerSize = 2
)

var ctxkey connkey

var _ netpoll.OnPrepare = prepare
var _ netpoll.OnConnect = connect
var _ netpoll.OnRequest = handle

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
func NewGate(address string, ops ...netpoll.Option) (*Gate, error) {
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
	gate := &Gate{
		listener:  listener,
		eventLoop: eventLoop,
	}
	return gate, nil
}

/*
Serve registers a listener and runs blockingly to provide services,
including listening to ports, accepting connections and processing trans data.
When an exception occurs or Shutdown is invoked,
Serve will return an error which describes the specific reason.
*/
func (gate *Gate) Start() error {
	return gate.eventLoop.Serve(gate.listener)
}

// Close is used to graceful exit.
// It will close all idle connections on the server, but will not change the underlying pollers.
func (gate *Gate) Close(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	gate.eventLoop.Shutdown(ctx)
}
