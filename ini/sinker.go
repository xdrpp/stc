package ini

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// A generic IniSink that uses fmt.Sscan to parse non-string fields.
type GenericIniSink struct {
	// If non-nil, only match this specific section (otherwise
	// ignore).
	Sec *IniSection

	// Pointers to the fields that should be parsed.
	Fields map[string]interface{}
}

// NewGenericSink([section [, subsection])
func NewGenericSink(args...string) *GenericIniSink {
	var sec *IniSection
	switch len(args) {
	case 0:
		sec = nil
	case 1:
		sec = &IniSection{
			Section: args[0],
		}
	case 2:
		sec = &IniSection{
			Section: args[0],
			Subsection: &args[1],
		}
	default:
		panic(fmt.Errorf("NewGenericSink takes at most 2 arguments, not %d",
			len(args)))
	}
	return &GenericIniSink{
		Sec: sec,
		Fields: make(map[string]interface{}),
	}
}

// Add a field to be parsed
func (s *GenericIniSink) AddField(name string, ptr interface{}) {
	s.Fields[name] = ptr
}

var errNotStructPtr = errors.New("argument must be pointer to struct")

// Populate a GenericIniSink with fields of a struct, using the field
// name or or the ini struct field tag (`ini:"field-name"`) if one
// exists.  Tag `ini:"-"` says to ignore a field.  Note that i must be
// a pointer to a structure or this function will panic.
func (s *GenericIniSink) AddStruct(i interface{}) {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		panic(errNotStructPtr)
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		panic(errNotStructPtr)
	}
	t := v.Type()
	for i, n := 0, t.NumField(); i < n; i++ {
		f := t.Field(i)
		name := f.Tag.Get("ini")
		if name == "-" {
			continue
		} else if name == "" {
			name = strings.ReplaceAll(f.Name, "_", "-")
		}
		s.AddField(name, v.Field(i).Addr().Interface())
	}
}

// Save the current state of an Ini-parsable structure to a set of
// IniEdits.  This is useful for creating an initial file.  If
// includeZero is true, then all fields are saved; otherwise, only
// ones with non-default values are saved.
func (s *GenericIniSink) SaveAll(ies *IniEdits, includeZero bool) {
	for name, i := range s.Fields {
		*ies = append(*ies, func(ie *IniEditor){
			v := reflect.ValueOf(i).Elem()
			if includeZero || !v.IsZero() {
				ie.Set(s.Sec, name, fmt.Sprint(v.Interface()))
			}
		})
	}
}

func (s *GenericIniSink) String() string {
	out := strings.Builder{}
	if s.Sec != nil {
		fmt.Fprintf(&out, "%s\n", s.Sec.String())
	}
	for name, i := range s.Fields {
		v := reflect.ValueOf(i).Elem()
		if v.Kind() == reflect.Slice {
			for j := 0; j < v.Len(); j++ {
				fmt.Fprintf(&out, "\t%s = %s\n", name,
					EscapeIniValue(fmt.Sprint(v.Index(j).Interface())))
			}
		} else {
			fmt.Fprintf(&out, "\t%s = %s\n", name,
				EscapeIniValue(fmt.Sprint(v.Interface())))
		}
	}
	return out.String()
}

func (s *GenericIniSink) Item(ii IniItem) error {
	if s.Sec.Eq(ii.IniSection) {
		if i, ok := s.Fields[ii.Key]; ok {
			v := reflect.ValueOf(i).Elem()
			if ii.Value == nil {
				v.Set(reflect.Zero(v.Type()))
			} else if v.Kind() == reflect.String {
				v.SetString(ii.Val())
			} else if v.Kind() == reflect.Slice {
				e := reflect.New(v.Type().Elem())
				if e.Kind() == reflect.String {
					e.Elem().SetString(ii.Val())
				} else if _, err :=
					fmt.Sscan(*ii.Value, e.Interface()); err != nil {
					return err
				}
				v.Set(reflect.Append(v, e.Elem()))
			} else {
				_, err := fmt.Sscan(*ii.Value, i)
				return err
			}
			return nil
		}
	}
	return nil
}

func (s *GenericIniSink) IniSink() IniSink {
	return s
}

var _ IniSinker = &GenericIniSink{} // XXX

type IniSinker interface {
	IniSink() IniSink
}

type IniSinks []IniSink

func (s *IniSinks) Push(i IniSink) {
	*s = append(*s, i)
}

func (s IniSinks) Init() {
	for i := range s {
		if init, ok := s[i].(interface{ Init() }); ok {
			init.Init()
		}
	}
}
func (s IniSinks) Item(ii IniItem) error {
	for i := range s {
		if err := s[i].Item(ii); err != nil {
			return err
		}
	}
	return nil
}
func (s IniSinks) Section(ss IniSecStart) error {
	for i := range s {
		if sec, ok := s[i].(interface{ Section(IniSecStart) error }); ok {
			if err := sec.Section(ss); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s IniSinks) Done(IniRange) {
	for i := range s {
		if done, ok := s[i].(interface{ Done() }); ok {
			done.Done()
		}
	}
}

func (s IniSinks) IniSink() IniSink {
	return s
}

func (s IniSinks) String() string {
	ret := strings.Builder{}
	for i := range s {
		fmt.Fprintln(&ret, s[i])
	}
	return ret.String()
}

var _ IniSinker = IniSinks{}
