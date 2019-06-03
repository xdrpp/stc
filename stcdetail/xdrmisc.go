package stcdetail

import (
	"bytes"
	"github.com/xdrpp/stc/stx"
	"reflect"
)

type getXdrType struct {
	t stx.XdrType
}
func (*getXdrType) Sprintf(f string, args ...interface{}) string {
	return ""
}
func (gxt *getXdrType) Marshal(name string, i stx.XdrType) {
	gxt.t = i
}

type fakeAggregate struct {
	stx.XdrType
	xdr_fn reflect.Value
}
func (a fakeAggregate) XdrMarshal(x stx.XDR, name string) {
	a.xdr_fn.Call([]reflect.Value{
		reflect.ValueOf(x),
		reflect.ValueOf(name),
		reflect.ValueOf(a.XdrType.XdrPointer()),
	})
}

var _ stx.XdrAggregate = fakeAggregate{}

// Turn any type T with an XDR_T marshaling funcation into an
// XdrAggregate.  Generic functions are easiest to write for
// XdrAggregate instances, which have an XdrMarshal method.  If you
// want to run such a generic function on a type T that is not an
// instance of XdrAggregate, you can turn a variable t of type T into
// aggregate by running:
//
//     MakeAggregate(XDR_T, &t)
func MakeAggregate(xdr_fn interface{}, t interface{}) stx.XdrAggregate {
	xfv := reflect.ValueOf(xdr_fn)
	tv := reflect.ValueOf(t)
	var gxt getXdrType
	xfv.Call([]reflect.Value{reflect.ValueOf(&gxt), reflect.ValueOf(""), tv})
	return fakeAggregate { gxt.t, xfv }
}

// Marshal an XDR aggregate to the raw binary bytes defined in
// RFC4506.
func XdrToBin(t stx.XdrAggregate) []byte {
	out := bytes.Buffer{}
	t.XdrMarshal(&stx.XdrOut{&out}, "")
	return out.Bytes()
}

// Unmarshal an XDR aggregate from the raw binary bytes defined in
// RFC4506.
func XdrFromBin(t stx.XdrAggregate, input []byte) (err error) {
	defer func() {
		if i := recover(); i != nil {
			if xe, ok := i.(stx.XdrError); ok {
				err = xe
				return
			}
			panic(i)
		}
	}()
	in := bytes.NewReader(input)
	t.XdrMarshal(&stx.XdrIn{in}, "")
	return
}

type forEachXdr struct {
	fn func(stx.XdrType) bool
}
func (fex forEachXdr) Marshal(_ string, val stx.XdrType) {
	if !fex.fn(val) {
		if xa, ok := val.(stx.XdrAggregate); ok {
			xa.XdrMarshal(fex, "")
		}
	}
}
func (forEachXdr) Sprintf(string, ...interface{}) string {
	return ""
}

// Calls fn, recursively, on every value inside an XdrAggregate.
// Prunes the recursion if fn returns true.
func ForEachXdr(t stx.XdrAggregate, fn func(stx.XdrType) bool) {
	t.XdrMarshal(forEachXdr{fn}, "")
}

// If out is of type **T, then *out is set to point to the first
// instance of T found when traversing t.
func XdrExtract(t stx.XdrAggregate, out interface{}) (ret bool) {
	vo := reflect.ValueOf(out).Elem()
	to := vo.Type()
	ForEachXdr(t, func(t stx.XdrType) bool {
		tp := t.XdrPointer()
		if tp == nil {
			return false
		}
		v := reflect.ValueOf(t.XdrPointer())
		if v.Type() != to {
			return false
		}
		vo.Set(v)
		ret = true
		return true
	})
	return
}
