package main

import "fmt"
import "io"
import "os"

type XP struct {
	out io.Writer
}

func (xp *XP) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case *PublicKey:
		switch v.Type {
		case PUBLIC_KEY_TYPE_ED25519:
			fmt.Fprintf(xp.out, "%s: %s\n", name,
				ToStrKey(STRKEY_PUBKEY_ED25519, v.Ed25519()[:]))
		}
	case XdrAggregate:
		v.XdrRecurse(xp, name)
	default:
		fmt.Fprintf(xp.out, "%s: %v\n", name, i)
	}
}

func main() {
	var t Transaction
	t.TimeBounds = &TimeBounds{MinTime: 12345}
	t.Memo.Type = MEMO_TEXT
	*t.Memo.Text() = "Enjoy this transaction"
	t.Operations = append(t.Operations, Operation{})
	t.Operations[0].Body.Type = CREATE_ACCOUNT
	XDR_Transaction(&XP{os.Stdout}, "t", &t)
	// XdrPrint(os.Stdout, "", &t)
}
