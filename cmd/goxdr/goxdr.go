// Please see the goxdr.1 man page for complete documentation of this
// command.  The man page is included in the release and available at
// https://xdrpp.github.io/stc/pkg/github.com/xdrpp/stc/cmd/goxdr/goxdr.1.html
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

//go:generate goyacc -o parse.go parse.y
//go:generate sh -c "sed -e 's!^//UNCOMMENT:!!' header.go.in > header.go"

var progname string

type emitter struct {
	syms *rpc_syms
	output strings.Builder
	footer strings.Builder
	emitted map[string]struct{}
	enum_comments bool
	lax_discriminants bool
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

func indent(s string) string {
	if s == "" {
		return ""
	}
	return "\t" + strings.Replace(s, "\n", "\n\t", -1) + "\n"
}

const xdrinline_prefix string = "XdrAnon_"

func xdrinline(s string) string {
	if strings.HasPrefix(s, xdrinline_prefix) {
		return s
	}
	return xdrinline_prefix + s
}

func parseXDR(out *rpc_syms, file string) {
	src, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
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

func (e *emitter) xappend(out interface{}) {
	var s string
	switch t := out.(type) {
	case string:
		s = t
	case fmt.Stringer:
		s = t.String()
	default:
		panic("emitter append non-String")
	}
	e.footer.WriteString(s)
}

func (e *emitter) xprintf(str string, args ...interface{}) {
	fmt.Fprintf(&e.footer, str, args...)
}

func (e *emitter) get_typ(context idval, d *rpc_decl) idval {
	if d.typ.getgo() == "" {
		d.typ.setlocal(xdrinline(context.getgo()) + "_" + d.id.getgo())
		*d.inline_decl.getsym() = d.typ
		e.emit(d.inline_decl)
	}
	return d.typ
}

func (e *emitter) decltype(context idval, d *rpc_decl) string {
	typ := e.get_typ(context, d)
	switch d.qual {
	case SCALAR:
		return typ.getgo()
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

// With line-ending comment showing bound
func (e *emitter) decltypeb(context idval, d *rpc_decl) string {
	ret := e.decltype(context, d)
	if (d.qual == VEC || d.typ.getgo() == "string") && d.bound.getgo() != "" {
		return fmt.Sprintf("%s // bound %s", ret, d.bound.getgo())
	}
	return ret
}

func (e *emitter) gen_ptr(typ idval) string {
	ptrtyp := "_XdrPtr_" + typ.getgo()
	if typ.getgo()[0] == '_' {
		ptrtyp = "_XdrPtr" + typ.getgo()
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
		XdrPanic("*$TYPE present flag value %d should be 0 or 1", nv)
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
	frag = strings.Replace(frag, "$TYPE", typ.getgo(), -1)
	e.footer.WriteString(frag)
	return ptrtyp
}

func (e *emitter) is_bool(t idval) bool {
	for {
		if t.getx() == "bool" {
			return true
		} else if gg := t.getgo(); gg == "" || (gg[0] >= 'a' && gg[0] <= 'z') {
			return false
		}
		s, ok := e.syms.SymbolMap[t.getx()]
		if !ok {
			return false
		}
		td, ok := s.(*rpc_typedef)
		if !ok || td.qual != SCALAR || td.typ.getgo() == "" ||
			td.inline_decl != nil {
			return false
		}
		t = td.typ
	}
}

// Return (bound, sbound) where bound is just the normalized bound,
// and sbound is a name suitable for embedding in strings (contains
// the more readable "unbounded" rather than 4294967295).
func (e *emitter) get_bound(b0 idval) (string, string) {
	b := b0
	for loop := map[string]bool{}; !loop[b.getx()]; {
		loop[b.getx()] = true
		if s, ok := e.syms.SymbolMap[b.getx()]; !ok {
			break
		} else if d2, ok := s.(*rpc_const); !ok {
			break
		} else {
			b = d2.val
		}
	}
	if b.getx() == "" {
		b.setlocal("0xffffffff")
	}
	if i32, err := strconv.ParseUint(b.getx(), 0, 32); err == nil {
		b.setlocal(fmt.Sprintf("%d", i32))
	} else if b.getx()[0] == '-' {
		// will make go compiler report the error
		return b0.getx(), b0.getx()
	}
	if b.getx() == "4294967295" {
		return b.getx(), "unbounded"
	}
	return b.getx(), b.getx()
}

func (e *emitter) gen_vec(typ, bound0 idval) string {
	bound, sbound := e.get_bound(bound0)
	vectyp := "_XdrVec_" + sbound + "_" + typ.getgo()
	if typ.getgo()[0] == '_' {
		// '_' starts inline declarations, so only one size
		vectyp = "_XdrVec" + typ.getgo()
	}
	if e.done(vectyp) {
		return vectyp
	}
	frag :=
`type $VEC []$TYPE
func (v *$VEC) XdrBound() uint32 {
	const bound uint32 = $BOUND // Force error if not const or doesn't fit
	return bound
}
func (*$VEC) XdrCheckLen(length uint32) {
	if length > uint32($BOUND) {
		XdrPanic("$VEC length %d exceeds bound $BOUND", length)
	} else if int(length) < 0 {
		XdrPanic("$VEC length %d exceeds max int", length)
	}
}
func (v *$VEC) GetVecLen() uint32 { return uint32(len(*v)) }
func (v *$VEC) SetVecLen(length uint32) {
	v.XdrCheckLen(length)
	if int(length) <= cap(*v) {
		if int(length) != len(*v) {
			*v = (*v)[:int(length)]
		}
		return
	}
	newcap := 2*cap(*v)
	if newcap < int(length) { // also catches overflow where 2*cap < 0
		newcap = int(length)
	} else if bound := uint($BOUND); uint(newcap) > bound {
		if int(bound) < 0 {
			bound = ^uint(0) >> 1
		}
		newcap = int(bound)
	}
	nv := make([]$TYPE, int(length), newcap)
	copy(nv, *v)
	*v = nv
}
func (v *$VEC) XdrMarshalN(x XDR, name string, n uint32) {
	v.XdrCheckLen(n)
	for i := 0; i < int(n); i++ {
		if (i >= len(*v)) {
			v.SetVecLen(uint32(i+1))
		}
		XDR_$TYPE(x, x.Sprintf("%s[%d]", name, i), &(*v)[i])
	}
	if int(n) < len(*v) {
		*v = (*v)[:int(n)]
	}
}
func (v *$VEC) XdrMarshal(x XDR, name string) {
	size := XdrSize{ Size: uint32(len(*v)), Bound: $BOUND }
	x.Marshal(name, &size)
	v.XdrMarshalN(x, name, size.Size)
}
func (v *$VEC) XdrPointer() interface{} { return (*[]$TYPE)(v) }
func (v *$VEC) XdrValue() interface{} { return ([]$TYPE)(*v) }
`
	frag = strings.Replace(frag, "$VEC", vectyp, -1)
	frag = strings.Replace(frag, "$TYPE", typ.getgo(), -1)
	frag = strings.Replace(frag, "$BOUND", bound, -1)
	e.footer.WriteString(frag)
	return vectyp
}

func (e *emitter) xdrgen(target, name string, context idval,
	d *rpc_decl) string {
	typ := e.get_typ(context, d)
	var frag string
	switch d.qual {
	case SCALAR:
		if typ.getgo() == "string" {
			frag = "\tx.Marshal($NAME, XdrString{$TARGET, $BOUND})\n"
		} else {
			frag = "\tXDR_$TYPE(x, $NAME, $TARGET)\n"
		}
	case PTR:
		ptrtype := e.gen_ptr(typ)
		frag = fmt.Sprintf("\tx.Marshal($NAME, %s{$TARGET})\n", ptrtype)
	case ARRAY:
		if typ.getgo() == "byte" {
			frag = "\tx.Marshal($NAME, XdrArrayOpaque((*$TARGET)[:]))\n"
			break;
		}
		frag =
`	for i := 0; i < len(*$TARGET); i++ {
		XDR_$TYPE(x, x.Sprintf("%s[%d]", $NAME, i), &(*$TARGET)[i])
	}
`
	case VEC:
		if typ.getgo() == "byte" {
			frag = "\tx.Marshal($NAME, XdrVecOpaque{$TARGET, $BOUND})\n"
			break;
		}
		vectyp := e.gen_vec(typ, d.bound)
		frag = fmt.Sprintf("\tx.Marshal($NAME, (*%s)($TARGET))\n", vectyp)
	}
	normbound := d.bound.getgo()
	if normbound == "" {
		normbound = "0xffffffff"
	}
	if len(target) >= 1 && target[0] == '&' {
		frag = strings.Replace(frag, "*$TARGET", target[1:], -1)
	}
	frag = strings.Replace(frag, "$TARGET", target, -1)
	frag = strings.Replace(frag, "$NAME", name, -1)
	frag = strings.Replace(frag, "$BOUND", normbound, -1)
	frag = strings.Replace(frag, "$TYPE", typ.getgo(), -1)
	return frag
}

func (e *emitter) emit(sym rpc_sym) {
	sym.(Emittable).emit(e)
}

func (r *rpc_const) emit(e *emitter) {
	e.printf("const %s = %s\n", r.id, r.val)
}

func (r0 *rpc_typedef) emit(e *emitter) {
	r := (*rpc_decl)(r0)
	if r.comment != "" {
		e.printf("%s\n", r.comment)
	}
	e.printf("type %s = %s\n", r.id, e.decltypeb(gid(""), r))
	e.xprintf(
`func XDR_%[1]s(x XDR, name string, v *%[1]s) {
	if xs, ok := x.(interface{
		Marshal_%[1]s(string, *%[1]s)
	}); ok {
		xs.Marshal_%[1]s(name, v)
	} else {
	%[2]s	}
}
`, r.id, e.xdrgen("v", "name", gid(""), r))
}

func normalize_comment(comment string) string {
	block := comment[2] == '*'
	out := &strings.Builder{}
	first := true
	for _, line := range strings.Split(comment, "\n") {
		if first || !block {
			line = line[2:]
		}
		line := strings.Trim(line, " \t")
		if first {
			first = false
		} else if line != "" {
			out.WriteString(" ")
		}
		out.WriteString(line)
	}
	ret := out.String()
	if block {
		ret = ret[:len(ret)-2]
	}
	return ret
}

func enum_comment(e *rpc_enum) string {
	var useful bool
	out := &strings.Builder{}
	fmt.Fprintf(out, "var _XdrComments_%[1]s = map[int32]string {\n", e.id)
	for _, c := range e.tags {
		if c.comment != "" {
			useful = true
			fmt.Fprintf(out, "\tint32(%s): %q,\n", c.id,
				normalize_comment(c.comment))
		}
	}
	fmt.Fprintf(out, `}
func (e %[1]s) XdrEnumComments() map[int32]string {
	return _XdrComments_%[1]s
}
`, e.id)
	if useful {
		return out.String()
	} else {
		return ""
	}
}

func (r *rpc_enum) emit(e *emitter) {
	out := &strings.Builder{}
	if r.comment != "" {
		fmt.Fprintf(out, "%s\n", r.comment)
	}
	fmt.Fprintf(out, "type %s int32\nconst (\n", r.id)
	for _, tag := range r.tags {
		out.WriteString(indent(tag.comment))
		if _, err := strconv.Atoi(tag.val.String()); err == nil {
			fmt.Fprintf(out, "\t%s %s = %s\n", tag.id, r.id, tag.val)
		} else {
			fmt.Fprintf(out, "\t%s %s = %s(%s)\n", tag.id, r.id, r.id, tag.val)
		}
	}
	fmt.Fprintf(out, ")\n")
	e.append(out)
	out.Reset()
	fmt.Fprintf(out,
`func XDR_%s(x XDR, name string, v *%[1]s) {
	x.Marshal(name, v)
}
`, r.id)
	fmt.Fprintf(out, "var _XdrNames_%s = map[int32]string{\n", r.id)
	for _, tag := range r.tags {
		fmt.Fprintf(out, "\tint32(%s): \"%s\",\n", tag.id, tag.id.getx())
	}
	fmt.Fprintf(out, "}\n")
	fmt.Fprintf(out, "var _XdrValues_%s = map[string]int32{\n", r.id)
	for _, tag := range r.tags {
		fmt.Fprintf(out, "\t\"%s\": int32(%s),\n", tag.id.getx(), tag.id)
	}
	fmt.Fprintf(out, "}\n")
	fmt.Fprintf(out,
`func (*%[1]s) XdrEnumNames() map[int32]string {
	return _XdrNames_%[1]s
}
func (v *%[1]s) String() string {
	if s, ok := _XdrNames_%[1]s[int32(*v)]; ok {
		return s
	}
	return fmt.Sprintf("%[1]s#%%d", *v)
}
func (v *%[1]s) Scan(ss fmt.ScanState, _ rune) error {
	if tok, err := ss.Token(true, XdrSymChar); err != nil {
		return err
	} else {
		stok := string(tok)
		if val, ok := _XdrValues_%[1]s[stok]; ok {
			*v = %[1]s(val)
			return nil
		}
		return XdrError(fmt.Sprintf("%%s is not a valid %[1]s.", stok))
	}
}
func (v *%[1]s) GetU32() uint32 {
	return uint32(*v)
}
func (v *%[1]s) SetU32(n uint32) {
	*v = %[1]s(n)
}
func (v *%[1]s) XdrPointer() interface{} {
	return v
}
func (v *%[1]s) XdrValue() interface{} {
	return *v
}
`, r.id)

	if e.enum_comments {
		out.WriteString(enum_comment(r))
	}

	alltags := &strings.Builder{}
	for _, tag := range r.tags {
		if val, _ := e.get_bound(tag.id); val == "0" {
			goto skip
		}
		if alltags.Len() > 0 {
			alltags.WriteString(", ")
		}
		alltags.WriteString(tag.id.String())
	}
	fmt.Fprintf(out,
`func (v *%[1]s) XdrInitialize() {
	switch %[1]s(0) {
	case %[2]s:
	default:
		if *v == %[1]s(0) { *v = %[3]s }
	}
}
`, r.id, alltags.String(), r.tags[0].id.String())
skip:

	e.xappend(out)
}

func (r *rpc_struct) emit(e *emitter) {
	out := &strings.Builder{}
	if r.comment != "" {
		fmt.Fprintf(out, "%s\n", r.comment)
	}
	fmt.Fprintf(out, "type %s struct {\n", r.id)
	for i := range r.decls {
		out.WriteString(indent(r.decls[i].comment))
		fmt.Fprintf(out, "\t%s %s\n", r.decls[i].id,
			e.decltypeb(r.id, &r.decls[i]))
	}
	fmt.Fprintf(out, "}\n")
	e.append(out)
	out.Reset()

	fmt.Fprintf(out,
`func (v *%[1]s) XdrPointer() interface{} {
	return v
}
func (v *%[1]s) XdrValue() interface{} {
	return *v
}
func (v *%[1]s) XdrMarshal(x XDR, name string) {
	if name != "" {
		name = x.Sprintf("%%s.", name)
	}
`, r.id)
	for i := range r.decls {
		out.WriteString(e.xdrgen("&v." + r.decls[i].id.getgo(),
			`x.Sprintf("%s` + r.decls[i].id.getx() + `", name)`,
			r.id, &r.decls[i]))
	}
	fmt.Fprintf(out, "}\n")
	fmt.Fprintf(out, "func XDR_%s(x XDR, name string, v *%s) {\n" +
		"\tx.Marshal(name, v)\n" +
		"}\n", r.id, r.id)
	e.xappend(out)
}

type unionHelper struct {
	castCases bool
}

func (uh unionHelper) castCase(v idval) string {
	if uh.castCases {
		return fmt.Sprintf("int32(%s)", v)
	}
	return v.String()
}

func (uh unionHelper) join(u *rpc_ufield) string {
	ret := uh.castCase(u.cases[0])
	for i := 1; i < len(u.cases); i++ {
		ret += ", "
		ret += uh.castCase(u.cases[i])
	}
	return ret
}

/*
func castCase(boolDiscriminant bool, v idval) string {
	if boolDiscriminant {
		return v.String()
	}
	return fmt.Sprintf("int32(%s)", v)
}

func (u *rpc_ufield) joinedCases(boolDiscriminant bool) string {
	ret := castCase(boolDiscriminant, u.cases[0])
	for i := 1; i < len(u.cases); i++ {
		ret += ", "
		ret += castCase(boolDiscriminant, u.cases[i])
	}
	return ret
}
*/

func (u *rpc_ufield) isVoid() bool {
	return u.decl.id.getx() == "" || u.decl.typ.getx() == "void"
}

func (r *rpc_union) emit(e *emitter) {
	out := &strings.Builder{}
	if r.comment != "" {
		fmt.Fprintf(out, "%s\n", r.comment)
	}
	fmt.Fprintf(out, "type %s struct {\n", r.id)
	fmt.Fprintf(out,
		"\t// The union discriminant %s selects among the following arms:\n",
		r.tagid)
	for i := range r.fields {
		u := &r.fields[i]
		if (u.hasdefault) {
			fmt.Fprintf(out, "\t//   default:\n")
		} else {
			fmt.Fprintf(out, "\t//   %s:\n",
				unionHelper{castCases: false}.join(u))
		}
		if u.isVoid() {
			fmt.Fprintf(out, "\t//      void\n")
		} else {
			fmt.Fprintf(out, "\t//      %s() *%s\n", u.decl.id,
				e.decltypeb(r.id, &u.decl))
		}
	}
	fmt.Fprintf(out, "\t%s %s\n", r.tagid, r.tagtype)
	fmt.Fprintf(out, "\t_u interface{}\n" +
		"}\n")
	e.append(out)
	out.Reset()

	uh := unionHelper {
		castCases: e.lax_discriminants && !e.is_bool(r.tagtype),
	}

	var discriminant string
	if !uh.castCases {
		discriminant = "u." + r.tagid.String()
	} else {
		discriminant = fmt.Sprintf("int32(u.%s)", r.tagid)
	}
	for i := range r.fields {
		u := &r.fields[i]
		if u.isVoid() {
			continue
		}
		ret := e.decltype(r.id, &u.decl)
/*
		if (u.hasdefault) {
			fmt.Fprintf(out,
				"// Valid by default when u.%s does not match another field.\n",
				r.tagid)
		} else if len(u.cases) == 1 {
			fmt.Fprintf(out, "// Valid when u.%s is %s.\n",
				r.tagid, u.joinedCases())
		} else {
			fmt.Fprintf(out, "// Valid when u.%s is one of: %s.\n",
				r.tagid, u.joinedCases())
		}
*/
		if u.decl.comment != "" {
			fmt.Fprintf(out, "%s\n", u.decl.comment)
		}
		fmt.Fprintf(out, "func (u *%s) %s() *%s {\n", r.id, u.decl.id, ret)
		goodcase := fmt.Sprintf(
`		if v, ok := u._u.(*%[1]s); ok {
			return v
		} else {
			var zero %[1]s
			u._u = &zero
			return &zero
		}
`, ret)
		badcase := fmt.Sprintf(
`		XdrPanic("%s.%s accessed when %s == %%v", u.%[3]s)
		return nil
`, r.id, u.decl.id, r.tagid)
		fmt.Fprintf(out, "\tswitch %s {\n", discriminant)
		if u.hasdefault && len(r.fields) > 1 {
			needcomma := false
			fmt.Fprintf(out, "\tcase ")
			for j := range r.fields {
				u1 := &r.fields[j]
				if u1.hasdefault {
					continue
				}
				if needcomma {
					fmt.Fprintf(out, ",")
				} else {
					needcomma = true
				}
				fmt.Fprintf(out, "%s", uh.join(u1))
			}
			fmt.Fprintf(out, ":\n%s\tdefault:\n%s", badcase, goodcase)
		} else {
			if u.hasdefault {
				fmt.Fprintf(out, "default:\n")
			} else {
				fmt.Fprintf(out, "\tcase %s:\n", uh.join(u))
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
		fmt.Fprintf(out, "\tswitch %s {\n" + "\tcase ", discriminant)
		needcomma := false
		for j := range r.fields {
			u1 := &r.fields[j]
			if needcomma {
				fmt.Fprintf(out, ",")
			} else {
				needcomma = true
			}
			fmt.Fprintf(out, "%s", uh.join(u1))
		}
		fmt.Fprintf(out, ":\n\t\treturn true\n\t}\n\treturn false\n")
	}
	fmt.Fprintf(out, "}\n")

	fmt.Fprintf(out, "func (u *%s) XdrUnionTag() interface{} {\n" +
		"\treturn &u.%s\n}\n", r.id, r.tagid)
	fmt.Fprintf(out, "func (u *%s) XdrUnionTagName() string {\n" +
		"\treturn \"%s\"\n}\n", r.id, r.tagid)

	fmt.Fprintf(out, "func (u *%s) XdrUnionBody() interface{} {\n" +
		"\tswitch %s {\n", r.id, discriminant)
	for i := range r.fields {
		u := &r.fields[i]
		if u.hasdefault {
			fmt.Fprintf(out, "\tdefault:\n")
		} else {
			fmt.Fprintf(out, "\tcase %s:\n", uh.join(u))
		}
		if u.isVoid() {
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
		"\tswitch %s {\n", r.id, discriminant)
	for i := range r.fields {
		u := &r.fields[i]
		if u.hasdefault {
			fmt.Fprintf(out, "\tdefault:\n")
		} else {
			fmt.Fprintf(out, "\tcase %s:\n", uh.join(u))
		}
		if u.isVoid() {
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

	fmt.Fprintf(out,
`func (v *%[1]s) XdrPointer() interface{} {
	return v
}
func (v *%[1]s) XdrValue() interface{} {
	return *v
}
func (u *%[1]s) XdrMarshal(x XDR, name string) {
	if name != "" {
		name = x.Sprintf("%%s.", name)
	}
	XDR_%[2]s(x, x.Sprintf("%%s%[4]s", name), &u.%[3]s)
	switch %[5]s {
`, r.id, r.tagtype, r.tagid, r.tagid.getx(), discriminant)
	for i := range r.fields {
		u := &r.fields[i]
		if u.hasdefault {
			fmt.Fprintf(out, "\tdefault:\n")
		} else {
			fmt.Fprintf(out, "\tcase %s:\n", uh.join(u))
		}
		if !u.isVoid() {
			out.WriteString("\t" + e.xdrgen("u." + u.decl.id.getgo() + "()",
				`x.Sprintf("%s` + u.decl.id.getx() + `", name)`,
				r.id, &u.decl))
		}
		out.WriteString("\t\treturn\n")
	}
	fmt.Fprintf(out, "\t}\n")
	if !r.hasdefault {
		fmt.Fprintf(out,
`	XdrPanic("invalid %[1]s (%%v) in %[2]s", u.%[1]s)
`, r.tagid, r.id)
	}
	fmt.Fprintf(out, "}\n")

	if !r.hasdefault {
		alltags := &strings.Builder{}
		for i := range r.fields {
			for j := range r.fields[i].cases {
				tag := r.fields[i].cases[j]
				if val, _ := e.get_bound(tag); val == "0" || val == "FALSE" {
					goto skip
				}
				if alltags.Len() > 0 {
					alltags.WriteString(", ")
				}
				alltags.WriteString(tag.String())
			}
		}
		// Note we declare a variable zero so that it works with
		// built-in types, especially bool, because go won't allow
		// bool(0) as an expression.
		fmt.Fprintf(out,
`func (v *%[1]s) XdrInitialize() {
	var zero %[2]s
	switch zero {
	case %[3]s:
	default:
		if v.%[4]s == zero { v.%[4]s = %[5]s }
	}
}
`, r.id, r.tagtype, alltags.String(), r.tagid, r.fields[0].cases[0])
	skip:
	}

	fmt.Fprintf(out, "func XDR_%s(x XDR, name string, v *%s) {\n" +
		"\tx.Marshal(name, v)\n" +
		"}\n", r.id, r.id)

	e.xappend(out)
}

func (r *rpc_program) emit(e *emitter) {
	// Do something?
}

func emitAll(syms *rpc_syms, comments bool, lax bool) string {
	e := emitter{
		syms: syms,
		emitted: map[string]struct{}{},
		enum_comments: comments,
		lax_discriminants: lax,
	}

	e.append(`
//
// Data types defined in XDR file
//
`)

	for _, s := range syms.Symbols {
		e.append("\n")
		e.emit(s)
	}
	e.append(`
//
// Helper types and generated marshaling functions
//

`)
	e.append(e.footer.String())
	e.footer.Reset()
	return e.output.String()
}

type stringlist []string
func (sl *stringlist) String() string {
	return fmt.Sprint(*sl)
}
func (sl *stringlist) ToImports() string {
	var out strings.Builder
	for _, s := range *sl {
		fmt.Fprintf(&out, "import . \"%s\"\n", s)
	}
	return out.String()
}
func (sl *stringlist) Set(v string) error {
	*sl = append(*sl, v)
	return nil
}

func main() {
	opt_nobp := flag.Bool("b", false, "Suppress initial boilerplate code")
	opt_output := flag.String("o", "", "Name of output file")
	opt_pkg := flag.String("p", "main", "Name of package")
	opt_help := flag.Bool("help", false, "Print usage")
	opt_enum_comments := flag.Bool("enum-comments", false,
		"for each enum, output a map containing source comments")
	opt_lax_discriminants := flag.Bool("lax-discriminants", false,
		"allow union discriminants and cases to be different enum types")
	var imports stringlist
	flag.Var(&imports, "i", "add import directive for `path`")
	progname = os.Args[0]
	if pos := strings.LastIndexByte(progname, '/'); pos >= 0 {
		progname = progname[pos+1:]
	}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s FLAGS [file1.x file2.x ...]\n", progname)
		flag.PrintDefaults()
	}
	flag.Parse()
	if (*opt_help) {
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		return
	}

	var syms rpc_syms
	for _, arg := range flag.Args() {
		parseXDR(&syms, arg)
	}
	if syms.Failed {
		os.Exit(1)
	}
	code := emitAll(&syms, *opt_enum_comments, *opt_lax_discriminants)

	out := os.Stdout
	if (*opt_output != "") {
		var err error
		out, err = os.Create(*opt_output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err.Error())
			os.Exit(1)
		}
	}

	// https://github.com/golang/go/issues/13560#issuecomment-288457920
	fmt.Fprintf(out, "// Code generated by %s", progname);
	if len(os.Args) >= 1 {
		for _, arg := range os.Args[1:] {
			fmt.Fprintf(out, " %s", arg)
		}
	}
	fmt.Fprint(out, "; DO NOT EDIT.\n\n");

	if *opt_pkg != "" {
		fmt.Fprintf(out, "package %s\n%s", *opt_pkg, imports.ToImports())
	}
	if !*opt_nobp {
		io.WriteString(out, header)
	}

	io.WriteString(out, code)
}
