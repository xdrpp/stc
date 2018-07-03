package main

import "fmt"
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
