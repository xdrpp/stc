/*

Stellar transaction compiler library.  Provides functions for
manipulating Stellar transactions, translating them back and forth
between txrep format, and posting them.

*/
package stc

import (
	"strings"
	"stc/stx"
)

type PublicKey = stx.PublicKey
type TransactionResult = stx.TransactionResult

type TransactionEnvelope struct {
	*stx.TransactionEnvelope
	Help map[string]struct{}
}

func NewTransactionEnvelope() *TransactionEnvelope {
	return &TransactionEnvelope{
		TransactionEnvelope: &stx.TransactionEnvelope{},
		Help: nil,
	}
}

func (txe *TransactionEnvelope) GetHelp(name string) bool {
	_, ok := txe.Help[name]
	return ok
}

func (txe *TransactionEnvelope) SetHelp(name string) {
	if txe.Help == nil {
		txe.Help = map[string]struct{}{ name: struct{}{} }
	} else {
		txe.Help[name] = struct{}{}
	}
}

func (net *StellarNet) TxToRep(txe *TransactionEnvelope) string {
	ntxe := struct{
		*TransactionEnvelope
		*StellarNet
	}{ txe, net }
	var out strings.Builder
	stx.XdrToTxrep(&out, ntxe)
	return out.String()
}

func TxFromRep(rep string) (*TransactionEnvelope, stx.TxrepError) {
	in := strings.NewReader(rep)
	txe := NewTransactionEnvelope()
    if err := stx.XdrFromTxrep(in, txe); err != nil {
		return txe, err
	}
	return txe, nil
}

// Convert a TransactionEnvelope to base64-encoded binary XDR format.
func TxToBase64(tx *TransactionEnvelope) string {
	return stx.XdrToBase64(tx)
}

// Parse a TransactionEnvelope from base64-encoded binary XDR format.
func TxFromBase64(input string) (*TransactionEnvelope, error) {
	tx := NewTransactionEnvelope()
	if err := stx.XdrFromBase64(tx, input); err != nil {
		return nil, err
	}
	return tx, nil
}

/*
// Parse a TransactionEnvelope from base64-encoded binary XDR format.
func MustTxFromBase64(input string) *TransactionEnvelope {
	if tx, err := TxFromBase64(input); err != nil {
		panic(err)
	} else {
		return tx
	}
}
*/
