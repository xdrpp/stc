package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func (pk *PublicKey) String() string {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ToStrKey(STRKEY_PUBKEY_ED25519, pk.Ed25519()[:])
	default:
		return fmt.Sprintf("KeyType#%d", int32(pk.Type))
	}
}

func (pk *PublicKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, func(c rune) bool {
		return c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
	})
	if err != nil {
		return err
	}
	key, vers := FromStrKey(string(bs))
	switch vers {
	case STRKEY_PUBKEY_ED25519:
		pk.Type = PUBLIC_KEY_TYPE_ED25519
		copy((*pk.Ed25519())[:], key)
		return nil
	default:
		return XdrError("Invalid public key")
	}
}

func txOut(e *TransactionEnvelope) string {
	out := &strings.Builder{}
	b64o := base64.NewEncoder(base64.StdEncoding, out)
	e.XdrMarshal(&XdrOut{b64o}, "")
	b64o.Close()
	return out.String()
}

func txIn(input string) *TransactionEnvelope {
	in := strings.NewReader(input)
	var e TransactionEnvelope
	b64i := base64.NewDecoder(base64.StdEncoding, in)
	e.XdrMarshal(&XdrIn{b64i}, "")
	return &e
}

func txString(t XdrAggregate) string {
	out := &strings.Builder{}
	t.XdrMarshal(&XdrPrint{out}, "")
	return out.String()
}

type XdrScan struct {
	kvs map[string]string
}

func (*XdrScan) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

type xdrPointer interface{
	XdrPointer() interface{}
}

func (xs *XdrScan) Marshal(name string, i interface{}) {
	val, ok := xs.kvs[name]
	switch v := i.(type) {
	case fmt.Scanner:
		if !ok { return }
		_, err := fmt.Sscan(val, v)
		if err != nil {
			xdrPanic("%s", err.Error())
		}
	case XdrPtr:
		val = xs.kvs[name + ".present"]
		for len(val) > 0 && val[0] == ' ' {
			val = val[1:]
		}
		switch val {
		case "false", "":
			v.SetPresent(false)
		case "true":
			v.SetPresent(true)
		default:
			xdrPanic("%s.present (%s) must be true or false", name,
				xs.kvs[name + ".present"])
		}
		v.XdrMarshalValue(xs, name)

	case *XdrSize:
		fmt.Sscan(xs.kvs[name + ".len"], v.XdrPointer())
	case XdrAggregate:
		v.XdrMarshal(xs, name)
	case xdrPointer:
		if !ok { return }
		fmt.Sscan(val, v.XdrPointer())
	default:
		xdrPanic("XdrScan: Don't know how to parse %s\n", name)
	}
	delete(xs.kvs, name)
}

func txScan(t XdrAggregate, in string) {
	kvs := map[string]string{}
	lineno := 0
	for _, line := range strings.Split(in, "\n") {
		lineno++
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		kvs[kv[0]] = kv[1]
	}
	t.XdrMarshal(&XdrScan{kvs}, "")
}

func main() {
	opt_compile := flag.Bool("c", false, "Compile output to binary XDR")
	opt_decompile := flag.Bool("d", false, "Decompile input from binary XDR")
	opt_output := flag.String("o", "", "Output to file instead of stdout")
	flag.Parse()

	var input []byte
	var err error
	switch (len(flag.Args())) {
	case 0:
		input, err = ioutil.ReadAll(os.Stdin)
	case 1:
		input, err = ioutil.ReadFile(flag.Args()[0])
	default:
		flag.Usage()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	sinput := string(input)

	var e *TransactionEnvelope
	if *opt_decompile {
		e = txIn(sinput)
	} else {
		e = &TransactionEnvelope{}
		txScan(e, sinput)
	}

	var output string
	if *opt_compile {
		output = txOut(e) + "\n"
	} else {
		output = txString(e)
	}

	if *opt_output == "" {
		fmt.Print(output)
	} else {
		ioutil.WriteFile(*opt_output, []byte(output), 0666)
	}
}
