package codec

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/netpoll"
)

type (
	Addr struct {
		Id   uint32
		Name string
	}
	ReqPack struct {
		Addr    Addr   // 4-7
		Session uint32 // 8-11
		Push    bool   // push package don't need to response
		Method  string
		Payload []byte
	}
)

func (addr *Addr) String() string {
	if addr.Id > 0 {
		return strconv.FormatUint(uint64(addr.Id), 10)
	}
	return strings.TrimPrefix(addr.Name, "@")
}

func (req *ReqPack) IsPush() bool {
	return req.Push
}

func readReqSessionAndArgs(pkg netpoll.Reader, req *ReqPack) (err error) {
	if req.Session, err = readSession(pkg); err != nil {
		return
	}

	if req.Session == 0 {
		req.Push = true
	}

	// rpc req must have method
	var method []byte
	if method, err = readString(pkg); err != nil || len(method) == 0 {
		err = fmt.Errorf("call session=%d,addr=%s not method", req.Session, req.Addr.String())
		return
	}
	// ToUpper first char
	char := method[0]
	if char >= 'a' && char <= 'z' {
		method[0] = char - 32
	}
	req.Method = string(method)

	if pkg.Len() > 0 {
		if req.Payload, err = readString(pkg); err != nil {
			return
		}
	}
	return nil
}

func unpackNumberAddrReq(pkg netpoll.Reader) (req *ReqPack, err error) {
	if pkg.Len() < 8 {
		err = fmt.Errorf("invalid message header (size=%d) expect=8", pkg.Len())
		return
	}
	req = &ReqPack{Addr: Addr{}}
	if req.Addr.Id, err = readUint32(pkg); err != nil {
		return
	}
	err = readReqSessionAndArgs(pkg, req)
	return
}

func unpackStrAddrReq(pkg netpoll.Reader) (req *ReqPack, err error) {
	if pkg.Len() < 8 {
		err = fmt.Errorf("invalid message header (size=%d) expect=8", pkg.Len())
		return
	}

	nameLen, _ := pkg.ReadByte()
	if pkg.Len() < int(nameLen) {
		err = fmt.Errorf("invalid req message (size=%d)", pkg.Len()+1)
		return
	}

	req = &ReqPack{Addr: Addr{}}
	if req.Addr.Name, err = pkg.ReadString(int(nameLen)); err != nil {
		return
	}
	err = readReqSessionAndArgs(pkg, req)
	return
}

func unpackNumberAddrPendingHeader(pkg netpoll.Reader, pending map[uint32]*ReqPack, push bool) (_req *ReqPack, err error) {
	if pkg.Len() != 12 {
		return nil, fmt.Errorf("invalid cluster message size %d (multi req must be 12)", pkg.Len())
	}

	req := &ReqPack{Addr: Addr{}, Push: push}
	if req.Addr.Id, err = readUint32(pkg); err != nil {
		return
	}
	if req.Session, err = readSession(pkg); err != nil {
		return
	}
	var bodyLen uint32
	if bodyLen, err = readUint32(pkg); err != nil {
		return
	}
	// NOTE: payload contain method
	req.Payload = make([]byte, 0, bodyLen)
	pending[req.Session] = req
	return
}

func unpackStrAddrPendingHeader(pkg netpoll.Reader, pending map[uint32]*ReqPack, push bool) (_req *ReqPack, err error) {
	if pkg.Len() < 10 {
		err = fmt.Errorf("invalid request message (size=%d)", pkg.Len())
		return
	}

	nameLen, _ := pkg.ReadByte()
	if pkg.Len() < int(nameLen)+8 {
		err = fmt.Errorf("invalid req message (size=%d)", pkg.Len()+1)
		return
	}
	req := &ReqPack{Addr: Addr{}, Push: push}
	if req.Addr.Name, err = pkg.ReadString(int(nameLen)); err != nil {
		return
	}
	if req.Session, err = readSession(pkg); err != nil {
		return
	}
	var bodyLen uint32
	if bodyLen, err = readUint32(pkg); err != nil {
		return
	}
	// NOTE: payload contain method
	req.Payload = make([]byte, 0, bodyLen)
	pending[req.Session] = req
	return
}

func unpackPendingPart(pkg netpoll.Reader, pending map[uint32]*ReqPack, final bool) (*ReqPack, error) {
	if pkg.Len() < 4 {
		return nil, fmt.Errorf("invalid request part headersz=(%d)", pkg.Len())
	}

	var err error
	var session uint32
	if session, err = readSession(pkg); err != nil {
		return nil, err
	}

	req, ok := pending[session]
	if !ok {
		err = fmt.Errorf("invalid request part session=%d", session)
		// reply client session error
		return &ReqPack{Session: session}, err
	}

	// read method if it's first part
	if len(req.Payload) == 0 && len(req.Method) == 0 {
		// rpc req must have method
		var method []byte
		if method, err = readString(pkg); err != nil || len(method) == 0 {
			err = fmt.Errorf("call session=%d,addr=%s not method", req.Session, req.Addr.String())
			delete(pending, session)
			return req, err
		}
		// ToUpper first char
		char := method[0]
		if char >= 'a' && char <= 'z' {
			method[0] = char - 32
		}
		req.Method = string(method)
	}

	if pkg.Len() > 0 {
		var payload []byte
		if payload, err = pkg.ReadBinary(pkg.Len()); err != nil {
			delete(pending, session)
			return req, err
		}
		req.Payload = append(req.Payload, payload...)
	}

	if !final {
		return nil, nil
	}

	delete(pending, session)
	req.Payload, _, err = decodeString(req.Payload)
	return req, err
}

func DecodeReq(pkg netpoll.Reader, pendingPack map[uint32]*ReqPack) (*ReqPack, error) {
	defer pkg.Release()

	len := pkg.Len()
	if len == 0 {
		return nil, errors.New("invalid request package size=0")
	}

	msgType, err := pkg.ReadByte()
	if err != nil {
		return nil, err
	}

	switch msgType {
	case 0:
		return unpackNumberAddrReq(pkg)
	case 1:
		// request
		return unpackNumberAddrPendingHeader(pkg, pendingPack, false)
	case '\x41':
		// push
		return unpackNumberAddrPendingHeader(pkg, pendingPack, true)
	case 2:
		return unpackPendingPart(pkg, pendingPack, false)
	case 3:
		return unpackPendingPart(pkg, pendingPack, true)
	case 4:
		return nil, errors.New("nonsupport skynet trace")
	case '\x80':
		return unpackStrAddrReq(pkg)
	case '\x81':
		// request
		return unpackStrAddrPendingHeader(pkg, pendingPack, false)
	case '\xc1':
		// push
		return unpackStrAddrPendingHeader(pkg, pendingPack, true)
	default:
		return nil, fmt.Errorf("invalid req package type=(%d)", msgType)
	}
}

func writeReqPack(writer netpoll.Writer, req *ReqPack, buf *bytes.Buffer) (err error) {
	bodyLen := buf.Len()
	if req.Addr.Id > 0 {
		// header type(1)+addr(4)+session(4)+bodyLen=9+bodyLen
		if err = writeHeader(writer, uint16(9+bodyLen)); err != nil {
			return
		}
		// type = 0
		writer.WriteByte(0)
		if err = writeUint32(writer, req.Addr.Id); err != nil {
			return
		}
	} else {
		nameLen := len(req.Addr.Name)
		// header type(1)+nameSize(1)+session(4) + nameLen + bodyLen
		if err = writeHeader(writer, uint16(6+nameLen+bodyLen)); err != nil {
			return
		}
		// type 0x80
		writer.WriteByte(0x80)
		writer.WriteByte(byte(nameLen))
		writer.WriteString(req.Addr.Name)
	}
	// session = 0 if it is push pack
	session := req.Session
	if req.IsPush() {
		session = 0
	}
	if err = writeSession(writer, session); err != nil {
		return
	}
	_, err = writer.WriteBinary(buf.Bytes())
	return
}

func writeLargeReqPack(writer netpoll.Writer, req *ReqPack, buf *bytes.Buffer) (err error) {
	bodyLen := buf.Len()
	if req.Addr.Id > 0 {
		// multi part header byte(1)+addr(4)+session(4)+msgsize(4)=13
		if err = writeHeader(writer, uint16(13)); err != nil {
			return
		}
		if req.Push {
			err = writer.WriteByte(0x41)
		} else {
			err = writer.WriteByte(1)
		}
		if err != nil {
			return
		}
		if err = writeUint32(writer, req.Addr.Id); err != nil {
			return
		}
	} else {
		nameLen := len(req.Addr.Name)
		// multi part header byte(1)+addr(1)+session(4)+msgsize(4)+nameLne=10+nameLen
		if err = writeHeader(writer, uint16(nameLen+10)); err != nil {
			return
		}
		if req.Push {
			err = writer.WriteByte(0xc1)
		} else {
			err = writer.WriteByte(0x81)
		}
		if err != nil {
			return
		}
		writer.WriteByte(byte(nameLen))
		writer.WriteString(req.Addr.Name)
	}
	// fill session
	if err = writeSession(writer, req.Session); err != nil {
		return
	}
	// body size
	if err = writeUint32(writer, uint32(bodyLen)); err != nil {
		return
	}

	// body part
	payload := buf.Bytes()
	remainLen := bodyLen
	part := (remainLen-1)/PartSize + 1
	index := 0
	for i := 0; i < part; i++ {
		var sz int
		var bType byte
		if remainLen > PartSize {
			sz = PartSize
			bType = 2 // multi part
		} else {
			sz = remainLen
			bType = 3 // multi end
		}
		// type(1)+session(4)+sz
		if err = writeHeader(writer, uint16(5+sz)); err != nil {
			return
		}

		if err = writer.WriteByte(bType); err != nil {
			return
		}

		if err = writeSession(writer, req.Session); err != nil {
			return
		}
		if _, err = writer.WriteBinary(payload[index : index+sz]); err != nil {
			return
		}
		index = sz
		remainLen = remainLen - sz
	}
	return
}

func EncodeReq(writer netpoll.Writer, req *ReqPack) (err error) {
	if req.Addr.Id == 0 && req.Addr.Name == "" {
		err = errors.New("invalid request addr")
		return
	}

	buf := &bytes.Buffer{}
	if err = writeStringsToBuf(buf, []byte(req.Method), req.Payload); err != nil {
		return
	}
	if buf.Len() < PartSize {
		return writeReqPack(writer, req, buf)
	} else {
		return writeLargeReqPack(writer, req, buf)
	}
}

func GetServiceAddress(address interface{}) (*Addr, error) {
	var addr = &Addr{}
	switch v := address.(type) {
	case string:
		if v[0] != '@' {
			addr.Name = "@" + v
		} else {
			addr.Name = v
		}
	case uint32:
		addr.Id = v
	case int:
		addr.Id = uint32(v)
	case int8:
		addr.Id = uint32(v)
	case int16:
		addr.Id = uint32(v)
	case int32:
		addr.Id = uint32(v)
	case int64:
		addr.Id = uint32(v)
	case uint:
		addr.Id = uint32(v)
	case uint8:
		addr.Id = uint32(v)
	case uint16:
		addr.Id = uint32(v)
	case uint64:
		addr.Id = uint32(v)
	default:
		return nil, errors.New("invalid skynet service address")
	}
	return addr, nil
}
