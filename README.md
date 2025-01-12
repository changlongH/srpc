# skynet-gorpc
skynet cluster with golang rpc

- [skynet开源框架](https://github.com/cloudwu/skynet/wiki)
- [netpoll Golang高性能网络库](https://github.com/cloudwego/netpoll/blob/develop/README_CN.md)
- [skynet_cluster编码设计](https://blog.codingnow.com/2017/03/skynet_cluster.html)

支持golang和skynet cluster通信。无侵入设计无需修改任何skyne代码，简单的api设计上手即用。

- Golang 无状态微服务（例如登陆大厅），有丰富的生态和框架可以选择。大量的第三方库。
- Skynet 有状态的服务，使用lua脚本可以方便编写业务和实现热更。拥有高的性能和虚拟沙盒隔离。

# 支持特性 #

- 支持payload codec 配置可选jsong/msgpack（建议使用msgpack）
- skynet 提供 libsrpc.lua 引入即可使用
- golang 支持 cluster节点动态变更和自动重连
- golang 支持 client级别和call级别的payload codec
- golang 支持 SkynetContext上下文传递更方便做链路追踪和分析
- golang 和 skynet都支持profile统计消耗

## golang API ##

服务端：

- server.Open("127.0.0.1:2531") 开启集群端口监听
- server.Shutdown(eventLoop netpoll.EventLoop, timeout time.Duration) 关闭集群监听

- server.Register(rcvr any, name string, opts ...Option) error 注册一个服务
  - server.WithPayloadCodec(&payloadcodec.MsgPack{}) 指定payload打包方式默认为msgpack
  - server.WithAccessLog(handler) 指定访问日志处理回调，如果传入nil 则使用默认输出日志。不调用则不输出
- server.GetRegisterMethods(name string) ([]string, error) 获取成功注册的方法，可用于开发调试。
- server.SetRecoveryHandler(handle func(string, any)) 服务器消息panic 回调

客户端：

- cluster.Register(node string, address string, opts ...client.Option) 注册一个远程skynet节点
- cluster.Remove(node string) 移除一个节点
- cluster.Query(node string) *client.Client 查询一个已经注册节点
- cluster.ReloadCluster(nodes map[string]string, opts ...client.Option) 批量注册或者更新节点（如何没有变化不会产生影响）

- sprc.Call(node string, addr any, cmd string, args any, reply any) error 通过node和addr支持一个简单的rpc请求,返回error表示调用结果
- srpc.Send(node string, addr any, cmd string, args any) error 发送消息，error表示是否失败
- sprc.Invoke(caller *client.Caller) error 支持构建复杂的调用。WithTimeout/WithPayloadCodec等

## skynet API ##

skynet_example 提供 [libsrpc](./skynet_example/libsrpc.lua) 一个很简单的封装参考（100行代码）

- sprc.set_default_codec(name) 设置payload 打包方式，默认msgpack

作为客户端请求：

- srpc.send(node, sname, cmd, req) 发送消息到golang 不阻塞
- srpc.call(node, sname, cmd, req) 阻塞等待，两个返回值：ok表示调用是否成功。ret 表示内容

作为服务端分发：

- srpc.new_dispatcher(svc) 初始化一个消息分发器 disp, disp 仅包含一个router方法注册回调函数
  - disp.router("SET", function(msg) end) 注册一个SET方法到svc

- 其他参考[skynet_example测试用例](./skynet_example/main_test.lua)

# 更多用法 #

客户端用法参考 [client_test](./srpc_client_test.go)
服务器用法参考 [server_test](./srpc_server_test.go)

# 设计思路 #

skynet cluster一次rpc请求需要携带的参数为：

1. `addr uint32/string`目标地址Id或者name
2. `session uint32` 分包合并标记或者区分请求和推送
3. `cmd string` 调用的方法
4. `payload string` 携带的参数

我们约定payload为一个请求参数，不支持可变参数，并且payload必须被encode为字符串。这样能简化实现保证稳定，

如果支持可变类型展开，实现golang解析并不复杂。但是支持可变参数需要大量的反射和条件判断会让整体实现变得复杂，没有统一写法的约束也更容易出BUG.


- 数据编码

> lua 支持数据`number string boolean nil table`等基础类型。`table`转成`golang struct`可通过`lua tag`实现编码和解码。

我们可以选择更为通用的`json/msgpack`作为codec，经过验证更可靠。

- rpc调用方式

> 通常rpc会通过代码生成的方式和反射两种方案

这里使用反射更合适，为了提高效率我们约定只有一个请求参数和一个返回参数。同时约定Go函数原型的第一个参数必须为`SkynetContext`

支持无输入参数和无返回值。返回值error作为可选。必须作为最后一个返回参数
```Go
// 标准用法
func (s *Service) Set(*SkynetContext, args) *reply, error
// 同时兼容以下 缺省参数原型
func (s *Service) Set(*SkynetContext) *reply, error
func (s *Service) Set(*SkynetContext) *reply
func (s *Service) Set(*SkynetContext) error

func (s *Service) Set(*SkynetContext, args) *reply
func (s *Service) Set(*SkynetContext, args) error

func (s *Service) Set(*SkynetContext)
```

skynet 请求参数同时约束为一个请求值和一个返回值，使用codec序列化成字符串,codec可以自定义(默认提供`json/msgpack`)

```Lua
local req = {k =1, v =1}
local ok, res = pcall(cluster.call, node, sname, "Set", codec.encode(req))
if ok then
    res = codec.decode(res)
end
```
> 如果使用json作为codec 则需要注意lua空table问题，建议encode 为`null`, 并且json序列化必须为`lua table`和`golang struct/map`,不支持`int/string/bool`等类型

> 默认skynet和golang使用msgpack作为codec,支持原子类型和复杂的类型。

