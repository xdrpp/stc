package stc

import "fmt"
import "os"
import "strings"
import "stc/stx"

type PublicKey = stx.PublicKey
type TransactionResult = stx.TransactionResult

type TransactionEnvelope struct {
	*stx.TransactionEnvelope
	Net *StellarNet
	Help map[string]struct{}
	FileName string
}

func (txe *TransactionEnvelope) AccountIDNote(acct *stx.AccountID) string {
	return txe.Net.AccountIDNote(acct)
}

func (txe *TransactionEnvelope) SignerNote(e *stx.TransactionEnvelope,
	sig *stx.DecoratedSignature) string {
	return txe.Net.SignerNote(e, sig)
}

func (txe *TransactionEnvelope) SetHelp(name string) {
	if txe.Help == nil {
		txe.Help = map[string]struct{}{ name: struct{}{} }
	} else {
		txe.Help[name] = struct{}{}
	}
}

func (txe *TransactionEnvelope) GetHelp(name string) bool {
	_, ok := txe.Help[name]
	return ok
}

func (txe *TransactionEnvelope) Error(lineno int, msg string) {
	if txe.FileName != "" {
		fmt.Fprintf(os.Stderr, "%s:%d: %s\n", txe.FileName, lineno, msg)
	}
}

func (net *StellarNet) TxToRep(txe *TransactionEnvelope) string {
	txe.Net = net
	var out strings.Builder
	stx.XdrToTxrep(&out, txe, txe)
	return out.String()
}

func (net *StellarNet) TxFromRep(
	rep string, errname string) *TransactionEnvelope {
	in := strings.NewReader(rep)
	txe := TransactionEnvelope{
		FileName: errname,
		Net: net,
	}
    if err := stx.XdrFromTxrep(in, &txe, &txe); err != nil {
		return &txe
	}
	return nil
}
