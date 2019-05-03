package stcdetail

import(
	"fmt"
	"reflect"
	"strings"
)

type printer struct {
	*strings.Builder
}

func canPrint(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8,
		reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64,
		reflect.Complex128, reflect.String:
		return true
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return true
		}
		return false
	}
	_, ok := v.Type().MethodByName("String")
	return ok
}

func (pp printer) recPretty(prefix string, field string, v reflect.Value) {
	if prefix != "" && field != "" && field[0] != '[' {
		prefix = prefix + "." + field
	} else {
		prefix += field
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
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
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
		panic(fmt.Errorf("cannot pretty-print %s", v.Type()))
	}
}

func PrettyPrint(arg interface{}) string {
	v := reflect.ValueOf(arg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	pp := printer{&strings.Builder{}}
	pp.doPretty("", v)
	return pp.String()
}
