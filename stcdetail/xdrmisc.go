package stcdetail

import (
	"github.com/xdrpp/stc/stx"
	"reflect"
	"strings"
)

type trivSprintf struct{}
func (trivSprintf) Sprintf(f string, args ...interface{}) string {
	return ""
}

// Marshal an XDR type to the raw binary bytes defined in RFC4506.
// The return value is not UTF-8.
func XdrToBin(t stx.XdrType) string {
	out := strings.Builder{}
	t.XdrMarshal(&stx.XdrOut{&out}, "")
	return out.String()
}

// Unmarshal an XDR type from the raw binary bytes defined in RFC4506.
func XdrFromBin(t stx.XdrType, input string) (err error) {
	defer func() {
		if i := recover(); i != nil {
			if xe, ok := i.(stx.XdrError); ok {
				err = xe
				return
			}
			panic(i)
		}
	}()
	in := strings.NewReader(input)
	t.XdrMarshal(&stx.XdrIn{in}, "")
	return
}

type forEachXdr struct {
	fn func(stx.XdrType) bool
	trivSprintf
}
func (fex forEachXdr) Marshal(_ string, val stx.XdrType) {
	if !fex.fn(val) {
		if xa, ok := val.(stx.XdrAggregate); ok {
			xa.XdrRecurse(fex, "")
		}
	}
}

// Calls fn, recursively, on every value inside an XdrAggregate.
// Prunes the recursion if fn returns true.
func ForEachXdr(t stx.XdrType, fn func(stx.XdrType) bool) {
	t.XdrMarshal(forEachXdr{fn: fn}, "")
}

// Calls fn on each instance of a type encountered while traversing a
// data structure.  fn should be of type func(*T) or func(*T)bool
// where T is an XDR structure.  By default, the traversal does not
// recurse into T.  In the case that T is part of a linked list (or
// otherwise contains a pointer to T internally), if the function
// returns false then fields within T will continue to be examined
// recursively.
func ForEachXdrType(a stx.XdrType, fn interface{}) {
	fnv := reflect.ValueOf(fn)
	fnt := fnv.Type()
	if fnt.Kind() != reflect.Func || fnt.NumIn() != 1 || fnt.NumOut() > 1 ||
		(fnt.NumOut() == 1 && fnt.Out(0).Kind() != reflect.Bool) {
		panic("ForEachXdrType: invalid function")
	}
	argt := fnt.In(0)
	argv := reflect.New(argt).Elem()
	ForEachXdr(a, func(t stx.XdrType) bool {
		if reflect.TypeOf(t).AssignableTo(argt) {
			argv.Set(reflect.ValueOf(t))
			res := fnv.Call([]reflect.Value{argv})
			if len(res) == 0 || len(res) == 1 && res[0].Bool() {
				return true
			}
		}
		return false
	})
}

type xdrExtract struct {
	out reflect.Value
	done bool
	trivSprintf
}
func (x *xdrExtract) Marshal(_ string, t stx.XdrType) {
	if x.done {
		return
	} else if reflect.TypeOf(t).AssignableTo(x.out.Type()) {
		x.out.Set(reflect.ValueOf(t))
		x.done = true
		return
	} else if a, ok := t.(stx.XdrAggregate); ok {
		a.XdrRecurse(x, "")
	}
}

// If out is of type **T, then *out is set to point to the first
// instance of T found when traversing t.
func XdrExtract(t stx.XdrType, out interface{}) bool {
	x := xdrExtract{ out: reflect.ValueOf(out).Elem() }
	t.XdrMarshal(&x, "")
	return x.done
}
