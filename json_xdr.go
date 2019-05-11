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
			stx.XdrPanic("JsonToXdr: field %s should be bool")
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
			stx.XdrPanic("JsonToXdr: field %s should be string")
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

const indentString = "    "

type jsonOut struct {
	out *bytes.Buffer
	indent string
	needComma bool
}
func (_ *jsonOut) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (j *jsonOut) Marshal(name string, val stx.XdrType) {
	startlen := j.out.Len()
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

	switch v := val.(type) {
	case *stx.XdrBool:
		fmt.Fprintf(j.out, "%v", bool(*v))
	case stx.XdrString:
		fmt.Fprintf(j.out, "%s", v.String())
	case stx.XdrBytes:
		j.out.WriteByte('"')
		base64.NewEncoder(base64.StdEncoding, j.out).Write(v.GetByteSlice())
		j.out.WriteByte('"')
	case fmt.Stringer:
		fmt.Fprintf(j.out, "%q", v.String())
	case stx.XdrPtr:
		if !v.GetPresent() {
			j.out.WriteString("null")
		} else {
			j.out.Truncate(startlen)
			v.XdrMarshalValue(j, name)
		}
	case stx.XdrVec:
		j.out.WriteString("[")
		v.XdrMarshalN(&jsonOut{
			out: j.out,
			indent: j.indent + indentString,
		}, "", v.GetVecLen())
		j.out.WriteString("\n" + j.indent + "]")
	case stx.XdrArray:
		j.out.WriteString("[")
		v.XdrMarshal(&jsonOut{
			out: j.out,
			indent: j.indent + indentString,
		}, "")
		j.out.WriteString("\n" + j.indent + "]")
	case stx.XdrAggregate:
		j.out.WriteString("{")
		v.XdrMarshal(&jsonOut{
			out: j.out,
			indent: j.indent + indentString,
		}, "")
		j.out.WriteString("\n" + j.indent + "}")
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
	switch src.(type) {
	case stx.XdrArray, stx.XdrVec:
		src.XdrMarshal(j, "")
		j.out.WriteString("\n")
	default:
		j.out.WriteString("{")
		j.indent += indentString
		src.XdrMarshal(j, "")
		j.out.WriteString("\n}\n")
	}
	return j.out.Bytes(), nil
}
