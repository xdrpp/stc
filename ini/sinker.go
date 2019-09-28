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

func (s *GenericIniSink) AddField(name string, ptr interface{}) {
	if s.Fields == nil {
		s.Fields = make(map [string]interface{})
	}
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

func (s *GenericIniSink) String() string {
	out := strings.Builder{}
	if s.Sec != nil {
		fmt.Fprintf(&out, "%s\n", s.Sec.String())
	}
	for name, i := range s.Fields {
		v := reflect.ValueOf(i).Elem().Interface()
		fmt.Fprintf(&out, "\t%s = %s\n", name, EscapeIniValue(fmt.Sprint(v)))
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

var _ IniSinker = IniSinks{}
