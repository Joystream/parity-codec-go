package codec

import (
	"fmt"
	"io"
)

// Derived from https://github.com/paritytech/parity-codec/blob/master/src/codec.rs

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// ParityEncoder - a wraper around a Writer that allows encoding data items to a stream
type ParityEncoder struct {
	writer *io.Writer
}

func (self ParityEncoder) Write(bytes []byte) {
	c, err := (*self.writer).Write(bytes)
	check(err)
	if c < len(bytes) {
		panic(fmt.Sprintf("Could not write %d bytes to writer", len(bytes)))
	}
}

func (self ParityEncoder) PushByte(b byte) {
	self.Write([]byte{b})
}

func (self ParityEncoder) Encode(value interface{}) {
	panic("TODO!")
	// Use Reflection
	// For primitives and arrays/slices, do standard encoding
	// For pointers, deref the pointer
	// For structs, use ParityEncodeable
	// Otherwise panic
}

func (self ParityEncoder) EncodeCompact(value interface{}) {
	panic("TODO!")
	// Use Reflection
	// For primitives and arrays, do compact encoding
	// For pointers, deref the pointer
	// Otherwise panic
}

// ParityDecoder - a wraper around a Reader that allows decoding data items from a stream
type ParityDecoder struct {
	reader *io.Reader
}

// Returns true only if the whole required number of bytes was read
func (self ParityDecoder) Read(bytes []byte) bool {
	c, err := (*self.reader).Read(bytes)
	check(err)
	return c == len(bytes)
}

func (self ParityEncoder) Decode(value interface{}) {
	panic("TODO!")
	// Use Reflection
	// For primitives and arrays, do standard decoding
	// For pointers, deref the pointer
	// For structs, use ParityDecodeable
	// Otherwise panic
}

func (self ParityEncoder) DecodeCompact(value interface{}) {
	panic("TODO!")
	// Use Reflection
	// For primitives and arrays, do standard decoding
	// For pointers, deref the pointer
	// Otherwise panic
}

type ParityEncodeable interface {
	ParityEncode(encoder ParityEncoder)
}

type ParityDecodeable interface {
	ParityDecode(decoder ParityDecoder)
}
