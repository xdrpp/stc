package main

import "fmt"
import "os"
import "strings"

const tabwidth = 8

type TokenType = int

type Token struct {
	Type TokenType
	Value string
	BlockComment string
	LineComment string
}

const EOFRune rune = -1

type Lexer struct {
	filename string
	input string
	lineno, colno int
	lastTokLine, lastTokCol int
	midline bool
	output *rpc_syms
	blockComment string
	Package string
	newsymbols map[string]struct{}
}

func NewLexer(out *rpc_syms, filename, input string) *Lexer {
	if out.Symbols == nil {
		out.Symbols = []rpc_sym{}
		out.SymbolMap = map[string]rpc_sym{}
	}
	l := &Lexer{
		filename: filename,
		input: input,
		lineno: 1,
		output: out,
		newsymbols: map[string]struct{}{},
	}
	l.clearComment()
	return l
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
	for i := 0; i < length; i++ {
		switch l.input[i] {
		case '\n':
			l.lineno++
			l.colno = 0
			l.midline = false
		case '\t':
			l.colno += 8 - (l.colno % tabwidth)
		case ' ':
			l.colno++
		default:
			l.colno++
			l.midline = true
		}
	}
	l.input = l.input[length:]
}

func (l *Lexer) findEOL() int {
	n := strings.IndexByte(l.input, '\n')
	if n > 0 && l.input[n-1] == '\r' {
		return n-1
	} else if n >= 0 {
		return n
	}
	return len(l.input)
}

func (l *Lexer) RestOfLine() string {
	i := l.findEOL()
	ret := l.input[:i]
	l.advance(i)
	l.skipLine()
	return ret
}

func (l *Lexer) skipWS() bool {
	i := strings.IndexFunc(l.input, func(c rune) bool {
		return c != ' ' && c != '\t'
	})
	if i > 0 {
		l.advance(i)
		return true
	}
	return false
}

func (l *Lexer) skipWSNL() bool {
	l.skipWS()
	n := len(l.input)
	if n >= 1 && l.input[0] == '\n' {
		l.advance(1)
		return true
	}
	if n >= 2 && l.input[0:2] == "\r\n" {
		l.advance(2)
		return true
	}
	return false
}

// Find a comment start, where tp should be '*' for block comments and
// '/' for line comments.
func (l *Lexer) findCommentStart(tp byte) bool {
	i := strings.IndexFunc(l.input, func(c rune) bool {
		return c != ' ' && c != '\t'
	})
	if i >= 0 && len(l.input) - i >= 2 &&
		l.input[i] == '/' && l.input[i+1] == tp {
		l.advance(i)
		return true
	}
	return false
}

func (l *Lexer) getLineComment() string {
	if !l.findCommentStart('/') {
		return ""
	}
	if l.midline {
		return l.RestOfLine()
	}
	out, column := strings.Builder{}, -1
	for l.findCommentStart('/') {
		if l.colno != column {
			out.Reset()
			column = l.colno
		} else {
			out.WriteByte('\n')
		}
		out.WriteString(l.RestOfLine())
	}
	return out.String()
}

func stripToColumn(column int, input string) string {
	i, c := 0, 0
	for ; c < column && i < len(input); i++ {
		if input[i] == '\t' {
			c = (c + tabwidth) % tabwidth
		} else if input[i] == ' ' {
			c++
		} else {
			break
		}
	}
	if c <= column {
		return input[i:]
	}
	return strings.Repeat(" ", c - column) + input[i:]
}

func (l *Lexer) skipComment() bool {
	wasMidline := l.midline
	if comment := l.getLineComment(); comment != "" {
		l.clearComment()
		if !wasMidline {
			l.blockComment = comment
		}
		return true
	}
	if !l.findCommentStart('*') {
		return false
	}

	l.clearComment()
	column, out := l.colno, &strings.Builder{}
	j := strings.Index(l.input[2:], "*/")
	if j < 0 {
		return true
	}
	comment := l.input[:j+4]
	l.advance(j+4)
	if wasMidline || !l.skipWSNL() {
		return true
	}

	first := true
	for _, line := range strings.Split(comment, "\n") {
		if first {
			first = false
		} else {
			out.WriteByte('\n')
			line = stripToColumn(column, line)
		}
		line = strings.TrimSuffix(line, "\r")
		out.WriteString(line)
	}
	l.blockComment = out.String()
	return true
}

func (l *Lexer) makeToken(typ TokenType, n int) *Token {
	if n <= 0 || n > len(l.input) {
		panic("Lexer::makeToken: length out of range")
	}
	l.lastTokLine = l.lineno
	l.lastTokCol = l.colno
	t := Token {
		Type: typ,
		Value: l.input[:n],
		BlockComment: l.blockComment,
	}
	l.clearComment()
	l.advance(n)
	if com := l.getLineComment(); com != "" {
		t.LineComment = com
	}
	return &t
}

func (l *Lexer) clearComment() {
	l.blockComment = ""
}

func (l *Lexer) skipSpace() {
	for {
		com := l.skipComment()
		nl := l.skipWSNL()
		if nl {
			l.clearComment()
		} else if !com {
			return
		}
	}
}

func (l *Lexer) skipLine() {
	if i := strings.IndexByte(l.input, '\n'); i >= 0 {
		l.advance(i+1)
	} else {
		l.advance(len(l.input))
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
	for {
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
		case c == '%' && l.colno == 0:
			l.skipLine()
		default:
			l.makeToken(-1, 1)
			l.Error(fmt.Sprintf("bad character %q", c))
		}
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
	}
	lval.tok = *t
	return t.Type
}

func (l *Lexer) Error(e string) {
	l.Warn(e)
	l.output.Failed = true
}

func (l *Lexer) Warn(e string) {
	fmt.Fprintf(os.Stderr, "%s:%d:%d: %s\n", l.filename, l.lastTokLine,
		l.lastTokCol+1, e)
}
