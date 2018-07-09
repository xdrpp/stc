package main

import (
	"encoding/binary"
	"fmt"
	"io"
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


var enc binary.ByteOrder = binary.BigEndian
var zerofill [4][]byte = [...][]byte{
	{}, {0,0,0}, {0,0}, {0},
}

func putBytes(out io.Writer, val []byte) {
	out.Write(val)
	out.Write(zerofill[len(val)&3])
}

func put32(out io.Writer, val uint32) {
	b := make([]byte, 4)
	enc.PutUint32(b, val)
	out.Write(b)
}

func put64(out io.Writer, val uint64) {
	b := make([]byte, 8)
	enc.PutUint64(b, val)
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
		put32(xo.Out, v.GetU32())
	case XdrNum64:
		put64(xo.Out, v.GetU64())
	case XdrVarBytes:
		put32(xo.Out, uint32(len(v.GetByteSlice())))
		putBytes(xo.Out, v.GetByteSlice())
	case XdrBytes:
		putBytes(xo.Out, v.GetByteSlice())
	case XdrAggregate:
		v.XdrMarshal(xo, name)
	default:
		panic(fmt.Sprintf("XdrOut: unhandled type %T", i))
	}
}

func readFull(in io.Reader, b []byte) {
	if _, err := io.ReadFull(in, b); err != nil {
		panic(err)
	}
}

func readN(in io.Reader, n uint32) []byte {
	// XXX for large n, must build up buffer to avoid DoS
	b := make([]byte, n)
	readFull(in, b)
	return b
}

func readPad(in io.Reader, n uint32) {
	if n & 3 != 0 {
		got := readN(in, 4-(n&3))
		for _, b := range got {
			if b != 0 {
				xdrPanic("padding contained non-zero bytes")
			}
		}
	}
}

func get32(in io.Reader) uint32 {
	b := readN(in, 4)
	return enc.Uint32(b)
}

func get64(in io.Reader) uint64 {
	b := readN(in, 8)
	return enc.Uint64(b)
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
		v.SetU32(get32(xi.In))
	case XdrNum64:
		v.SetU64(get64(xi.In))
	case XdrVarBytes:
		n := get32(xi.In)
		v.SetByteSlice(readN(xi.In, n))
		readPad(xi.In, n)
	case XdrBytes:
		readFull(xi.In, v.GetByteSlice())
		readPad(xi.In, uint32(len(v.GetByteSlice())))
	case XdrAggregate:
		v.XdrMarshal(xi, name)
	}
}
