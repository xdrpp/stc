package stc

import "bytes"
import "encoding/base64"
import "encoding/json"
import "fmt"
import "github.com/xdrpp/stc/stx"

func unmarshalOneValue(jval interface{}, xval stx.XdrType) {
	switch v := xval.(type) {
	case *stx.XdrBool:
		if b, ok := jval.(bool); ok {
			*v = stx.XdrBool(b)
		} else {
			stx.XdrPanic("JsonToXdr: field XXX should be bool")
		}
	case stx.XdrPtr:
		if jval == nil {
			v.SetPresent(false)
		} else {
			v.SetPresent(true)
		}

	case stx.XdrString:
		if s, ok := jval.(string); ok {
			v.SetString(s)
		} else {
			stx.XdrPanic("JsonToXdr: field XXX should be string")
		}
	}

}

type jsonArrayIn []interface{}
func (_ jsonArrayIn) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (j jsonArrayIn) Marshal(name string, val stx.XdrType) {
}

type jsonObjIn map[string]interface{}
func (_ jsonObjIn) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (j jsonObjIn) Marshal(name string, val stx.XdrType) {
	jval, ok := j[name]
	if !ok {
		stx.XdrPanic("JsonToXdr: missing field %s", name)
	}
	_ = jval
}


// Parse JSON into an XDR structure
func JsonToXdr(dst stx.XdrAggregate, src []byte) (err error) {
	defer func() {
		if i := recover(); i != nil {
			err = i.(error)
		}
	}()

	var obj jsonObjIn
	json.Unmarshal(src, (*map[string]interface{})(&obj))

	fmt.Printf("%v\n", obj)

	dst.XdrMarshal(obj, "")
	return nil
}

type jsonOut struct {
	out *bytes.Buffer
	indent string
	needComma bool
}

func (j *jsonOut) printField(name string, f string, args ...interface{}) {
	if j.needComma {
		j.out.WriteString(",\n")
	} else {
		j.out.WriteString("\n")
		j.needComma = true
	}
	j.out.WriteString(j.indent)
	if len(name) > 0 && name[0] != '[' {
		fmt.Fprintf(j.out, "%q: ", name)
	}
	fmt.Fprintf(j.out, f, args...)
}

func (j *jsonOut) aggregate(val stx.XdrAggregate) {
	oldIndent := j.indent
	defer func() {
		j.needComma = true
		j.indent = oldIndent
	}()
	j.indent = j.indent + "    "
	j.needComma = false
	switch v := val.(type) {
	case stx.XdrVec:
		j.out.WriteString("[")
		v.XdrMarshalN(j, "", v.GetVecLen())
		j.out.WriteString("\n" + oldIndent + "]")
	case stx.XdrArray:
		j.out.WriteString("[")
		v.XdrMarshal(j, "")
		j.out.WriteString("\n" + oldIndent + "]")
	case stx.XdrAggregate:
		j.out.WriteString("{")
		v.XdrMarshal(j, "")
		j.out.WriteString("\n" + oldIndent + "}")
	}
}

func (_ *jsonOut) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (j *jsonOut) Marshal(name string, val stx.XdrType) {
	switch v := val.(type) {
	case *stx.XdrBool:
		j.printField(name, "%v", *v)
	case stx.XdrEnum:
		j.printField(name, "%q", v.String())
	case stx.XdrNum32:
		j.printField(name, "%s", v.String())
    // Intentionally don't do the same for 64-bit, which get passed as
    // strings to avoid any loss of precision.
	case stx.XdrString:
		j.printField(name, "%s", v.String())
	case stx.XdrBytes:
		j.printField(name, "\"")
		base64.NewEncoder(base64.StdEncoding, j.out).Write(v.GetByteSlice())
		j.out.WriteByte('"')
	case fmt.Stringer:
		j.printField(name, "%q", v.String())
	case stx.XdrPtr:
		if !v.GetPresent() {
			j.printField(name, "null")
		} else {
			v.XdrMarshalValue(j, name)
		}
	case stx.XdrAggregate:
		j.printField(name, "")
		j.aggregate(v)
	default:
		stx.XdrPanic("XdrToJson can't handle type %T", val)
	}
}

// Format an XDR structure as JSON
func XdrToJson(src stx.XdrAggregate) (json []byte, err error) {
	defer func() {
		if i := recover(); i != nil {
			json = nil
			err = i.(error)
		}
	}()
	j := &jsonOut{out: &bytes.Buffer{}}
	j.aggregate(src)
	return j.out.Bytes(), nil
}
