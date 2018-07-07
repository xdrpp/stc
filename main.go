package main

import "fmt"
import "os"

func (pk *PublicKey) String() string {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ToStrKey(STRKEY_PUBKEY_ED25519, pk.Ed25519()[:])
	default:
		return fmt.Sprintf("KeyType#%d", int32(pk.Type))
	}
}

func main() {
	var e TransactionEnvelope
	e.Tx.TimeBounds = &TimeBounds{MinTime: 12345}
	e.Tx.Memo.Type = MEMO_TEXT
	*e.Tx.Memo.Text() = "Enjoy this transaction"
	e.Tx.Operations = append(e.Tx.Operations, Operation{})
	e.Tx.Operations[0].Body.Type = CREATE_ACCOUNT
	//e.XdrMarshal(&XdrPrint{os.Stdout}, "")
	e.XdrMarshal(&XdrOut{os.Stdout}, "")
	//e.Tx.SourceAccount.XdrMarshal(&XdrOut{os.Stdout}, "")
}
