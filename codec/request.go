package codec

import (
	"encoding/binary"
	"errors"
	"fmt"

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
		//Cmd     string
		//Message []byte
		Payload []byte
	}
)

func (addr *Addr) String() string {
	return fmt.Sprintf("(%d:%s)", addr.Id, addr.Name)
}

// a whole packge of number address
func unpackReqNumber(pkg netpoll.Reader) (*ReqPack, error) {
	len := pkg.Len()
	if len < 8 {
		return nil, fmt.Errorf("invalid cluster message (size=%d)", len)
	}

	// address(4)
	bLen, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	sid := binary.LittleEndian.Uint32(bLen)

	// session(4)
	bLen, err = pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	session := binary.LittleEndian.Uint32(bLen)

	req := &ReqPack{
		Addr:    Addr{Id: sid},
		Session: session,
	}

	if pkg.Len() > 0 {
		payload, err := pkg.ReadBinary(pkg.Len())
		if err != nil {
			return nil, err
		}
		req.Payload = payload
	}

	/*
		// cmd
		bcmd, err := readOneArgs(pkg)
		if err != nil {
			return req, err
		}
		req.Cmd = string(bcmd)

		// args
		data, err := readOneArgs(pkg)
		if err != nil {
			return req, err
		}
		req.Message = data
	*/
	return req, nil
}

// a whole packge of str address
func unpackReqStr(pkg netpoll.Reader) (*ReqPack, error) {
	len := pkg.Len()
	if len < 2 {
		return nil, fmt.Errorf("invalid cluster message headersize (size=%d)", len)
	}
	bLen, err := pkg.ReadByte()
	if err != nil {
		return nil, err
	}
	namesize := int(bLen)
	if len < namesize+5 {
		return nil, fmt.Errorf("invalid cluster message (size=%d)", len)
	}

	// 1 + namesize
	sname, err := pkg.ReadString(namesize)
	if err != nil {
		return nil, err
	}

	// session(4)
	bSession, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	session := binary.BigEndian.Uint32(bSession)
	req := &ReqPack{
		Addr:    Addr{Name: sname},
		Session: session,
	}
	if pkg.Len() > 0 {
		payload, err := pkg.ReadBinary(pkg.Len())
		if err != nil {
			return nil, err
		}
		req.Payload = payload
	}
	/*
		// cmd
		bcmd, err := readOneArgs(pkg)
		fmt.Println(bcmd)
		if err != nil {
			return req, err
		}
		req.Cmd = string(bcmd)

		// args
		data, err := readOneArgs(pkg)
		fmt.Println(data)
		if err != nil {
			return req, err
		}
		req.Message = data
	*/

	return req, nil
}

// header of int address
func unpackLargeReqNumber(pkg netpoll.Reader, pending map[uint32]*ReqPack, push bool) (*ReqPack, error) {
	len := pkg.Len()
	if len != 12 {
		return nil, fmt.Errorf("invalid cluster message size %d (multi req must be 13)", len)
	}
	// address(4)
	bLen, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	sid := binary.LittleEndian.Uint32(bLen)
	// session(4)
	if bLen, err = pkg.ReadBinary(4); err != nil {
		return nil, err
	}

	session := binary.LittleEndian.Uint32(bLen)
	req := &ReqPack{
		Addr:    Addr{Id: sid},
		Session: session,
	}
	// msgsize(4)
	bSize, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	msgsize := binary.LittleEndian.Uint32(bSize)
	req.Payload = make([]byte, 0, msgsize)
	pending[session] = req
	return nil, nil
}

// header of str address
func unpackLargeReqStr(pkg netpoll.Reader, pending map[uint32]*ReqPack, push bool) (*ReqPack, error) {
	len := pkg.Len()
	if len < 2 {
		return nil, fmt.Errorf("invalid request message (size=%d)", len)
	}
	bSize, err := pkg.ReadByte()
	if err != nil {
		return nil, err
	}
	namesize := int(bSize)
	if len < namesize+8 {
		return nil, fmt.Errorf("invalid request message (size=%d)", len)
	}

	// 1 + namesize
	addr, err := pkg.ReadString(namesize)
	if err != nil {
		return nil, err
	}
	// session(4)
	bLen, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	session := binary.LittleEndian.Uint32(bLen)
	req := &ReqPack{
		Addr:    Addr{Name: addr},
		Session: session,
	}
	// msgsize(4)
	bLen, err = pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	msgsize := binary.LittleEndian.Uint32(bLen)
	req.Payload = make([]byte, 0, msgsize)
	pending[session] = req
	return nil, nil
}

// part of large package
func unpackLargeReqPart(pkg netpoll.Reader, pending map[uint32]*ReqPack, final bool) (*ReqPack, error) {
	sz := pkg.Len()
	if sz < 4 {
		return nil, fmt.Errorf("invalid large request part headersz=(%d)", sz)
	}
	bLen, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	session := binary.LittleEndian.Uint32(bLen)

	req, ok := pending[session]
	if !ok {
		errmsg := fmt.Sprintf("invalid large request part session=%d", session)
		return nil, errors.New(errmsg)
	}
	p, err := pkg.Next(sz - 4)
	if err != nil {
		return nil, err
	}
	req.Payload = append(req.Payload, p...)
	if final {
		delete(pending, session)
		/*
			args, err := unpackStrings(req.Message)
			if err != nil {
				return req, err
			}
			req.Cmd = args[0]
			req.Message = []byte(args[1])
		*/
	}
	return req, nil
}

func DecodeReq(pkg netpoll.Reader, largeReq map[uint32]*ReqPack) (*ReqPack, error) {
	defer pkg.Release()

	len := pkg.Len()
	if len == 0 {
		return nil, errors.New("invalid request package size=0")
	}

	msgType, err := pkg.ReadByte()
	if err != nil {
		return nil, err
	}
	fmt.Println("---msg type:", msgType)

	switch msgType {
	case 0:
		return unpackReqNumber(pkg)
	case 1:
		// request
		return unpackLargeReqNumber(pkg, largeReq, false)
	case '\x41':
		// push
		return unpackLargeReqNumber(pkg, largeReq, true)
	case 2:
		return unpackLargeReqPart(pkg, largeReq, false)
	case 3:
		return unpackLargeReqPart(pkg, largeReq, true)
	case 4:
		return nil, errors.New("nonsupport trace msg")
	case '\x80':
		return unpackReqStr(pkg)
	case '\x81':
		// request
		return unpackLargeReqStr(pkg, largeReq, false)
	case '\xc1':
		// push
		return unpackLargeReqStr(pkg, largeReq, true)
	default:
		return nil, fmt.Errorf("invalid req package (type=%d)", msgType)
	}
}

func EncodeReq(writer netpoll.Writer, msg *ReqPack) error {
	payload := msg.Payload
	sz := uint32(len(payload))

	var isPush = false
	var session = msg.Session
	if session <= 0 {
		isPush = true
	}

	if msg.Addr.Id == 0 && msg.Addr.Name == "" {
		return errors.New("invalid request addr")
	}

	// first WORD is size of the package with big-endian
	if sz < PartSize {
		if msg.Addr.Id > 0 {
			header, _ := writer.Malloc(2)
			// header byte(1)+addr(4)+session(4)=9
			binary.BigEndian.PutUint16(header, uint16(sz+9))
			writer.WriteByte(0) // type 0
			addr, err := writer.Malloc(4)
			if err != nil {
				return err
			}
			binary.LittleEndian.PutUint32(addr, msg.Addr.Id)
		} else {
			header, err := writer.Malloc(2)
			if err != nil {
				return err
			}
			namelen := uint32(len(msg.Addr.Name))
			binary.BigEndian.PutUint16(header, uint16(sz+6+namelen))
			writer.WriteByte(0x80) // type 0x80
			writer.WriteByte(byte(namelen))
			writer.WriteString(msg.Addr.Name)
		}
		wsession, err := writer.Malloc(4)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(wsession, session)
		writer.WriteBinary(payload)
	} else {
		if msg.Addr.Id > 0 {
			header, err := writer.Malloc(2)
			if err != nil {
				return err
			}
			// multi part header byte(1)+addr(4)+session(4)+msgsize(4)=13
			binary.BigEndian.PutUint16(header, uint16(sz+13))
			if isPush {
				writer.WriteByte(0x41)
			} else {
				writer.WriteByte(1)
			}
			addr, err := writer.Malloc(4)
			if err != nil {
				return err
			}
			binary.LittleEndian.PutUint32(addr, msg.Addr.Id)
		} else {
			header, err := writer.Malloc(2)
			if err != nil {
				return err
			}
			namelen := uint32(len(msg.Addr.Name))
			// multi part header byte(1)+addr(1)+session(4)+msgsize(4)=10
			binary.BigEndian.PutUint16(header, uint16(namelen+10))
			if isPush {
				writer.WriteByte(0xc1)
			} else {
				writer.WriteByte(0x81)
			}
			writer.WriteByte(byte(namelen))
			writer.WriteString(msg.Addr.Name)
		}
		wsession, err := writer.Malloc(4)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(wsession, session)
		wsize, err := writer.Malloc(4)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(wsize, sz)

		part := int((sz-1)/PartSize + 1)
		index := uint32(0)
		for i := 0; i < part; i++ {
			var s uint32
			var bType byte
			if sz > PartSize {
				s = PartSize
				bType = 2 // multi part
			} else {
				s = sz
				bType = 3 // multi end
			}
			// type(1)+session(4)=5
			header, err := writer.Malloc(2)
			if err != nil {
				return err
			}
			binary.BigEndian.PutUint16(header, uint16(s+5))
			writer.WriteByte(bType)
			session, err := writer.Malloc(4)
			if err != nil {
				return err
			}
			binary.LittleEndian.PutUint32(session, msg.Session)
			writer.WriteBinary(payload[index : index+s])
			index = s
			sz = sz - s
		}
	}
	return nil
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

func PackPayload(args ...[]byte) ([]byte, error) {
	return packStrings(args...)
}

func UnpackPayload(data []byte) ([]string, error) {
	return unpackStrings(data)
}
