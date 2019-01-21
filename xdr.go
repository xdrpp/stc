package stc

import "strings"
import "stc/stx"

type PublicKey = stx.PublicKey
type TransactionResult = stx.TransactionResult

type TransactionEnvelope struct {
	*stx.TransactionEnvelope
	Help map[string]struct{}
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
	txe := TransactionEnvelope{
		&stx.TransactionEnvelope{},
		make(map[string]struct{}),
	}
    if err := stx.XdrFromTxrep(in, &txe); err != nil {
		return nil, err
	}
	return &txe, nil
}
