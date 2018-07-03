package main

import "fmt"
import "io"
import "io/ioutil"
import "os"
import "strings"

//go:generate goyacc -o parse.go parse.y

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
	if len(s) > 0 && s[0] == '_' {
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

type emitter struct {
	syms *rpc_syms
	declarations []string
}

func (e *emitter) chase_typedef(d *rpc_decl, inner bool) *rpc_decl {
	if d1, ok := e.syms.SymbolMap[d.typ]; (inner || d.qual == SCALAR) && ok {
		if d2, ok := d1.(*rpc_typedef); ok {
			if d2.qual == SCALAR {
				return e.chase_typedef((*rpc_decl)(d2), false)
			}
			return (*rpc_decl)(d2)
		}
	}
	return d
}

func (e *emitter) decltype(parent rpc_sym, d *rpc_decl) string {
	out := &strings.Builder{}
	switch (d.qual) {
	case PTR:
		fmt.Fprintf(out, "*");
	case ARRAY:
		fmt.Fprintf(out, "[%s]", d.bound)
	case VEC:
		fmt.Fprintf(out, "[]")
	}
	if d.typ == "" {
		if _, isTypedef := parent.(*rpc_typedef); isTypedef {
			d.typ = underscore(d.id)
		} else {
			d.typ = underscore(*parent.symid()) + "_" + d.id
		}
		*d.inline_decl.symid() = d.typ
		e.emit(d.inline_decl)
	}
	fmt.Fprintf(out, "%s", d.typ)
	d1 := e.chase_typedef(d, false)
	if _, isStruct := parent.(*rpc_struct); isStruct &&
		(d1.qual == VEC || d1.typ == "string") {
		bound := d1.bound
		if (bound == "") {
			bound = "0xffffffff"
		}
		fmt.Fprintf(out, " `xdrbound:\"%s\"`", bound)
	}
	return out.String()
}

func (e *emitter) emit(sym rpc_sym) {
	out := &strings.Builder{}
	switch r := sym.(type) {
	case *rpc_const:
		fmt.Fprintf(out, "const %s = %s\n", r.id, r.val)
	case *rpc_enum:
		fmt.Fprintf(out, "type %s int32\nconst (\n", r.id);
		for _, tag := range r.tags {
			fmt.Fprintf(out, "\t%s = %s(%s)\n", tag.id, r.id, tag.val)
		}
		fmt.Fprintf(out, ")\n");
		fmt.Fprintf(out, "var _%s_names = map[int32]string{\n", r.id);
		for _, tag := range r.tags {
			fmt.Fprintf(out, "\tint32(%s): \"%s\",\n", tag.id, tag.id);
		}
		fmt.Fprintf(out, "}\n");
		fmt.Fprintf(out, "func (*%s) EnumNames() map[int32]string {\n" +
			"\treturn _%s_names\n}\n", r.id, r.id)
		fmt.Fprintf(out, "func (v *%s) EnumVal() *int32 {\n" +
			"\treturn (*int32)(v)\n" +
			"}\n", r.id)
		fmt.Fprintf(out, "func (v *%s) String() string {\n" +
			"\tif s, ok := _%s_names[int32(*v)]; ok {\n" +
			"\t\treturn s\n\t}\n" +
			"\treturn \"unknown_%s\"\n}\n",
			r.id, r.id, r.id)
	case *rpc_decl:
		panic("rpc_decl shouldn't happen here")
	case *rpc_typedef:
		fmt.Fprintf(out, "type %s = %s\n", r.id,
			e.decltype(sym, (*rpc_decl)(r)))
	case *rpc_struct:
		fmt.Fprintf(out, "type %s struct {\n", r.id);
		for _, decl := range r.decls {
			fmt.Fprintf(out, "\t%s %s\n", decl.id, e.decltype(sym, &decl))
		}
		fmt.Fprintf(out, "}\n")
	case *rpc_union:
		fmt.Fprintf(out, "type %s struct {\n", r.id);
		fmt.Fprintf(out, "\t%s %s\n", r.tagid, r.tagtype);
		fmt.Fprintf(out, "\t_u interface{}\n");
		fmt.Fprintf(out, "}\n");
		for _, u := range r.fields {
			if u.decl.id == "" || u.decl.typ == "void" {
				continue
			}
			ret := e.decltype(sym, &u.decl)
			fmt.Fprintf(out, "func (u *%s) %s() *%s {\n", r.id, u.decl.id, ret)
			goodcase := fmt.Sprintf("\t\tif v, ok := u._u.(*%s); ok {\n" +
				"\t\t\treturn v\n" +
				"\t\t} else {\n" +
				"\t\t\tvar zero %s\n" +
				"\t\t\tu._u = &zero\n" +
				"\t\t\treturn &zero\n" +
				"\t\t}\n", ret, ret)
			badcase := fmt.Sprintf(
				"\t\tpanic(\"%s accessed when not selected\")\n", u.decl.id)
			fmt.Fprintf(out, "\tswitch u.%s {\n", r.tagid);
			if u.hasdefault && len(r.fields) > 1 {
				needcomma := false
				fmt.Fprintf(out, "\tcase ");
				for _, u1 := range r.fields {
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
			fmt.Fprintf(out, "\t}\n");
		    fmt.Fprintf(out, "}\n")
		}
	default:
		fmt.Fprintf(out, "%T%v\n", r, r);
	}
	e.declarations = append(e.declarations, out.String())
}

/*
func (e *emitter) traverse_field(out io.Writer, name string, sym rpc_sym) {
}
*/

func (e *emitter) traverse(sym rpc_sym) {
	out := &strings.Builder{}
	switch r := sym.(type) {
	case *rpc_const:
		return
	case *rpc_enum:
		fmt.Fprintf(out, "func (v *%s) XdrTraverse(x XDR, name string) {\n" +
			"\tx.enum(v, name)\n" +
			"}\n", r.id)
	}
	e.declarations = append(e.declarations, out.String())
}

func emit(syms *rpc_syms) {
	e := emitter{
		declarations: []string{},
		syms: syms,
	}

	e.declarations = append(e.declarations, fmt.Sprintf("package main\n"))
	for _, s := range syms.Symbols  {
		e.declarations = append(e.declarations, "\n")
		e.emit(s)
		e.traverse(s)
	}
	for _, d := range e.declarations {
		io.WriteString(os.Stdout, d)
	}
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
		emit(&syms)
	}
}
