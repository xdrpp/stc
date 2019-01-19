package main

type qual_t int
const (
	SCALAR qual_t = iota
	PTR
	ARRAY
	VEC
)

type idval struct {
	xid string		   // The symbol in the xdr file
	goid string		   // The name we should use in go
	comment string	   // block comment (XXX only used during parsing)
}
func (iv *idval) getx() string { return iv.xid }
func (iv *idval) getgo() string { return iv.goid }
func (iv *idval) setglobal(s string) { iv.xid = s; iv.goid = capitalize(s) }
func (iv *idval) setlocal(s string) { iv.xid = s; iv.goid = s }
func (iv idval) String() string { return iv.getgo() }
func gid(s string) (iv idval) { iv.setglobal(s); return }
func lid(s string) (iv idval) { iv.setlocal(s); return }

type rpc_decl struct {
	id idval
	qual qual_t
	bound idval

	typ idval
	inline_decl rpc_sym
	comment string
}

type rpc_typedef rpc_decl

type rpc_const struct {
	id, val idval
	comment string
}

type rpc_struct struct {
	id idval
	decls []rpc_decl
	comment string
}

type rpc_enum struct {
	id idval
	tags []rpc_const
	comment string
}

type rpc_ufield struct {
	cases []idval
	decl rpc_decl
	hasdefault bool
}

type rpc_union struct {
	id, tagtype, tagid idval
	fields []rpc_ufield
	hasdefault bool
	comment string
}

type rpc_proc struct {
	id idval
	val uint32
	arg []idval
	res idval
}

type rpc_vers struct {
	id idval
	val uint32
	procs []rpc_proc
}

type rpc_program struct {
	id idval
	val uint32
	vers []rpc_vers
}

func (r *rpc_decl) getsym() *idval { return &r.id }
func (r *rpc_typedef) getsym() *idval { return &r.id }
func (r *rpc_const) getsym() *idval { return &r.id }
func (r *rpc_struct) getsym() *idval { return &r.id }
func (r *rpc_enum) getsym() *idval { return &r.id }
func (r *rpc_union) getsym() *idval { return &r.id }
func (r *rpc_program) getsym() *idval { return &r.id }

type rpc_sym interface {
	getsym() *idval
}

type rpc_syms struct {
	SymbolMap map[string]rpc_sym
	Symbols []rpc_sym
	Failed bool
}
