
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

func txOut(e XdrAggregate) string {
	out := &strings.Builder{}
	b64o := base64.NewEncoder(base64.StdEncoding, out)
	e.XdrMarshal(&XdrOut{b64o}, "")
	b64o.Close()
	return out.String()
}

func txIn(e XdrAggregate, input string) (err error) {
	defer func() {
		if i := recover(); i != nil {
			if xe, ok := recover().(XdrError); ok {
				err = xe
				fmt.Fprintln(os.Stderr, xe)
				return
			}
			panic(i)
		}
	}()
	in := strings.NewReader(input)
	b64i := base64.NewDecoder(base64.StdEncoding, in)
	e.XdrMarshal(&XdrIn{b64i}, "")
	return nil
}

type TxStringCtx struct {
	Out io.Writer
	Env *TransactionEnvelope
	Signers SignerCache
	Accounts AccountHints
	Net *StellarNet
	Verbose bool
	Help map[string]bool
}

func (xp *TxStringCtx) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

type xdrPointer interface {
	XdrPointer() interface{}
}

type xdrEnumNames interface {
	fmt.Stringer
	fmt.Scanner
	XdrEnumNames() map[int32]string
}

func (xp *TxStringCtx) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case *AccountID:
		ac := v.String()
		if hint := xp.Accounts[ac]; hint != "" {
			fmt.Fprintf(xp.Out, "%s: %s (%s)\n", name, ac, hint)
		} else {
			fmt.Fprintf(xp.Out, "%s: %s\n", name, ac)
		}
	case xdrEnumNames:
		if xp.Verbose || xp.Help[name] {
			fmt.Fprintf(xp.Out, "%s: %s (", name, v.String())
			var notfirst bool
			for _, name := range v.XdrEnumNames() {
				if notfirst {
					fmt.Fprintf(xp.Out, ", %s", name)
				} else {
					notfirst = true
					fmt.Fprintf(xp.Out, "%s", name)
				}
			}
			fmt.Fprintf(xp.Out, ")\n")
		} else {
			fmt.Fprintf(xp.Out, "%s: %s\n", name, v.String())
		}
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

func (ctx TxStringCtx) Exec() {
	ctx.Env.Tx.XdrMarshal(&ctx, "Tx")
	fmt.Fprintf(ctx.Out, "Signatures.len: %d\n", len(ctx.Env.Signatures))
	for i := range(ctx.Env.Signatures) {
		var hint string
		if ski := ctx.Signers.Lookup(ctx.Net, ctx.Env, i); ski != nil {
			hint = fmt.Sprintf("%x (%s)", ctx.Env.Signatures[i].Hint, *ski)
		} else {
			hint = fmt.Sprintf("%x BAD SIGNATURE", ctx.Env.Signatures[i].Hint)
		}
		fmt.Fprintf(ctx.Out,
`Signatures[%d].Hint: %s
Signatures[%[1]d].Signature: %[3]x
`, i, hint, ctx.Env.Signatures[i].Signature)
	}
}


type XdrScan struct {
	kvs map[string]string
	help map[string]bool
	err bool
}

func (*XdrScan) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

func (xs *XdrScan) Marshal(name string, i interface{}) {
	val, ok := xs.kvs[name]
	switch v := i.(type) {
	case fmt.Scanner:
		if !ok { return }
		_, err := fmt.Sscan(val, v)
		if err != nil {
			xs.err = true
			fmt.Fprintln(os.Stderr, err.Error())
			xs.help[name] = true
		} else if len(val) > 0 && val[len(val)-1] == '?' {
			xs.help[name] = true
		}
	case XdrPtr:
		fmt.Sscanf(xs.kvs[name + ".present"], "%s", &val)
		switch val {
		case "false", "":
			v.SetPresent(false)
		case "true":
			v.SetPresent(true)
		default:
			xs.err = true
			fmt.Fprintf(os.Stderr, "%s.present (%s) must be true or false\n",
				name, val)
		}
		v.XdrMarshalValue(xs, name)
	case *XdrSize:
		var size uint32
		fmt.Sscan(xs.kvs[name + ".len"], &size)
		if size <= v.XdrBound() {
			v.SetU32(size)
		} else {
			v.SetU32(v.XdrBound())
			xs.err = true
			fmt.Fprintf(os.Stderr, "%s.len (%d) exceeds maximum size %d.\n",
				name, size, v.XdrBound())
		}
	case XdrAggregate:
		v.XdrMarshal(xs, name)
	case xdrPointer:
		if !ok { return }
		fmt.Sscan(val, v.XdrPointer())
	default:
		xdrPanic("XdrScan: Don't know how to parse %s.\n", name)
	}
	delete(xs.kvs, name)
}

func txScan(t XdrAggregate, in string) (help map[string]bool, err error) {
	defer func() {
		if i := recover(); i != nil {
			switch i.(type) {
			case XdrError, StrKeyError:
				err = i.(error)
				fmt.Fprintln(os.Stderr, err)
				return
			}
			panic(i)
		}
	}()
	kvs := map[string]string{}
	help = map[string]bool{}
	lineno := 0
	for _, line := range strings.Split(in, "\n") {
		lineno++
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		kvs[kv[0]] = kv[1]
	}
	x := XdrScan{kvs: kvs, help: help}
	t.XdrMarshal(&x, "")
	if x.err {
		err = XdrError("Some fields could not be parsed")
	}
	return
}

