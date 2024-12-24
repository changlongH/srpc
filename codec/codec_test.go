package codec

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateRandomString(length int) string {
	result := make([]byte, length)
	_, err := rand.Read(result)
	if err != nil {
		panic(err)
	}
	mod := byte(len(letterBytes))
	for i, b := range result {
		result[i] = letterBytes[b%mod]
	}
	return string(result)
}

func assertCheckPackRet(rawMsg string, packedMsg string, headerSize int) bool {
	rawLen := len(rawMsg)
	packedLen := len(packedMsg)
	expectPackLen := rawLen + headerSize
	if expectPackLen != packedLen {
		panic(fmt.Sprintf("assertCheckPackRet rawLen=%d,packedLen=%d,expectHeaderSize=%d", rawLen, packedLen, headerSize))
	}

	return true
}

func assertCheckUnpack(packedMsg string, rawMsgs []string) {
	if ret, err := unpackStrings([]byte(packedMsg)); err != nil {
		panic(err.Error())
	} else {
		if !reflect.DeepEqual(ret, rawMsgs) {
			panic("unpack not equal")
		}
	}
}

func TestSeri(t *testing.T) {
	// zero
	msg := ""
	packedMsg, err := packStrings([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}
	assertCheckPackRet(msg, string(packedMsg), 1)
	assertCheckUnpack(string(packedMsg), []string{msg})

	// 31
	msg = generateRandomString(31)
	packedMsg, err = packStrings([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}
	assertCheckPackRet(msg, string(packedMsg), 1)
	assertCheckUnpack(string(packedMsg), []string{msg})

	// 32
	msg = generateRandomString(32)
	packedMsg, err = packStrings([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}
	assertCheckPackRet(msg, string(packedMsg), 3)
	assertCheckUnpack(string(packedMsg), []string{msg})

	// 0x10000-1
	msg = generateRandomString(0x10000 - 1)
	packedMsg, err = packStrings([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}
	assertCheckPackRet(msg, string(packedMsg), 3)
	assertCheckUnpack(string(packedMsg), []string{msg})

	// 0x10000
	msg = generateRandomString(0x10000)
	packedMsg, err = packStrings([]byte(msg))
	if err != nil {
		t.Error(err)
		return
	}
	assertCheckPackRet(msg, string(packedMsg), 5)
	assertCheckUnpack(string(packedMsg), []string{msg})

	// multi str
	msg1, msg2, msg3 := "", generateRandomString(32), generateRandomString(0x10000)
	packedMsg, err = packStrings([]byte(msg1), []byte(msg2), []byte(msg3))
	if err != nil {
		t.Error(err)
		return
	}
	assertCheckPackRet(msg1+msg2+msg3, string(packedMsg), 1+3+5)
	assertCheckUnpack(string(packedMsg), []string{msg1, msg2, msg3})
}

func TestCombineType(t *testing.T) {
	header := combineType(5, 4)
	tp, v := uncombineType(header)
	if int(tp) != 5 || int(v) != 4 {
		panic("combine test faile")
	}

	header = combineType(5, 2)
	tp, v = uncombineType(header)
	if int(tp) != 5 || int(v) != 2 {
		panic("combine test faile")
	}

	header = combineType(4, 0)
	tp, v = uncombineType(header)
	if int(tp) != 4 || int(v) != 0 {
		panic("combine test faile")
	}

	header = combineType(4, 31)
	tp, v = uncombineType(header)
	if int(tp) != 4 || int(v) != 31 {
		panic("combine test faile")
	}

	header = combineType(4, 32)
	tp, v = uncombineType(header)
	if int(tp) != 4 || int(v) != 0 {
		panic("combine test faile")
	}
}

func TestReqAndResp(t *testing.T) {
	cmd := generateRandomString(31)
	message := []byte(generateRandomString(32))
	payload, err := PackPayload([]byte(cmd), message)
	if err != nil {
		t.Error(err)
		return
	}
	rawPack := &ReqPack{
		Addr:    Addr{Id: 0, Name: "sdb"},
		Session: 1,
		Payload: payload,
	}

	var decPack = rawPack
	if !reflect.DeepEqual(rawPack, decPack) {
		t.Error("not equal")
		return
	}
}
