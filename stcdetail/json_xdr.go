package stcdetail

import "bytes"
import "encoding/base64"
import "encoding/json"
import "fmt"
import "github.com/xdrpp/goxdr/xdr"

type jsonIn struct {
	obj interface{}
}

func mustString(i interface{}) string {
	switch i.(type) {
	case map[string]interface{}, []interface{}:
		xdr.XdrPanic("JsonToXdr: Expected scalar got %T", i)
	}
	return fmt.Sprint(i)
}

func (j *jsonIn) get(name string) interface{} {
	if len(name) == 0 {
		xdr.XdrPanic("JsonToXdr: empty field name")
	} else if name[0] == '[' {
		a := j.obj.([]interface{})
		if len(a) == 0 {
			xdr.XdrPanic("JsonToXdr: insufficient elements in array")
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
func (j *jsonIn) Marshal(name string, xval xdr.XdrType) {
	jval := j.get(name)
	if jval == nil {
		return
	}
	switch v := xval.(type) {
	case xdr.XdrString:
		v.SetString(mustString(jval))
	case xdr.XdrVecOpaque:
		bs, err := base64.StdEncoding.DecodeString(mustString(jval))
		if err != nil {
			panic(err)
		}
		v.SetByteSlice(bs)
	case xdr.XdrArrayOpaque:
		dst := v.GetByteSlice()
		bs, err := base64.StdEncoding.DecodeString(mustString(jval))
		if err != nil {
			panic(err)
		} else if len(bs) != len(dst) {
			xdr.XdrPanic("JsonToXdr: %s decodes to %d bytes, want %d bytes",
				name, len(bs), len(dst))
		}
		copy(dst, bs)
	case fmt.Scanner:
		if _, err := fmt.Sscan(mustString(jval), v); err != nil {
			xdr.XdrPanic("JsonToXdr: %s: %s", name, err.Error())
		}
	case xdr.XdrPtr:
		v.SetPresent(true)
		v.XdrMarshalValue(j, name)
	case xdr.XdrVec:
		v.XdrMarshalN(&jsonIn{jval}, "", uint32(len(jval.([]interface{}))))
	case xdr.XdrAggregate:
		v.XdrRecurse(&jsonIn{jval}, "")
	}
}

// Parse JSON into an XDR structure.  This function assumes that dst
// is in a pristine state--for example, it won't reset pointers to nil
// if the corresponding JSON is null.  Moreover, it is somewhat strict
// in expecting the JSON to conform to the XDR.
func JsonToXdr(dst xdr.XdrAggregate, src []byte) (err error) {
	defer func() {
		if i := recover(); i != nil {
			err = i.(error)
		}
	}()
	var j jsonIn
	json.Unmarshal(src, &j.obj)
	dst.XdrRecurse(&j, "")
	return nil
}

type jsonOut struct {
	out       *bytes.Buffer
	indent    string
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

func (j *jsonOut) aggregate(val xdr.XdrAggregate) {
	oldIndent := j.indent
	defer func() {
		j.needComma = true
		j.indent = oldIndent
	}()
	j.indent = j.indent + "    "
	j.needComma = false
	switch v := val.(type) {
	case xdr.XdrVec:
		j.out.WriteString("[")
		v.XdrMarshalN(j, "", v.GetVecLen())
		j.out.WriteString("\n" + oldIndent + "]")
	case xdr.XdrArray:
		j.out.WriteString("[")
		v.XdrRecurse(j, "")
		j.out.WriteString("\n" + oldIndent + "]")
	case xdr.XdrAggregate:
		j.out.WriteString("{")
		v.XdrRecurse(j, "")
		j.out.WriteString("\n" + oldIndent + "}")
	}
}

func (_ *jsonOut) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (j *jsonOut) Marshal(name string, val xdr.XdrType) {
	switch v := val.(type) {
	case *xdr.XdrBool:
		j.printField(name, "%v", *v)
	case xdr.XdrEnum:
		j.printField(name, "%q", v.String())
	case xdr.XdrNum32:
		j.printField(name, "%s", v.String())
		// Intentionally don't do the same for 64-bit, which gets
		// passed as strings to avoid any loss of precision.
	case xdr.XdrString:
		j.printField(name, "%s", v.String())
	case xdr.XdrBytes:
		j.printField(name, "\"")
		enc := base64.NewEncoder(base64.StdEncoding, j.out)
		enc.Write(v.GetByteSlice())
		enc.Close()
		j.out.WriteByte('"')
	case fmt.Stringer:
		j.printField(name, "%q", v.String())
	case xdr.XdrPtr:
		if !v.GetPresent() {
			j.printField(name, "null")
		} else {
			v.XdrMarshalValue(j, name)
		}
	case xdr.XdrAggregate:
		j.printField(name, "")
		j.aggregate(v)
	default:
		xdr.XdrPanic("XdrToJson can't handle type %T", val)
	}
}

// Format an XDR structure as JSON.
func XdrToJson(src xdr.XdrAggregate) (json []byte, err error) {
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
