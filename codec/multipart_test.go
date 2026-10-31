package codec_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/changlongH/srpc/codec"
	payloadcodec "github.com/changlongH/srpc/payload_codec"
	"github.com/cloudwego/netpoll"
)

func testPayload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte((i*17 + i/codec.PartSize) % 251)
	}
	return data
}

func packedString(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := codec.WriteStringToBuf(&buf, data); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Match Skynet's cluster.packresponse, independently of WriteResp.
func responseFrames(data []byte) [][]byte {
	frame := func(kind codec.RetType, body []byte) []byte {
		buf := make([]byte, 5+len(body))
		binary.LittleEndian.PutUint32(buf, 42)
		buf[4] = byte(kind)
		copy(buf[5:], body)
		return buf
	}
	if len(data) <= codec.PartSize {
		return [][]byte{frame(codec.RespTypeOk, data)}
	}
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(data)))
	frames := [][]byte{frame(codec.RespTypeMBegin, length)}
	for len(data) > codec.PartSize {
		frames = append(frames, frame(codec.RespTypeMPart, data[:codec.PartSize]))
		data = data[codec.PartSize:]
	}
	return append(frames, frame(codec.RespTypeMEnd, data))
}

func frameReader(t *testing.T, data []byte) *netpoll.LinkBuffer {
	t.Helper()
	buf := netpoll.NewLinkBuffer()
	if _, err := buf.WriteBinary(data); err != nil {
		t.Fatal(err)
	}
	if err := buf.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf
}

func decodeResponse(t *testing.T, frames [][]byte) (*codec.RespPack, error) {
	t.Helper()
	pending := make(map[uint32]*codec.RespPack)
	var result *codec.RespPack
	for i, frame := range frames {
		resp, err := codec.ReadResp(frameReader(t, frame), pending)
		if err != nil {
			return nil, err
		}
		if i < len(frames)-1 && resp != nil {
			t.Fatal("response completed before the final frame")
		}
		result = resp
	}
	if len(pending) != 0 {
		t.Fatal("completed response left pending state")
	}
	return result, nil
}

func TestResponseFrames(t *testing.T) {
	for _, size := range []int{0, 31, 32, codec.PartSize - 4, codec.PartSize - 3, codec.PartSize - 2, 2*codec.PartSize - 5, 2 * codec.PartSize, 4*codec.PartSize + 17} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			payload := testPayload(size)
			packed := packedString(t, payload)
			frames := responseFrames(packed)

			t.Run("decode_skynet", func(t *testing.T) {
				resp, err := decodeResponse(t, frames)
				if err != nil {
					t.Fatal(err)
				}
				if resp == nil || !resp.Ok || resp.Session != 42 || !bytes.Equal(resp.Payload, payload) {
					t.Fatal("decoded response differs from original")
				}
			})
			t.Run("encode_skynet", func(t *testing.T) {
				buf := netpoll.NewLinkBuffer()
				defer buf.Release()
				if err := codec.WriteResp(buf, &codec.RespPack{Ok: true, Session: 42, Payload: packed}); err != nil {
					t.Fatal(err)
				}
				for i, want := range frames {
					header, err := buf.ReadBinary(2)
					if err != nil {
						t.Fatal(err)
					}
					got, err := buf.ReadBinary(int(binary.BigEndian.Uint16(header)))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, want) {
						t.Fatalf("frame %d differs from Skynet wire format", i)
					}
				}
				if buf.Len() != 0 {
					t.Fatal("unexpected trailing frames")
				}
			})
		})
	}
}

func TestMultipartPayloadCodecs(t *testing.T) {
	type message struct {
		Data []byte
	}
	for name, pc := range map[string]codec.PayloadCodec{"json": payloadcodec.Json{}, "msgpack": payloadcodec.MsgPack{}} {
		t.Run(name, func(t *testing.T) {
			want := message{Data: testPayload(4*codec.PartSize + 17)}
			payload, err := pc.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := decodeResponse(t, responseFrames(packedString(t, payload)))
			if err != nil {
				t.Fatal(err)
			}
			var got message
			if err := pc.Unmarshal(resp.Payload, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("payload codec round trip changed the message")
			}
		})
	}
}

func TestMultipartResponseLength(t *testing.T) {
	packed := packedString(t, testPayload(codec.PartSize))
	for _, delta := range []int{-1, 1} {
		t.Run(fmt.Sprint(delta), func(t *testing.T) {
			frames := responseFrames(packed)
			binary.LittleEndian.PutUint32(frames[0][5:], uint32(len(packed)+delta))
			if _, err := decodeResponse(t, frames); err == nil {
				t.Fatal("accepted a response whose length differs from its header")
			}
		})
	}
}

func TestMultipartRequest(t *testing.T) {
	for _, addr := range []codec.Addr{{Id: 1}, {Name: "@test"}} {
		for _, size := range []int{codec.PartSize, 2 * codec.PartSize, 4*codec.PartSize + 17} {
			t.Run(fmt.Sprintf("%v/%d", addr, size), func(t *testing.T) {
				payload := testPayload(size)
				buf := netpoll.NewLinkBuffer()
				defer buf.Release()
				if err := codec.EncodeReq(buf, &codec.ReqPack{Addr: addr, Session: 42, Method: "Echo", Payload: payload}); err != nil {
					t.Fatal(err)
				}
				if err := buf.Flush(); err != nil {
					t.Fatal(err)
				}
				pending := make(map[uint32]*codec.ReqPack)
				var result *codec.ReqPack
				for buf.Len() > 0 {
					header, err := buf.ReadBinary(2)
					if err != nil {
						t.Fatal(err)
					}
					frame, err := buf.Slice(int(binary.BigEndian.Uint16(header)))
					if err != nil {
						t.Fatal(err)
					}
					result, err = codec.DecodeReq(frame, pending)
					if err != nil {
						t.Fatal(err)
					}
					if result != nil && buf.Len() != 0 {
						t.Fatal("request completed before the final frame")
					}
				}
				if result == nil || result.Session != 42 || result.Addr != addr || result.Method != "Echo" || !bytes.Equal(result.Payload, payload) {
					t.Fatal("decoded request differs from original")
				}
				if len(pending) != 0 {
					t.Fatal("completed request left pending state")
				}
			})
		}
	}
}
