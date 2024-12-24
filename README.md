# skynet-gorpc
skynet cluster with golang rpc


[skynet](https://github.com/cloudwu/skynet/wiki)

[netpoll](https://github.com/cloudwego/netpoll/blob/develop/README_CN.md)

[skynet_cluster](https://blog.codingnow.com/2017/03/skynet_cluster.html)

# example #

## Go Client Call Skynet ##

更多用例参考 [client_test](./srpc_client_test.go)

```(Golang)

type FooBar struct {
    Id   int    `json:"id"`
    Name string `json:"name"`
}

main() {
    // init nodes
    var nodes = map[string]string{
        "db":  "127.0.0.1:2528",
        "db2": "127.0.0.1:2529",
    }

    // register all nodes
    if errs := cluster.ReloadCluster(nodes, cluster.WithArgsCodec(&cluster.ArgsCodecJson{})); errs != nil{
        return
    }

    // remove/register node db
    cluster.Remove("db")
    cluster.Register("db", "127.0.0.1:2528")

    var fooBar = &FooBar{Id: 1, Name: "foobar"}

    // skynet -> cluster.call("db", "sdb", "set", "srpc", json.encode(fooBar))
    // call 支持多参数需要使用 WithCallParams
    if err := srpc.Call("db", "sdb", "set", nil, nil, srpc.WithCallParams("srpc", &fooBar)); err != nil {
        return
    }

    var replyFooBar = FooBar{}
    // call 默认一个请求和回复参数
    if err := srpc.Call("db", "sdb", "get", "srpc", &replyFooBar); err != nil {
        return
    }
    if fooBar.Id != replyFooBar.Id || fooBar.Name != replyFooBar.Name {
        fmt.Error("call failed")
        return
    }

    // Send 支持多请求参数 无回复参数
    srpc.Send("db2", "sdb", "set", "srpc2", "foobar2")

    // Call 默认30s 超时时间，可以指定超时时间，影响单次调用
    err := srpc.Call("db2", "sdb", "TEST", "ping", nil, srpc.WithCallTimeout(time.Second*3))
    if err.Error() != "timeout" {
        fmt.Error("call timeout failed")
        return
    }
}
```
