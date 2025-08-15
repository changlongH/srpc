package srpc

import (
	"errors"
	"fmt"

	"github.com/changlongH/srpc/client"
	"github.com/changlongH/srpc/cluster"
)

type Host struct {
	Name string
	Addr string
}

/*
	Send equal skynet cluster.send(node, addr, cmd, args)

node: registered cluster node name

addr: skynet service name or number address

args: type *struct will be marshal with argsCodec

return don't waits for it to complete, returns its error status.
*/
func Send(node string, addr any, cmd string, args any) error {
	caller := client.NewCaller(node, addr, cmd, args).WithPush()
	return Invoke(caller)
}

/*
Call srpc.call equal skynet cluster.call(node, addr, cmd, args, reply)

# Or srpc.Invoke to custom remote call

node: registered skynet cluster node name

addr: skynet service name or number address

args: type *struct will be marshal with argsCodec

reply: providing a new value for the reply. value type must be exported. *sturct will be unmarshal with argsCodec

return waits for it to complete or timeout(default 30s) and returns its error status.
*/
func Call(node string, addr any, cmd string, args any, reply any) error {
	caller := client.NewCaller(node, addr, cmd, args).WithReply(reply)
	return Invoke(caller)
}

/*
Invoker invoker will query registered client then invoker with caller

caller = client.NewCaller(node, addr, cmd, args).WithTimeout(5*time.Second).WithPayloadCodec("json")

return returns its error status. If not push then waits for it to complete or timeout
*/
func Invoke(caller *client.Caller) error {
	var err error
	if caller, err = caller.Done(); err != nil {
		return err
	}
	c := cluster.Query(caller.Node)
	if c == nil {
		return errors.New("not found cluster node: " + caller.Node)
	}
	return c.Invoke(caller)
}

type NewClientOptsHandle func() []client.Option

var (
	newClientOptsHandle NewClientOptsHandle = func() []client.Option {
		return []client.Option{}
	}
)

func SetClientOptsHandler(handler NewClientOptsHandle) {
	newClientOptsHandle = handler
}

// CallHost 使用名称和地址调用，无需提前注册
// NOTE: 但是必须提前设定 SetClientOptsHandler 否则使用默认配置初始化客户端可能导致预期之外的问题
// Host{Name:"gate",Addr:"127.0.0.1:8888"}
func CallHost(host *Host, addr any, cmd string, args any, reply any) error {
	if len(host.Name) <= 0 {
		return errors.New("invalid host name")
	}
	caller := client.NewCaller(host.Name, addr, cmd, args).WithReply(reply)
	return InvokeWithHost(host, caller)
}

func SendHost(host *Host, addr any, cmd string, args any) error {
	if len(host.Name) <= 0 {
		return errors.New("invalid host name")
	}
	caller := client.NewCaller(host.Name, addr, cmd, args).WithPush()
	return InvokeWithHost(host, caller)
}

func InvokeWithHost(host *Host, caller *client.Caller) error {
	var err error
	if caller, err = caller.Done(); err != nil {
		return err
	}
	c := cluster.Query(host.Name)
	if c == nil {
		if len(host.Addr) > 0 {
			opts := newClientOptsHandle()
			if c, err = cluster.Register(host.Name, host.Addr, opts...); err != nil {
				return fmt.Errorf("register node: %s addr:%s FAILED", host.Name, host.Addr)
			}
		} else {
			return errors.New("cluster not found node: " + host.Name)
		}
	} else if c.Address != host.Addr {
		if len(host.Addr) > 0 {
			opts := newClientOptsHandle()
			if c, err = cluster.Register(host.Name, host.Addr, opts...); err != nil {
				return fmt.Errorf("register node: %s addr:%s FAILED", host.Name, host.Addr)
			}
		} else {
			return errors.New("cluster not found node: " + host.Name)
		}
	}
	return c.Invoke(caller)
}
