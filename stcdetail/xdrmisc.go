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
}
func (a fakeAggregate) XdrMarshal(x stx.XDR, name string) {
	x.Marshal(name, a.XdrType)
}

var _ stx.XdrAggregate = fakeAggregate{}

var xdr_type reflect.Type = reflect.TypeOf((*stx.XDR)(nil)).Elem()

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
	if a, ok := gxt.t.(stx.XdrAggregate); ok {
		return a
	}
	return fakeAggregate { gxt.t }
}

// Marshal an XDR aggregate to the raw binary bytes defined in
// RFC4506.
func XdrToBin(t stx.XdrAggregate) []byte {
	out := bytes.Buffer{}
	t.XdrMarshal(&stx.XdrOut{&out}, "")
	return out.Bytes()
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

