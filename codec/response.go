package codec

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/cloudwego/netpoll"
)

type RetType byte

const (
	RespTypeErr    RetType = 0
	RespTypeOk     RetType = 1
	RespTypeMBegin RetType = 2
	RespTypeMPart  RetType = 3
	RespTypeMEnd   RetType = 4
)

type (
	RespPack struct {
		Ok      bool   // msg pack/unpack
		Session uint32 // DWORD
		Message []byte // 0: errmsg  1: msg  2: DWORD size   3/4: msg
	}
)

const (
	PartSize uint32 = 0x8000
)

func EncodeResp(writer netpoll.Writer, msg *RespPack) error {
	data, err := packStrings(msg.Message)
	if err != nil {
		return err
	}
	sz := uint32(len(data))
	bType := RespTypeOk
	if msg.Ok {
		if sz > PartSize {
			// multi part header session(4)+byte(1)+msgsize(4)=9
			header, _ := writer.Malloc(2)
			binary.BigEndian.PutUint16(header, 9)
			session, _ := writer.Malloc(4)
			binary.LittleEndian.PutUint32(session, msg.Session)
			writer.WriteByte(byte(RespTypeMBegin))
			msgsz, _ := writer.Malloc(4)
			binary.LittleEndian.PutUint32(msgsz, sz)

			part := int((sz-1)/PartSize + 1)
			index := uint32(0)
			// multi part others
			for i := 0; i < part; i++ {
				var s uint32
				var bType RetType
				if sz > PartSize {
					s = PartSize
					bType = RespTypeMPart
				} else {
					s = sz
					bType = RespTypeMEnd
				}
				header, _ := writer.Malloc(2)
				binary.BigEndian.PutUint16(header, uint16(s+5))
				session, _ := writer.Malloc(4)
				binary.LittleEndian.PutUint32(session, msg.Session)
				writer.WriteByte(byte(bType))
				writer.WriteBinary(data[index : index+s])
				index = s
				sz = sz - s
			}
			err = writer.Flush()
			return err
		}
	} else {
		bType = RespTypeErr
		// truncate the error msg if too long
		if sz > PartSize {
			sz = PartSize
			data = data[:sz]
		}
	}

	// header(2)
	header, _ := writer.Malloc(2)
	binary.BigEndian.PutUint16(header, uint16(sz+5))
	// session(4)
	session, _ := writer.Malloc(4)
	binary.LittleEndian.PutUint32(session, msg.Session)
	//type(1)
	writer.WriteByte(byte(bType))
	// msg(uint32-7)
	writer.WriteBinary(data)
	return writer.Flush()
}

func DecodeResp(pkg netpoll.Reader, largeResp map[uint32]*RespPack) (*RespPack, error) {
	defer pkg.Release()
	headersz := 5
	sz := pkg.Len()
	if sz < headersz {
		return nil, errors.New("invalid response package size=0")
	}

	// session(4)
	bLen, err := pkg.ReadBinary(4)
	if err != nil {
		return nil, err
	}
	session := binary.LittleEndian.Uint32(bLen)

	// code(1)
	code, err := pkg.ReadByte()
	if err != nil {
		return nil, err
	}

	switch code {
	case 0: // error
		msg, err := readOneArgs(pkg)
		if err != nil {
			return nil, err
		}
		resp := &RespPack{
			Session: session,
			Ok:      false,
			Message: msg,
		}
		return resp, nil
	case 1: // ok
		msg, err := readOneArgs(pkg)
		if err != nil {
			return nil, err
		}
		resp := &RespPack{
			Session: session,
			Ok:      true,
			Message: msg,
		}
		return resp, nil
	case 4: // multi end
		msg, err := pkg.ReadBinary(sz - headersz)
		if err != nil {
			return nil, err
		}
		if resp, ok := largeResp[session]; ok {
			data := append(resp.Message, msg...)
			args, err := unpackStrings(data)
			if err != nil {
				return nil, err
			}
			resp.Message = []byte(args[0])
			return resp, nil
		} else {
			return nil, fmt.Errorf("invalid large response end part session=(%d)", session)
		}
	case 2: // multi begin
		if sz != 9 {
			return nil, fmt.Errorf("invalid multi begin headersz=(%d)", sz)
		}
		msg, err := pkg.ReadBinary(sz - headersz)
		if err != nil {
			return nil, err
		}
		resp := &RespPack{
			Session: session,
			Ok:      true,
			Message: msg,
		}
		largeResp[session] = resp
		return nil, nil
	case 3: // multi part
		msg, err := pkg.ReadBinary(sz - headersz)
		if err != nil {
			return nil, err
		}
		if resp, ok := largeResp[session]; ok {
			resp.Message = append(resp.Message, msg...)
			return nil, nil
		} else {
			return nil, fmt.Errorf("invalid large response part session=(%d)", session)
		}
	default:
		return nil, nil
	}
}
