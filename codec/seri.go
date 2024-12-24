package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/cloudwego/netpoll"
)

func combineType(t, v uint8) uint8 {
	return t | v<<3
}

func uncombineType(header uint8) (uint8, uint8) {
	return header & 0x7, header >> 3
}

func readOneArgs(pkg netpoll.Reader) ([]byte, error) {
	if pkg.Len() <= 0 {
		return []byte{}, nil
	}

	header, err := pkg.ReadByte()
	if err != nil {
		return nil, err
	}

	vType, sz := uncombineType(header)
	if vType == 4 && sz == 0 {
		return []byte{}, nil
	}
	vLen := int(sz)
	switch vType {
	case 0:
		// lua nil
		return []byte{}, nil
	case 1:
		// boolean
		return nil, fmt.Errorf("non support lua boolean")
	case 2:
		// lua number
		return nil, fmt.Errorf("non support lua number")
	case 3:
		// lua userdate
		return nil, fmt.Errorf("non support lua userdate")
	case 6:
		// lua table
		return nil, fmt.Errorf("non support lua table")
	case 4:
		// short string lt 32
		return pkg.ReadBinary(vLen)
	case 5:
		// long string gte 32
		if vLen == 2 || vLen == 4 {
			bsize, err := pkg.ReadBinary(vLen)
			if err != nil {
				return nil, err
			}
			var size int
			if vLen == 2 {
				size = int(binary.LittleEndian.Uint16(bsize))
			} else {
				size = int(binary.LittleEndian.Uint32(bsize))
			}
			return pkg.ReadBinary(size)
		} else {
			return nil, fmt.Errorf("nonsupport data stream (type=%d,cookie=%d)", vType, vLen)
		}
	default:
		return nil, fmt.Errorf("nonsupport data unpack (type=%d)", vType)
	}
}

func packStrings(strs ...[]byte) ([]byte, error) {
	buffer := bytes.Buffer{}

	for _, s := range strs {
		len := len(s)
		if len < 32 {
			buffer.WriteByte(combineType(4, uint8(len)))
			if len > 0 {
				if _, err := buffer.Write(s); err != nil {
					return nil, err
				}
			}
		} else {
			lengthBytes := []byte{}
			if len < 0x10000 {
				buffer.WriteByte(combineType(5, 2))
				lengthBytes = binary.LittleEndian.AppendUint16(lengthBytes, uint16(len))
			} else {
				buffer.WriteByte(combineType(5, 4))
				lengthBytes = binary.LittleEndian.AppendUint32(lengthBytes, uint32(len))
			}
			buffer.Write(lengthBytes)
			if _, err := buffer.Write(s); err != nil {
				return nil, err
			}
		}
	}
	return buffer.Bytes(), nil
}

func unpackStrings(data []byte) ([]string, error) {
	strs := []string{}
	for {
		datasz := len(data)
		if datasz <= 0 {
			return strs, nil
		}
		header := data[0]
		vType, sz := uncombineType(header)
		if vType == 4 && sz == 0 {
			strs = append(strs, "")
			data = data[1:]
			continue
		}
		vLen := int(sz)
		if datasz < int(vLen+1) {
			return nil, fmt.Errorf("unpackString datasz=%d,vLen=%d", datasz, vLen)
		}

		switch vType {
		case 0:
			// lua nil
			return []string{}, nil
		case 1:
			// boolean
			return nil, fmt.Errorf("non support lua boolean")
		case 2:
			// lua number
			return nil, fmt.Errorf("non support lua number")
		case 3:
			// lua userdate
			return nil, fmt.Errorf("non support lua userdate")
		case 6:
			// lua table
			return nil, fmt.Errorf("non support lua table")
		case 4:
			// short string
			end := vLen + 1
			s := data[1:end]
			strs = append(strs, string(s))
			data = data[end:]
			continue
		case 5:
			// long string
			if vLen == 2 || vLen == 4 {
				headerLen := data[1 : 1+vLen]
				var strLen int
				if vLen == 2 {
					strLen = int(binary.LittleEndian.Uint16(headerLen))
				} else {
					strLen = int(binary.LittleEndian.Uint32(headerLen))
				}
				start := int(1 + vLen)
				end := start + strLen
				s := data[start:end]
				strs = append(strs, string(s))
				data = data[end:]
				continue
			} else {
				return nil, fmt.Errorf("nonsupport data stream (type=%d,cookie=%d)", vType, vLen)
			}
		default:
			return nil, fmt.Errorf("nonsupport data unpack (type=%d)", vType)
		}
	}
}
