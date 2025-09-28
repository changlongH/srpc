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
)

type (
	// dispatcher represents an RPC disoatcher.
	Dispatcher struct {
		serviceMap sync.Map // map[string]*service
	}
)

var (
	once sync.Once
	inst *Dispatcher
)

func GetDispatcher() *Dispatcher {
	if inst == nil {
		once.Do(func() {
			inst = &Dispatcher{}
		})
	}
	return inst
}

func (disp *Dispatcher) GetService(sname string) *service {
	svci, ok := disp.serviceMap.Load(sname)
	if !ok {
		return nil
	}
	return svci.(*service)
}

func (disp *Dispatcher) DispatchReq(sname string, methodName string, data []byte, isPush bool) ([]byte, error) {
	svci, ok := disp.serviceMap.Load(sname)
	if !ok {
		return nil, errors.New("not find service " + sname)
	}

	svc := svci.(*service)
	mtype := svc.method[methodName]
	if mtype == nil {
		return nil, fmt.Errorf("not find method (%s.%s)", sname, methodName)
	}

	startTime := time.Now()
	ctx := NewSkynetContext(context.Background())
	replyData, err := svc.call(mtype, ctx, data, isPush)
	if svc.Options.AccessHdle != nil {
		svc.Options.AccessHdle(ctx, sname, methodName, time.Since(startTime), err)
	}
	return replyData, err
}

// Register publishes uses the provided name in the dispatcher the set of methods of the
// receiver value that satisfy the following conditions:
//   - exported method of exported type
//   - two arguments, both of exported type
//   - the second argument is a pointer
//   - one return value, of type error
//
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func Register(rcvr any, name string, opts ...Option) error {
	s := NewService(opts...)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	if name == "" {
		name = reflect.Indirect(s.rcvr).Type().Name()
		if name == "" || !token.IsExported(name) {
			s := "srpc.Register: type " + name + " is not exported"
			log.Print(s)
			return errors.New(s)
		}
	}
	s.name = name

	logErr := false
	// Install the methods
	s.method = suitableMethods(s.typ, logErr)

	if len(s.method) == 0 {
		str := ""
		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PointerTo(s.typ), false)
		if len(method) != 0 {
			str = "rpc.Register: type " + name + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			str = "rpc.Register: type " + name + " has no exported methods of suitable type"
		}
		log.Print(str)
		return errors.New(str)
	}

	disp := GetDispatcher()
	if _, dup := disp.serviceMap.LoadOrStore(name, s); dup {
		return errors.New("rpc: service already defined: " + name)
	}
	if s.Options.SyncDisptch {
		go s.processMsgQueue()
	}
	return nil
}

func GetRegisterMethods(name string) ([]string, error) {
	disp := GetDispatcher()
	svci, ok := disp.serviceMap.Load(name)
	if !ok {
		return nil, errors.New("not find service " + name)
	}
	svc := svci.(*service)
	methods := svc.getAllMethods()
	return methods, nil
}
