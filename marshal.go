package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

type XdrPrint struct {
	Out io.Writer
}

func (xp *XdrPrint) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case fmt.Stringer:
		fmt.Fprintf(xp.Out, "%s: %s\n", name, v.String())
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

func (xo *XdrOut) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case XdrNum32:
		put32(xo.Out, v.GetU32())
	case XdrNum64:
		put64(xo.Out, v.GetU64())
	case XdrBytes:
		putBytes(xo.Out, v.GetByteSlice())
	case XdrAggregate:
		v.XdrMarshal(xo, name)
	default:
		panic(fmt.Sprintf("XdrOut: unhandled type %T", i))
	}
}

