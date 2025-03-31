package srpc_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/changlongH/srpc"
	"github.com/changlongH/srpc/client"
	"github.com/changlongH/srpc/cluster"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
	"github.com/changlongH/srpc/server"
)

type Args struct {
	A int `json:"a" msgpack:"a"`
	B int `json:"b" msgpack:"b"`
}

type Reply struct {
	C int `json:"c" msgpack:"c"`
}

type Arith int

// Some of Arith's methods have value args, some have pointer args. That's deliberate.

func (t *Arith) Add(ctx *server.SkynetContext, args Args) (reply *Reply, err error) {
	reply = new(Reply)
	reply.C = args.A + args.B
	return
}

func (t *Arith) Mul(ctx *server.SkynetContext, args *Args) (reply *Reply, err error) {
	if args == nil {
		panic("nil args")
	}
	reply = new(Reply)
	reply.C = args.A * args.B
	return
}

func (t *Arith) Div(ctx *server.SkynetContext, args Args) (reply *Reply, err error) {
	reply = new(Reply)
	if args.B == 0 {
		err = errors.New("divide by zero")
		return
	}
	reply.C = args.A / args.B
	return
}

func (t *Arith) String(ctx *server.SkynetContext, args *Args) (reply *string, err error) {
	reply = new(string)
	*reply = fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B)
	return
}

func (t *Arith) Scan(ctx *server.SkynetContext, args string) (reply *Reply, err error) {
	reply = new(Reply)
	_, err = fmt.Sscan(args, &reply.C)
	return
}

func (t *Arith) Error(ctx *server.SkynetContext) error {
	panic("ERROR")
}

func (t *Arith) Echo(ctx *server.SkynetContext, args *any) *any {
	if args == nil {
		return nil
	}
	return args
}

func (t *Arith) EchoSlice(ctx *server.SkynetContext, args []string) *[]string {
	if args == nil {
		return nil
	}
	return &args
}

func (t *Arith) EchoMap(ctx *server.SkynetContext, args map[string]int) *map[string]int {
	return &args
}

func (t *Arith) SleepMilli(ctx *server.SkynetContext, n int) error {
	time.Sleep(time.Duration(n) * time.Millisecond)
	return nil
}

func TestRpcServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	addr := "127.0.0.1:2531"
	var gate *server.Gate
	var err error
	if gate, err = server.NewGate(addr); err != nil {
		panic(err.Error())
	}

	go func() {
		if err := gate.Start(); err != nil {
			t.Error(err.Error())
			return
		}
	}()
	time.Sleep(time.Second)
	log.Println("waiting for skynet cluster listen;", addr)

	arith := new(Arith)
	sname := "airth"
	svcOpts := []server.Option{
		server.WithPayloadCodec(&payloadcodec.MsgPack{}),
		server.WithAccessLog(nil),
	}
	if err := server.Register(arith, sname, svcOpts...); err != nil {
		t.Error(err.Error())
	}
	if methods, err := server.GetRegisterMethods(sname); err != nil {
		t.Error(err)
		return
	} else {
		log.Println("methods:", methods)
	}

	node := "test"
	_, err = cluster.Register(node, addr, client.WithPayloadCodec(&payloadcodec.MsgPack{}))
	if err != nil {
		t.Error(err)
		return
	}

	reply := new(Reply)
	args := &Args{7, 8}
	err = srpc.Call(node, sname, "Add", args, reply)
	if err != nil {
		t.Error(err)
		return
	}
	if reply.C != args.A+args.B {
		t.Errorf("Add: expected %d got %d", reply.C, args.A+args.B)
		return
	}

	<-ctx.Done()
	gate.Close(3 * time.Second)
	fmt.Println("TestRpcServerSuccess")
}
