package codec

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if reflect.DeepEqual(a, b) {
		return
	}
	// debug.PrintStack()
	t.Errorf("Received %v (type %v), expected %v (type %v)", a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

func hexify(bytes []byte) string {
	res := make([]string, len(bytes))
	for i, b := range bytes {
		res[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(res, " ")
}

func encodeToBytes(value interface{}) []byte {
	var buffer = bytes.Buffer{}
	ParityEncoder{&buffer}.Encode(value)
	return buffer.Bytes()
}

func assertRoundtrip(t *testing.T, value interface{}) {
	var buffer = bytes.Buffer{}
	ParityEncoder{&buffer}.Encode(value)
	target := reflect.New(reflect.TypeOf(value))
	ParityDecoder{&buffer}.Decode(target.Interface())
	assertEqual(t, target.Elem().Interface(), value)
}

func TestSliceOfBytesEncodedAsExpected(t *testing.T) {
	value := []byte{0, 1, 1, 2, 3, 5, 8, 13, 21, 34}
	assertRoundtrip(t, value)
	assertEqual(t, hexify(encodeToBytes(value)), "28 00 01 01 02 03 05 08 0d 15 22")
}

func TestSliceOfInt16EncodedAsExpected(t *testing.T) {
	value := []int16{0, 1, -1, 2, -2, 3, -3}
	assertRoundtrip(t, value)
	assertEqual(t, hexify(encodeToBytes(value)), "1c 00 00 01 00 ff ff 02 00 fe ff 03 00 fd ff")
}

type OptionInt8 struct {
	hasValue bool
	value    int8
}

func (self OptionInt8) ParityEncode(encoder ParityEncoder) {
	encoder.EncodeOption(self.hasValue, self.value)
}

func (self *OptionInt8) ParityDecode(decoder ParityDecoder) {
	decoder.DecodeOption(&self.hasValue, &self.value)
}

func TestSliceOfOptionInt8EncodedAsExpected(t *testing.T) {
	value := []OptionInt8{OptionInt8{true, 1}, OptionInt8{true, -1}, OptionInt8{false, 0}}
	assertRoundtrip(t, value)
	assertEqual(t, hexify(encodeToBytes(value)), "0c 01 01 01 ff 00")
}

func TestSliceOfOptionBoolEncodedAsExpected(t *testing.T) {
	value := []OptionBool{NewOptionBool(true), NewOptionBool(false), NewOptionBoolEmpty()}
	assertRoundtrip(t, value)
	assertEqual(t, hexify(encodeToBytes(value)), "0c 01 02 00")
}

func TestSliceOfStringEncodedAsExpected(t *testing.T) {
	value := []string{
		"Hamlet",
		"Война и мир",
		"三国演义",
		"أَلْف لَيْلَة وَلَيْلَة‎"}
	assertRoundtrip(t, value)
	assertEqual(t, hexify(encodeToBytes(value)), "10 18 48 61 6d 6c 65 74 50 d0 92 d0 be d0 b9 d0 bd d0 b0 20 d0 "+
		"b8 20 d0 bc d0 b8 d1 80 30 e4 b8 89 e5 9b bd e6 bc 94 e4 b9 89 bc d8 a3 d9 8e d9 84 d9 92 "+
		"d9 81 20 d9 84 d9 8e d9 8a d9 92 d9 84 d9 8e d8 a9 20 d9 88 d9 8e d9 84 d9 8e d9 8a d9 92 "+
		"d9 84 d9 8e d8 a9 e2 80 8e")
}

func TestCompactIntegersEncodedAsExpected(t *testing.T) {
	tests := map[uint64]string{
		0:              "00",
		63:             "fc",
		64:             "01 01",
		16383:          "fd ff",
		16384:          "02 00 01 00",
		1073741823:     "fe ff ff ff",
		1073741824:     "03 00 00 00 40",
		1<<32 - 1:      "03 ff ff ff ff",
		1 << 32:        "07 00 00 00 00 01",
		1 << 40:        "0b 00 00 00 00 00 01",
		1 << 48:        "0f 00 00 00 00 00 00 01",
		1<<56 - 1:      "0f ff ff ff ff ff ff ff",
		1 << 56:        "13 00 00 00 00 00 00 00 01",
		math.MaxUint64: "13 ff ff ff ff ff ff ff ff"}
	for value, expectedHex := range tests {
		var buffer = bytes.Buffer{}
		ParityEncoder{&buffer}.EncodeUintCompact(value)
		assertEqual(t, hexify(buffer.Bytes()), expectedHex)
		decoded := ParityDecoder{&buffer}.DecodeUintCompact()
		assertEqual(t, decoded, value)
	}
}