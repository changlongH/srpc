package codec

import (
	"bytes"
	"crypto/rand"
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
