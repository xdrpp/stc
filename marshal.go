package main

import "encoding/hex"
import "fmt"
import "io"
import "reflect"

type XdrEnumer interface {
	fmt.Stringer
	XdrEnumInt() *int32
	XdrEnumNames() map[int32]string
	XdrEnumValue() interface{}
}

type XdrStringer interface {
	fmt.Stringer
	XdrBound() uint32
	XdrValid() bool
	Set(string)
	XdrString() *string
}

type XdrOpaquer interface {
	XdrBound() uint32
	XdrValid() bool
	XdrOpaque() *[]byte
}

type XdrUnioner interface {
	XdrUnionTag() interface{}
	XdrUnionTagName() string
	XdrValid() bool
	XdrUnionBody() interface{}
	XdrUnionBodyName() string
}

func deref(p interface{}) interface{} {
	return reflect.ValueOf(p).Elem().Interface()
}

func dot(n1, n2 string) string {
	if n1 == "" {
		return n2
	}
	return n1 + "." + n2
}

func XdrPrint(out io.Writer, name string, v interface{}) {
	switch r := v.(type) {
	case XdrStringer:
		fmt.Fprintf(out, "%s: %q\n", name, r)
	case fmt.Stringer:
		fmt.Fprintf(out, "%s: %v\n", name, r)
	case *bool, *int32, *uint32, *int64, *uint64, *float32, *float64:
		fmt.Fprintf(out, "%s: %v\n", name, deref(r))
	case XdrOpaquer:
		fmt.Fprintf(out, "%s: %s\n", name, hex.EncodeToString(*r.XdrOpaque()))
	case XdrUnioner:
		XdrPrint(out, dot(name, r.XdrUnionTagName()), r.XdrUnionTag())
		b := r.XdrUnionBody()
		if b != nil {
			XdrPrint(out, dot(name, r.XdrUnionBodyName()), r.XdrUnionBody())
		}
	default:
		goto tryReflect
	}
	return
tryReflect:
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("XdrPrint: %T is not pointer", v))
	}
	val = val.Elem()
	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			fmt.Fprintf(out, "%s: nil\n", name)
		} else {
			XdrPrint(out, name, val.Interface())
		}
	case reflect.Array:
		if val.Type().Elem().Kind() == reflect.Uint8 {
			fmt.Fprintf(out, "%s: %s\n", name,
				hex.EncodeToString(val.Slice(0, val.Len()).Bytes()))
			break
		}
		fallthrough
	case reflect.Slice:
		n := val.Len()
		for i := 0; i < n; i++ {
			XdrPrint(out, fmt.Sprintf("%s[%d]", name, i),
				val.Index(i).Addr().Interface())
		}
	case reflect.Struct:
		n := val.NumField()
		t := val.Type()
		for i := 0; i < n; i++ {
			XdrPrint(out, dot(name, t.Field(i).Name),
				val.Field(i).Addr().Interface())
		}
	default:
		panic(fmt.Sprintf("XdrPrint: cannot print type %T", deref(v)))
	}
}

/*

import "math"
import "io"

type XdrBadValue string
func (v XdrBadValue) Error() string { return string(v) }

const (
	TRUE = true
	FALSE = false
)

type XdrVoid = struct{}

type XdrNum32 interface {
	GetU32() uint32
	SetU32(uint32)
	Pointer() interface{}
	Value() interface{}
}

type XdrBool bool
func (v *XdrBool) GetU32() uint32 {
	if *v {
		return 1
	}
	return 0
}
func (v *XdrBool) SetU32(nv uint32) {
	switch nv {
	case 0:
		*v = false
	case 1:
		*v = true
	}
	panic(XdrBadValue("Bool must be 0 or 1"))
}
func (v *XdrBool) Pointer() interface{} { return (*bool)(v) }
func (v *XdrBool) Value() interface{} { return bool(*v) }

type XdrInt32 int32
func (v *XdrInt32) GetU32() uint32 { return uint32(*v) }
func (v *XdrInt32) SetU32(nv uint32) { *v = XdrInt32(nv) }
func (v *XdrInt32) Pointer() interface{} { return (*int32)(v) }
func (v *XdrInt32) Value() interface{} { return int32(*v) }

type XdrUint32 uint32
func (v *XdrUint32) GetU32() uint32 { return uint32(*v) }
func (v *XdrUint32) SetU32(nv uint32) { *v = XdrUint32(nv) }
func (v *XdrUint32) Pointer() interface{} { return (*uint32)(v) }
func (v *XdrUint32) Value() interface{} { return uint32(*v) }

type XdrFloat32 float32
func (v *XdrFloat32) GetU32() uint32 { return math.Float32bits(float32(*v)) }
func (v *XdrFloat32) SetU32(nv uint32) {
	*v = XdrFloat32(math.Float32frombits(nv))
}
func (v *XdrFloat32) Pointer() interface{} { return (*float32)(v) }
func (v *XdrFloat32) Value() interface{} { return float32(*v) }

type XdrNum64 interface {
	GetU64() uint64
	SetU64(uint64)
	Pointer() interface{}
	Value() interface{}
}

type XdrInt64 int64
func (v *XdrInt64) GetU64() uint64 { return uint64(*v) }
func (v *XdrInt64) SetU64(nv uint64) { *v = XdrInt64(nv) }
func (v *XdrInt64) Pointer() interface{} { return (*int64)(v) }
func (v *XdrInt64) Value() interface{} { return int64(*v) }

type XdrUint64 uint64
func (v *XdrUint64) GetU64() uint64 { return uint64(*v) }
func (v *XdrUint64) SetU64(nv uint64) { *v = XdrUint64(nv) }
func (v *XdrUint64) Pointer() interface{} { return (*uint64)(v) }
func (v *XdrUint64) Value() interface{} { return uint64(*v) }

type XdrFloat64 float64
func (v *XdrFloat64) GetU64() uint64 { return math.Float64bits(float64(*v)) }
func (v *XdrFloat64) SetU64(nv uint64) {
	*v = XdrFloat64(math.Float64frombits(nv))
}
func (v *XdrFloat64) Pointer() interface{} { return (*float64)(v) }
func (v *XdrFloat64) Value() interface{} { return float64(*v) }

type XdrEnum interface {
	EnumNames() map[int32]string
	EnumVal() *int32
	String() string
	Value() interface{}
}

type XDR interface {
	num32(v XdrNum32, name string)
	num64(v XdrNum64, name string)
	enum(v XdrEnum, name string)
	bytes(v *[]byte, maxSize uint32, name string)
	str(v *string, maxSize uint32, name string)
	size(v *uint32, maxSize uint32, name string)
}

type XdrPrint struct {
	Out io.Writer
}

func (x *XdrPrint) num32(v XdrNum32, name string) {
	fmt.Fprintf(x.Out, "%s: %v\n", name, v.Value())
}
func (x *XdrPrint) num64(v XdrNum64, name string) {
	fmt.Fprintf(x.Out, "%s: %v\n", name, v.Value())
}
func (x *XdrPrint) enum(v XdrEnum, name string) {
	fmt.Fprintf(x.Out, "%s: %v\n", name, v.Value())
}
func (x *XdrPrint) bytes(v *[]byte, maxSize uint32, name string) {
	fmt.Fprintf(x.Out, "%s: %v\n", name, *v)
}
func (x *XdrPrint) str(v *string, maxSize uint32, name string) {
	fmt.Fprintf(x.Out, "%s: %%v\n", name, *v)
}
func (x *XdrPrint) size(v *uint32, maxSize uint32, name string) {}




import "encoding/binary"
import "io"
import "math"
import "reflect"

var enc binary.ByteOrder = binary.BigEndian
var zerofill [4][]byte = [...][]byte{
	{}, {0,0,0}, {0,0}, {0},
}

func put32(out io.Writer, val uint32) {
	b := make([]byte, 4)
	enc.PutUint32(b, val)
	out.Write(b)
}

func putint(out io.Writer, val int) {
	if uint64(val) > 0xffffffff {
		panic("XDR marshal putint overflow")
	}
	put32(out, uint32(val))
}

func put64(out io.Writer, val uint64) {
	b := make([]byte, 4)
	enc.PutUint64(b, val)
	out.Write(b)
}

type Saver interface {
	Save(out io.Writer)
}

func reflect_save(out io.Writer, val reflect.Value) {
	switch val.Kind() {
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			f := val.Field(i)
		}
	}
}

func Save(out io.Writer, p interface{}) {
	switch v := p.(type) {
	case *bool:
		if *v {
			putint(out, 1)
		} else {
			putint(out, 0)
		}
	case *int32:
		put32(out, uint32(*v))
	case *uint32:
		put32(out, *v)
	case *int64:
		put64(out, uint64(*v))
	case *uint64:
		put64(out, *v)
	case *float32:
		put32(out, math.Float32bits(*v))
	case *float64:
		put64(out, math.Float64bits(*v))
	case *string:
		putint(out, len(*v))
		out.Write([]byte(*v))
		out.Write(zerofill[len(*v)&3])
	case *[]byte:
		putint(out, len(*v))
		out.Write(*v)
		out.Write(zerofill[len(*v)&3])
	case Saver:
		v.Save(out)
	default:
		val := reflect.ValueOf(v)
		if val.Kind() != reflect.Ptr {
			panic("Save requires pointer")
		}
		reflect_save(out, val.Elem())
	}
}
*/
