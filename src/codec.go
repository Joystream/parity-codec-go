package codec

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
)

const MaxUint = ^uint(0)
const MinUint = 0
const MaxInt = int(MaxUint >> 1)
const MinInt = -MaxInt - 1

// Derived from https://github.com/paritytech/parity-codec/blob/master/src/codec.rs

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// ParityEncoder - a wraper around a Writer that allows encoding data items to a stream
type ParityEncoder struct {
	writer io.Writer
}

func (self ParityEncoder) Write(bytes []byte) {
	c, err := self.writer.Write(bytes)
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
	// Currently, Rust implementation only seems to support u128

	if v < 1<<30 {
		if v < 1<<6 {
			self.PushByte(byte(v) << 2)
		} else if v < 1<<14 {
			binary.Write(self.writer, binary.LittleEndian, uint16(v<<2)+1)
		} else {
			binary.Write(self.writer, binary.LittleEndian, uint32(v<<2)+2)
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
		binary.Write(self.writer, binary.LittleEndian, value)
	case reflect.Ptr:
		rv := reflect.ValueOf(value)
		if rv.IsNil() {
			panic("Encoding null pointers not supported; consider using Option type")
		} else {
			dereferenced := rv.Elem()
			self.Encode(dereferenced.Interface())
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
			self.Encode(rv.Index(i).Interface())
		}

	// Strings are encoded as UTF-8 byte slices, just as in Rust
	case reflect.String:
		self.Encode([]byte(value.(string)))

	case reflect.Struct:
		encodeable := reflect.TypeOf((*ParityEncodeable)(nil)).Elem()
		if t.Implements(encodeable) {
			value.(ParityEncodeable).ParityEncode(self)
		} else {
			panic(fmt.Sprintf("Type %s does not support ParityEncodeable interface", t))
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
		panic(fmt.Sprintf("Type %s cannot be encoded", t.Kind()))
	}
}

func (self ParityEncoder) EncodeOption(hasValue bool, value interface{}) {
	if !hasValue {
		self.PushByte(0)
	} else {
		self.PushByte(1)
		self.Encode(value)
	}
}

// ParityDecoder - a wraper around a Reader that allows decoding data items from a stream
type ParityDecoder struct {
	reader io.Reader
}

// Returns true only if the whole required number of bytes was read
func (self ParityDecoder) Read(bytes []byte) bool {
	c, err := self.reader.Read(bytes)
	check(err)
	return c == len(bytes)
}

func (self ParityDecoder) ReadByte() byte {
	buf := []byte{0}
	self.Read(buf)
	return buf[0]
}

// TODO: return error instead of panic?
func (self ParityDecoder) Decode(target interface{}) {
	t0 := reflect.TypeOf(target)
	if t0.Kind() != reflect.Ptr {
		panic("Target must be a non-nil pointer, but was " + fmt.Sprint(t0))
	}
	rv := reflect.ValueOf(target).Elem()
	self.DecodeIntoReflectValue(rv)
}

// TODO: return error instead of panic?
func (self ParityDecoder) DecodeIntoReflectValue(target reflect.Value) {
	t := target.Type()
	if !target.CanSet() {
		panic("Unsettable value " + fmt.Sprint(t))
	}

	switch t.Kind() {

	// Boolean and numbers are trivially decoded via binary.Read
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
		intHolder := reflect.New(t)
		intPointer := intHolder.Interface()
		binary.Read(self.reader, binary.LittleEndian, intPointer)
		target.Set(intHolder.Elem())

	// Pointer in Go is nullable, so we are applying the same rules as there are for
	// Option<T> in Rust implementation
	// TODO(kyegupov): think about it more, is it what the users would expect?
	// Perhaps we should just fail on nils and instead use Option wrapper or EncodeOptional method
	case reflect.Ptr:
		self.DecodeIntoReflectValue(target.Elem())

	// Arrays and slices: first compact-encode length, then each item individually
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		codedLen64 := self.DecodeUintCompact()
		if codedLen64 > math.MaxUint32 {
			panic("Encoded array length is higher than allowed by the protocol (32-bit unsigned integer)")
		}
		if codedLen64 > uint64(MaxInt) {
			panic("Encoded array length is higher than allowed by the platform")
		}
		codedLen := int(codedLen64)
		targetLen := target.Len()
		if codedLen != targetLen {
			if t.Kind() == reflect.Array {
				panic(fmt.Sprintf(
					"We want to decode an array of length %d, but the encoded length is %d",
					target.Len(), codedLen))
			}
			if t.Kind() == reflect.Slice {
				if int(codedLen) > target.Cap() {
					newSlice := reflect.MakeSlice(t, int(codedLen), int(codedLen))
					target.Set(newSlice)
				} else {
					target.SetLen(int(codedLen))
				}
			}
		}
		for i := 0; i < codedLen; i++ {
			self.DecodeIntoReflectValue(target.Index(i))
		}

	// Strings are encoded as UTF-8 byte slices, just as in Rust
	case reflect.String:
		var bytes []byte
		self.Decode(&bytes)
		target.SetString(string(bytes))

	case reflect.Struct:
		encodeable := reflect.TypeOf((*ParityDecodeable)(nil)).Elem()
		ptrType := reflect.PtrTo(t)
		if ptrType.Implements(encodeable) {
			ptrVal := reflect.New(t)
			ptrVal.Interface().(ParityDecodeable).ParityDecode(self)
			target.Set(ptrVal.Elem())
		} else {
			panic(fmt.Sprintf("Type %s does not support ParityDecodeable interface", ptrType))
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
		panic(fmt.Sprintf("Type %s cannot be decoded", t.Kind()))
	}
}

func (self ParityDecoder) DecodeUintCompact() uint64 {
	b := self.ReadByte()
	mode := b & 3
	switch mode {
	case 0:
		return uint64(b >> 2)
	case 1:
		r := uint64(self.ReadByte())
		r <<= 6
		r += uint64(b >> 2)
		return r
	case 2:
		buf := make([]byte, 4)
		buf[0] = b
		self.Read(buf[1:4])
		r := binary.LittleEndian.Uint32(buf)
		r >>= 2
		return uint64(r)
	case 3:
		n := b >> 2
		if n > 4 {
			panic("Not supported: n>4 encountered when decoding a compact-encoded uint")
		}
		buf := make([]byte, 8)
		self.Read(buf[:n+4])
		return binary.LittleEndian.Uint64(buf)
	default:
		panic("Code should be unreachable")
	}
}

func (self ParityDecoder) DecodeOption(hasValue *bool, valuePointer interface{}) {
	b := self.ReadByte()
	switch b {
	case 0:
		*hasValue = false
	case 1:
		*hasValue = true
		self.Decode(valuePointer)
	default:
		panic(fmt.Sprintf("Unknown byte prefix for encoded OptionBool: %d", b))
	}
}

type ParityEncodeable interface {
	// ParityEncode - encode and write this structure into a stream
	ParityEncode(encoder ParityEncoder)
}

type ParityDecodeable interface {
	// ParityDecode - populate this structure from a stream (overwriting the current contents), return false on failure
	ParityDecode(decoder ParityDecoder)
}

type OptionBool struct {
	hasValue bool
	value    bool
}

func NewOptionBoolEmpty() OptionBool {
	return OptionBool{false, false}
}

func NewOptionBool(value bool) OptionBool {
	return OptionBool{true, value}
}

func (self OptionBool) ParityEncode(encoder ParityEncoder) {
	if !self.hasValue {
		encoder.PushByte(0)
	} else {
		if self.value {
			encoder.PushByte(1)
		} else {
			encoder.PushByte(2)
		}
	}
}

func (self *OptionBool) ParityDecode(decoder ParityDecoder) {
	b := decoder.ReadByte()
	switch b {
	case 0:
		self.hasValue = false
		self.value = false
	case 1:
		self.hasValue = true
		self.value = true
	case 2:
		self.hasValue = true
		self.value = false
	default:
		panic(fmt.Sprintf("Unknown byte prefix for encoded OptionBool: %d", b))
	}
}
