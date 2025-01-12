package payloadcodec

import (
	"bytes"
	"encoding/json"
)

type Json struct{}

func (c Json) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (c Json) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

var nullBytes = []byte("null")

func (c Json) IsNull(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if len(data) == 4 && bytes.Equal(data, nullBytes) {
		return true
	}
	return false
}

func (c Json) Name() string {
	return "json"
}
