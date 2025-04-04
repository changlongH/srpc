package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/changlongH/srpc/codec"
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
)

type (
	Req struct {
		Caller *Caller
		Error  error
		Done   chan *Req
	}

	Client struct {
		// connecting lock
		sync.Mutex

		Options Options
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

var ErrClosing = errors.New("client is closing")

func (c *Client) IsClosing() bool {
	return c.closing
}

func (c *Client) Seq() uint32 {
	if c.seq == 0 {
		c.seq = 1
	}
	var seq = c.seq
	c.seq++
	return seq
}

func (c *Client) decodeRspArgs(req *Req, msg *codec.RespPack) {
	if req.Caller.Reply == nil {
		return
	}

	if len(msg.Payload) <= 0 {
		return
	}

	if !msg.Ok {
		req.Error = errors.New(string(msg.Payload))
		return
	}

	pcodec := c.Options.PayloadCodec
	if req.Caller.PayloadCodec != nil {
		pcodec = req.Caller.PayloadCodec
	}
	if err := pcodec.Unmarshal(msg.Payload, req.Caller.Reply); err != nil {
		req.Error = errors.New("payload unmarshal err: " + err.Error())
	}
}

func (c *Client) readResponse(conn netpoll.Connection) {
	defer func() {
		c.mutex.Lock()
		if err := recover(); err != nil {
			// TODO: need handle?
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
			// header bigEndian
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
			msg, err = codec.ReadResp(pkg, pendingPack)
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
				c.decodeRspArgs(req, msg)
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

func (c *Client) Invoke(caller *Caller) error {
	payload, err := c.EncodePayload(caller)
	if err != nil {
		return fmt.Errorf("invoke %s encode failed. %s", caller.String(), err.Error())
	}

	if c.conn == nil || !c.conn.IsActive() {
		if c.closing {
			return ErrClosing
		}
		if caller.IsPush() {
			// try async connect once
			go func() {
				if err := c.syncConnect(); err != nil {
					log.Printf("invoke %s connect failed. %s", caller.String(), err.Error())
				} else {
					c.mutex.Lock()
					var seq = c.Seq()
					c.mutex.Unlock()
					c.invoke(caller.Addr, seq, caller.Method, payload, caller.IsPush())
				}
			}()
			// return immediately
			return nil
		} else {
			// try connect once
			if err := c.syncConnect(); err != nil {
				return fmt.Errorf("invoke %s connect failed. %s", caller.String(), err.Error())
			}
		}
	}

	var req *Req
	c.mutex.Lock()
	var seq = c.Seq()
	if !caller.IsPush() {
		req = &Req{
			Caller: caller,
			Done:   make(chan *Req),
		}
		c.pending[seq] = req
	}
	c.mutex.Unlock()

	if err := c.invoke(caller.Addr, seq, caller.Method, payload, caller.IsPush()); err != nil {
		if !caller.IsPush() {
			c.mutex.Lock()
			delete(c.pending, seq)
			c.mutex.Unlock()
		}
		return fmt.Errorf("invoke %s socker failed. %s", caller.String(), err.Error())
	}

	if caller.IsPush() {
		return nil
	}

	if caller.Timeout == 0 {
		caller.Timeout = c.Options.CallTimeout
	}

	// wait call done
	select {
	case <-req.Done:
		return req.Error
	case <-time.After(caller.Timeout):
		return fmt.Errorf("invoke %s timeout", caller.String())
	}
}

func (c *Client) invoke(addr *codec.Addr, seq uint32, method string, payload []byte, push bool) error {
	pack := &codec.ReqPack{
		Addr:    *addr,
		Session: seq,
		Method:  method,
		Payload: payload,
		Push:    push,
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

func NewClient(address string, opts ...Option) (*Client, error) {
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
	var addr = c.Address
	time.AfterFunc(time.Second*15, func() {
		if c.conn != nil && c.conn.IsActive() {
			if err := c.conn.Close(); err != nil {
				log.Printf("close address:[%s] err:%s", addr, err.Error())
			}
			c.conn = nil
		}
	})
	return nil
}

func (c *Client) EncodePayload(caller *Caller) ([]byte, error) {
	if caller.Args == nil {
		return nil, nil
	}

	pcodec := c.Options.PayloadCodec
	// caller codec preference
	if caller.PayloadCodec != nil {
		pcodec = caller.PayloadCodec
	}
	// client codec default
	return pcodec.Marshal(caller.Args)
}
