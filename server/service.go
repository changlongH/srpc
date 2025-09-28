package server

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/changlongH/srpc/codec"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
)

type (
	msg struct {
		req   *codec.ReqPack
		agent *GateAgent
	}
	service struct {
		name    string                 // name of service
		rcvr    reflect.Value          // receiver of methods for the service
		typ     reflect.Type           // type of the receiver
		method  map[string]*methodType // registered methods
		Options Options

		sessionMutex   sync.Mutex
		currentMethod  string
		currentSession uint64    //  monitor sync dispatch (maybe in endless loop)
		sessionCounter uint64    // increment session id
		msgQueue       chan *msg // msgqueue
	}
)

// Precompute the reflect type for *SkynetContext.
var typeOfSkynetCtx = reflect.TypeOf((*SkynetContext)(nil))

// Precompute the reflect type for error.
var typeOfError = reflect.TypeFor[error]()

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return token.IsExported(t.Name()) || t.PkgPath() == ""
}

// suitableMethods returns suitable Rpc methods of typ. It will log
// errors if logErr is true.
func suitableMethods(typ reflect.Type, logErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if !method.IsExported() {
			continue
		}
		numIn := mtype.NumIn()
		numOut := mtype.NumOut()
		if numIn < 2 || numIn > 3 {
			if logErr {
				log.Printf("rpc.Register: method %q has %d input parameters; needs exactly two\n", mname, mtype.NumIn())
			}
			continue
		}

		if numOut > 2 {
			if logErr {
				log.Printf("rpc.Register: method %q is not exported\n", mname)
			}
			continue
		}

		// First arg must be *SkynetContext
		ctxType := mtype.In(1)
		if mtype.In(1) != typeOfSkynetCtx {
			if logErr {
				log.Printf("rpc.Register: first augument type of method %q is %q, must be %s\n", mname, ctxType, typeOfSkynetCtx)
			}
			continue
		}

		var argType reflect.Type
		if numIn > 2 {
			// Second arg need not be a pointer.
			argType = mtype.In(2)
			if !isExportedOrBuiltinType(argType) {
				if logErr {
					log.Printf("rpc.Register: second argument type of method %q is not exported: %q\n", mname, argType)
				}
				continue
			}
		}

		// reply must be a pointer.
		var replyType reflect.Type

		// returns (*reply, error) or (*reply) or (error)
		var hasReply = false
		var errIndex = -1
		if numOut > 0 {
			if returnType := mtype.Out(0); returnType != typeOfError {
				replyType := mtype.Out(0)
				if replyType.Kind() != reflect.Pointer {
					if logErr {
						log.Printf("rpc.Register: reply type of method %q is not a pointer: %q\n", mname, replyType)
					}
					continue
				}
				// Reply type must be exported.
				if !isExportedOrBuiltinType(replyType) {
					if logErr {
						log.Printf("rpc.Register: reply type of method %q is not exported: %q\n", mname, replyType)
					}
					continue
				}
				hasReply = true
			} else {
				errIndex = 0
			}
		}
		if numOut > 1 {
			if returnType := mtype.Out(1); returnType != typeOfError {
				if logErr {
					log.Printf("rpc.Register: return type of method %q is %q, must be error\n", mname, returnType)
				}
				continue
			}
			errIndex = 1
		}

		methods[mname] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
			hasReply:  hasReply,
			errIndex:  errIndex,
		}
	}
	return methods
}

func NewService(opts ...Option) *service {
	options := Options{
		PayloadCodec: payloadcodec.MsgPack{},
	}
	for _, opt := range opts {
		opt(&options)
	}
	svc := &service{
		Options: options,
	}
	if svc.Options.SyncDisptch {
		svc.msgQueue = make(chan *msg, 5000)
	}
	return svc
}

func (s *service) call(mtype *methodType, ctx *SkynetContext, data []byte, isPush bool) ([]byte, error) {
	callVals := make([]reflect.Value, 0, 3)
	callVals = append(callVals, s.rcvr, reflect.ValueOf(ctx))
	// Decode the argument value.
	if mtype.ArgType != nil {
		var argv reflect.Value
		argIsValue := false // if true, need to indirect before calling.
		if mtype.ArgType.Kind() == reflect.Pointer {
			argv = reflect.New(mtype.ArgType.Elem())
		} else {
			argv = reflect.New(mtype.ArgType)
			argIsValue = true
		}

		if s.Options.PayloadCodec.IsNull(data) {
			if argIsValue {
				return nil, errors.New("missing request parameters")
			} else {
				argv = reflect.Zero(mtype.ArgType)
			}
		} else {
			if err := s.Options.PayloadCodec.Unmarshal(data, argv.Interface()); err != nil {
				return nil, errors.New("unmarshal args err:" + err.Error())
			}
		}
		if argIsValue {
			argv = argv.Elem()
		}
		callVals = append(callVals, argv)
	}

	mtype.Lock()
	mtype.numCalls++
	ctx.numCall = mtype.numCalls
	mtype.Unlock()

	returnValues := mtype.method.Func.Call(callVals)
	if isPush {
		return nil, nil
	}

	errIndex := mtype.errIndex
	if errIndex >= 0 && !returnValues[errIndex].IsNil() {
		if errInter := returnValues[errIndex].Interface().(error); errInter != nil {
			return nil, errInter
		}
	}

	if mtype.hasReply {
		replyVal := returnValues[0]
		if replyVal.IsNil() {
			return nil, nil
		}
		if data, err := s.Options.PayloadCodec.Marshal(replyVal.Elem().Interface()); err != nil {
			return nil, errors.New("marshal reply err:" + err.Error())
		} else {
			return data, nil
		}
	}
	return nil, nil
}

func (s *service) getAllMethods() []string {
	methods := make([]string, 0, len(s.method))
	for name := range s.method {
		methods = append(methods, name)
	}
	return methods
}

func (s *service) incCurrentSessionID(method string) uint64 {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	s.sessionCounter++
	s.currentMethod = method
	s.currentSession = s.sessionCounter
	return s.currentSession
}

func (s *service) getCurrentSession() uint64 {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	return s.currentSession
}

func (s *service) resetCurrentSession() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.currentSession = 0
}

func (s *service) startMonitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastSession uint64
	var sessionStartTime time.Time

	for {
		<-ticker.C

		currentSession := s.getCurrentSession()
		if currentSession == 0 {
			lastSession = 0
			continue
		}

		if currentSession == lastSession && currentSession != 0 {
			duration := time.Since(sessionStartTime)
			if duration.Seconds() >= s.Options.MonitorInterval.Seconds() {
				var err = fmt.Errorf("ERROR: service:[%s] maybe in endlessloop method [%s] duration:%dms", s.name, s.currentMethod, duration.Milliseconds())
				recoveryHandle(s.name, err)
			}
		} else {
			lastSession = currentSession
			sessionStartTime = time.Now()
		}
	}
}

func (s *service) pushMsgToDispatchQueue(agent *GateAgent, req *codec.ReqPack) {
	s.msgQueue <- &msg{req: req, agent: agent}
}

func (s *service) processMsgQueue() {
	go s.startMonitor()
	for msg := range s.msgQueue {
		var req = msg.req
		s.incCurrentSessionID(req.Method)
		msg.agent.callServiceMethod(s, req.Method, req.Session, req.Payload, req.Push)
		s.resetCurrentSession()
	}
}

func (s *service) dispatch(methodName string, data []byte, isPush bool) ([]byte, error) {
	mtype := s.method[methodName]
	if mtype == nil {
		return nil, fmt.Errorf("not find method (%s.%s)", s.name, methodName)
	}

	startTime := time.Now()
	ctx := NewSkynetContext(context.Background())
	replyData, err := s.call(mtype, ctx, data, isPush)
	if s.Options.AccessHdle != nil {
		s.Options.AccessHdle(ctx, s.name, methodName, time.Since(startTime), err)
	}
	return replyData, err
}
