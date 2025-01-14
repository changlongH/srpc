package server

import (
	"bytes"
	"fmt"
	"log"
	"runtime"

	"github.com/changlongH/srpc/codec"
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
)

type (
	GateAgent struct {
		conn   netpoll.Connection
		wqueue *mux.ShardQueue     // use for write
		Reader chan netpoll.Reader // use for reader socker data
	}
)

func defaultRecoveryHandle(remoteAddr string, err any) {
	var buf [2048]byte
	n := runtime.Stack(buf[:], false)
	log.Printf("[recover] clientAddr=%s err=%v\n %s", remoteAddr, err, string(buf[:n]))
}

var recoveryHandle = defaultRecoveryHandle

// By default, it will print the time and stack information of the error
func SetRecoveryHandler(handle func(string, any)) {
	recoveryHandle = handle
}

func NewGateAgent(conn netpoll.Connection) *GateAgent {
	agent := &GateAgent{
		conn:   conn,
		wqueue: mux.NewShardQueue(mux.ShardSize, conn),
		Reader: make(chan netpoll.Reader, 1000),
	}
	return agent
}

func (agent *GateAgent) Start() {
	defer func() {
		if err := recover(); err != nil {
			if agent.conn.IsActive() {
				agent.conn.Close()
			}
			recoveryHandle(agent.conn.RemoteAddr().String(), err)
		}
	}()

	var closeCh = make(chan struct{})
	agent.conn.AddCloseCallback(func(conn netpoll.Connection) error {
		log.Printf("close connect %s\n", conn.RemoteAddr().String())
		close(closeCh)
		agent.wqueue = nil
		return nil
	})

	var pendingReqPack = make(map[uint32]*codec.ReqPack)
	var err error
	var req *codec.ReqPack
	for {
		select {
		case r := <-agent.Reader:
			if req, err = codec.DecodeReq(r, pendingReqPack); err != nil {
				if req != nil && !req.IsPush() {
					agent.ResponseErr(req.Session, err)
				}
				continue
			}

			if req != nil {
				go agent.Dispatch(req)
			}
		case <-closeCh:
			// conn close can't respone
			pendingReqPack = nil
			return
		}
	}
}

func (agent *GateAgent) response(session uint32, ok bool, payload []byte) {
	if agent.wqueue == nil {
		return
	}
	writer := netpoll.NewLinkBuffer()
	err := codec.WriteResp(writer, &codec.RespPack{
		Session: session,
		Ok:      ok,
		Payload: payload,
	})
	if err != nil {
		return
	}

	// Put puts the buffer getter back to the queue.
	agent.wqueue.Add(func() (buf netpoll.Writer, isNil bool) {
		return writer, false
	})
}

func (agent *GateAgent) ResponseErr(session uint32, err error) {
	agent.response(session, false, []byte(err.Error()))
}

func (agent *GateAgent) ResponseOk(session uint32, payload []byte) {
	agent.response(session, true, payload)
}

func (agent *GateAgent) Dispatch(req *codec.ReqPack) {
	var sname = req.Addr.String()
	var method = req.Method
	defer func() {
		if err := recover(); err != nil {
			log.Printf("[panic] call %s %s.%s err=%v", agent.conn.RemoteAddr().String(), sname, method, err)
			if !req.IsPush() {
				agent.ResponseErr(req.Session, fmt.Errorf("[panic] call=%s.%s err=%v", sname, method, err))
			}
			return
		}
	}()

	//log.Printf("dispatch session=%d push=%v call=%s.%s  args=%v", req.Session, req.IsPush(), sname, method, string(req.Payload))
	replyBytes, err := GetDispatcher().DispatchReq(sname, method, req.Payload, req.IsPush())
	if req.IsPush() {
		return
	}

	if err != nil {
		agent.ResponseErr(req.Session, err)
		return
	}

	//log.Printf("reply session=%d msg=%s", req.Session, string(replyBytes))
	buf := &bytes.Buffer{}
	if err := codec.WriteStringToBuf(buf, replyBytes); err != nil {
		agent.ResponseErr(req.Session, err)
	} else {
		agent.ResponseOk(req.Session, buf.Bytes())
	}
}
