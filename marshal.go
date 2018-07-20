package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

type XdrPrint struct {
	Out io.Writer
}

func (xp *XdrPrint) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

func (xp *XdrPrint) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case fmt.Stringer:
		fmt.Fprintf(xp.Out, "%s: %s\n", name, v.String())
	case XdrPtr:
		fmt.Fprintf(xp.Out, "%s.present: %v\n", name, v.GetPresent())
		v.XdrMarshalValue(xp, name)
	case XdrVec:
		fmt.Fprintf(xp.Out, "%s.len: %d\n", name, v.GetVecLen())
		v.XdrMarshalN(xp, name, v.GetVecLen())
	case XdrAggregate:
		v.XdrMarshal(xp, name)
	default:
		fmt.Fprintf(xp.Out, "%s: %v\n", name, i)
	}
}

func XdrToString(t XdrAggregate) string {
	out := &strings.Builder{}
	t.XdrMarshal(&XdrPrint{out}, "")
	return out.String()
}

var xdrZerofill [4][]byte = [...][]byte{
	{}, {0,0,0}, {0,0}, {0},
}

func xdrPutBytes(out io.Writer, val []byte) {
	out.Write(val)
	out.Write(xdrZerofill[len(val)&3])
}

func xdrPut32(out io.Writer, val uint32) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, val)
	out.Write(b)
}

func xdrPut64(out io.Writer, val uint64) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, val)
	out.Write(b)
}

type XdrOut struct {
	Out io.Writer
}

func (xp *XdrOut) Sprintf(f string, args ...interface{}) string {
	return ""
}

func (xo *XdrOut) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case XdrNum32:
		xdrPut32(xo.Out, v.GetU32())
	case XdrNum64:
		xdrPut64(xo.Out, v.GetU64())
	case XdrString:
		s := v.GetString()
		xdrPut32(xo.Out, uint32(len(s)))
		io.WriteString(xo.Out, s)
		xo.Out.Write(xdrZerofill[len(s)&3])
	case XdrVarBytes:
		xdrPut32(xo.Out, uint32(len(v.GetByteSlice())))
		xdrPutBytes(xo.Out, v.GetByteSlice())
	case XdrBytes:
		xdrPutBytes(xo.Out, v.GetByteSlice())
	case XdrAggregate:
		v.XdrMarshal(xo, name)
	default:
		panic(fmt.Sprintf("XdrOut: unhandled type %T", i))
	}
}

func xdrReadN(in io.Reader, n uint32) []byte {
	// XXX for large n, must build up buffer to avoid DoS
	b := make([]byte, n)
	if _, err := io.ReadFull(in, b); err != nil {
		panic(err)
	}
	return b
}

func xdrReadPad(in io.Reader, n uint32) {
	if n & 3 != 0 {
		got := xdrReadN(in, 4-(n&3))
		for _, b := range got {
			if b != 0 {
				xdrPanic("padding contained non-zero bytes")
			}
		}
	}
}

func xdrGet32(in io.Reader) uint32 {
	b := xdrReadN(in, 4)
	return binary.BigEndian.Uint32(b)
}

func xdrGet64(in io.Reader) uint64 {
	b := xdrReadN(in, 8)
	return binary.BigEndian.Uint64(b)
}


type XdrIn struct {
	In io.Reader
}

func (xp *XdrIn) Sprintf(f string, args ...interface{}) string {
	return ""
}

func (xi *XdrIn) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case XdrNum32:
		v.SetU32(xdrGet32(xi.In))
	case XdrNum64:
		v.SetU64(xdrGet64(xi.In))
	case XdrVarBytes:
		n := xdrGet32(xi.In)
		v.SetByteSlice(xdrReadN(xi.In, n))
		xdrReadPad(xi.In, n)
	case XdrBytes:
		if _, err := io.ReadFull(xi.In, v.GetByteSlice()); err != nil {
			panic(err)
		}
		xdrReadPad(xi.In, uint32(len(v.GetByteSlice())))
	case XdrAggregate:
		v.XdrMarshal(xi, name)
	}
}
