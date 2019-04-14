package stc

import "strings"
import "testing"

import "github.com/xdrpp/stc/stx"

func failUnlessPanic(t *testing.T) {
	if i := recover(); i == nil {
		t.Error("should have panicked but didn't")
	}
}

func TestSetOverflowString(t *testing.T) {
	var m stx.Memo
	// This should work
	Set(&m, stx.MEMO_TEXT, strings.Repeat("@", 28))
	// This shouldn't
	defer failUnlessPanic(t)
	Set(&m, stx.MEMO_TEXT, strings.Repeat("@", 29))
}

func TestSetOverflowVector(t *testing.T) {
	var acct AccountID
	asset := MkAsset(acct, "1234")
	var op stx.PathPaymentOp
	// This should work
	Set(&op, asset, 0, acct, asset, 0, make([]stx.Asset, 5))
	// This shoudn't
	defer failUnlessPanic(t)
	Set(&op, asset, int64(0), acct, asset, int64(0), make([]stx.Asset, 6))
}

func TestInvalidDefault(t *testing.T) {
	net := StellarTestNet
	rep := net.TxToRep(NewTransactionEnvelope())
	rep += "tx.operations.len: 1\n"
	rep += "tx.operations[0].type: ALLOW_TRUST\n"
	if _, err := TxFromRep(rep); err != nil {
		t.Error("Could not translate default AllowTrustOp to/from Txrep")
	}
}
