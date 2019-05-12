package stcdetail

import "bytes"
import "encoding/base64"
import "encoding/json"
import "fmt"
import "github.com/xdrpp/stc/stx"

type jsonIn struct {
	obj interface{}
}

func mustString(i interface{}) string {
	switch i.(type) {
	case map[string]interface{}, []interface{}:
		stx.XdrPanic("JsonToXdr: Expected scalar got %T", i)
	}
	return fmt.Sprint(i)
}

func (j *jsonIn) get(name string) interface{} {
	if len(name) == 0 {
		stx.XdrPanic("JsonToXdr: empty field name")
	} else if name[0] == '[' {
		a := j.obj.([]interface{})
		if len(a) == 0 {
			stx.XdrPanic("JsonToXdr: insufficient elements in array")
		}
		j.obj = a[1:]
		return a[0]
	} else if obj, ok := j.obj.(map[string]interface{})[name]; ok {
		return obj
	}
	return nil
}

func (_ *jsonIn) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (j *jsonIn) Marshal(name string, xval stx.XdrType) {
	jval := j.get(name)
	if jval == nil {
		return
	}
	switch v := xval.(type) {
	case stx.XdrString:
		v.SetString(mustString(jval))
	case stx.XdrVecOpaque:
		bs, err := base64.StdEncoding.DecodeString(mustString(jval))
		if err != nil {
			panic(err)
		}
		v.SetByteSlice(bs)
	case stx.XdrArrayOpaque:
		dst := v.GetByteSlice()
		bs, err := base64.StdEncoding.DecodeString(mustString(jval))
		if err != nil {
			panic(err)
		} else if len(bs) != len(dst) {
			stx.XdrPanic("JsonToXdr: %s decodes to %d bytes, want %d bytes",
				name, len(bs), len(dst))
		}
		copy(dst, bs)
	case fmt.Scanner:
		if _, err := fmt.Sscan(mustString(jval), v); err != nil {
			stx.XdrPanic("JsonToXdr: %s: %s", name, err.Error())
		}
	case stx.XdrPtr:
		v.SetPresent(true)
		v.XdrMarshalValue(j, name)
	case stx.XdrVec:
		v.XdrMarshalN(&jsonIn{jval}, "", uint32(len(jval.([]interface{}))))
	case stx.XdrAggregate:
		v.XdrMarshal(&jsonIn{jval}, "")
	}
}

// Parse JSON into an XDR structure.  This function assumes that dst
// is in a pristine state--for example, it won't reset pointers to nil
// if the corresponding JSON is null.  Moreover, it is somewhat strict
// in expecting the JSON to conform to the XDR.
func JsonToXdr(dst stx.XdrAggregate, src []byte) (err error) {
	defer func() {
		if i := recover(); i != nil {
			err = i.(error)
		}
	}()
	var j jsonIn
	json.Unmarshal(src, &j.obj)
	dst.XdrMarshal(&j, "")
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
		// Intentionally don't do the same for 64-bit, which gets
		// passed as strings to avoid any loss of precision.
	case stx.XdrString:
		j.printField(name, "%s", v.String())
	case stx.XdrBytes:
		j.printField(name, "\"")
		enc := base64.NewEncoder(base64.StdEncoding, j.out)
		enc.Write(v.GetByteSlice())
		enc.Close()
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

// Format an XDR structure as JSON.
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
