package main

import "fmt"
import "io"
import "io/ioutil"
import "os"
import "strconv"
import "strings"

//go:generate goyacc -o parse.go parse.y
//go:generate sh -c "sed -e 's!^//UNCOMMENT:!!' header.go.in > header.go"

type emitter struct {
	syms *rpc_syms
	output strings.Builder
	emitted map[string]struct{}
}

type Emittable interface {
	emit(e *emitter)
}

func capitalize(s string) string {
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0] &^ 0x20) + s[1:]
	}
	return s
}

func uncapitalize(s string) string {
	if len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' {
		return string(s[0] | 0x20) + s[1:]
	}
	return s
}

func underscore(s string) string {
	if s == "" || s[0] == '_' {
		return s
	}
	return "_" + s
}

func parseXDR(out *rpc_syms, file string) {
	src, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		out.Failed = true
		return
	}
	l := NewLexer(out, file, string(src))
	yyParse(l)
}

func (e *emitter) done(typ string) bool {
	if _, ok := e.emitted[typ]; ok {
		return true
	}
	e.emitted[typ] = struct{}{}
	return false
}

func (e *emitter) append(out interface{}) {
	var s string
	switch t := out.(type) {
	case string:
		s = t
	case fmt.Stringer:
		s = t.String()
	default:
		panic("emitter append non-String")
	}
	e.output.WriteString(s)
}

func (e *emitter) printf(str string, args ...interface{}) {
	fmt.Fprintf(&e.output, str, args...)
}

func (e *emitter) get_typ(context string, d *rpc_decl) string {
	if d.typ == "" {
		d.typ = underscore(context) + "_" + d.id
		*d.inline_decl.symid() = d.typ
		e.emit(d.inline_decl)
	}
	return d.typ
}

func (e *emitter) decltype(context string, d *rpc_decl) string {
	typ := e.get_typ(context, d)
	switch d.qual {
	case SCALAR:
		return typ
	case PTR:
		return fmt.Sprintf("*%s", typ)
	case ARRAY:
		return fmt.Sprintf("[%s]%s", d.bound, typ)
	case VEC:
		return fmt.Sprintf("[]%s", typ)
	default:
		panic("emitter::decltype invalid qual_t")
	}
}

func (e *emitter) gen_ptr(typ string) string {
	ptrtyp := "XdrPtr_" + typ
	if typ[0] == '_' {
		ptrtyp = "_XdrPtr" + typ
	}
	if e.done(ptrtyp) {
		return ptrtyp
	}
	frag :=
`type $PTR struct {
	p **$TYPE
}
type _ptrflag_$TYPE $PTR
func (v _ptrflag_$TYPE) String() string {
	if *v.p == nil {
		return "nil"
	}
	return "non-nil"
}
func (v _ptrflag_$TYPE) GetU32() uint32 {
	if *v.p == nil {
		return 0
	}
	return 1
}
func (v _ptrflag_$TYPE) SetU32(nv uint32) {
	switch nv {
	case 0:
		*v.p = nil
	case 1:
		if *v.p == nil {
			*v.p = new($TYPE)
		}
	default:
		xdrPanic("*$TYPE present flag value %d should be 0 or 1", nv)
	}
}
func (v _ptrflag_$TYPE) XdrPointer() interface{} { return nil }
func (v _ptrflag_$TYPE) XdrValue() interface{} { return v.GetU32() != 0 }
func (v _ptrflag_$TYPE) XdrBound() uint32 { return 1 }
func (v $PTR) GetPresent() bool { return *v.p != nil }
func (v $PTR) SetPresent(present bool) {
	if !present {
		*v.p = nil
	} else if *v.p == nil {
		*v.p = new($TYPE)
	}
}
func (v $PTR) XdrMarshalValue(x XDR, name string) {
	if *v.p != nil {
		XDR_$TYPE(x, name, *v.p)
	}
}
func (v $PTR) XdrMarshal(x XDR, name string) {
	x.Marshal(name, _ptrflag_$TYPE(v))
	v.XdrMarshalValue(x, name)
}
func (v $PTR) XdrPointer() interface{} { return v.p }
func (v $PTR) XdrValue() interface{} { return *v.p }
`
	frag = strings.Replace(frag, "$PTR", ptrtyp, -1)
	frag = strings.Replace(frag, "$TYPE", typ, -1)
	e.append(frag)
	return ptrtyp
}

func (e *emitter) get_bound(b string) string {
	for loop := map[string]bool{}; !loop[b]; {
		loop[b] = true
		if s, ok := e.syms.SymbolMap[b]; !ok {
			break
		} else if d2, ok := s.(*rpc_const); !ok {
			break
		} else {
			b = d2.val
		}
	}
	if b == "" {
		b = "0xffffffff"
	}
	if i32, err := strconv.ParseUint(b, 0, 32); err == nil {
		b = fmt.Sprintf("%d", i32)
	}
	return b
}

func (e *emitter) gen_vec(typ, bound string) string {
	bound = e.get_bound(bound)
	vectyp := "XdrVec_" + bound + "_" + typ
	if typ[0] == '_' {
		// '_' starts inline declarations, so only one size
		vectyp = "_XdrVec" + typ
	}
	if e.done(vectyp) {
		return vectyp
	}
	frag :=
`type $VEC []$TYPE
func (v *$VEC) XdrBound() uint32 { return uint32($BOUND) }
func (v *$VEC) SetVecLen(length uint32) {
	l := int(length)
	if l > int($BOUND) {
		xdrPanic("SetVecLen length %d exceeds bound $BOUND", l)
	} else if l <= cap(*v) {
		if l != len(*v) {
			*v = (*v)[:l]
		}
		return
	}
	newcap := 2*cap(*v)
	if l > newcap {
		newcap = l
	} else if newcap > int($BOUND) {
		newcap = int($BOUND)
	}
	nv := make([]$TYPE, l, newcap)
	copy(nv, *v)
	*v = nv
}
func (v *$VEC) XdrMarshalN(x XDR, name string, n uint32) {
	if int(n) < len(*v) {
		*v = (*v)[:int(n)]
	}
	for i := 0; i < int(n); i++ {
		if (i >= len(*v)) {
			v.SetVecLen(uint32(i+1))
		}
		XDR_$TYPE(x, fmt.Sprintf("%s[%d]", name, i), &(*v)[i])
	}
}
func (v *$VEC) XdrMarshal(x XDR, name string) {
	size := XdrSize{ size: uint32(len(*v)), bound: $BOUND }
	x.Marshal(name, &size)
	v.XdrMarshalN(x, name, size.size)
}
func (v *$VEC) XdrPointer() interface{} { return (*[]$TYPE)(v) }
func (v *$VEC) XdrValue() interface{} { return ([]$TYPE)(*v) }
`
	frag = strings.Replace(frag, "$VEC", vectyp, -1)
	frag = strings.Replace(frag, "$TYPE", typ, -1)
	frag = strings.Replace(frag, "$BOUND", bound, -1)
	e.append(frag)
	return vectyp
}

func (e *emitter) xdrgen(target, name, context string, d *rpc_decl) string {
	typ := e.get_typ(context, d)
	var frag string
	switch d.qual {
	case SCALAR:
		if typ == "string" {
			frag = "\tx.Marshal($NAME, &XdrString{$TARGET, $BOUND})\n"
		} else {
			frag = "\tXDR_$TYPE(x, $NAME, $TARGET)\n"
		}
	case PTR:
		ptrtype := e.gen_ptr(typ)
		frag = fmt.Sprintf("\tx.Marshal($NAME, %s{$TARGET})\n", ptrtype)
	case ARRAY:
		if typ == "byte" {
			frag = "\tx.Marshal($NAME, XdrArrayOpaque((*$TARGET)[:]))\n"
			break;
		}
		frag =
`	for i := 0; i < len(*$TARGET); i++ {
			XDR_$TYPE(x, fmt.Sprintf("%s[%d]", $NAME, i), &(*$TARGET)[i])
	}
`
	case VEC:
		if typ == "byte" {
			frag = "\tx.Marshal($NAME, &XdrVecOpaque{$TARGET, $BOUND})\n"
			break;
		}
		vectyp := e.gen_vec(typ, d.bound)
		frag = fmt.Sprintf("\tx.Marshal($NAME, (*%s)($TARGET))\n", vectyp)
	}
	normbound := d.bound
	if normbound == "" {
		normbound = "0xffffffff"
	}
	if len(target) >= 1 && target[0] == '&' {
		frag = strings.Replace(frag, "*$TARGET", target[1:], -1)
	}
	frag = strings.Replace(frag, "$TARGET", target, -1)
	frag = strings.Replace(frag, "$NAME", name, -1)
	frag = strings.Replace(frag, "$BOUND", normbound, -1)
	frag = strings.Replace(frag, "$TYPE", typ, -1)
	return frag
}

func (e *emitter) emit(sym rpc_sym) {
	sym.(Emittable).emit(e)
}

func (r *rpc_const) emit(e *emitter) {
	e.printf("const %s = %s\n", r.id, r.val)
}

func (r *rpc_decl) emit(e *emitter) {
	e.printf("type %s %s\n", r.id, e.decltype("", r))
	e.printf("func XDR_%s(x XDR, name string, v *%s) {\n%s}\n",
		r.id, r.id, e.xdrgen("v", "name", "", r))
}

func (r0 *rpc_typedef) emit(e *emitter) {
	r := (*rpc_decl)(r0)
	e.printf("type %s = %s\n", r.id, e.decltype("", r))
	e.printf("func XDR_%s(x XDR, name string, v *%s) {\n%s}\n",
		r.id, r.id, e.xdrgen("v", "name", "", r))
}

func (r *rpc_enum) emit(e *emitter) {
	out := &strings.Builder{}
	fmt.Fprintf(out, "type %s int32\nconst (\n", r.id)
	for _, tag := range r.tags {
		fmt.Fprintf(out, "\t%s = %s(%s)\n", tag.id, r.id, tag.val)
	}
	fmt.Fprintf(out, ")\n")
	fmt.Fprintf(out, "func XDR_%s(x XDR, name string, v *%s) {\n" +
		"\tx.Marshal(name, v)\n" +
		"}\n", r.id, r.id)
	fmt.Fprintf(out, "var _XdrNames_%s = map[int32]string{\n", r.id)
	for _, tag := range r.tags {
		fmt.Fprintf(out, "\tint32(%s): \"%s\",\n", tag.id, tag.id)
	}
	fmt.Fprintf(out, "}\n")
	fmt.Fprintf(out, "func (*%s) XdrEnumNames() map[int32]string {\n" +
		"\treturn _XdrNames_%s\n}\n", r.id, r.id)
	fmt.Fprintf(out, "func (v *%s) String() string {\n" +
		"\tif s, ok := _XdrNames_%s[int32(*v)]; ok {\n" +
		"\t\treturn s\n\t}\n" +
		"\treturn fmt.Sprintf(\"%s#%%d\", *v)\n" +
		"}\n", r.id, r.id, r.id)
	fmt.Fprintf(out, "func (v *%s) GetU32() uint32 {\n" +
		"\treturn uint32(*v)\n" +
		"}\n", r.id)
	fmt.Fprintf(out, "func (v *%s) SetU32(n uint32) {\n" +
		"\t*v = %s(n)\n" +
		"}\n", r.id, r.id)
	fmt.Fprintf(out, "func (v *%s) XdrPointer() interface{} {\n" +
		"\treturn v\n" +
		"}\n", r.id)
	fmt.Fprintf(out, "func (v *%s) XdrValue() interface{} {\n" +
		"\treturn *v\n" +
		"}\n", r.id)
	e.append(out)
}

func (r *rpc_struct) emit(e *emitter) {
	out := &strings.Builder{}
	fmt.Fprintf(out, "type %s struct {\n", r.id)
	for i := range r.decls {
		fmt.Fprintf(out, "\t%s %s\n", r.decls[i].id,
			e.decltype(r.id, &r.decls[i]))
	}
	fmt.Fprintf(out, "}\n")
	fmt.Fprintf(out, "func (v *%s) XdrMarshal(x XDR, name string) {\n" +
		"\tif name != \"\" {\n" +
		"\t\tname = name + \".\"\n" +
		"\t}\n", r.id)
	for i := range r.decls {
		fmt.Fprintf(out, "%s",
			e.xdrgen("&v." + r.decls[i].id,
				"name + \"" + r.decls[i].id + "\"",
				r.id, &r.decls[i]))
	}
	fmt.Fprintf(out, "}\n")
	fmt.Fprintf(out, "func XDR_%s(x XDR, name string, v *%s) {\n" +
		"\tx.Marshal(name, v)\n" +
		"}\n", r.id, r.id)
	e.append(out)
}

func (r *rpc_union) emit(e *emitter) {
	out := &strings.Builder{}
	fmt.Fprintf(out, "type %s struct {\n", r.id)
	fmt.Fprintf(out, "\t%s %s\n", r.tagid, r.tagtype)
	fmt.Fprintf(out, "\t_u interface{}\n")
	fmt.Fprintf(out, "}\n")
	for i := range r.fields {
		u := &r.fields[i]
		if u.decl.id == "" || u.decl.typ == "void" {
			continue
		}
		ret := e.decltype(r.id, &u.decl)
		fmt.Fprintf(out, "func (u *%s) %s() *%s {\n", r.id, u.decl.id, ret)
		goodcase := fmt.Sprintf("\t\tif v, ok := u._u.(*%s); ok {\n" +
			"\t\t\treturn v\n" +
			"\t\t} else {\n" +
			"\t\t\tvar zero %s\n" +
			"\t\t\tu._u = &zero\n" +
			"\t\t\treturn &zero\n" +
			"\t\t}\n", ret, ret)
		badcase := fmt.Sprintf(
			"\t\txdrPanic(\"%s.%s accessed when %s == %%v\", u.%s)\n" +
				"\t\treturn nil\n",
			r.id, u.decl.id, r.tagid, r.tagid)
		fmt.Fprintf(out, "\tswitch u.%s {\n", r.tagid)
		if u.hasdefault && len(r.fields) > 1 {
			needcomma := false
			fmt.Fprintf(out, "\tcase ")
			for j := range r.fields {
				u1 := &r.fields[j]
				if r.hasdefault {
					continue
				}
				if needcomma {
					fmt.Fprintf(out, ",")
				} else {
					needcomma = true
				}
				fmt.Fprintf(out, "%s", strings.Join(u1.cases, ","))
			}
			fmt.Fprintf(out, ":\n%s\tdefault:\n%s", badcase, goodcase)
		} else {
			if u.hasdefault {
				fmt.Fprintf(out, "default:\n")
			} else {
				fmt.Fprintf(out, "\tcase %s:\n", strings.Join(u.cases, ","))
			}
			fmt.Fprintf(out, "%s", goodcase)
			if !u.hasdefault {
				fmt.Fprintf(out, "\tdefault:\n%s", badcase)
			}
		}
		fmt.Fprintf(out, "\t}\n")
		fmt.Fprintf(out, "}\n")
	}

	fmt.Fprintf(out, "func (u *%s) XdrValid() bool {\n", r.id)
	if r.hasdefault {
		fmt.Fprintf(out, "\treturn true\n")
	} else {
		fmt.Fprintf(out, "\tswitch u.%s {\n" + "\tcase ", r.tagid)
		needcomma := false
		for j := range r.fields {
			u1 := &r.fields[j]
			if needcomma {
				fmt.Fprintf(out, ",")
			} else {
				needcomma = true
			}
			fmt.Fprintf(out, "%s", strings.Join(u1.cases, ","))
		}
		fmt.Fprintf(out, ":\n\t\treturn true\n\t}\n\treturn false\n")
	}
	fmt.Fprintf(out, "}\n")

	fmt.Fprintf(out, "func (u *%s) XdrUnionTag() interface{} {\n" +
		"\treturn &u.%s\n}\n", r.id, r.tagid)
	fmt.Fprintf(out, "func (u *%s) XdrUnionTagName() string {\n" +
		"\treturn \"%s\"\n}\n", r.id, r.tagid)

	fmt.Fprintf(out, "func (u *%s) XdrUnionBody() interface{} {\n" +
		"\tswitch u.%s {\n", r.id, r.tagid)
	for i := range r.fields {
		u := &r.fields[i]
		if u.hasdefault {
			fmt.Fprintf(out, "\tdefault:\n")
		} else {
			fmt.Fprintf(out, "\tcase %s:\n", strings.Join(u.cases, ","))
		}
		if u.decl.id == "" || u.decl.typ == "void" {
			fmt.Fprintf(out, "\t\treturn nil\n")
		} else {
			fmt.Fprintf(out, "\t\treturn u.%s()\n", u.decl.id)
		}
	}
	fmt.Fprintf(out, "\t}\n")
	if !r.hasdefault {
		fmt.Fprintf(out, "\treturn nil\n")
	}
	fmt.Fprintf(out, "}\n")

	fmt.Fprintf(out, "func (u *%s) XdrUnionBodyName() string {\n" +
		"\tswitch u.%s {\n", r.id, r.tagid)
	for i := range r.fields {
		u := &r.fields[i]
		if u.hasdefault {
			fmt.Fprintf(out, "\tdefault:\n")
		} else {
			fmt.Fprintf(out, "\tcase %s:\n", strings.Join(u.cases, ","))
		}
		if u.decl.id == "" || u.decl.typ == "void" {
			fmt.Fprintf(out, "\t\treturn \"\"\n")
		} else {
			fmt.Fprintf(out, "\t\treturn \"%s\"\n", u.decl.id)
		}
	}
	fmt.Fprintf(out, "\t}\n")
	if !r.hasdefault {
		fmt.Fprintf(out, "\treturn \"\"\n")
	}
	fmt.Fprintf(out, "}\n")

	fmt.Fprintf(out, "func (v *%s) XdrMarshal(x XDR, name string) {\n" +
		"\tif name != \"\" {\n" +
		"\t\tname = name + \".\"\n" +
		"\t}\n" +
		"\tXDR_%s(x, name + \"%s\", &v.%s)\n" +
		"\tswitch v.%s {\n",
		r.id, r.tagtype, r.tagid, r.tagid, r.tagid)
	for i := range r.fields {
		u := &r.fields[i]
		if u.hasdefault {
			fmt.Fprintf(out, "\tdefault:\n")
		} else {
			fmt.Fprintf(out, "\tcase %s:\n", strings.Join(u.cases, ","))
		}
		if u.decl.id != "" && u.decl.typ != "void" {
			out.WriteString(e.xdrgen("v." + u.decl.id + "()",
				"name + \"" + u.decl.id + "\"", r.id, &u.decl))
		}
	}
	fmt.Fprintf(out, "\t}\n}\n")

	fmt.Fprintf(out, "func XDR_%s(x XDR, name string, v *%s) {\n" +
		"\tx.Marshal(name, v)\n" +
		"}\n", r.id, r.id)

	e.append(out)
}

func (r *rpc_program) emit(e *emitter) {
	// Do something?
}

func emitAll(syms *rpc_syms) {
	e := emitter{
		syms: syms,
		emitted: map[string]struct{}{},
	}

	e.printf("package main\n")
	e.append(header)
	for _, s := range syms.Symbols  {
		e.append("\n")
		e.emit(s)
	}
	io.WriteString(os.Stdout, e.output.String())
}

func main() {
	args := os.Args
	if len(args) <= 1 { return }
	args = args[1:]
	var syms rpc_syms
	for _, arg := range args {
		parseXDR(&syms, arg)
	}
	if syms.Failed {
		os.Exit(1)
	} else {
		emitAll(&syms)
	}
}
