%{

package main

import "strconv"

%}

%union {
	str string
	num uint32
	symlist []rpc_sym
	def rpc_sym
	cnst rpc_const
	constlist []rpc_const
	decl rpc_decl
	decllist []rpc_decl
	ufield rpc_ufield
	ubody rpc_union
	arrayqual struct{ qual qual_t; bound string }
	prog rpc_program
	vers rpc_vers
	proc rpc_proc
}

%token <str> T_ID
%token <str> T_NUM

%token T_CONST
%token T_STRUCT
%token T_UNION
%token T_ENUM
%token T_TYPEDEF
%token T_PROGRAM
%token T_NAMESPACE

%token T_BOOL
%token T_UNSIGNED
%token T_INT
%token T_HYPER
%token T_FLOAT
%token T_DOUBLE
%token T_QUADRUPLE
%token T_VOID

%token T_VERSION
%token T_SWITCH
%token T_CASE
%token T_DEFAULT

%token<str> T_OPAQUE
%token<str> T_STRING

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
%type <str> id newid qid value vec_len type base_type union_case type_or_void
%type <num> number
%type <arrayqual> vec_qual array_qual any_qual

%%

file:
	/* empty */
|	file definition
	{
		if $2 != nil {
			syms := yylex.(*Lexer).output
			syms.Symbols = append(syms.Symbols, $2)
			syms.SymbolMap[*$2.symid()] = $2
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

def_namespace: T_NAMESPACE T_ID
	{
		l := yylex.(*Lexer)
		if l.output.Package == "" {
			l.output.Package = $2
		}
	}
	'{' file '}'
	{
		$$ = nil
	};

def_const: T_CONST newid '=' value ';'
	{
		$$ = &rpc_const{ $2, $4 }
	};

def_enum: T_ENUM newid enum_body ';'
	{
		$$ = &rpc_enum{ $2, $3 }
	};

enum_body: '{' enum_tag_list comma_warn '}'
	{
		$$ = $2
	};

enum_tag_list: enum_tag
	{
		$$ = []rpc_const{$1}
	}
|	enum_tag_list ',' enum_tag
	{
		$$ = append($1, $3)
	};

enum_tag: newid '=' value
	{
		tag := rpc_const{$1, $3}
		yylex.(*Lexer).output.SymbolMap[$1] = &tag
		$$ = tag
	}
| newid
	{
	  yylex.(*Lexer).Warn("RFC4506 requires a value for each enum tag");
		$$ = rpc_const{$1, "iota"}
	};

comma_warn: /* empty */
|	','
	{
		yylex.(*Lexer).
		Warn("RFC4506 disallows comma after last enum tag")
	};

def_struct: T_STRUCT newid struct_body ';'
	{
		$$ = &rpc_struct{$2, $3}
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
		if $1 == "" {
			$$.hasdefault = true
			$$.cases = []string{}
		} else {
			$$.cases = []string{$1}
		}
	}
| union_case_list union_case
	{
		$$ = $1
		if $2 != "" {
			$$.cases = append($$.cases, $2)
		} else if !$$.hasdefault {
			$$.hasdefault = true
		} else {
			yylex.Error("duplicate default case")
		}
	};

union_case: T_CASE value ':' { $$ = $2 }
| T_DEFAULT ':' { $$ = "" }

union_decl: declaration
| T_VOID ';'
	{
		$$ = rpc_decl{qual: SCALAR, typ: "void"}
	};

def_type: T_TYPEDEF declaration
	{
		ret := rpc_typedef($2)
		$$ = &ret
	};

declaration: type_specifier id any_qual ';'
	{
		$$ = $1
		$$.id = $2
		$$.qual = $3.qual
		$$.bound = $3.bound
	}
| type_specifier '*' id ';'
	{
		$$ = $1
		$$.id = $3
		$$.qual = PTR
	}
| T_OPAQUE id array_qual ';'
	{
		$$.id = $2
		$$.typ = "byte"
		$$.qual = $3.qual
		$$.bound = $3.bound
	}
| T_STRING id vec_qual ';'
	{
		$$.id = $2
		$$.typ = "string"
		$$.qual = SCALAR
		$$.bound = $3.bound
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
		$$ = rpc_decl{typ: $1}
	}
| T_ENUM enum_body
	{
		$$ = rpc_decl{inline_decl: &rpc_enum{tags: $2}}
	}
| T_STRUCT struct_body
	{
		$$ = rpc_decl{inline_decl: &rpc_struct{decls: $2}}
	}
| T_UNION union_body
	{
		decl := $2
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

type_or_void: type | T_VOID { $$ = "xdr.Void" };

void_or_arg_list: T_VOID { $$ = rpc_proc{} } | arg_list;

arg_list: type
	{
		$$ = rpc_proc{arg: []string{$1}}
	}
| arg_list ',' type
	{
		$$.arg = append($$.arg, $3)
	};

type: base_type | qid;

base_type: T_INT { $$ = "int32" }
| T_BOOL { $$ = "bool" }
| T_UNSIGNED T_INT { $$ = "uint32" }
| T_UNSIGNED {
		yylex.(*Lexer).
			Warn("RFC4506 requires \"int\" after \"unsigned\"")
		$$ = "uint32"
	}
| T_HYPER { $$ = "int64" }
| T_UNSIGNED T_HYPER { $$ = "uint64" }
| T_FLOAT { $$ = "float32" }
| T_DOUBLE { $$ = "float64" }
| T_QUADRUPLE { $$ = "float128" }
  ;

vec_len: value
| /* empty */ { $$ = "" };

value: qid | T_NUM;

number: T_NUM
	{
		val, err := strconv.ParseUint($1, 0, 32)
		if err != nil {
			yylex.Error(err.Error() + " at " + $1)
		}
		$$ = uint32(val)
	}

newid: T_ID
	{
		yylex.(*Lexer).Checkdup($1)
		$$ = capitalize($1)
	};

qid: id;	// might make sense for qid to allow package-qualified
id: T_ID { $$ = capitalize($1) };

%%
