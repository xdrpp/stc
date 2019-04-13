/*

Stellar transaction compiler library.  Provides functions for
manipulating Stellar transactions, translating them back and forth
between txrep format, and posting them.

*/
package stc

import (
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
	"reflect"
	"strings"
)

type TxrepError = stcdetail.TxrepError
type PublicKey = stx.PublicKey
type AccountID = stx.AccountID
type SignerKey = stx.SignerKey
type Signature = stx.Signature
type TransactionResult = stx.TransactionResult
type LedgerHeader = stx.LedgerHeader

func NativeAsset() stx.Asset {
	return stx.Asset {
		Type: stx.ASSET_TYPE_NATIVE,
	}
}

func MkAsset(acc AccountID, code string) stx.Asset {
	var ret stx.Asset
	if len(code) <= 4 {
		ret.Type = stx.ASSET_TYPE_CREDIT_ALPHANUM4
		copy(ret.AlphaNum4().AssetCode[:], code)
		ret.AlphaNum4().Issuer = acc
	} else if len(code) <= 12 {
		ret.Type = stx.ASSET_TYPE_CREDIT_ALPHANUM12
		copy(ret.AlphaNum12().AssetCode[:], code)
		ret.AlphaNum4().Issuer = acc
	} else {
		stx.XdrPanic("MkAsset: %q exceeds 12 characters", code)
	}
	return ret
}

func MkAllowTrustAsset(code string) stx.AllowTrustAsset {
	var ret stx.AllowTrustAsset
	if len(code) <= 4 {
		ret.Type = stx.ASSET_TYPE_CREDIT_ALPHANUM4
		copy(ret.AssetCode4()[:], code)
	} else if len(code) <= 12 {
		ret.Type = stx.ASSET_TYPE_CREDIT_ALPHANUM12
		copy(ret.AssetCode12()[:], code)
	} else {
		stx.XdrPanic("MkAllowTrustAsset: %q exceeds 12 characters", code)
	}
	return ret
}

type TransactionEnvelope struct {
	*stx.TransactionEnvelope
	Help map[string]struct{}
}

func NewTransactionEnvelope() *TransactionEnvelope {
	return &TransactionEnvelope{
		TransactionEnvelope: &stx.TransactionEnvelope{},
		Help:                nil,
	}
}

// Interface for placeholder types that are named by camel-cased
// versions of the OperationType enum and can be transformed into the
// body of an Operation
type OperationBody interface {
	ToXdrAnon_Operation_Body() stx.XdrAnon_Operation_Body
}

// Append an operation to a transaction envelope.
func (txe *TransactionEnvelope) Append(
	sourceAccount *stx.AccountID,
	body OperationBody) {
	if len(txe.Tx.Operations) >= stx.MAX_OPS_PER_TX {
		stx.XdrPanic("TransactionEnvelope.Op: attempt to exceed %d operations",
			stx.MAX_OPS_PER_TX)
	} else if len(txe.Signatures) > 0 {
		stx.XdrPanic("TransactionEnvelope.Op: transaction already signed")
	}
	txe.Tx.Operations = append(txe.Tx.Operations, stx.Operation {
		SourceAccount: sourceAccount,
		Body: body.ToXdrAnon_Operation_Body(),
	})
}

func (txe *TransactionEnvelope) GetHelp(name string) bool {
	_, ok := txe.Help[name]
	return ok
}

func (txe *TransactionEnvelope) SetHelp(name string) {
	if txe.Help == nil {
		txe.Help = map[string]struct{}{name: struct{}{}}
	} else {
		txe.Help[name] = struct{}{}
	}
}

type txrepHelper = StellarNet

func (net *txrepHelper) SignerNote(txe *stx.TransactionEnvelope,
	sig *stx.DecoratedSignature) string {
	if txe == nil {
		return ""
	} else if ski := net.Signers.Lookup(net.GetNetworkId(), txe, sig);
	ski != nil {
		return ski.String()
	}
	return fmt.Sprintf("bad signature/unknown key/%s is wrong network",
		net.Name)
}

func (net *txrepHelper) AccountIDNote(acct *stx.AccountID) string {
	return net.Accounts[acct.String()]
}

// Convert an arbitrary XDR data structure to human-readable Txrep
// format.
func (net *StellarNet) ToRep(txe stx.XdrAggregate) string {
	var out strings.Builder

	type helper interface {
		stx.XdrAggregate
		GetHelp(string) bool
	}
	if e, ok := txe.(helper); ok {
		ntxe := struct {
			helper
			*txrepHelper
		}{e, (*txrepHelper)(net)}
		stcdetail.XdrToTxrep(&out, ntxe)
	} else {
		ntxe := struct {
			stx.XdrAggregate
			*txrepHelper
		}{txe, (*txrepHelper)(net)}
		stcdetail.XdrToTxrep(&out, ntxe)
	}

	return out.String()
}

// Convert a TransactionEnvelope to human-readable Txrep format.
func (net *StellarNet) TxToRep(txe *TransactionEnvelope) string {
	return net.ToRep(txe)
}

// Parse a transaction in human-readable Txrep format into a
// TransactionEnvelope.
func TxFromRep(rep string) (*TransactionEnvelope, TxrepError) {
	in := strings.NewReader(rep)
	txe := NewTransactionEnvelope()
	if err := stcdetail.XdrFromTxrep(in, txe); err != nil {
		return txe, err
	}
	return txe, nil
}

// Convert a TransactionEnvelope to base64-encoded binary XDR format.
func TxToBase64(tx *TransactionEnvelope) string {
	return stcdetail.XdrToBase64(tx)
}

// Parse a TransactionEnvelope from base64-encoded binary XDR format.
func TxFromBase64(input string) (*TransactionEnvelope, error) {
	tx := NewTransactionEnvelope()
	if err := stcdetail.XdrFromBase64(tx, input); err != nil {
		return nil, err
	}
	return tx, nil
}

type forEachXdr struct{
	fn func(stx.XdrType)bool
}
func (fex forEachXdr) Marshal(_ string, val stx.XdrType) {
	if !fex.fn(val) {
		if xa, ok := val.(stx.XdrAggregate); ok {
			xa.XdrMarshal(fex, "")
		}
	}
}
func (forEachXdr) Sprintf(string, ...interface{}) string {
	return ""
}

// Calls fn, recursively, on every value inside an XdrAggregate.
// Prunes the recursion if fn returns true.
func ForEachXdr(t stx.XdrAggregate, fn func(stx.XdrType)bool) {
	t.XdrMarshal(forEachXdr{fn}, "")
}

type assignXdr struct {
	fields []interface{}
}

func copyOpaqueArray(to stx.XdrArrayOpaque, from interface{}, name string) {
	switch f := from.(type) {
	case []byte:
		if len(f) > len(to) {
			stx.XdrPanic("Set: length %d exceeded for %s",
				len(to), name)
		}
		n := copy(to, f)
		copy(to[n:], make([]byte, len(to)-n))
	case string:
		if len(f) > len(to) {
			stx.XdrPanic("Set: length %d exceeded for %s",
				len(to), name)
		}
		n := copy(to, f)
		copy(to[n:], make([]byte, len(to)-n))
	default:
		vf := reflect.ValueOf(from)
		if vf.Kind() != reflect.Array ||
			vf.Elem().Kind() != reflect.Uint8 ||
			vf.Len() != len(to) {
			stx.XdrPanic("Set: cannot assign %T to %s (type opaque[%d])",
				from, name, len(to))
		}
		reflect.Copy(reflect.ValueOf(to), vf)
	}
}

func (ax *assignXdr) Marshal(name string, val stx.XdrType) {
	if len(ax.fields) == 0 {
		stx.XdrPanic("Set: too few arguments at %s", name)
	}
	if v := reflect.ValueOf(val.XdrPointer()); v.Kind() == reflect.Ptr &&
		reflect.TypeOf(ax.fields[0]).AssignableTo(v.Type().Elem()) {
		f := reflect.ValueOf(ax.fields[0])
		if b, ok := val.(interface{XdrBound() uint32}); ok &&
			(f.Kind() == reflect.Slice || f.Kind() == reflect.String) &&
			uint(f.Len()) > uint(b.XdrBound()) {
			stx.XdrPanic("Set: length %d exceeded for %s",
				b.XdrBound(), name)
		}
		v.Elem().Set(f)
		ax.fields = ax.fields[1:]
		return
	}
	switch t := val.(type) {
	case stx.XdrPtr:
		// Don't recurse, val should have been a pointer
		break
	case stx.XdrVec:
		// Don't recurse, val should have been a slice
		break
	case stx.XdrArrayOpaque:
		copyOpaqueArray(t, ax.fields[0], name)
		ax.fields = ax.fields[1:]
		return
	case stx.XdrAggregate:
		t.XdrMarshal(ax, name)
		return
	}
	stx.XdrPanic("Set: cannot assign %T to %s (type %T)",
		ax.fields[0], name, val.XdrValue())
}

func (*assignXdr) Sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

/*

Assign a set of values to successive fields of an XDR structure in a
type-safe way, flattening out nested structures.  For example, given
the following XDR:

	union Asset switch (AssetType type) {
	case ASSET_TYPE_NATIVE: // Not credit
		void;

	case ASSET_TYPE_CREDIT_ALPHANUM4:
		struct {
			opaque assetCode[4]; // 1 to 4 characters
			AccountID issuer;
		} alphaNum4;

	case ASSET_TYPE_CREDIT_ALPHANUM12:
		struct {
			opaque assetCode[12]; // 5 to 12 characters
			AccountID issuer;
		} alphaNum12;
	};

You can initalize it with the following:

	var asset Asset
	Set(&asset, ASSET_TYPE_CREDIT_ALPHANUM12, "Asset Code", AccountID{})

Fixed-length arrays of size n must be assigned from n successive
arguments passed to Set and cannot be passed as an array.  Slices, by
contrast, must be assigned from slices.  The one exception is
fixed-size array of bytes opaque[n], which can be initialized from a
string, a slice []byte, or an array [n]byte.  The string or slice may
be shorter than n (in which case the remainig bytes are filled with
0), but a byte array must be exactly the same length.  (If you really
must assign from a shorter fixed-length byte array, just slice the
array.)

Note that aggregates can be passed as arguments to assign, in which
case Set will take fewer arguments.  The recursive traversal of
structures stops when it is possible to assign the next value to the
current aggregate.  For example, it is valid to say:

	var asset Asset
	Set(&asset, ASSET_TYPE_CREDIT_ALPHANUM12, otherAsset.AlphaNum12)

*/
func Set(t stx.XdrAggregate, fieldValues ...interface{}) {
	t.XdrMarshal(&assignXdr{fieldValues}, "")
}
