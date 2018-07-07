package main

import "fmt"
import "io"
import "os"

type XP struct {
	out io.Writer
}

func (pk *PublicKey) String() string {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ToStrKey(STRKEY_PUBKEY_ED25519, pk.Ed25519()[:])
	default:
		return fmt.Sprintf("KeyType#%d", int32(pk.Type))
	}
}

func (xp *XP) Marshal(name string, i interface{}) {
	switch v := i.(type) {
	case fmt.Stringer:
		fmt.Fprintf(xp.out, "%s*: %s\n", name, v.String())
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
