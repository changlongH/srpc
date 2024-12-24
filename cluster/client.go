package cluster

import (
	"encoding/binary"
	"errors"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/changlongH/srpc/codec"
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
)

type ArgsCodec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

type (
	Req struct {
		Error error
		Reply any
		Done  chan *Req
	}

	Client struct {
		// connecting lock
		sync.Mutex

		Options ClientOptions
		Address string

		mutex   sync.Mutex
		seq     uint32
		pending map[uint32]*Req

		conn   netpoll.Connection
		wqueue *mux.ShardQueue // use for write

		closing bool // address changed or use has called Close
		//shutdown bool // server has told us to stop
	}
)

func (c *Client) Seq() uint32 {
	if c.seq == 0 {
		c.seq = 1
	}
	var seq = c.seq
	c.seq++
	return seq
}

func (c *Client) readResponse(conn netpoll.Connection) {
	defer func() {
		c.mutex.Lock()
		if err := recover(); err != nil {
			log.Println(err)
			if c.conn != nil && c.conn.IsActive() {
				c.conn.Close()
			}
		}
		for _, req := range c.pending {
			req.Error = errors.New("socker close")
			req.Done <- req
		}
		c.pending = map[uint32]*Req{}
		c.mutex.Unlock()
	}()

	var closeCh = make(chan struct{})
	var recv = make(chan netpoll.Reader, 1000)
	var headerSize = 2
	go func() error {
		defer func() {
			closeCh <- struct{}{}
		}()
		for {
			reader := conn.Reader()
			bLen, err := reader.ReadBinary(headerSize)
			if err != nil {
				return err
			}
			pkgsize := int(binary.BigEndian.Uint16(bLen))
			pkg, err := reader.Slice(pkgsize)
			if err != nil {
				return err
			}
			recv <- pkg
		}
	}()

	var pendingPack = make(map[uint32]*codec.RespPack)
	var err error
	var msg *codec.RespPack
	for {
		select {
		case pkg := <-recv:
			msg, err = codec.DecodeResp(pkg, pendingPack)
			if err != nil {
				return
			}
			if msg == nil {
				continue
			}
			session := msg.Session
			c.mutex.Lock()
			req, ok := c.pending[session]
			delete(c.pending, session)
			c.mutex.Unlock()
			if ok {
				if len(msg.Message) > 0 && req.Reply != nil {
					switch replay := req.Reply.(type) {
					case *string:
						*replay = string(msg.Message)
					case *[]byte:
						*replay = msg.Message
					default:
						if err = c.Options.ArgsCodec.Unmarshal(msg.Message, req.Reply); err != nil {
							req.Error = errors.New("Unmarshal err: " + err.Error())
						}
					}
				}
				req.Done <- req
			} else {
				// invalid session
			}
		case <-closeCh:
			return
		}
	}
}

// should check closing before connect
func (c *Client) syncConnect() error {
	c.Lock()
	defer func() {
		if err := recover(); err != nil {
			c.Unlock()
			return
		}
		c.Unlock()
	}()

	// connected
	if c.conn != nil && c.conn.IsActive() {
		return nil
	}

	conn, err := netpoll.DialConnection("tcp", c.Address, time.Second*5)
	if err != nil {
		return err
	}

	conn.AddCloseCallback(func(connection netpoll.Connection) error {
		if c.wqueue != nil {
			q := c.wqueue
			c.wqueue = nil
			q.Close()
		}
		c.conn = nil
		return nil
	})

	c.conn = conn
	c.wqueue = mux.NewShardQueue(mux.ShardSize, conn)
	go c.readResponse(conn)
	return nil
}

// Send Send equal skynet cluster.send(node, addr, cmd, ...)
// node: registered cluster node name
// service: support skynet string name or number address
func (c *Client) Send(node string, addr *codec.Addr, payload []byte) error {
	if c.conn == nil || !c.conn.IsActive() {
		// closeing represents node address changed
		if c.closing {
			proxy := Query(node)
			if proxy != nil {
				return proxy.Send(node, addr, payload)
			}
			return errors.New("cluster client shutdown node: " + node)
		} else {
			// try connect once
			go func() {
				if err := c.syncConnect(); err != nil {
					log.Printf("cluster send node=%s,addr=%s,err=%s", node, addr, err)
				} else {
					c.invoke(addr, 0, payload)
				}
			}()
			return nil
		}
	}
	if err := c.invoke(addr, 0, payload); err != nil {
		return errors.New("send invoke err:" + err.Error())
	}
	return nil
}

func (c *Client) Call(node string, addr *codec.Addr, payload []byte, reply any, timeout time.Duration) error {
	if reply != nil {
		valueType := reflect.TypeOf(reply)
		if valueType.Kind() != reflect.Ptr {
			return errors.New("reply kind must be (ptr or nil)")
		}
	}

	if c.conn == nil || !c.conn.IsActive() {
		// closeing represents node address changed
		if c.closing {
			proxy := Query(node)
			if proxy != nil {
				return proxy.Call(node, addr, payload, reply, timeout)
			}
			return errors.New("cluster client shutdown node: " + node)
		} else {
			// try connect once
			if err := c.syncConnect(); err != nil {
				return errors.New("call connect err:" + err.Error())
			}
			// connect success continue
		}
	}

	req := &Req{
		Reply: reply,
		Done:  make(chan *Req),
	}

	c.mutex.Lock()
	var seq = c.Seq()
	c.pending[seq] = req
	c.mutex.Unlock()

	if err := c.invoke(addr, seq, payload); err != nil {
		c.mutex.Lock()
		delete(c.pending, seq)
		c.mutex.Unlock()
		return errors.New("call invoke err:" + err.Error())
	}

	select {
	case <-req.Done:
		return req.Error
	case <-time.After(timeout):
		return errors.New("timeout")
	}
}

func (c *Client) invoke(addr *codec.Addr, seq uint32, payload []byte) error {
	pack := &codec.ReqPack{
		Addr:    *addr,
		Session: seq,
		Payload: payload,
	}

	writer := netpoll.NewLinkBuffer()
	if err := codec.EncodeReq(writer, pack); err != nil {
		return err
	}

	// Put puts the buffer getter back to the queue.
	c.wqueue.Add(func() (buf netpoll.Writer, isNil bool) {
		return writer, false
	})
	return nil
}

func NewClient(address string, opts ...ClientOption) (*Client, error) {
	options := defaultClientOptions
	for _, opt := range opts {
		opt(&options)
	}

	c := &Client{
		Options: options,
		Address: address,
		pending: map[uint32]*Req{},
	}
	// __waiting = false
	return c, nil
}

func (c *Client) Close() error {
	c.closing = true
	// delay close socket
	time.AfterFunc(time.Second*15, func() {
		if c.conn != nil && c.conn.IsActive() {
			if err := c.conn.Close(); err != nil {
				log.Println("close err: " + err.Error())
			}
			c.conn = nil
		}
	})
	return nil
}

func (c *Client) EncodePayload(cmd string, args ...any) ([]byte, error) {
	var data = [][]byte{[]byte(cmd)}
	for _, arg := range args {
		var msg []byte
		switch v := arg.(type) {
		case nil:
			msg = []byte{}
		case string:
			msg = []byte(v)
		case []byte:
			msg = v
		default:
			if enc, err := c.Options.ArgsCodec.Marshal(arg); err != nil {
				return nil, err
			} else {
				msg = enc
			}
		}
		data = append(data, msg)
	}
	payload, err := codec.PackPayload(data...)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
