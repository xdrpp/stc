package main

//go:generate stringer -type qual_t syms.go
type qual_t int
const (
	SCALAR qual_t = iota
	PTR
	ARRAY
	VEC
)

type rpc_decl struct {
	id string
	qual qual_t
	bound string

	typ string
	inline_decl rpc_sym
}

type rpc_typedef rpc_decl

type rpc_const struct {
	id, val string
}

type rpc_struct struct {
	id string
	decls []rpc_decl
}

type rpc_enum struct {
	id string
	tags []rpc_const
}

type rpc_ufield struct {
	cases []string
	decl rpc_decl
	hasdefault bool
}

type rpc_union struct {
	id, tagtype, tagid string
	fields []rpc_ufield
	hasdefault bool
}

type rpc_proc struct {
	id string
	val uint32
	arg []string
	res string
}

type rpc_vers struct {
	id string
	val uint32
	procs []rpc_proc
}

type rpc_program struct {
	id string
	val uint32
	vers []rpc_vers
}

func (r *rpc_decl) symid() *string { return &r.id }
func (r *rpc_typedef) symid() *string { return &r.id }
func (r *rpc_const) symid() *string { return &r.id }
func (r *rpc_struct) symid() *string { return &r.id }
func (r *rpc_enum) symid() *string { return &r.id }
func (r *rpc_union) symid() *string { return &r.id }
func (r *rpc_program) symid() *string { return &r.id }

type rpc_sym interface {
	symid() *string
}

type rpc_syms struct {
	SymbolMap map[string]rpc_sym
	Symbols []rpc_sym
	Failed bool
}
