package main

import "fmt"
import "reflect"
import "sort"
import "strings"
import "unicode"
import "github.com/xdrpp/stc/stx"

type enum interface {
	fmt.Stringer
	stx.XdrNum32
	XdrEnumNames() map[int32]string
}

type union interface {
	stx.XdrType
	XdrUnionTag() interface{}
	XdrUnionTagName() string
	XdrUnionBody() interface{}
	XdrUnionBodyName() string
}

type enumVal = struct{
	val int32
	symbol string
}
type enumVals []enumVal
func (ev enumVals) Len() int { return len(ev) }
func (ev enumVals) Swap(i, j int) { ev[i], ev[j] = ev[j], ev[i] }
func (ev enumVals) Less(i, j int) bool { return ev[i].val < ev [j].val }

func camelize(s string) string {
	ret := &strings.Builder{}
	capitalize := true
	for _, c := range s {
		if c == '_' {
			capitalize = true
		} else if capitalize {
			capitalize = false
			fmt.Fprintf(ret, "%c", unicode.ToUpper(c))
		} else {
			fmt.Fprintf(ret, "%c", unicode.ToLower(c))
		}
	}
	return ret.String()
}

func gen(prefix string, u union, useArmName bool) {
	typ := reflect.TypeOf(u.XdrValue()).Name()
	tag := u.XdrUnionTag().(enum)
	var evs enumVals
	for k, v := range tag.XdrEnumNames() {
		evs = append(evs, enumVal{k, v})
	}
	sort.Sort(evs)
	for _, ev := range evs {
		tag.SetU32(uint32(ev.val))
		gentype := camelize(ev.symbol)
		if useArmName {
			if armname := u.XdrUnionBodyName(); armname != "" {
				gentype = armname
			}
		}
		arm := u.XdrUnionBody()
		if arm == nil {
			fmt.Printf(
`type %[1]s struct{}
func (%[1]s) To%[2]s() (ret %[3]s) {
	ret.%[4]s = %[5]s
	return
}

`, gentype, typ, prefix+typ, u.XdrUnionTagName(), prefix+ev.symbol)
		} else {
			armtype := reflect.TypeOf(arm).Elem().Name()
			if armtype == "" {
				armtype = reflect.TypeOf(arm).Elem().String()
			} else if unicode.IsUpper(rune(armtype[0])) {
				armtype = prefix + armtype
			}
			fmt.Printf(
`type %[1]s %[7]s
func (arg %[1]s) To%[2]s() (ret %[3]s) {
	ret.%[4]s = %[5]s
	*ret.%[6]s() = %[7]s(arg)
	return
}

`, gentype, typ, prefix+typ, u.XdrUnionTagName(), prefix+ev.symbol,
				u.XdrUnionBodyName(), armtype)
		}
	}
}

func main() {
	fmt.Printf(`package stc

import "github.com/xdrpp/stc/stx"

`)
	gen("stx.", &stx.XdrAnon_Operation_Body{}, false)
	gen("stx.", &stx.SignerKey{}, false)
	gen("stx.", &stx.Memo{}, false)
}
