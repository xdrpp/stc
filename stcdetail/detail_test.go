package stcdetail_test

import (
	"fmt"
	"github.com/xdrpp/stc"
	. "github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

func ExampleScaleFmt() {
	fmt.Println(ScaleFmt(987654321, 7))
	// Output:
	// 98.7654321e7
}

func TestJsonInt64e7Conv(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 10000; i++ {
		j := JsonInt64e7(r.Uint64())
		var k JsonInt64e7
		if text, err := j.MarshalText(); err != nil {
			t.Errorf("error marshaling JsonInt64e7 %d: %s", int64(j), err)
		} else if err = k.UnmarshalText(text); err != nil {
			t.Errorf("error unmarshaling JsonInt64e7 %d: %s", int64(j), err)
		} else if k != j {
			t.Errorf("JsonInt64e7 %d (%s) round-trip marshal returns %d",
				int64(j), text, int64(k))
		}
	}
	j := JsonInt64e7(0x7fffffffffffffff)
	var k JsonInt64e7
	if text, err := j.MarshalText(); err != nil {
		t.Errorf("error marshaling JsonInt64e7 %d: %s", int64(j), err)
	} else if err = k.UnmarshalText(text); err != nil {
		t.Errorf("error unmarshaling JsonInt64e7 %d: %s", int64(j), err)
	} else if k != j {
		t.Errorf("JsonInt64e7 %d (%s) round-trip marshal returns %d",
			int64(j), text, int64(k))
	}
}

func TestJsonInt64Conv(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 10000; i++ {
		j := JsonInt64(r.Uint64())
		var k JsonInt64
		if text, err := j.MarshalText(); err != nil {
			t.Errorf("error marshaling JsonInt64 %d: %s", int64(j), err)
		} else if err = k.UnmarshalText(text); err != nil {
			t.Errorf("error unmarshaling JsonInt64 %d: %s", int64(j), err)
		} else if k != j {
			t.Errorf("JsonInt64 %d (%s) round-trip marshal returns %d",
				int64(j), text, int64(k))
		}
	}
}

func ExampleXdrToJson() {
	var mykey stc.PrivateKey
	fmt.Sscan("SDWHLWL24OTENLATXABXY5RXBG6QFPLQU7VMKFH4RZ7EWZD2B7YRAYFS",
		&mykey)

	var yourkey stc.PublicKey
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
		&yourkey)

	// Build a transaction
	txe := stc.NewTransactionEnvelope()
	txe.SetSourceAccount(mykey.Public())
	txe.V1().Tx.SeqNum = 3319833626148865
	txe.V1().Tx.Memo = stc.MemoText("Hello")
	txe.Append(nil, stc.Payment{
		Destination: *yourkey.ToMuxedAccount(),
		Asset:       stc.NativeAsset(),
		Amount:      20000000,
	})
	// ... Can keep appending operations with txe.Append
	txe.SetFee(100)

	// Sign the transaction
	stc.DefaultStellarNet("main").SignTx(&mykey, txe)

	// Print the transaction in JSON
	j, _ := XdrToJson(txe)
	fmt.Print(string(j))

	// Output:
	// {
	//     "tx": {
	//         "sourceAccount": "GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G",
	//         "fee": 100,
	//         "seqNum": "3319833626148865",
	//         "timeBounds": null,
	//         "memo": {
	//             "type": "MEMO_TEXT",
	//             "text": "Hello"
	//         },
	//         "operations": [
	//             {
	//                 "sourceAccount": null,
	//                 "body": {
	//                     "type": "PAYMENT",
	//                     "paymentOp": {
	//                         "destination": "GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
	//                         "asset": "native",
	//                         "amount": "20000000"
	//                     }
	//                 }
	//             }
	//         ],
	//         "ext": {
	//             "v": 0
	//         }
	//     },
	//     "signatures": [
	//         {
	//             "hint": "4TdHQQ==",
	//             "signature": "O/lsKauVcwd1YStamg7GMNd5hGqzGy4H3o0k3pJ5YfhmdgQJGjlC51bg3BTdlEZeK2EyiASB5AMFXsM5BUKVAg=="
	//         }
	//     ]
	// }
}

func TestJsonToXdr(t *testing.T) {
	var mykey stc.PrivateKey
	fmt.Sscan("SDWHLWL24OTENLATXABXY5RXBG6QFPLQU7VMKFH4RZ7EWZD2B7YRAYFS",
		&mykey)

	var yourkey stc.PublicKey
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L",
		&yourkey)

	// Build a transaction
	txe := stc.NewTransactionEnvelope()
	txe.SetSourceAccount(mykey.Public())
	txe.V1().Tx.SeqNum = 3319833626148865
	txe.V1().Tx.Memo = stc.MemoText("Hello")
	txe.Append(nil, stc.Payment{
		Destination: *yourkey.ToMuxedAccount(),
		Asset:       stc.NativeAsset(),
		Amount:      20000000,
	})
	txe.Append(nil, stc.Inflation{})
	txe.Append(yourkey.ToMuxedAccount(), stc.AllowTrust{
		Trustor:   mykey.Public(),
		Asset:     stc.MkAllowTrustAsset("ABCDE"),
		Authorize: uint32(stx.AUTHORIZED_FLAG),
	})
	txe.Append(nil, stc.SetOptions{
		InflationDest: stc.NewAccountID(mykey.Public()),
		HomeDomain:    stc.NewString("stellar.org"),
		MasterWeight:  stc.NewUint(255),
		Signer:        stc.NewSignerKey(yourkey, 1),
	})
	txe.SetFee(100)

	net := stc.DefaultStellarNet("test")
	if net == nil {
		t.Fatal("could not load test net")
	}
	// Sign the transaction
	net.SignTx(&mykey, txe)

	// Print the transaction in JSON
	j, err := XdrToJson(txe)
	if err != nil {
		t.Errorf("XdrToJson: %s", err)
		return
	}

	txe2 := stc.NewTransactionEnvelope()
	if err = JsonToXdr(txe2, j); err != nil {
		t.Errorf("JsonToXdr: %s", err)
		return
	}

	if stc.TxToBase64(txe) != stc.TxToBase64(txe2) {
		t.Errorf("Round-trip error\nWant:\n%sHave:\n%sJson:\n%s",
			net.TxToRep(txe), net.TxToRep(txe2), string(j))
	}
}

func TestMissingByteArray(t *testing.T) {
	in := strings.NewReader("type: MEMO_HASH")
	var m stx.Memo
	err := XdrFromTxrep(in, "", &m)
	if err != nil {
		t.Errorf("%s", err)
	}
}

func TestForEachXdrType(t *testing.T) {
	var e stx.TransactionMetaV1
	e.TxChanges = make([]stx.LedgerEntryChange, 5)
	n := 0
	ForEachXdrType(&e, func(acct *stx.AccountID) {
		n++
	})
	if n != 5 {
		t.Errorf("expected 5 AccountIDs got %d", n)
	}
}

func TestXdrExtract(t *testing.T) {
	var e stx.TransactionMetaV1
	e.TxChanges = make([]stx.LedgerEntryChange, 5)
	var accp *stx.AccountID
	if XdrExtract(&e, &accp) != true ||
		accp != &e.TxChanges[0].Created().Data.Account().AccountID {
		t.Fail()
	}
}

func TestFileChanged(t *testing.T) {
	fi1, e := os.Stat("/etc/fstab")
	if e != nil {
		t.Skip(e)
	}

	f, e := ioutil.TempFile("", "TestFileChanged")
	if e != nil {
		t.Fatal(e)
	}
	tmp := f.Name()
	f.WriteString("Hello world\n")
	f.Sync()
	f.Close()
	fi4, e := os.Stat(tmp)

	// Scary, but the sleep is needed on linux or the tv_nsec fields
	// don't seem to change.
	time.Sleep(time.Second)

	ioutil.ReadFile("/etc/fstab")
	fi2, e := os.Stat("/etc/fstab")
	if e != nil {
		t.Skip(e)
	}
	fi3, e := os.Stat("/etc/hosts")
	if e != nil {
		t.Skip(e)
	}
	if FileChanged(fi1, fi2) {
		t.Errorf("falsely reported file change")
	} else if !FileChanged(fi2, fi3) {
		t.Errorf("failed to detect file change")
	}

	if e = os.Link(tmp, tmp+"~"); e != nil {
		os.Remove(tmp)
		t.Fatal(e)
	}
	fi5, e := os.Stat(tmp)
	if e = os.Remove(tmp + "~"); e != nil {
		os.Remove(tmp)
		t.Fatal(e)
	}
	fi6, e := os.Stat(tmp)

	if !FileChanged(fi4, fi5) {
		t.Errorf("Failed to detect nlink change in %s\n%#v",
			tmp, fi4.Sys())
	} else if !FileChanged(fi4, fi6) {
		t.Errorf("Failed to detect ctime change in %s\n%#v\n%#v",
			tmp, fi5.Sys(), fi6.Sys())
	} else {
		os.Remove(tmp)
	}
}

func ExampleLockFile() error {
	lf, err := LockFile("testfile", 0666)
	if err != nil {
		return err
	}
	defer lf.Abort()

	fmt.Fprintf(lf, "New file contents\n")

	return lf.Commit()
}

func ExampleGetTxrepField() {
	var a1, a2 stx.MuxedAccount
	fmt.Sscan("GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L", &a1)
	fmt.Sscan("GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G", &a2)
	txe := stc.NewTransactionEnvelope()
	txe.Append(nil, stc.Payment{
		Destination: a1,
		Asset:       stc.NativeAsset(),
		Amount:      10000000,
	})

	// The sourceAccount field of a transaction is an AccountID
	*GetTxrepField(txe, "tx.sourceAccount").XdrPointer().(*stx.MuxedAccount) =
		a1

	// The sourceAccount field of an operation is a *AccountID, so the
	// field we get back is of type **AccountID
	op0src := "tx.operations[0].sourceAccount"
	*GetTxrepField(txe, op0src).XdrPointer().(**stx.MuxedAccount) = &a2

	XdrToTxrep(os.Stdout, "", txe)
	// output:
	// type: ENVELOPE_TYPE_TX
	// tx.sourceAccount: GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L
	// tx.fee: 0
	// tx.seqNum: 0
	// tx.timeBounds._present: false
	// tx.memo.type: MEMO_NONE
	// tx.operations.len: 1
	// tx.operations[0].sourceAccount._present: true
	// tx.operations[0].sourceAccount: GDFR4HZMNZCNHFEIBWDQCC4JZVFQUGXUQ473EJ4SUPFOJ3XBG5DUCS2G
	// tx.operations[0].body.type: PAYMENT
	// tx.operations[0].body.paymentOp.destination: GATPALHEEUERWYW275QDBNBMCM4KEHYJU34OPIZ6LKJAXK6B4IJ73V4L
	// tx.operations[0].body.paymentOp.asset: native
	// tx.operations[0].body.paymentOp.amount: 10000000 (1e7)
	// tx.ext.v: 0
	// signatures.len: 0
}
