package srpc_test

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/changlongH/srpc"
	"github.com/changlongH/srpc/cluster"
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

	// 指定额外参数序列化方式JSON
	errs := cluster.ReloadCluster(nodes, cluster.WithArgsCodec(&cluster.ArgsCodecJson{}))
	if errs != nil {
		t.Errorf("register cluster nodes failed cnt: %d", len(errs))
		return
	}

	// remove node db
	cluster.Remove("db")

	// register again with default codec
	_, err := cluster.Register("db", "127.0.0.1:2528")
	if err != nil {
		t.Error(err)
		return
	}

	var fooBar = &FooBar{Id: 1, Name: "foobar"}
	err = srpc.Call("db", "sdb", "set", nil, nil, srpc.WithCallParams("srpc", &fooBar))
	if err != nil {
		t.Error(err)
		return
	}

	var replyFooBar = FooBar{}
	err = srpc.Call("db", "sdb", "get", "srpc", &replyFooBar)
	if err != nil {
		t.Error(err)
		return
	}
	if fooBar.Id != replyFooBar.Id || fooBar.Name != replyFooBar.Name {
		t.Error("get/set sprc foobar struct failed")
		return
	}

	err = srpc.Send("db2", "sdb", "set", "srpc2", "foobar2")
	if err != nil {
		t.Error(err)
		return
	}
	// runtime.Gosched
	time.Sleep(time.Second)

	var reply = []byte{}
	err = srpc.Call("db2", "sdb", "get", "srpc2", &reply)
	if err != nil {
		t.Error(err)
		return
	}

	if string(reply) != "foobar2" {
		t.Error("unmatch set/get key=name2")
		return
	}

	// change node address  db2 -> db
	_, err = cluster.Register("db2", "127.0.0.1:2528")
	if err != nil {
		t.Error(err)
		return
	}

	var fooBar2 = FooBar{}
	err = srpc.Call("db2", "sdb", "get", "srpc", &fooBar2)
	if err != nil {
		t.Error(err)
		return
	}
	if fooBar.Id != fooBar2.Id || fooBar.Name != fooBar2.Name {
		t.Error("change node failed")
		return
	}

	// test with timeout
	err = srpc.Call("db", "sdb", "TEST", "ping", &reply, srpc.WithCallTimeout(time.Second*3))
	if err.Error() != "timeout" {
		t.Error("timeout failed")
		return
	}

	// wait close
	time.Sleep(time.Second * 10)
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
			var reply = []byte{}
			// call will response after 10s
			err = srpc.Call("db", "sdb", "TEST", "ping: "+strconv.Itoa(i), &reply)
			if err != nil {
				t.Error(err)
				return
			}
			fmt.Println(time.Now().Local(), string(reply))
		}()
	}

	time.Sleep(time.Second)
	_, err = cluster.Register("db", "127.0.0.1:2529")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("change node success")

	wg.Wait()
}

func TestBench(t *testing.T) {
	_, err := cluster.Register("db", "127.0.0.1:2528")
	if err != nil {
		t.Error(err)
		return
	}
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var k = fmt.Sprintf("key-%d", i)
			var v = fmt.Sprintf("foobar-%d", i)
			srpc.Send("db", "sdb", "set", k, v)
			runtime.Gosched()

			var reply = []byte{}
			err := srpc.Call("db", "sdb", "get", k, &reply)
			if err != nil {
				t.Error(err)
				return
			}

			if string(reply) == v {
				return
			}

			// retry
			err = srpc.Call("db", "sdb", "get", k, &reply)
			if err != nil {
				t.Error(err)
				return
			}
			if string(reply) != v {
				t.Error(k, v, string(reply))
			}
		}()
	}
	wg.Wait()
	fmt.Println("test bench call success")
}
