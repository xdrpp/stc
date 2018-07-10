package main

import (
	"encoding/base64"
	"fmt"
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

func txOut(e *TransactionEnvelope) {
	b64o := base64.NewEncoder(base64.StdEncoding, os.Stdout)
	e.XdrMarshal(&XdrOut{b64o}, "")
	b64o.Close()
	os.Stdout.Write([]byte("\n"))
}

func txIn() *TransactionEnvelope {
	var e TransactionEnvelope
	b64i := base64.NewDecoder(base64.StdEncoding, os.Stdin)
	e.XdrMarshal(&XdrIn{b64i}, "")
	return &e
}

func txPrint(t XdrAggregate) {
	t.XdrMarshal(&XdrPrint{os.Stdout}, "")
}

func txString(t XdrAggregate) string {
	out := &strings.Builder{}
	t.XdrMarshal(&XdrPrint{out}, "")
	return out.String()
}

func main() {

	var e TransactionEnvelope
	_ = e
	e.Tx.TimeBounds = &TimeBounds{MinTime: 12345}
	e.Tx.Memo.Type = MEMO_TEXT
	*e.Tx.Memo.Text() = "Enjoy this transaction"
	e.Tx.Operations = append(e.Tx.Operations, Operation{})
	e.Tx.Operations[0].Body.Type = CREATE_ACCOUNT

	{
		out := &strings.Builder{}
		e.XdrMarshal(&XdrOut{out}, "")
		var e1 TransactionEnvelope
		e1.XdrMarshal(&XdrIn{strings.NewReader(out.String())}, "")
		if (txString(&e) != txString(&e1)) {
			panic("unmarshal does not match")
		}
	}
	//txPrint(&e)

	txPrint(txIn())
	return

	//txOut(&e)

	//e.XdrMarshal(&XdrPrint{os.Stdout}, "")
	//e.XdrMarshal(&XdrOut{os.Stdout}, "")
	//e.Tx.SourceAccount.XdrMarshal(&XdrOut{os.Stdout}, "")
}
