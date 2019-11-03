package stcdetail

import (
	"fmt"
	"reflect"
	"strings"
)

type printer struct {
	*strings.Builder
	aux func (interface{})(string, bool)

}

func canPrint(v reflect.Value) bool {
	if _, ok := v.Type().MethodByName("String"); ok {
		return true
	}

	switch v.Kind() {
	case reflect.Struct, reflect.Map:
		return false
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return true
		}
		return false
	default:
		return true
	}
}

func (pp printer) recPretty(prefix string, field string, v reflect.Value) {
	if prefix != "" && field != "" && field[0] != '[' {
		prefix = prefix + "." + field
	} else {
		prefix += field
	}
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if pp.aux != nil {
		if s, ok := pp.aux(v.Interface()); ok {
			if s != "" {
				fmt.Fprintf(pp, "%s: %s\n", prefix, s)
			}
			return
		}
	}
	if canPrint(v) {
		s := fmt.Sprint(v.Interface())
		if s != "" {
			fmt.Fprintf(pp, "%s: %s\n", prefix, s)
		}
	} else {
		pp.doPretty(prefix, v)
	}
}

func (pp printer) doPretty(prefix string, v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		n := v.NumField()
		for i := 0; i < n; i++ {
			pp.recPretty(prefix, v.Type().Field(i).Name, v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		n := v.Len()
		for i := 0; i < n; i++ {
			pp.recPretty(fmt.Sprintf("%s[%d]", prefix, i), "", v.Index(i))
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			pp.recPretty(fmt.Sprintf("%s[%q]", prefix, iter.Key().Interface()),
				"", iter.Value())
		}
	default:
		pp.recPretty(prefix, "", v)
	}
}

func PrettyPrintAux(aux func (interface{})(string, bool),
	arg interface{}) string {
	v := reflect.ValueOf(arg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	pp := printer{&strings.Builder{}, aux}
	pp.doPretty("", v)
	return pp.String()
}

func PrettyPrint(arg interface{}) string {
	return PrettyPrintAux(nil, arg)
}
