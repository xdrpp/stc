package ini

import (
	"container/list"
	"fmt"
	"io"
	"strings"
)

// You can parse an INI file into an IniEditor, Set, Del, or Add
// key-value pairs, then write out the result using WriteTo.
// Preserves most comments and file ordering.
type IniEditor struct {
	fragments list.List
	secEnd    map[string]*list.Element
	values    map[string][]*list.Element
	lastSec   *IniSection
}

// Write the contents of IniEditor to a Writer after applying edits
// have been made.
func (ie *IniEditor) WriteTo(w io.Writer) (int64, error) {
	var ret int64
	for e := ie.fragments.Front(); e != nil; e = e.Next() {
		n, err := w.Write(e.Value.([]byte))
		ret += int64(n)
		if err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (ie *IniEditor) String() string {
	ret := strings.Builder{}
	ie.WriteTo(&ret)
	return ret.String()
}

// Delete all instances of a key from the file.
func (ie *IniEditor) Del(is *IniSection, key string) {
	k := IniQKey(is, key)
	for _, e := range ie.values[k] {
		ie.fragments.Remove(e)
	}
	delete(ie.values, k)
}

func iniLine(key, value string) []byte {
	return []byte(fmt.Sprintf("\t%s = %s\n", key, EscapeIniValue(value)))
}

func (ie *IniEditor) newItem(is *IniSection, key, value string) *list.Element {
	ss := is.String()
	e, ok := ie.secEnd[ss]
	if !ok {
		e = ie.fragments.Back()
		if ssb := []byte(ss + "\n"); e != nil && len(e.Value.([]byte)) == 0 {
			e.Value = ssb
		} else {
			e = ie.fragments.PushBack(ssb)
		}
		e = ie.fragments.InsertAfter([]byte{}, e)
		ie.secEnd[ss] = e
	}
	e = ie.fragments.InsertBefore(iniLine(key, value), e)
	k := IniQKey(is, key)
	ie.values[k] = append(ie.values[k], e)
	return e
}

// Replace all instances of key with a single one equal to value.
func (ie *IniEditor) Set(is *IniSection, key, value string) {
	k := IniQKey(is, key)
	vs := ie.values[k]
	if len(vs) > 0 {
		ie.values[k] = []*list.Element{
			ie.fragments.InsertAfter(iniLine(key, value), vs[len(vs)-1]),
		}
		for _, e := range vs {
			ie.fragments.Remove(e)
		}
	} else {
		ie.newItem(is, key, value)
	}
}

// Add a new instance of key to the file without deleting any previous
// instance of the key.
func (ie *IniEditor) Add(is *IniSection, key, value string) {
	k := IniQKey(is, key)
	vs := ie.values[k]
	if len(vs) > 0 {
		e := ie.fragments.InsertAfter(iniLine(key, value), vs[len(vs)-1])
		ie.values[k] = append(vs, e)
	} else {
		ie.newItem(is, key, value)
	}
}

func (ie *IniEditor) appendItem(r *IniRange) (e1, e2 *list.Element) {
	if r.StartIndex > r.PrevEndIndex {
		e1 = ie.fragments.PushBack(r.Input[r.PrevEndIndex:r.StartIndex])
	}
	if r.EndIndex > r.StartIndex {
		e2 = ie.fragments.PushBack(r.Input[r.StartIndex:r.EndIndex])
	}
	if e1 == nil {
		e1 = e2
	}
	return
}

// Called by IniParseContents; do not call directly.
func (ie *IniEditor) Section(ss IniSecStart) error {
	// git-config associates comments with following section
	e, _ := ie.appendItem(&ss.IniRange)
	ie.secEnd[ie.lastSec.String()] = e
	ie.lastSec = &ss.IniSection
	return nil
}

// Called by IniParseContents; do not call directly.
func (ie *IniEditor) Item(ii IniItem) error {
	k := ii.QKey()
	_, e := ie.appendItem(&ii.IniRange)
	ie.values[k] = append(ie.values[k], e)
	return nil
}

// Called by IniParseContents; do not call directly.
func (ie *IniEditor) Done(r IniRange) {
	e, _ := ie.appendItem(&r)
	if e == nil {
		e = ie.fragments.PushBack([]byte{})
	}
	ie.secEnd[ie.lastSec.String()] = e
	ie.lastSec = nil
}

// Create an IniEdit for a file with contents.  Note that filename is
// only used for parse errors; the file must already be read before
// calling this function.
func NewIniEdit(filename string, contents []byte) (*IniEditor, error) {
	ret := IniEditor{
		secEnd: make(map[string]*list.Element),
		values: make(map[string][]*list.Element),
	}
	err := IniParseContents(&ret, filename, contents)
	return &ret, err
}

// A bunch of edits to be applied to an INI file.
type IniEdits []func(*IniEditor)

// Delete a key.  Invoke as Del(sec, subsec, key) or Del(sec, key).
func (ie *IniEdits) Del(sec string, args ...string) error {
	s, k := &IniSection{Section: sec}, ""
	switch len(args) {
	case 1:
		k = args[0]
	case 2:
		s.Subsection = &args[0]
		k = args[1]
	default:
		return ErrInvalidNumArgs
	}
	if !s.Valid() {
		return ErrInvalidSection
	}
	*ie = append(*ie, func(ie *IniEditor) { ie.Del(s, k) })
	return nil
}

// Add a key, value pair.  Invoke as Add(sec, subsec, key, value) or
// Add(sec, key, value).
func (ie *IniEdits) Add(sec string, args ...string) error {
	s, k, v := &IniSection{Section: sec}, "", ""
	switch len(args) {
	case 2:
		k = args[0]
		v = args[1]
	case 3:
		s.Subsection = &args[0]
		k = args[1]
		v = args[2]
	default:
		return ErrInvalidNumArgs
	}
	if !s.Valid() {
		return ErrInvalidSection
	}
	*ie = append(*ie, func(ie *IniEditor) { ie.Add(s, k, v) })
	return nil
}

// Add a key, value pair.  Invoke as Set(sec, subsec, key, value) or
// Set(sec, key, value).
func (ie *IniEdits) Set(sec string, args ...string) error {
	s, k, v := &IniSection{Section: sec}, "", ""
	switch len(args) {
	case 2:
		k = args[0]
		v = args[1]
	case 3:
		s.Subsection = &args[0]
		k = args[1]
		v = args[2]
	default:
		return ErrInvalidNumArgs
	}
	if !s.Valid() {
		return ErrInvalidSection
	}
	*ie = append(*ie, func(ie *IniEditor) { ie.Set(s, k, v) })
	return nil
}

// Apply edits.
func (ie *IniEdits) Apply(target *IniEditor) {
	for _, f := range *ie {
		f(target)
	}
	*ie = nil
}
