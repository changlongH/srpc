package codec

import (
	"bytes"
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

func TestSeriString(t *testing.T) {
	str31 := generateRandomString(31)
	str32 := generateRandomString(32)
	strShort := generateRandomString(0x10000 - 1)
	strLong := generateRandomString(0x10000)

	buf := &bytes.Buffer{}
	var rawStrs = []string{"", str31, str32, strShort, strLong}
	err := writeStringsToBuf(buf, []byte(""), []byte(str31), []byte(str32), []byte(strShort), []byte(strLong))
	if err != nil {
		t.Error(err)
		return
	}

	strs, err := decodeStrings(buf.Bytes())
	if err != nil {
		t.Error(err)
		return
	}
	for i, str := range strs {
		rawStr := rawStrs[i]
		if !reflect.DeepEqual(rawStr, str) {
			t.Errorf("unpack not equal got=%s,expect=%s", str, rawStr)
			return
		}
	}
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

func TestPayloadCodec(t *testing.T) {
	type Args struct {
		Key string `json:"key" msgpack:"key"`
		Val string `json:"val" msgpack:"val"`
	}
	args := Args{Key: "foo", Val: "foobar"}
	var c PayloadCodec
	var ok bool
	var err error
	for _, name := range []string{"json", "msgpack"} {
		if c, ok = GetPayloadCodec(name); !ok {
			continue
		}
		var out []byte
		if out, err = c.Marshal(args); err != nil {
			t.Error(err.Error())
			return
		}
		var v = Args{}
		if err = c.Unmarshal(out, &v); err != nil {
			t.Error(err.Error())
			return
		}
		fmt.Println(v, args)
		if !reflect.DeepEqual(v, args) {
			t.Error("not equal codec " + name)
			return
		}
	}

	msg := []byte("foobar")
	if c, ok = GetPayloadCodec("text"); ok {
		out, err := c.Marshal(&msg)
		if err != nil {
			t.Error(err)
			return
		}
		var v []byte
		if err = c.Unmarshal(out, &v); err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(v, msg) {
			t.Errorf("text: raw=%v,dec=%v", msg, v)
		}
	}
}
