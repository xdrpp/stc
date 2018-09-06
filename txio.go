
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
)

// pseudo-selectors
const (
	ps_len = "#len"
	ps_present = "#present"
)

func renderByte(b byte) string {
	if b <= ' ' || b >= '\x7f' {
		return fmt.Sprintf("\\x%02x", b)
	} else if b == '\\' {
		return "\\" + string(b)
	}
	return string(b)
}

func renderCode(bs []byte) string {
	var n int
	for n = len(bs); n > 0 && bs[n-1] == 0; n-- {
	}
	out := &strings.Builder{}
	for i := 0; i < n; i++ {
		out.WriteString(renderByte(bs[i]))
	}
	return out.String()
}

// Slightly convoluted logic to avoid throwing away the account name
// in case the code is bad
func scanCode(out []byte, input string) error {
	ss := strings.NewReader(input)
skipspace:
	if r, _, err := ss.ReadRune(); unicode.IsSpace(r) {
		goto skipspace
	} else if err == nil {
		ss.UnreadRune()
	}
	var i int
	var r = ' '
	var err error
	for i = 0; i < len(out); i++ {
		r, _, err = ss.ReadRune()
		if err == io.EOF || unicode.IsSpace(r) {
			break
		} else if err != nil {
			return err
		} else if r <= ' ' || r >= 127 {
			err = StrKeyError("Invalid character in AssetCode")
			break
		} else if r != '\\' {
			out[i] = byte(r)
			continue
		}
		r, _, err = ss.ReadRune()
		if err != nil {
			return err
		} else if r != 'x' {
			out[i] = byte(r)
		} else if _, err = fmt.Fscanf(ss, "%02x", &out[i]); err != nil {
			return err
		}
	}
	for ; i < len(out); i++ {
		out[i] = 0
	}
	r, _, err = ss.ReadRune()
	if err != io.EOF && !unicode.IsSpace(r) {
		return StrKeyError("AssetCode too long")
	}
	return nil
}

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
	Net *StellarNet
	Help XdrHelp
	inAsset bool
}

func (xp *TxStringCtx) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

func (xp *TxStringCtx) Marshal_SequenceNumber(name string, v *SequenceNumber) {
	fmt.Fprintf(xp.Out, "%s: %d\n", name, *v)
}

type xdrPointer interface {
	XdrPointer() interface{}
}

var exp10 [20]uint64
func init() {
	val := uint64(1)
	for i := 0; i < len(exp10); i++ {
		exp10[i] = val
		val *= 10
	}
}

func ScalePrint(val int64, exp int) string {
	mag := uint64(val)
	if val < 0 { mag = uint64(-val) }
	unit := exp10[exp]

	out := ""
	for tmag := mag/unit;; tmag /= 1000 {
		if out != "" { out = "," + out }
		if tmag < 1000 {
			out = fmt.Sprintf("%d", tmag) + out
			break
		}
		out = fmt.Sprintf("%03d", tmag % 1000) + out
	}
	if val < 0 { out = "-" + out }

	mag %= unit
	if mag > 0 {
		out += strings.TrimRight(fmt.Sprintf(".%0*d", exp, mag), "0")
	}
	return out + "e" + fmt.Sprintf("%d", exp)
}

func dateComment(ut uint64) string {
	it := int64(ut)
	if it <= 0 { return "" }
	return fmt.Sprintf(" (%s)", time.Unix(it, 0).Format(time.UnixDate))
}

type xdrEnumNames interface {
	fmt.Stringer
	fmt.Scanner
	XdrEnumNames() map[int32]string
}

func (xp *TxStringCtx) Marshal(name string, i XdrType) {
	switch v := i.(type) {
	case *TimeBounds:
		fmt.Fprintf(xp.Out, "%s.MinTime: %d%s\n%s.MaxTime: %d%s\n",
			name, v.MinTime, dateComment(v.MinTime),
			name, v.MaxTime, dateComment(v.MaxTime))
	case *AccountID:
		ac := v.String()
		if hint := xp.Net.Accounts[ac]; hint != "" {
			fmt.Fprintf(xp.Out, "%s: %s (%s)\n", name, ac, hint)
		} else {
			fmt.Fprintf(xp.Out, "%s: %s\n", name, ac)
		}
	case xdrEnumNames:
		if xp.Help[name] {
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
	case XdrArrayOpaque:
		if xp.inAsset {
			fmt.Fprintf(xp.Out, "%s: %s\n", name, renderCode(v.GetByteSlice()))
		} else {
			fmt.Fprintf(xp.Out, "%s: %s\n", name, v.String())
		}
	case *XdrInt64:
		fmt.Fprintf(xp.Out, "%s: %s (%s)\n", name, v.String(),
			ScalePrint(int64(*v), 7))
	case fmt.Stringer:
		fmt.Fprintf(xp.Out, "%s: %s\n", name, v.String())
	case XdrPtr:
		fmt.Fprintf(xp.Out, "%s%s: %v\n", name, ps_present, v.GetPresent())
		v.XdrMarshalValue(xp, name)
	case XdrVec:
		fmt.Fprintf(xp.Out, "%s%s: %d\n", name, ps_len, v.GetVecLen())
		v.XdrMarshalN(xp, name, v.GetVecLen())
	case *Asset:
		xp.inAsset = true
		defer func() { xp.inAsset = false }()
		v.XdrMarshal(xp, name)
	case XdrAggregate:
		v.XdrMarshal(xp, name)
	default:
		fmt.Fprintf(xp.Out, "%s: %v\n", name, i)
	}
}

func (ctx TxStringCtx) Exec() {
	ctx.Env.Tx.XdrMarshal(&ctx, "tx")
	fmt.Fprintf(ctx.Out, "signatures%s: %d\n", ps_len, len(ctx.Env.Signatures))
	for i := range(ctx.Env.Signatures) {
		var hint string
		if ski := ctx.Net.Signers.Lookup(ctx.Net, ctx.Env, i); ski != nil {
			hint = fmt.Sprintf("%x (%s)", ctx.Env.Signatures[i].Hint, *ski)
		} else {
			hint = fmt.Sprintf(
				"%x (bad signature/unknown key/-net=%s is wrong)",
				ctx.Env.Signatures[i].Hint, ctx.Net.Name)
		}
		fmt.Fprintf(ctx.Out,
			`signatures[%d].hint: %s
signatures[%[1]d].signature: %[3]x
`, i, hint, ctx.Env.Signatures[i].Signature)
	}
}

type XdrHelp map[string]bool

type XdrScan struct {
	kvs map[string]string
	help XdrHelp
	err bool
	inAsset bool
}

func (*XdrScan) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

func (xs *XdrScan) Marshal(name string, i XdrType) {
	val, ok := xs.kvs[name]
	switch v := i.(type) {
	case XdrArrayOpaque:
		var err error
		if xs.inAsset {
			err = scanCode(v.GetByteSlice(), val)
		} else if !ok {
			return
		} else {
			_, err = fmt.Sscan(val, v)
		}
		if err != nil {
			xs.err = true
			fmt.Fprintln(os.Stderr, err.Error())
			xs.help[name] = true
		}
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
		val = "false"
		fmt.Sscanf(xs.kvs[name + ps_present], "%s", &val)
		switch val {
		case "false":
			v.SetPresent(false)
		case "true":
			v.SetPresent(true)
		default:
			// We are throwing error anyway, so also try parsing any fields
			v.SetPresent(true)
			xs.err = true
			fmt.Fprintf(os.Stderr, "%s%s (%s) must be true or false\n",
				name, ps_present, val)
		}
		v.XdrMarshalValue(xs, name)
	case *XdrSize:
		var size uint32
		fmt.Sscan(xs.kvs[name + ps_len], &size)
		if size <= v.XdrBound() {
			v.SetU32(size)
		} else {
			v.SetU32(v.XdrBound())
			xs.err = true
			fmt.Fprintf(os.Stderr, "%s%s (%d) exceeds maximum size %d.\n",
				name, ps_len, size, v.XdrBound())
		}
	case *Asset:
		xs.inAsset = true
		defer func() { xs.inAsset = false }()
		v.XdrMarshal(xs, name)
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

func txScan(t XdrAggregate, in string) (help XdrHelp, err error) {
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
	help = make(XdrHelp)
	lineno := 0
	for _, line := range strings.Split(in, "\n") {
		lineno++
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			if err == nil {
				err = XdrError(fmt.Sprintf("Syntax error on line %d", lineno))
			}
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

