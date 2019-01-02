// Copyright 2018 Jsgenesis
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package noreflect

// Adapted from https://github.com/Joystream/parity-codec-go
import (
	"bytes"
	"encoding/binary"
	"io"
	"strconv"
)

// Implementation for Parity codec in Tinygo.
// Derived from https://github.com/paritytech/parity-codec/

const maxUint = ^uint(0)
const maxInt = int(maxUint >> 1)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// Encoder is a wrapper around a Writer that allows encoding data items to a stream.
type Encoder struct {
	Writer io.Writer
}

// Write several bytes to the encoder.
func (pe Encoder) Write(bytes []byte) {
	c, err := pe.Writer.Write(bytes)
	check(err)
	if c < len(bytes) {
		panic("Could not write " + strconv.Itoa(len(bytes)) + " bytes to writer")
	}
}

// PushByte writes a single byte to an encoder.
func (pe Encoder) PushByte(b byte) {
	pe.Write([]byte{b})
}

// EncodeUintCompact writes an unsigned integer to the stream using the compact encoding.
// A typical usage is storing the length of a collection.
// Definition of compact encoding:
// 0b00 00 00 00 / 00 00 00 00 / 00 00 00 00 / 00 00 00 00
//   xx xx xx 00															(0 ... 2**6 - 1)		(u8)
//   yL yL yL 01 / yH yH yH yL												(2**6 ... 2**14 - 1)	(u8, u16)  low LH high
//   zL zL zL 10 / zM zM zM zL / zM zM zM zM / zH zH zH zM					(2**14 ... 2**30 - 1)	(u16, u32)  low LMMH high
//   nn nn nn 11 [ / zz zz zz zz ]{4 + n}									(2**30 ... 2**536 - 1)	(u32, u64, u128, U256, U512, U520) straight LE-encoded
// Rust implementation: see impl<'a> Encode for CompactRef<'a, u64>
func (pe Encoder) EncodeUintCompact(v uint64) {

	// TODO: handle numbers wide than 64 bits (byte slices?)
	// Currently, Rust implementation only seems to support u128

	buf := make([]byte, 8)

	if v < 1<<30 {
		if v < 1<<6 {
			pe.PushByte(byte(v) << 2)
		} else if v < 1<<14 {
			binary.LittleEndian.PutUint16(buf, uint16(v<<2)+1)
			pe.Write(buf[:2])
		} else {
			binary.LittleEndian.PutUint32(buf, uint32(v<<2)+2)
			pe.Write(buf[:4])
		}
		return
	}

	n := byte(0)
	limit := uint64(1 << 32)
	for v >= limit && limit > 256 { // when overflows, limit will be < 256
		n++
		limit <<= 8
	}
	if n > 4 {
		panic("Assertion error: n>4 needed to compact-encode uint64")
	}
	pe.PushByte((n << 2) + 3)
	binary.LittleEndian.PutUint64(buf, v)
	pe.Write(buf[:4+n])
}

// TODO: codegeneration

func (pe Encoder) EncodeUint(value uint64, bytes int) {
	mask := uint64(255)
	for i := 0; i < bytes; i++ {
		pe.PushByte(byte(value & mask))
		value = value >> 8
	}
}

func (pe Encoder) EncodeInt(value int64, bytes int) {
	mask := int64(255)
	for i := 0; i < bytes; i++ {
		pe.PushByte(byte(value & mask))
		value = value >> 8
	}
}

func (pe Encoder) EncodeBool(b bool) {
	if b {
		pe.PushByte(1)
	} else {
		pe.PushByte(0)
	}
}

func (pe Encoder) EncodeByteSlice(value []byte) {
	pe.EncodeUintCompact(uint64(len(value)))
	pe.Write(value)
}

func (pe Encoder) EncodeString(value string) {
	pe.EncodeByteSlice([]byte(value))
}

// EncodeOption stores optionally present value to the stream.
func (pe Encoder) EncodeOption(hasValue bool, value Encodeable) {
	if !hasValue {
		pe.PushByte(0)
	} else {
		pe.PushByte(1)
		value.ParityEncode(pe)
	}
}

// Decoder - a wraper around a Reader that allows decoding data items from a stream.
// Unlike Rust implementations, decoder methods do not return success state, but just
// panic on error. Since decoding failue is an "unexpected" error, this approach should
// be justified.
type Decoder struct {
	Reader io.Reader
}

// Read reads bytes from a stream into a buffer and panics if cannot read the required
// number of bytes.
func (pd Decoder) Read(bytes []byte) {
	c, err := pd.Reader.Read(bytes)
	check(err)
	if c < len(bytes) {
		panic("Cannot read the required number of bytes " + strconv.Itoa(len(bytes)) + ", only " + strconv.Itoa(c) + " available")
	}
}

// ReadOneByte reads a next byte from the stream.
// Named so to avoid a linter warning about a clash with io.ByteReader.ReadByte
func (pd Decoder) ReadOneByte() byte {
	buf := []byte{0}
	pd.Read(buf)
	return buf[0]
}

func (pd Decoder) DecodeUint(bytes uint) uint64 {
	var value uint64
	for i := uint(0); i < bytes; i++ {
		b := uint64(pd.ReadOneByte())
		value |= b << (i * 8)
	}
	return value
}

func (pd Decoder) DecodeInt(bytes uint) int64 {
	var value uint64
	var b uint64
	for i := uint(0); i < bytes; i++ {
		b = uint64(pd.ReadOneByte())
		value |= b << (i * 8)
	}
	if b >= 128 {
		for i := bytes; i < 8; i++ {
			value |= b << 255
		}
	}
	return int64(value)
}

func (pd Decoder) DecodeBool() bool {
	return pd.ReadOneByte() > 0
}

// DecodeUintCompact decodes a compact-encoded integer. See EncodeUintCompact method.
func (pd Decoder) DecodeUintCompact() uint64 {
	b := pd.ReadOneByte()
	mode := b & 3
	switch mode {
	case 0:
		return uint64(b >> 2)
	case 1:
		r := uint64(pd.ReadOneByte())
		r <<= 6
		r += uint64(b >> 2)
		return r
	case 2:
		buf := make([]byte, 4)
		buf[0] = b
		pd.Read(buf[1:4])
		r := binary.LittleEndian.Uint32(buf)
		r >>= 2
		return uint64(r)
	case 3:
		n := b >> 2
		if n > 4 {
			panic("Not supported: n>4 encountered when decoding a compact-encoded uint")
		}
		buf := make([]byte, 8)
		pd.Read(buf[:n+4])
		return binary.LittleEndian.Uint64(buf)
	default:
		panic("Code should be unreachable")
	}
}

func (pd Decoder) DecodeByteSlice() []byte {
	value := make([]byte, pd.DecodeUintCompact())
	pd.Read(value)
	return value
}

func (pd Decoder) DecodeString() string {
	return string(pd.DecodeByteSlice())
}

// DecodeOption decodes a optionally available value into a boolean presence field and a value.
func (pd Decoder) DecodeOption(hasValue *bool, valuePointer Decodeable) {
	b := pd.ReadOneByte()
	switch b {
	case 0:
		*hasValue = false
	case 1:
		*hasValue = true
		valuePointer.ParityDecode(pd)
	default:
		panic("Unknown byte prefix for encoded Option: " + strconv.Itoa(int(b)))
	}
}

// Encodeable is an interface that defines a custom encoding rules for a data type.
// Should be defined for structs (not pointers to them).
// See OptionBool for an example implementation.
type Encodeable interface {
	// ParityEncode encodes and write this structure into a stream
	ParityEncode(encoder Encoder)
}

// See Int16Slice in tests as an example
type EncodeableSlice interface {
	Len() int
	ParityEncodeElement(index int, encoder Encoder)
}

func (pe Encoder) EncodeSlice(s EncodeableSlice) {
	len := s.Len()
	pe.EncodeUintCompact(uint64(len))
	for i := 0; i < len; i++ {
		s.ParityEncodeElement(i, pe)
	}
}

// Decodeable is an interface that defines a custom encoding rules for a data type.
// Should be defined for pointers to structs.
// See OptionBool for an example implementation.
type Decodeable interface {
	// ParityDecode populates this structure from a stream (overwriting the current contents), return false on failure
	ParityDecode(decoder Decoder)
}

type DecodeableSlice interface {
	Make(size int)
	ParityDecodeElement(index int, decoder Decoder)
}

func (pd Decoder) DecodeSlice(s DecodeableSlice) {
	l := int(pd.DecodeUintCompact())
	s.Make(l)
	for i := 0; i < l; i++ {
		s.ParityDecodeElement(i, pd)
	}
}

// OptionBool is a structure that can store a boolean or a missing value.
// Note that encoding rules are slightly different from other "Option" fields.
type OptionBool struct {
	hasValue bool
	value    bool
}

// NewOptionBoolEmpty creates an OptionBool without a value.
func NewOptionBoolEmpty() OptionBool {
	return OptionBool{false, false}
}

// NewOptionBool creates an OptionBool with a value.
func NewOptionBool(value bool) OptionBool {
	return OptionBool{true, value}
}

// ParityEncode implements encoding for OptionBool as per Rust implementation.
func (o OptionBool) ParityEncode(encoder Encoder) {
	if !o.hasValue {
		encoder.PushByte(0)
	} else {
		if o.value {
			encoder.PushByte(1)
		} else {
			encoder.PushByte(2)
		}
	}
}

// ParityDecode implements decoding for OptionBool as per Rust implementation.
func (o *OptionBool) ParityDecode(decoder Decoder) {
	b := decoder.ReadOneByte()
	switch b {
	case 0:
		o.hasValue = false
		o.value = false
	case 1:
		o.hasValue = true
		o.value = true
	case 2:
		o.hasValue = true
		o.value = false
	default:
		panic("Unknown byte prefix for encoded OptionBool: " + strconv.Itoa(int(b)))
	}
}

func Encode(value Encodeable) []byte {
	var buffer = bytes.Buffer{}
	value.ParityEncode(Encoder{&buffer})
	return buffer.Bytes()
}

func Decode(value Decodeable, encoded []byte) {
	var buffer = bytes.NewBuffer(encoded)
	value.ParityDecode(Decoder{buffer})
}
