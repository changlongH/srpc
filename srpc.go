package srpc

import (
	"errors"
	"time"

	"github.com/changlongH/srpc/client"
	"github.com/changlongH/srpc/cluster"
)

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
	caller := client.NewCaller(node, addr, cmd, args).WithReply(reply).WithTimeout(30 * time.Second)
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
