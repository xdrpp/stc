package stcdetail

import "bytes"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "strings"

const tabwidth = 8
const eofRune rune = -1

// Section of an INI file.
type IniSection struct {
	Section    string
	Subsection *string
}

func (s IniSection) String() string {
	if s.Subsection != nil {
		ret := strings.Builder{}
		fmt.Fprintf(&ret, "[%s \"", s.Section)
		for i := 0; i < len(*s.Subsection); i++ {
			switch b := (*s.Subsection)[i]; b {
			case '\n', '\000':
				panic("illegal character in IniSection Subsection")
			case '\\', '"':
				ret.WriteByte('\\')
				fallthrough
			default:
				ret.WriteByte(b)
			}
		}
		ret.WriteString("\"]")
		return ret.String()
	}
	return fmt.Sprintf("[%s]", s.Section)
}

func (s *IniSection) Eq(s2 *IniSection) bool {
	if s == nil && s2 == nil {
		return true
	} else if s == nil || s2 == nil {
		return false
	} else if s.Section != s2.Section {
		return false
	} else if s.Subsection == nil && s2.Subsection == nil {
		return true
	} else if s.Subsection == nil || s2.Subsection == nil {
		return false
	}
	return *s.Subsection == *s2.Subsection
}

type IniRange struct {
	StartIndex, EndIndex int
}

type IniItem struct {
	*IniSection
	Key string
	Value *string
	IniRange
}

// Type that receives and processes the parsed INI file.
type IniSink interface {
	Item(IniItem) error
}

type IniSecStart struct {
	IniSection
	IniRange
}

// If an IniSink also implements IniSecSink, then it will receive a
// callback for each new section of the file.  This allows the cost of
// looking up a section to be amortized over multiple key=value pairs.
// (The Value method of an IniSecSink can reasonably ignore its sec
// argument.)
type IniSecSink interface {
	IniSink
	Section(sec IniSecStart) error
}

// Error that an IniSink's Value method should return when there is a
// problem with the key, rather than a problem with the value.  By
// default, the line and column number of an error will correspond to
// the start of the value, but with BadKey the error will point to the
// key.
type BadKey string

func (err BadKey) Error() string {
	return string(err)
}

// Just a random error type useful for bad values in INI files.
// Exists for symmetry with BadKey, though BadValue is in no way
// special.
type BadValue string

func (err BadValue) Error() string {
	return string(err)
}

// A single parse error in an IniFile.
type ParseError struct {
	File          string
	Lineno, Colno int
	Msg           string
}

func (err ParseError) Error() string {
	if err.File == "" {
		return fmt.Sprintf("%d:%d: %s", err.Lineno, err.Colno, err.Msg)
	}
	return fmt.Sprintf("%s:%d:%d: %s", err.File, err.Lineno, err.Colno, err.Msg)
}

// The collection of parse errors that resulted from parsing a file.
type ParseErrors []ParseError

func (err ParseErrors) Error() string {
	ret := &strings.Builder{}
	for i, e := range err {
		if i != 0 {
			ret.WriteByte('\n')
		}
		ret.WriteString(e.Error())
	}
	return ret.String()
}

type position struct {
	index, lineno, colno int
}

type iniParse struct {
	position
	input   []byte
	file    string
	sec     *IniSection
	Value   func(IniItem) error
	Section func(sec IniSecStart) error
}

func (l *iniParse) throwAt(pos position, msg string) {
	panic(ParseError{
		File:   l.file,
		Lineno: pos.lineno + 1,
		Colno:  pos.colno + 1,
		Msg:    msg,
	})
}

func (l *iniParse) throw(msg string, args ...interface{}) {
	l.throwAt(l.position, fmt.Sprintf(msg, args...))
}

func (l *iniParse) peek() rune {
	if l.index >= len(l.input) {
		return eofRune
	}
	return rune(l.input[l.index])
}

func (l *iniParse) at(n int) rune {
	n += l.index
	if n > len(l.input) || n < 0 {
		return eofRune
	}
	return rune(l.input[n])
}

func (l *iniParse) remaining() int {
	return len(l.input) - l.index
}

func (l *iniParse) skip(n int) {
	if n < 0 || n > l.remaining() {
		n = l.remaining()
	}
	i := l.index
	stop := i + n
	for ; i < stop; i++ {
		switch l.input[i] {
		case '\n':
			l.lineno++
			l.colno = 0
		case '\t':
			l.colno += 8 - (l.colno % tabwidth)
		default:
			l.colno++
		}
	}
	l.index = i
}

func (l *iniParse) take(n int) string {
	i := l.index
	l.skip(n)
	return string(l.input[i:l.index])
}

func (l *iniParse) match(text string) bool {
	n := len(text)
	if l.remaining() >= n && string(l.input[l.index:l.index+n]) == text {
		l.skip(n)
		return true
	}
	return false
}

func (l *iniParse) skipWhile(fn func(rune) bool) bool {
	i := l.index
	for ; i < len(l.input) && fn(rune(l.input[i])); i++ {
	}
	if i > l.index {
		l.skip(i - l.index)
		return true
	}
	return false
}

func (l *iniParse) skipTo(c byte) bool {
	if i := bytes.IndexByte(l.input[l.index:], c); i >= 0 {
		l.skip(i)
		return true
	}
	l.skip(l.remaining())
	return false
}

func (l *iniParse) takeWhile(fn func(rune) bool) string {
	i := l.index
	l.skipWhile(fn)
	return string(l.input[i:l.index])
}

func (l *iniParse) skipWS() bool {
	return l.skipWhile(func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\r'
	})
}

func isAlpha(c rune) bool {
	c &^= 0x20
	return c >= 'A' && c <= 'Z'
}
func isKeyChar(c rune) bool {
	return isAlpha(c) || (c >= '0' && c <= '9') || c == '-'
}

func (l *iniParse) getKey() string {
	return l.takeWhile(isKeyChar)
}

func (l *iniParse) getSubsection() *string {
	if l.remaining() < 2 || l.peek() != '"' {
		return nil
	}
	ret := &strings.Builder{}
	var i int
loop:
	for i = l.index + 1; i+1 < len(l.input); i++ {
		switch c := l.input[i]; c {
		case '"':
			break loop
		case '\000', '\n':
			return nil
		case '\\':
			nc := l.input[i+1]
			if nc == '\\' || nc == '"' {
				ret.WriteByte(nc)
			}
			i++
		default:
			ret.WriteByte(c)
		}
	}
	if l.input[i] != '"' {
		return nil
	}
	l.skip(i + 1 - l.index)
	s := ret.String()
	return &s
}

func (l *iniParse) getSection() *IniSection {
	if !l.match("[") {
		return nil
	}
	var ret IniSection
	ret.Section = l.getKey()
	if len(ret.Section) == 0 {
		l.throw("expected section name after '['")
	}
	if l.match("]") {
		return &ret
	}
	if !l.skipWS() {
		l.throw("expected ']' or space followed by quoted-subsection")
	}
	if ret.Subsection = l.getSubsection(); ret.Subsection == nil {
		l.throw("expected quoted subsection after space")
	}
	if !l.match("]") {
		l.throw("expected ']'")
	}
	return &ret
}

func needQuotes(val string) bool {
	if val == "" {
		return false
	} else if val[0] == ' ' || val[0] == '\t' {
		return true
	}
	for _, c := range ([]byte)(val) {
		if c < ' ' || c >= 0x7f || strings.IndexByte("\"#;\\", c) != -1 {
			return true
		}
	}
	return false
}

func EscapeIniValue(val string) string {
	if !needQuotes(val) {
		return val
	}
	ret := strings.Builder{}
	ret.WriteByte('"')
	for _, b := range []byte(val) {
		switch b {
		case '"', '\\':
			ret.WriteByte('\\')
			ret.WriteByte(b)
		case '\b':
			ret.WriteString("\\b")
		case '\n':
			ret.WriteString("\\n")
		case '\t':
			ret.WriteString("\\t")
		default:
			ret.WriteByte(b)
		}
	}
	ret.WriteByte('"')
	return ret.String()
}

func (l *iniParse) getValue() string {
	ret := strings.Builder{}
	escape, inquote := false, false
	for {
		c := l.peek()
		if escape {
			escape = false
			switch c {
			case '"', '\\':
				ret.WriteByte(byte(c))
			case 'n':
				ret.WriteByte('\n')
			case 't':
				ret.WriteByte('\t')
			case 'b':
				ret.WriteByte('\b')
			case '\n':
				// ignore
			case '\r':
				if l.at(1) == '\n' {
					l.skip(1)
					break
				}
				fallthrough
			default:
				if c == eofRune {
					l.throw("incomplete escape sequence at EOF")
				}
				l.throw("invalid escape sequence \\%c", c)
			}
		} else if c == '\\' {
			escape = true
		} else if c == '"' {
			inquote = !inquote
		} else if c == '\n' || c == eofRune || (c == '\r' && l.at(1) == '\n') {
			if c == '\r' {
				l.skip(1)
			}
			if inquote {
				l.throw("missing close quotes")
			}
			l.skip(1)
			return ret.String()
		} else if !inquote && (c == '#' || c == ';') {
			l.skipTo('\n')
		} else {
			ret.WriteByte(byte(c))
		}
		l.skip(1)
	}
}

func (l *iniParse) do1() (err *ParseError) {
	defer func() {
		if i := recover(); i != nil {
			if pe, ok := i.(ParseError); ok {
				err = &pe
				l.skipTo('\n')
			} else {
				panic(i)
			}
		}
	}()
	startindex := l.index
	l.skipWS()
	keypos := l.position
	if sec := l.getSection(); sec != nil {
		l.skipWS()
		l.match("\n")
		l.sec = sec
		if err := l.Section(IniSecStart{
			IniSection: *sec,
			IniRange: IniRange{
				StartIndex: startindex,
				EndIndex:   l.index,
			},
		}); err != nil {
			l.throwAt(keypos, err.Error())
		}
	} else if isAlpha(l.peek()) {
		k := l.getKey()
		l.skipWS()
		var v *string
		var valpos position
		if !l.match("=") {
			if c := l.peek(); c != '\n' && c != '#' &&
				c != ';' && c != eofRune {
				l.throw("Expected '=' after %s", k)
			}
			valpos = l.position
			if l.skipTo('\n') {
				l.skip(1)
			}
		} else {
			l.skipWS()
			valpos = l.position
			val := l.getValue()
			v = &val
		}
		if err := l.Value(IniItem{
			IniSection: l.sec,
			Key:        k,
			Value:      v,
			IniRange: IniRange{
				StartIndex: startindex,
				EndIndex:   l.index,
			}}); err != nil {
			if ke, ok := err.(BadKey); ok {
				l.throwAt(keypos, string(ke))
			} else {
				l.throwAt(valpos, err.Error())
			}
		}
	} else if c := l.peek(); c == '#' || c == ';' || c == '\n' {
		l.skipTo('\n')
		l.skip(1)
	} else {
		l.throw("Expected section or key")
	}
	return
}

func (l *iniParse) do() error {
	var err ParseErrors
	for l.remaining() > 0 {
		if e := l.do1(); e != nil {
			err = append(err, *e)
		}
	}
	if err == nil {
		return nil
	}
	return err
}

func newParser(sink IniSink, path string, input []byte) *iniParse {
	var ret iniParse
	ret.file = path
	ret.input = input
	ret.Value = sink.Item
	if iss, ok := sink.(IniSecSink); ok {
		ret.Section = iss.Section
	} else {
		ret.Section = func(IniSecStart) error { return nil }
	}
	return &ret
}

// Parse the contents of an INI file.  The filename argument is used
// only for error messages.
func IniParseContents(sink IniSink, filename string, contents []byte) error {
	return newParser(sink, filename, contents).do()
}

// Open, read, and parse an INI file.  If the file is incorrectly
// formatted, will return an error of type ParseErrors.
func IniParse(sink IniSink, filename string) error {
	if f, err := os.Open(filename); err != nil {
		return err
	} else {
		contents, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
		return newParser(sink, filename, contents).do()
	}
}


type IniEdit struct {
	Fragments [][]byte
	SecEnd    map[string]int
	Values    map[string][]int
}

func (ie *IniEdit) WriteTo(w io.Writer) (int64, error) {
	var ret int64
	for i := range ie.Fragments {
		n, err := w.Write(ie.Fragments[i])
		ret += int64(n)
		if err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (ie IniEdit) String() string {
	ret := strings.Builder{}
	ie.WriteTo(&ret)
	return ret.String()
}

// Delete an entry that was already in the Ini file.  (Does not delete
// new keys that were just added with Add.)
func (ie *IniEdit) Del(is IniSection, key string) {
	k := is.String() + key
	for _, i := range ie.Values[k] {
		ie.Fragments[i] = nil
	}
}

func (ie *IniEdit) Set(is IniSection, key, value string) {
	ie.Del(is, key)
	k := is.String() + key
	vs := ie.Values[k]
	if len(vs) > 0 {
		ie.Fragments[vs[0]] = []byte(
			fmt.Sprintf("\t%s = %s\n", key, EscapeIniValue(value)))
	} else {
		ie.Add(is, key, value)
	}
}

func (ie *IniEdit) Add(is IniSection, key, value string) {
	k := is.String()
	i, ok := ie.SecEnd[k]
	if !ok {
		i = len(ie.Fragments)
		ie.SecEnd[k] = i
		ie.Fragments = append(ie.Fragments, []byte(k + "\n"))
	}
	ie.Fragments[i] = append(ie.Fragments[i],
		fmt.Sprintf("\t%s = %s\n", key, EscapeIniValue(value))...)
}


type iniEditParser struct {
	*IniEdit
	input   []byte
	lastIdx int
}

func (iep *iniEditParser) fill(ir IniRange) int {
	if ir.StartIndex > iep.lastIdx {
		iep.Fragments = append(iep.Fragments,
			iep.input[iep.lastIdx:ir.StartIndex])
	}
	iep.Fragments = append(iep.Fragments, iep.input[ir.StartIndex:ir.EndIndex])
	iep.lastIdx = ir.EndIndex
	return len(iep.Fragments) - 1
}

func (iep *iniEditParser) Section(ss IniSecStart) error {
	iep.SecEnd[ss.IniSection.String()] = iep.fill(ss.IniRange)
	return nil
}

func (iep *iniEditParser) Item(ii IniItem) error {
	k, n := ii.IniSection.String() + ii.Key, iep.fill(ii.IniRange)
	iep.Values[k] = append(iep.Values[k], n)
	iep.SecEnd[ii.IniSection.String()] = n
	return nil
}

func NewIniEdit(filename string, contents []byte) (*IniEdit, error) {
	var ret IniEdit
	iep := iniEditParser{
		IniEdit: &ret,
		input: contents,
	}
	return &ret, IniParseContents(&iep, filename, contents)
}


/*
type iniUpdater struct {
	targetSec  *IniSection
	targetKey  string
	sectionEnd int
	items      []IniItem
}

func (iu *iniUpdater) Section(ss IniSecStart) error {
	if ss.Eq(iu.targetSec) {
		iu.sectionEnd = ss.EndIndex
	}
	return nil
}

func (iu *iniUpdater) Item(ii IniItem) error {
	if ii.Eq(iu.targetSec) {
		iu.sectionEnd = ii.EndIndex
		if ii.Key == iu.targetKey {
			iu.items = append(iu.items, ii)
		}
	}
	return nil
}

func IniDelKeyContents(sec IniSection, key string, valpred func(string) bool,
	filename string, contents []byte) ([][]byte, error) {
	iu := iniUpdater{
		targetSec: &sec,
		targetKey: key,
	}
	if err := IniParseContents(&iu, filename, contents); err != nil {
		return nil, err
	}
	var ret [][]byte
	index := 0
	for i := range iu.items {
		ii := &iu.items[i]
		if valpred != nil && !valpred(ii.Value) {
			continue
		}
		if ii.StartIndex > index {
			ret = append(ret, contents[index:ii.StartIndex])
		}
		index = ii.EndIndex
	}
	if len(contents) > index {
		ret = append(ret, contents[index:])
	}
	if len(ret) <= 1 && iu.sectionEnd != 0 {
		ret = [][]byte{contents[0:iu.sectionEnd], contents[iu.sectionEnd:]}
	}
	return ret, nil
}

func IniDel(filename string, sec IniSection, key string) error {
	lf, err := LockFile(filename, 0666)
	if err != nil {
		return err
	}
	defer lf.Abort()

	contents, err := lf.ReadFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out, err := IniDelKeyContents(sec, key, nil, filename, contents)
	if err != nil {
		return err
	}
	for i := range out {
		if _, err := lf.Write(out[i]); err != nil {
			return err
		}
	}
	return lf.Commit()
}

func IniSet(filename string, sec IniSection, key string, value string) error {
	lf, err := LockFile(filename, 0666)
	if err != nil {
		return err
	}
	defer lf.Abort()

	contents, err := lf.ReadFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out, err := IniDelKeyContents(sec, key, nil, filename, contents)
	if err != nil {
		return err
	}
	if len(out) > 1 {
		if _, err := lf.Write(out[0]); err != nil {
			return err
		}
		fmt.Fprintf(lf, "\t%s = %s\n", key, EscapeIniValue(value))
		for _, o := range out[1:] {
			if _, err := lf.Write(o); err != nil {
				return err
			}
		}
	} else {
		if len(out) == 1 {
			if _, err := lf.Write(out[0]); err != nil {
				return err
			}
		}
		fmt.Fprintf(lf, "%s\n\t%s = %s\n", sec.String(),
			key, EscapeIniValue(value))
	}
	return lf.Commit()
}

func IniAdd(filename string, sec IniSection, key string, value string) error {
	lf, err := LockFile(filename, 0666)
	if err != nil {
		return err
	}
	defer lf.Abort()

	contents, err := lf.ReadFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	iu := iniUpdater{
		targetSec: &sec,
		targetKey: key,
	}
	if err := IniParseContents(&iu, filename, contents); err != nil {
		return err
	}
	var out [][]byte
	if iu.sectionEnd > 0 {
		out = [][]byte{
			contents[:iu.sectionEnd],
			[]byte(fmt.Sprintf("\t%s = %s\n", key, value)),
			contents[iu.sectionEnd:],
		}
	} else {
		out = [][]byte{
			contents,
			[]byte(fmt.Sprintf("%s\n\t%s = %s\n", sec.String(),
				key, EscapeIniValue(value))),
		}
	}
	for i := range out {
		if _, err := lf.Write(out[i]); err != nil {
			return err
		}
	}
	return lf.Commit()
}

type IniSetItem struct {
	IniSection
	Key   string
	Value *string
}

type iniMultiSetterSecinfo struct {
	lastIndex int // XXX doesn't work
	keys      map[string]*string
}
type iniMultiSetter struct {
	secs map[string]*iniMultiSetterSecinfo
	out  io.Writer
}

func (ims *iniMultiSetter) Section(ss IniSecStart) error {
	if si, ok := ims.secs[ss.IniSection.String()]; ok {
		si.lastIndex = ss.EndIndex
	}
	return nil
}

func (ims *iniMultiSetter) Item(ii IniItem) error {
	return nil
}

func IniMultiSet(filename string, actions []IniSetItem) error {
	ims := &iniMultiSetter{
		secs: make(map[string]*iniMultiSetterSecinfo),
	}
	for i := range actions {
		ss := actions[i].IniSection.String()
		si := ims.secs[ss]
		if si == nil {
			si = &iniMultiSetterSecinfo{
				keys: make(map[string]*string),
			}
			ims.secs[ss] = si
		}
		si.keys[actions[i].Key] = actions[i].Value
	}

	// XXX

	return nil
}
*/
