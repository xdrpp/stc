package stc

import (
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"reflect"
	"strings"
	"testing"
)

import "github.com/xdrpp/stc/stx"

func failUnlessPanic(t *testing.T) {
	if i := recover(); i == nil {
		t.Error("should have panicked but didn't")
	}
}

func TestShortStrKey(t *testing.T) {
	mykey := "GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G"
	for i := 1; i < i; i++ {
		var pk PublicKey
		var sgk SignerKey
		if n, err := fmt.Sscan(mykey[:len(mykey)-i], &pk); err == nil ||
			n >= 1 {
			t.Errorf("incorrectly accepted PubKey strkey of length %d",
				len(mykey)-1)
		}
		if n, err := fmt.Sscan(mykey[:len(mykey)-i], &sgk); err == nil ||
			n >= 1 {
			t.Errorf("incorrectly accepted SignerKey strkey of length %d",
				len(mykey)-1)
		}
	}
}

func TestLongStrKey(t *testing.T) {
	mykey := "GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G"
	mykey += mykey
	for i := 1; i < i; i++ {
		var pk PublicKey
		var sgk SignerKey
		if n, err := fmt.Sscan(mykey[:len(mykey)-i], &pk); err == nil ||
			n >= 1 {
			t.Errorf("incorrectly accepted PubKey strkey of length %d",
				len(mykey)-1)
		}
		if n, err := fmt.Sscan(mykey[:len(mykey)-i], &sgk); err == nil ||
			n >= 1 {
			t.Errorf("incorrectly accepted SignerKey strkey of length %d",
				len(mykey)-1)
		}
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

func TestAppend(t *testing.T) {
	acct := AccountID{}
	txe := NewTransactionEnvelope()
	txe.Append(nil, CreateAccount{
		Destination:     AccountID{},
		StartingBalance: 15000000,
	})
	txe.Tx.Operations = make([]stx.Operation, stx.MAX_OPS_PER_TX-1)
	txe.Append(nil, AllowTrust{
		Trustor:   acct,
		Asset:     MkAllowTrustAsset("ABCDE"),
		Authorize: true,
	})
	defer failUnlessPanic(t)
	txe.Append(nil, CreateAccount{
		Destination:     AccountID{},
		StartingBalance: 15000000,
	})
}

func TestMaxInt64(t *testing.T) {
	if MaxInt64 != 9223372036854775807 {
		t.Error("MaxInt64 is wrong")
	}
	if MaxInt64 != int64(^uint64(0)>>1) {
		t.Error("MaxInt64 is wrong")
	}
}

func TestParseTxrep(t *testing.T) {
	var yourkey PublicKey
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
		&yourkey)

	txe := NewTransactionEnvelope()
	fmt.Sscan("GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G",
		&txe.Tx.SourceAccount)
	var ot stx.OperationType
	for i := range ot.XdrEnumNames() {
		var op stx.Operation
		op.Body.Type = stx.OperationType(i)
		txe.Tx.Operations = append(txe.Tx.Operations, op)
	}
	stcdetail.ForEachXdr(txe, func(i stx.XdrType) bool {
		switch v := i.(type) {
		case interface{ XdrInitialize() }:
			v.XdrInitialize()
		case stx.XdrPtr:
			v.SetPresent(true)
		case *stx.AccountID:
			*v = yourkey
		case stx.XdrNum64:
			v.SetU64(1)
		case stx.XdrVarBytes:
			v.SetByteSlice([]byte("X"))
		case stx.XdrBytes:
			v.GetByteSlice()[0] = 'Y'
		}
		return false
	})

	rep := StellarTestNet.TxToRep(txe)
	txe2, err := TxFromRep(rep)
	if err != nil {
		t.Errorf("parsing txrep failed: %s", err)
	} else if TxToBase64(txe) != TxToBase64(txe2) {
		t.Error("txrep round-trip failed")
	}
}

func TestXdr(t *testing.T) {
	var yourkey PublicKey
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
		&yourkey)

	txe := NewTransactionEnvelope()
	fmt.Sscan("GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G",
		&txe.Tx.SourceAccount)
	var ot stx.OperationType
	for i := range ot.XdrEnumNames() {
		var op stx.Operation
		op.Body.Type = stx.OperationType(i)
		txe.Tx.Operations = append(txe.Tx.Operations, op)
	}
	stcdetail.ForEachXdr(txe, func(i stx.XdrType) bool {
		switch v := i.(type) {
		case interface{ XdrInitialize() }:
			v.XdrInitialize()
		case stx.XdrPtr:
			v.SetPresent(true)
		case *stx.AccountID:
			*v = yourkey
		case stx.XdrNum64:
			v.SetU64(1)
		case stx.XdrVarBytes:
			v.SetByteSlice([]byte("X"))
		case stx.XdrBytes:
			v.GetByteSlice()[0] = 'Y'
		}
		return false
	})

	bin := TxToBase64(txe)
	txe2, err := TxFromBase64(bin)
	if err != nil {
		t.Errorf("unmarshaling failed: %s", err)
		return
	}

	bin2 := TxToBase64(txe2)
	if bin != bin2 || !reflect.DeepEqual(txe, txe2) {
		t.Errorf("binary round-trip failed")
	}
}

func Example_txrep() {
	var mykey PrivateKey
	fmt.Sscan("SDWHLWL24OTENLATXABXY5RXBG6QFPLQU7VMKFH4RZ7EWZD2B7YRAYFS",
		&mykey)

	var yourkey PublicKey
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
		&yourkey)

	// Build a transaction
	txe := NewTransactionEnvelope()
	txe.Tx.SourceAccount = mykey.Public()
	txe.Tx.Fee = 100
	txe.Tx.SeqNum = 3319833626148865
	txe.Tx.Memo = MemoText("Hello")
	txe.Append(nil, Payment{
		Destination: yourkey,
		Asset:       NativeAsset(),
		Amount:      20000000,
	})
	// ... Can keep appending operations with txe.Append

	// Sign the transaction
	StellarTestNet.SignTx(&mykey, txe)

	// Print the transaction in multi-line human-readable "txrep" form
	fmt.Print(StellarTestNet.TxToRep(txe))

	// Output:
	// tx.sourceAccount: GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G
	// tx.fee: 100
	// tx.seqNum: 3319833626148865
	// tx.timeBounds._present: false
	// tx.memo.type: MEMO_TEXT
	// tx.memo.text: "Hello"
	// tx.operations.len: 1
	// tx.operations[0].sourceAccount._present: false
	// tx.operations[0].body.type: PAYMENT
	// tx.operations[0].body.paymentOp.destination: GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L
	// tx.operations[0].body.paymentOp.asset: TestXLM
	// tx.operations[0].body.paymentOp.amount: 20000000 (2e7)
	// tx.ext.v: 0
	// signatures.len: 1
	// signatures[0].hint: e1374741 (bad signature/unknown key/test is wrong network)
	// signatures[0].signature: 5cfdc4be4c35956876fe0688058d17e34dd481c475237a001def46236877461075f233c87b63b92ddfb5cde09c27f8361c325b72825bc3137e4b2b38130fd801
}

func Example_postTransaction() {
	var mykey PrivateKey
	fmt.Sscan("SDWHLWL24OTENLATXABXY5RXBG6QFPLQU7VMKFH4RZ7EWZD2B7YRAYFS",
		&mykey)

	var yourkey PublicKey
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
		&yourkey)

	// Fetch account entry to get sequence number
	myacct, err := StellarTestNet.GetAccountEntry(mykey.Public().String())
	if err != nil {
		panic(err)
	}

	// Build a transaction
	txe := NewTransactionEnvelope()
	txe.Tx.SourceAccount = mykey.Public()
	txe.Tx.SeqNum = myacct.NextSeq()
	txe.Tx.Memo = MemoText("Hello")
	txe.Append(nil, SetOptions{
		SetFlags:      NewUint(uint32(stx.AUTH_REQUIRED_FLAG)),
		LowThreshold:  NewUint(2),
		MedThreshold:  NewUint(2),
		HighThreshold: NewUint(2),
		Signer:        NewSignerKey(yourkey, 1),
	})

	// Pay the median per-operation fee of recent ledgers
	fees, err := StellarTestNet.GetFeeStats()
	if err != nil {
		panic(err)
	}
	txe.Tx.Fee = uint32(len(txe.Tx.Operations)) * fees.Percentile(50)

	// Sign and post the transaction
	StellarTestNet.SignTx(&mykey, txe)
	result, err := StellarTestNet.Post(txe)
	if err != nil {
		panic(err)
	}

	fmt.Println(result)
}
