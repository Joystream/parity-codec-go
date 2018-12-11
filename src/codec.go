package codec

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
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

func (self ParityEncoder) EncodeAnyInteger(v uint64, size uintptr) {
	buf := make([]byte, size)
	switch size {
	case 2:
		binary.LittleEndian.PutUint16(buf, uint16(v))
	case 4:
		binary.LittleEndian.PutUint32(buf, uint32(v))
	case 8:
		binary.LittleEndian.PutUint64(buf, v)
	}
	self.Write(buf)
}

func (self ParityEncoder) Encode(value interface{}) {
	t := reflect.TypeOf(value)
	switch t.Kind() {
	case reflect.Bool:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Int:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Uintptr:
		fallthrough
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		binary.Write(*self.writer, binary.LittleEndian, value)
	case reflect.Ptr:
		panic("TODO!")
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		rv := reflect.ValueOf(value)
		len := int64(rv.Len())
		if len > math.MaxUint32 {
			panic("Attempted to serialize a collection with too many elements.")
		}
		self.EncodeCompact(uint32(len))
		panic("TODO!")
	case reflect.String:
		panic("TODO!")
	case reflect.Struct:
		panic("TODO!")

	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Interface:
		fallthrough
	case reflect.Map:
		fallthrough
	case reflect.UnsafePointer:
		fallthrough
	case reflect.Invalid:
		panic(fmt.Sprintf("Type %s cannot be encoded", t.Name()))
	}

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
