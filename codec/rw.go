package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/cloudwego/netpoll"
)

// lua type
const (
	typeNil         = 0
	typeBoolean     = 1 // hibits 0 false 1 true
	typeNumber      = 2
	typeUserdata    = 3
	typeShortString = 4 // hibits 0-31 : len
	typeLongString  = 5
	typeTable       = 6
)

func combineType(t, v uint8) uint8 {
	return t | v<<3
}

func uncombineType(header uint8) (uint8, uint8) {
	return header & 0x7, header >> 3
}

func writeHeader(writer netpoll.Writer, header uint16) error {
	buf, err := writer.Malloc(2)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(buf, uint16(header))
	return nil
}

func readUint32(pkg netpoll.Reader) (uint32, error) {
	b, err := pkg.ReadBinary(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func writeUint32(writer netpoll.Writer, v uint32) error {
	buf, err := writer.Malloc(4)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(buf, v)
	return nil
}

func readSession(pkg netpoll.Reader) (uint32, error) {
	return readUint32(pkg)
}

func readString(pkg netpoll.Reader) ([]byte, error) {
	len := pkg.Len()
	if len <= 0 {
		return []byte{}, nil
	}

	header, _ := pkg.ReadByte()
	switch luaType, sz := uncombineType(header); luaType {
	case typeShortString:
		if sz == 0 {
			return []byte{}, nil
		}
		return pkg.ReadBinary(int(sz))
	case typeLongString:
		if sz != 2 && sz != 4 {
			return nil, fmt.Errorf("invalid long string size=%d,expect=2 or 4", sz)
		}
		header, err := pkg.ReadBinary(int(sz))
		if err != nil {
			return nil, err
		}
		if sz == 2 {
			len = int(binary.LittleEndian.Uint16(header))
		} else {
			len = int(binary.LittleEndian.Uint32(header))
		}
		return pkg.ReadBinary(len)
	case typeNil:
		return nil, nil
	default:
		return nil, fmt.Errorf("nonsupport decode luaSeriType=%d", luaType)
	}
}

func decodeString(data []byte) ([]byte, int, error) {
	dataLen := len(data)
	if dataLen <= 0 {
		return []byte{}, 0, nil
	}

	header := data[0]
	switch luaType, sz := uncombineType(header); luaType {
	case typeShortString:
		if sz == 0 {
			return []byte{}, 1, nil
		}
		index := int(sz) + 1
		if dataLen < index {
			return []byte{}, 0, fmt.Errorf("unpack short string size=%d,expect=%d", dataLen, index)
		}
		return data[1:index], index, nil
	case typeLongString:
		if sz == 2 || sz == 4 {
			index := int(sz) + 1
			if dataLen < index {
				return []byte{}, 0, fmt.Errorf("unpack long string size=%d,expect=%d", dataLen, index)
			}
			headerLen := data[1:index]
			var strLen int
			if sz == 2 {
				strLen = int(binary.LittleEndian.Uint16(headerLen))
			} else {
				strLen = int(binary.LittleEndian.Uint32(headerLen))
			}
			start := index
			index = index + strLen
			if dataLen < index {
				return []byte{}, 0, fmt.Errorf("unpack long string size=%d,expect=%d", dataLen, index)
			}
			return data[start:index], index, nil
		} else {
			return nil, 0, fmt.Errorf("invalid long string size=%d,expect=2 or 4", sz)
		}
	case typeNil:
		return nil, 1, nil
	default:
		return nil, 0, fmt.Errorf("nonsupport decode luaSeriType=%d", luaType)
	}
}

func writeSession(writer netpoll.Writer, session uint32) error {
	return writeUint32(writer, session)
}

func WriteStringToBuf(buffer *bytes.Buffer, b []byte) error {
	len := len(b)
	// write header
	if len < 32 {
		if err := buffer.WriteByte(combineType(typeShortString, uint8(len))); err != nil {
			return err
		}
		if len <= 0 {
			return nil
		}
	} else {
		buf := make([]byte, 0, 2)
		if len < 0x10000 {
			if err := buffer.WriteByte(combineType(typeLongString, 2)); err != nil {
				return err
			}
			buf = binary.LittleEndian.AppendUint16(buf, uint16(len))
		} else {
			if err := buffer.WriteByte(combineType(typeLongString, 4)); err != nil {
				return err
			}
			buf = binary.LittleEndian.AppendUint32(buf, uint32(len))
		}
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	}

	// write body
	if _, err := buffer.Write(b); err != nil {
		return err
	}
	return nil
}

func writeStringsToBuf(buffer *bytes.Buffer, args ...[]byte) error {
	for _, v := range args {
		if err := WriteStringToBuf(buffer, v); err != nil {
			return err
		}
	}
	return nil
}

func decodeStrings(data []byte) ([]string, error) {
	strs := []string{}
	for {
		if str, n, err := decodeString(data); err != nil || n == 0 {
			return strs, err
		} else {
			strs = append(strs, string(str))
			data = data[n:]
		}
	}
}
