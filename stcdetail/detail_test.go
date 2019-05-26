package stcdetail_test

import "fmt"
import "math/rand"
import "testing"
import "github.com/xdrpp/stc"
import . "github.com/xdrpp/stc/stcdetail"

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
	txe.Tx.SourceAccount = mykey.Public()
	txe.Tx.Fee = 100
	txe.Tx.SeqNum = 3319833626148865
	txe.Tx.Memo = stc.MemoText("Hello")
	txe.Append(nil, stc.Payment{
		Destination: yourkey,
		Asset: stc.NativeAsset(),
		Amount: 20000000,
	})
	// ... Can keep appending operations with txe.Append

	// Sign the transaction
	stc.StellarTestNet.SignTx(&mykey, txe)

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
	//                         "asset": "NATIVE",
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
	//             "signature": "XP3Evkw1lWh2/gaIBY0X403UgcR1I3oAHe9GI2h3RhB18jPIe2O5Ld+1zeCcJ/g2HDJbcoJbwxN+Sys4Ew/YAQ=="
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
	txe.Tx.SourceAccount = mykey.Public()
	txe.Tx.Fee = 100
	txe.Tx.SeqNum = 3319833626148865
	txe.Tx.Memo = stc.MemoText("Hello")
	txe.Append(nil, stc.Payment{
		Destination: yourkey,
		Asset: stc.NativeAsset(),
		Amount: 20000000,
	})
	txe.Append(nil, stc.Inflation{})
	txe.Append(&yourkey, stc.AllowTrust{
		Trustor: mykey.Public(),
		Asset: stc.MkAllowTrustAsset("ABCDE"),
		Authorize: true,
	})
	txe.Append(nil, stc.SetOptions{
		InflationDest: stc.NewAccountID(mykey.Public()),
		HomeDomain: stc.NewString("stellar.org"),
		MasterWeight: stc.NewUint(255),
		Signer: stc.NewSignerKey(yourkey, 1),
	})

	// Sign the transaction
	stc.StellarTestNet.SignTx(&mykey, txe)

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
			stc.StellarTestNet.TxToRep(txe), stc.StellarTestNet.TxToRep(txe2),
			string(j))
	}
}

