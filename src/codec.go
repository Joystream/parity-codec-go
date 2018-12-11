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

func (self ParityEncoder) EncodeInteger(v uint64, size uintptr) {
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

// compact encoding:
// 0b00 00 00 00 / 00 00 00 00 / 00 00 00 00 / 00 00 00 00
//   xx xx xx 00															(0 ... 2**6 - 1)		(u8)
//   yL yL yL 01 / yH yH yH yL												(2**6 ... 2**14 - 1)	(u8, u16)  low LH high
//   zL zL zL 10 / zM zM zM zL / zM zM zM zM / zH zH zH zM					(2**14 ... 2**30 - 1)	(u16, u32)  low LMMH high
//   nn nn nn 11 [ / zz zz zz zz ]{4 + n}									(2**30 ... 2**536 - 1)	(u32, u64, u128, U256, U512, U520) straight LE-encoded

// Rust implementation: see impl<'a> Encode for CompactRef<'a, u64>

func (self ParityEncoder) EncodeUintCompact(v uint64) {

	// TODO: handle numbers wide than 64 bits (byte slices?)

	if v < 1<<30 {
		if v < 1<<6 {
			self.PushByte(byte(v) << 2)
		} else if v < 1<<14 {
			binary.Write(*self.writer, binary.LittleEndian, uint16(v<<2)+1)
		} else {
			binary.Write(*self.writer, binary.LittleEndian, uint32(v<<2)+2)
		}
	}

	n := byte(0)
	limit := uint64(1 << 38)
	for v > limit && limit > 256 { // when overflows, limit will be < 256
		n++
		limit <<= 8
	}
	if n > 4 {
		panic("Assertion error: n>4 needed to compact-encode uint64")
	}
	self.PushByte((n << 2) + 3)
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	self.Write(buf[:4+n])
}

func (self ParityEncoder) Encode(value interface{}) {
	t := reflect.TypeOf(value)
	switch t.Kind() {

	// Boolean and numbers are trivially encoded via binary.Write
	// It will use reflection again and take a performance hit
	// TODO: consider handling every case directly
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

	// Pointer in Go is nullable, so we are applying the same rules as there are for
	// Option<T> in Rust implementation
	case reflect.Ptr:
		t2 := t.Elem()
		rv := reflect.ValueOf(value)
		if rv.IsNil() {
			self.PushByte(0)
		} else {
			dereferenced := reflect.Indirect(rv)
			if t2.Kind() == reflect.Bool {
				// See OptionBool in Rust implementation
				if dereferenced.Bool() {
					self.PushByte(2)
				} else {
					self.PushByte(1)
				}
			} else {
				self.Encode(dereferenced.Interface())
			}
		}

	// Arrays and slices: first compact-encode length, then each item individually
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		rv := reflect.ValueOf(value)
		len := rv.Len()
		len64 := uint64(len)
		if len64 > math.MaxUint32 {
			panic("Attempted to serialize a collection with too many elements.")
		}
		self.EncodeUintCompact(len64)
		for i := 0; i < len; i++ {
			self.Encode(rv.Index(i))
		}

	// Strings are encoded as UTF-8 byte slices, just as in Rust
	case reflect.String:
		self.Encode([]byte(value.(string)))

	case reflect.Struct:
		encodeable := reflect.TypeOf((*ParityEncodeable)(nil)).Elem()
		if t.Implements(encodeable) {
			value.(ParityEncodeable).ParityEncode(self)
		}

	// Currently unsupported types
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
	// ParityEncode - encode and write this structure into a stream
	ParityEncode(encoder ParityEncoder)
	// ParityDecode - populate this structure from a stream (overwriting the current contents), return false on failure
	ParityDecode(decoder ParityDecoder) bool
}
