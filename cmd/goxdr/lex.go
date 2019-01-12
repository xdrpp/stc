package main

import "fmt"
import "os"
import "strings"

type TokenType = int

type Token struct {
	Type TokenType
	Value string
}

const EOFRune rune = -1

type Lexer struct {
	filename string
	input string
	lineno int
	midline bool
	output *rpc_syms
	Package string
	newsymbols map[string]struct{}
}

func NewLexer(out *rpc_syms, filename, input string) *Lexer {
	if out.Symbols == nil {
		out.Symbols = []rpc_sym{}
		out.SymbolMap = map[string]rpc_sym{}
	}
	return &Lexer{
		filename: filename,
		input: input,
		lineno: 1,
		output: out,
		newsymbols: map[string]struct{}{},
	}
}

var keywords map[string]int = map[string]int {
	"const": T_CONST,
	"struct": T_STRUCT,
	"union": T_UNION,
	"enum": T_ENUM,
	"typedef": T_TYPEDEF,
	"program": T_PROGRAM,
	"namespace": T_NAMESPACE,
	"bool": T_BOOL,
	"unsigned": T_UNSIGNED,
	"int": T_INT,
	"hyper": T_HYPER,
	"float": T_FLOAT,
	"double": T_DOUBLE,
	"quadruple": T_QUADRUPLE,
	"void": T_VOID,
	"version": T_VERSION,
	"switch": T_SWITCH,
	"case": T_CASE,
	"default": T_DEFAULT,
	"opaque": T_OPAQUE,
	"string": T_STRING,
}

func (l *Lexer) at(i int) rune {
	if i < 0 || i >= len(l.input) {
		return EOFRune
	}
	return rune(l.input[i])
}

func (l *Lexer) advance(length int) {
	if length < 0 || length > len(l.input) {
		panic("Lexer::advance: length out of range")
	}
	if length > 0 {
		l.midline = l.input[length-1] != '\n'
		l.lineno += strings.Count(l.input[:length], "\n")
		l.input = l.input[length:]
	}
}

func (l *Lexer) makeToken(typ TokenType, n int) *Token {
	if n < 0 || n > len(l.input) {
		panic("Lexer::makeToken: length out of range")
	}
	t := Token {
		Type: typ,
		Value: l.input[:n],
	}
	l.advance(n)
	return &t
}

func (l *Lexer) skipSpace() {
	for {
		if i := strings.IndexFunc(l.input, func(c rune) bool {
			return strings.IndexAny(" \t\r\n", string(c)) == -1
		}); i > 0 {
			l.advance(i)
		} else if i < 0 {
			l.advance(len(l.input))
			return
		} else if strings.HasPrefix(l.input, "//") {
				l.skipLine()
		} else if strings.HasPrefix(l.input, "/*") {
			i := strings.Index(l.input[2:], "*/")
			if i < 0 {
				return
			}
			l.advance(i+4)
		} else {
			return
		}
	}
}

func (l *Lexer) skipLine() {
	if i := strings.IndexByte(l.input, '\n'); i > 0 {
		l.advance(i+1)
	}
}

func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

func isHexDigit(c rune) bool {
	if isDigit(c) {
		return true
	}
	c &^= 0x20
	return c >= 'A' && c <= 'F'
}

func isIdStart(c rune) bool {
	if (c == '_') {
		return true
	}
	c &^= 0x20
	return c >= 'A' && c <= 'Z'
}

func isIdRest(c rune) bool {
	return isIdStart(c) || isDigit(c)
}

func (l *Lexer) identifier() *Token {
	if !isIdStart(l.at(0)) {
		return nil
	}
	i := 1
	for ; isIdRest(l.at(i)); i++ {
	}
	return l.makeToken(T_ID, i)
}

func (l *Lexer) integer() *Token {
	i, c := 0, l.at(0)
	if (c == '+' || c == '-') {
		i, c = 1, l.at(1)
	}
	if !isDigit(c) {
		return nil
	}
	if l.at(i) == '0' && l.at(i+1)|0x20 == 'x' && isHexDigit(l.at(i+2)) {
		for i += 2; isHexDigit(l.at(i)); i++ {
		}
	} else {
		for i++; isDigit(l.at(i)); i++ {
		}
	}
	return l.makeToken(T_NUM, i)
}

func (l *Lexer) next() *Token {
again:
	l.skipSpace()
	switch c := l.at(0); {
	case c == EOFRune:
		return nil
	case strings.ContainsRune("=;{}<>[]*,:()", c):
		return l.makeToken(TokenType(c), 1)
	case isIdStart(c):
		t := l.identifier()
		if kw, ok := keywords[t.Value]; ok {
			t.Type = kw
		}
		return t
	case isDigit(c) || c == '+' || c == '-':
		return l.integer()
	case c == '%' && !l.midline:
		l.skipLine()
		goto again
	default:
		panic(fmt.Sprintf("%s:%d: bad character %q",
			l.filename, l.lineno, c))
	}
}

func (l *Lexer) Checkdup(sym string) {
	if _, exists := l.newsymbols[sym]; exists {
		l.Error(sym + " re-defined")
	} else {
		l.newsymbols[sym] = struct{}{}
	}
}

func (l *Lexer) Lex(lval *yySymType) int {
	t := l.next()
	if (t == nil) {
		return 0
	} else {
		lval.str = t.Value
	}
	return t.Type
}

func (l *Lexer) Error(e string) {
	l.Warn(e)
	l.output.Failed = true
}

func (l *Lexer) Warn(e string) {
	fmt.Fprintf(os.Stderr, "%s:%d: %s\n", l.filename, l.lineno, e)
}
