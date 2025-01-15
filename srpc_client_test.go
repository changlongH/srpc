package srpc_test

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/changlongH/srpc"
	"github.com/changlongH/srpc/client"
	"github.com/changlongH/srpc/cluster"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
)

type FooBar struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func TestClusterClient(t *testing.T) {
	// init nodes
	var nodes = map[string]string{
		"db":  "127.0.0.1:2528",
		"db2": "127.0.0.1:2529",
	}

	errs := cluster.ReloadCluster(nodes, client.WithPayloadCodec(&payloadcodec.MsgPack{}))
	if errs != nil {
		t.Errorf("register cluster nodes failed cnt: %d", len(errs))
		return
	}

	// remove node db
	cluster.Remove("db")

	// register again with default codec
	_, err := cluster.Register("db", "127.0.0.1:2528", client.WithPayloadCodec(&payloadcodec.MsgPack{}))
	if err != nil {
		t.Error(err)
		return
	}

	var pongMsg = map[string]string{}
	var pingMsg = map[string]string{"ping": "pong"}
	err = srpc.Call("db", "sdb", "PING", pingMsg, &pongMsg)
	if err != nil || !reflect.DeepEqual(pingMsg, pongMsg) {
		t.Error("PING err:", err)
		return
	}

	var fooBar = map[string]string{"key": "srpc", "val": "foobar"}
	caller := client.NewCaller("db", "sdb", "SETX", fooBar).WithTimeout(5 * time.Second)
	if err = srpc.Invoke(caller); err != nil {
		t.Error(err)
		return
	}

	var reply = new(map[string]string)
	err = srpc.Call("db", "sdb", "GETX", map[string]string{"key": "srpc"}, reply)
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(*reply, fooBar) {
		t.Errorf("fooBar=%v,reply=%v", fooBar, *reply)
		return
	}

	var fooBar2 = map[string]string{"key": "srpc2", "val": "foobar2"}
	err = srpc.Send("db2", "sdb", "SETX", fooBar2)
	if err != nil {
		t.Error(err)
		return
	}

	// runtime.Gosched
	time.Sleep(time.Second)
	var reply2 = new(map[string]string)
	err = srpc.Call("db2", "sdb", "GETX", map[string]string{"key": "srpc2"}, reply2)
	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(*reply2, fooBar2) {
		t.Errorf("fooBar2=%v,reply2=%v", fooBar2, reply2)
		return
	}

	// change node address  db2 -> db
	_, err = cluster.Register("db2", "127.0.0.1:2528")
	if err != nil {
		t.Error(err)
		return
	}

	err = srpc.Call("db2", "sdb", "GETX", map[string]string{"key": "srpc"}, reply)
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(*reply, fooBar) {
		t.Errorf("fooBar=%v,reply=%v", fooBar, *reply)
		return
	}

	err = srpc.Call("db2", "sdb", "SETX", map[string]string{"key": "srpc"}, nil)
	if err != nil {
		t.Error(err)
		return
	}
	// test call with payload codec text
	var replyText string
	var text = "hello world"
	caller = client.NewCaller("db2", "sdb", "TEXT", text).WithPayloadCodec("text").WithReply(&replyText)
	if err = srpc.Invoke(caller); err != nil {
		t.Error("call with payload codec failed", err)
		return
	}
	if !reflect.DeepEqual(text, replyText) {
		t.Errorf("call with payload codec failed, raw=%s,reply=%s", text, replyText)
		return
	}

	// test with timeout. not pararms and reply
	caller = client.NewCaller("db2", "sdb", "SLEEP", 5).WithTimeout(3 * time.Second)
	if err = srpc.Invoke(caller); err == nil {
		t.Error("timeout call failed. not timeout")
		return
	}

	// wait close
	time.Sleep(time.Second * 6)
	fmt.Println("test cluster client succ")
}

func TestChangeNode(t *testing.T) {
	_, err := cluster.Register("db", "127.0.0.1:2528")
	if err != nil {
		t.Error(err)
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var reply = map[string]string{}
			var req = map[string]string{"msg": "ping: " + strconv.Itoa(i)}
			// call will response after 10s
			err = srpc.Call("db", "sdb", "TESTX", req, &reply)
			if err != nil {
				t.Error(err)
				return
			}
			fmt.Println(time.Now().Local(), reply["val"])
		}()
	}

	time.Sleep(time.Second)
	_, err = cluster.Register("db", "127.0.0.1:2529")
	if err != nil {
		t.Error(err)
		return
	}
	wg.Wait()
	fmt.Println("test change node success")
}
