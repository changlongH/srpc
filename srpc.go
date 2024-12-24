package srpc

import (
	"errors"
	"time"

	"github.com/changlongH/srpc/cluster"
	"github.com/changlongH/srpc/codec"
)

type CallOptions struct {
	Timeout time.Duration
	Params  []any
}

type CallOption func(*CallOptions)

func WithCallParams(args ...any) CallOption {
	return func(o *CallOptions) {
		o.Params = args
	}
}

func WithCallTimeout(t time.Duration) CallOption {
	return func(o *CallOptions) {
		o.Timeout = t
	}
}

/*
	Send equal skynet cluster.send(node, addr, cmd, args)

node: registered cluster node name

service: support skynet string name or number address

args: maybe []byte or string. if it's *struct will be marshal with argsCodec

return don't waits for it to complete, returns its error status.
*/
func Send(node string, service any, cmd string, args ...any) error {
	addr, err := codec.GetServiceAddress(service)
	if err != nil {
		return err
	}

	c := cluster.Query(node)
	if c == nil {
		return errors.New("not found cluster node: " + node)
	}

	payload, err := c.EncodePayload(cmd, args...)
	if err != nil {
		return errors.New("encodePayload err:" + err.Error())
	}
	return c.Send(node, addr, payload)
}

/*
Call equal skynet cluster.call(node, addr, cmd, args, reply)

node: registered cluster node name

service: support skynet string name or number address

args: maybe []byte or string. if it's *struct will be marshal with argsCodec

reply: maybe *[]byte or *string. if it's *sturct will be unmarshal with argsCodec

opts:  WithCallTimeout() or WithCallParams() params will replace args

return waits for it to complete or timeout(default 30s) and returns its error status.
*/
func Call(node string, service any, cmd string, args any, reply any, opts ...CallOption) error {
	addr, err := codec.GetServiceAddress(service)
	if err != nil {
		return err
	}

	options := CallOptions{Timeout: 30 * time.Second}
	for _, opt := range opts {
		opt(&options)
	}

	c := cluster.Query(node)
	if c == nil {
		return errors.New("not found cluster node: " + node)
	}

	var payload []byte
	if options.Params != nil {
		payload, err = c.EncodePayload(cmd, options.Params...)
	} else if args != nil {
		payload, err = c.EncodePayload(cmd, args)
	}
	if err != nil {
		return errors.New("encodePayload err:" + err.Error())
	}

	return c.Call(node, addr, payload, reply, options.Timeout)
}
