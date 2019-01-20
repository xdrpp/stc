package stc

import "stc/stx"

type PublicKey = stx.PublicKey
type TransactionResult = stx.TransactionResult

type TransactionEnvelope struct {
	*stx.TransactionEnvelope
	Net *StellarNet
	Help map[string]struct{}
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

var _ stx.TxrepAnnotate = &TransactionEnvelope{}
