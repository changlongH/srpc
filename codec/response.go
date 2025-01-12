package codec

import (
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
		Payload []byte // 0: errmsg  1: msg  2: DWORD size   3/4: msg
	}
)

const (
	PartSize int = 0x8000
)

func WriteResp(writer netpoll.Writer, msg *RespPack) (err error) {
	data := msg.Payload
	sz := len(data)
	bType := RespTypeOk
	if !msg.Ok {
		// truncate the error msg if too long
		if sz > PartSize {
			sz = PartSize
			data = data[:sz]
		}
		bType = RespTypeErr
	}

	if sz <= PartSize {
		// header = session(4) + type(1) + bodysize
		if err = writeHeader(writer, uint16(sz+5)); err != nil {
			return
		}
		if err = writeSession(writer, msg.Session); err != nil {
			return
		}
		if err = writer.WriteByte(byte(bType)); err != nil {
			return
		}
		if _, err = writer.WriteBinary(data); err != nil {
			return
		}
	} else {
		// multi part header session(4)+byte(1)+msgsize(4)=9
		if err = writeHeader(writer, 9); err != nil {
			return
		}
		if err = writeSession(writer, msg.Session); err != nil {
			return
		}
		if err = writer.WriteByte(byte(RespTypeMBegin)); err != nil {
			return
		}
		if err = writeUint32(writer, uint32(sz)); err != nil {
			return
		}

		part := int((sz-1)/PartSize + 1)
		index := 0
		// multi part others
		for i := 0; i < part; i++ {
			var s int
			var bType RetType
			if sz > PartSize {
				s = PartSize
				bType = RespTypeMPart
			} else {
				s = sz
				bType = RespTypeMEnd
			}
			// session(4) + type(1)
			if err = writeHeader(writer, uint16(s+5)); err != nil {
				return
			}
			if err = writeSession(writer, msg.Session); err != nil {
				return
			}
			if err = writer.WriteByte(byte(bType)); err != nil {
				return
			}
			// body
			if _, err = writer.WriteBinary(data[index : index+s]); err != nil {
				return
			}
			index = s
			sz = sz - s
		}
	}
	err = writer.Flush()
	return
}

func ReadResp(pkg netpoll.Reader, pendingResp map[uint32]*RespPack) (resp *RespPack, err error) {
	defer pkg.Release()
	headersz := 5
	sz := pkg.Len()
	if sz < headersz {
		return nil, errors.New("invalid response package size=0")
	}

	var session uint32
	if session, err = readSession(pkg); err != nil {
		return nil, err
	}

	var code byte
	if code, err = pkg.ReadByte(); err != nil {
		return
	}

	var payload []byte
	switch code {
	case 0: // error
		if payload, err = pkg.ReadBinary(pkg.Len()); err != nil {
			return
		}
		resp = &RespPack{
			Session: session,
			Ok:      false,
			Payload: payload,
		}
		return
	case 1: // ok
		if payload, err = readString(pkg); err != nil {
			return
		}
		resp = &RespPack{
			Session: session,
			Ok:      true,
			Payload: payload,
		}
		return
	case 4: // multi end
		if payload, err = pkg.ReadBinary(sz - headersz); err != nil {
			return
		}
		var ok bool
		if resp, ok = pendingResp[session]; !ok {
			err = fmt.Errorf("invalid pack end part session=(%d)", session)
			return
		}
		delete(pendingResp, session)
		payload = append(resp.Payload, payload...)
		resp.Payload, _, err = decodeString(payload)
		return
	case 2: // multi begin
		if sz != 9 {
			err = fmt.Errorf("invalid pack multi begin headersz=(%d)", sz)
			return
		}
		if payload, err = pkg.ReadBinary(sz - headersz); err != nil {
			return
		}
		pendingResp[session] = &RespPack{
			Session: session,
			Ok:      true,
			Payload: payload,
		}
		return
	case 3: // multi part
		if payload, err = pkg.ReadBinary(sz - headersz); err != nil {
			return
		}
		if pack, ok := pendingResp[session]; ok {
			pack.Payload = append(pack.Payload, payload...)
		} else {
			err = fmt.Errorf("invalid large response part session=(%d)", session)
		}
		return
	default:
		return nil, nil
	}
}
