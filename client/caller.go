package client

import (
	"errors"
	"fmt"
	"go/token"
	"reflect"
	"time"

	"github.com/changlongH/srpc/codec"
)

type (
	Caller struct {
		Node         string
		Addr         *codec.Addr
		Method       string
		Args         any
		Reply        any
		Timeout      time.Duration
		PayloadCodec string
		push         bool // not wait reply
	}
)

func NewCaller(node string, addr any, method string, args any) *Caller {
	service, _ := codec.GetServiceAddress(addr)
	return &Caller{Node: node, Addr: service, Method: method, Args: args}
}

func (c *Caller) WithArgs(args any) *Caller {
	c.Args = args
	return c
}

func (c *Caller) WithReply(reply any) *Caller {
	c.Reply = reply
	return c
}

func (c *Caller) WithTimeout(timeout time.Duration) *Caller {
	c.Timeout = timeout
	return c
}

// async send req don't waits for it to complete
func (c *Caller) WithPush() *Caller {
	c.push = true
	return c
}

// WithPayloadCodec registered name, For this call only
func (c *Caller) WithPayloadCodec(name string) *Caller {
	c.PayloadCodec = name
	return c
}

// like cluster.send
func (c *Caller) IsPush() bool {
	return c.push
}

func (c *Caller) Done() (*Caller, error) {
	if c.Node == "" {
		return nil, errors.New("caller node name is nil")
	}

	if c.Addr == nil || (c.Addr.Id == 0 && c.Addr.Name == "") {
		return nil, errors.New("caller skynet service addr invalid. need name or id")
	}

	if c.Reply != nil {
		// Reply type must be exported.
		var replyType = reflect.TypeOf(c.Reply)
		if replyType.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("srpc.Call: reply type must be ptr or nil, got %q", replyType)
		}
		if !isExportedOrBuiltinType(replyType) {
			return nil, fmt.Errorf("srpc.Call: reply type is not exported: %q", replyType)
		}

	}

	return c, nil
}

func (c *Caller) String() string {
	return fmt.Sprintf("[%s.%s] [%s]", c.Node, c.Addr.String(), c.Method)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return token.IsExported(t.Name()) || t.PkgPath() == ""
}
