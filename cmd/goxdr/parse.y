%{

package main

import "strconv"

%}

%union {
	tok Token
	idval idval
	num uint32
	symlist []rpc_sym
	def rpc_sym
	cnst rpc_const
	constlist []rpc_const
	decl rpc_decl
	decllist []rpc_decl
	ufield rpc_ufield
	ubody rpc_union
	arrayqual struct{ qual qual_t; bound idval }
	prog rpc_program
	vers rpc_vers
	proc rpc_proc
	comment string
}

%token <tok> T_ID
%token <tok> T_NUM
%token <tok> T_CONST
%token <tok> T_STRUCT
%token <tok> T_UNION
%token <tok> T_ENUM
%token <tok> T_TYPEDEF
%token <tok> T_PROGRAM
%token <tok> T_NAMESPACE

%token <tok> T_BOOL
%token <tok> T_UNSIGNED
%token <tok> T_INT
%token <tok> T_HYPER
%token <tok> T_FLOAT
%token <tok> T_DOUBLE
%token <tok> T_QUADRUPLE
%token <tok> T_VOID

%token <tok> T_VERSION
%token <tok> T_SWITCH
%token <tok> T_CASE
%token <tok> T_DEFAULT

%token <tok> T_OPAQUE
%token <tok> T_STRING

%type <def> definition def_enum def_const def_namespace def_type
%type <def> def_struct def_union def_program
%type <constlist> enum_body enum_tag_list
%type <cnst> enum_tag
%type <decllist> struct_body declaration_list
%type <decl> declaration type_specifier
%type <decl> union_decl
%type <prog> version_list
%type <vers> version_decl proc_list
%type <proc> proc_decl void_or_arg_list arg_list
%type <ubody> union_case_spec_list union_body
%type <ufield> union_case_list union_case_spec
%type <idval> id newid qid value vec_len type base_type union_case type_or_void
%type <num> number
%type <arrayqual> vec_qual array_qual any_qual
%type <comment> comma_warn
%type <tok> ';' ','

%%

file:
	/* empty */
|	file definition
	{
		if $2 != nil {
			syms := yylex.(*Lexer).output
			syms.Symbols = append(syms.Symbols, $2)
			syms.SymbolMap[$2.getsym().getx()] = $2
		}
	}
	;

definition:
	def_namespace
| 	def_const
|	def_enum
|	def_struct
|	def_union
|	def_type
|	def_program
	;

def_namespace: T_NAMESPACE T_ID '{' file '}'
	{
		$$ = nil
	};

def_const: T_CONST newid '=' value ';'
	{
		$$ = &rpc_const{ $2, $4, nonEmpty($1.BlockComment, $5.LineComment) }
	};

def_enum: T_ENUM newid enum_body ';'
	{
		$$ = &rpc_enum{ $2, $3, $1.BlockComment }
	};

enum_body: '{' enum_tag_list comma_warn '}'
	{
		$$ = $2
		$$[len($$)-1].comment = nonEmpty($$[len($$)-1].comment, $3)
	};

enum_tag_list: enum_tag
	{
		$$ = []rpc_const{$1}
	}
|	enum_tag_list ',' enum_tag
	{
		last := &$1[len($1)-1]
		last.comment = nonEmpty($2.LineComment, last.comment)
		$$ = append($1, $3)
	};

enum_tag: newid '=' value
	{
		tag := rpc_const{$1, $3, nonEmpty($1.comment, $3.comment) }
		yylex.(*Lexer).output.SymbolMap[$1.getx()] = &tag
		$$ = tag
	}
| newid
	{
		yylex.(*Lexer).Warn("RFC4506 requires a value for each enum tag")
		$$ = rpc_const{$1, lid("iota"), $1.comment}
	};

comma_warn: /* empty */ { $$ = "" }
|	','
	{
		$$ = $1.LineComment
		yylex.(*Lexer).
		Warn("RFC4506 disallows comma after last enum tag")
	};

def_struct: T_STRUCT newid struct_body ';'
	{
		$$ = &rpc_struct{$2, $3, $1.BlockComment}
	};

struct_body: '{' declaration_list '}'
	{
		$$ = $2
	}

declaration_list: declaration
	{
		$$ = []rpc_decl{$1}
	}
| declaration_list declaration
	{
		$$ = append($1, $2)
	};

def_union: T_UNION newid union_body ';'
	{
		ret := $3				// Copy it
		ret.id = $2
		ret.comment = $1.BlockComment
		$$ = &ret
	};

union_body: T_SWITCH '(' type id ')' '{' union_case_spec_list '}'
	{
		$$ = $7
		$$.tagtype = $3
		$$.tagid = $4
	};

union_case_spec_list: union_case_spec
	{
		$$ = rpc_union{
			fields: []rpc_ufield{$1},
			hasdefault:$1.hasdefault,
		}
	}
|	union_case_spec_list union_case_spec
	{
		$$ = $1
		if !$$.hasdefault {
			$$.fields = append($$.fields, $2)
			$$.hasdefault = $2.hasdefault
		} else if !$2.hasdefault {
			n := len($$.fields)
			$$.fields = append($$.fields, $$.fields[n])
			$$.fields[n] = $2
		} else {
			yylex.Error("duplicate default case")
		}
	};

union_case_spec: union_case_list union_decl
	{
		$$ = $1
		$$.decl = $2
	};

union_case_list: union_case
	{
		$$ = rpc_ufield{}
		if $1.getx() == "" {
			$$.hasdefault = true
			$$.cases = []idval{}
		} else {
			$$.cases = []idval{$1}
		}
	}
| union_case_list union_case
	{
		$$ = $1
		if $2.getx() != "" {
			$$.cases = append($$.cases, $2)
		} else if !$$.hasdefault {
			$$.hasdefault = true
		} else {
			yylex.Error("duplicate default case")
		}
	};

union_case: T_CASE value ':' {
	$$ = $2
}
| T_DEFAULT ':' { $$.setlocal("") }

union_decl: declaration
| T_VOID ';'
	{
		$$ = rpc_decl{qual: SCALAR, typ: lid("void")}
	};

def_type: T_TYPEDEF declaration
	{
		ret := rpc_typedef($2)
		if $1.BlockComment != "" {
			ret.comment = $1.BlockComment
		}
		$$ = &ret
	};

declaration: type_specifier id any_qual ';'
	{
		$$ = $1
		$$.id = $2
		$$.qual = $3.qual
		$$.bound = $3.bound
		$$.comment = nonEmpty($1.comment, $4.LineComment)
	}
| type_specifier '*' id ';'
	{
		$$ = $1
		$$.id = $3
		$$.qual = PTR
		$$.comment = nonEmpty($1.comment, $4.LineComment)
	}
| T_OPAQUE id array_qual ';'
	{
		$$.id = $2
		$$.typ.setlocal("byte")
		$$.qual = $3.qual
		$$.bound = $3.bound
		$$.comment = nonEmpty($1.BlockComment, $4.LineComment)
	}
| T_STRING id vec_qual ';'
	{
		$$.id = $2
		$$.typ.setlocal("string")
		$$.qual = SCALAR
		$$.bound = $3.bound
		$$.comment = nonEmpty($1.BlockComment, $4.LineComment)
	}
	;

vec_qual: '<' vec_len '>'
	{
		$$.qual = VEC
		$$.bound = $2
	}

array_qual: vec_qual
| '[' value ']'
	{
		$$.qual = ARRAY
		$$.bound = $2
	}

any_qual: array_qual
| /* empty */
	{
		$$.qual = SCALAR
	}

type_specifier: type
	{
		$$ = rpc_decl{typ: $1, comment: $1.comment}
	}
| T_ENUM enum_body
	{
		$$ = rpc_decl{inline_decl: &rpc_enum{tags: $2},
			comment : $1.BlockComment}
	}
| T_STRUCT struct_body
	{
		$$ = rpc_decl{inline_decl: &rpc_struct{decls: $2,
			comment: $1.BlockComment}}
	}
| T_UNION union_body
	{
		decl := $2
		decl.comment = $1.BlockComment
		$$.inline_decl = &decl
	};

def_program: T_PROGRAM newid '{' version_list '}' '=' number ';'
	{
		ret := $4
		ret.id = $2
		ret.val = $7
		$$ = &ret
	};

version_list: version_decl
	{
		$$.vers = []rpc_vers{$1}
	}
| version_list version_decl
	{
		$$ = $1
		$$.vers = append($1.vers, $2)
	};

version_decl: T_VERSION newid '{' proc_list '}'  '=' number ';'
	{
		$$ = $4
		$$.id = $2
		$$.val = $7
	};

proc_list: proc_decl
	{
		$$.procs = []rpc_proc{$1}
	}
| proc_list proc_decl
	{
		$$.procs = append($1.procs, $2)
	};

proc_decl: type_or_void newid '(' void_or_arg_list ')' '=' number ';'
	{
		$$ = $4
		$$.res = $1
		$$.id = $2
		$$.val = $7
	};

type_or_void: type
| T_VOID
	{
		$$.setglobal("XdrVoid")
		$$.xid = "void"
	};

void_or_arg_list: T_VOID { $$ = rpc_proc{} } | arg_list;

arg_list: type
	{
		$$ = rpc_proc{arg: []idval{$1}}
	}
| arg_list ',' type
	{
		$$.arg = append($$.arg, $3)
	};

type: base_type | qid;

base_type: T_INT { $$.setlocal("int32"); $$.comment = $1.BlockComment }
| T_BOOL { $$.setlocal("bool"); $$.comment = $1.BlockComment }
| T_UNSIGNED T_INT { $$.setlocal("uint32"); $$.comment = $1.BlockComment }
| T_UNSIGNED {
		yylex.(*Lexer).
			Warn("RFC4506 requires \"int\" after \"unsigned\"")
		$$.setlocal("uint32")
		$$.comment = $1.BlockComment
	}
| T_HYPER { $$.setlocal("int64"); $$.comment = $1.BlockComment }
| T_UNSIGNED T_HYPER { $$.setlocal("uint64"); $$.comment = $1.BlockComment }
| T_FLOAT { $$.setlocal("float32"); $$.comment = $1.BlockComment }
| T_DOUBLE { $$.setlocal("float64"); $$.comment = $1.BlockComment }
| T_QUADRUPLE { $$.setlocal("float128"); $$.comment = $1.BlockComment }
  ;

vec_len: value
| /* empty */ { $$.setlocal("") };

value: qid
| T_NUM
	{
		$$ = gid($1.Value)
		$$.comment = $1.LineComment
	}
	;

number: T_NUM
	{
		val, err := strconv.ParseUint($1.Value, 0, 32)
		if err != nil {
			yylex.Error(err.Error() + " at " + $1.Value)
		}
		$$ = uint32(val)
	}

newid: T_ID
	{
		yylex.(*Lexer).Checkdup($1.Value)
		$$ = gid($1.Value)
		$$.comment = nonEmpty($1.BlockComment, $1.LineComment)
		if $$.xid != "" && $$.xid[0] == '_' {
			yylex.(*Lexer).Warn(
				"RFC4506 disallows '_' as first character of identifier")
		}
	};

qid: T_ID		// might make sense for qid to allow package-qualified
	{
		$$ = gid($1.Value)
		$$.comment = nonEmpty($1.BlockComment, $1.LineComment)
		switch $$.xid {
		case "TRUE":
			$$.goid = "true"
		case "FALSE":
			$$.goid = "false"
		}
	};
id: T_ID
	{
		$$ = gid($1.Value)
		$$.comment = nonEmpty($1.BlockComment, $1.LineComment)
		if $$.xid != "" && $$.xid[0] == '_' {
			yylex.(*Lexer).Warn(
				"RFC4506 disallows '_' as first character of identifier")
		}
	};

%%

func nonEmpty(args ...string) string {
	for i := range args {
		if args[i] != "" {
			return args[i]
		}
	}
	return ""
}
