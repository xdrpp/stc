
package stx

import "github.com/xdrpp/goxdr/xdr"
import "io"

func (acct *MuxedAccount) ToMuxedAccount() *MuxedAccount {
	return acct
}

func (acct AccountID) ToMuxedAccount() *MuxedAccount {
	switch acct.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		ret := &MuxedAccount{
			Type: KEY_TYPE_ED25519,
		}
		*ret.Ed25519() = *acct.Ed25519()
		return ret
	default:
		return nil
	}
}

type IsAccount interface {
	String() string
	ToMuxedAccount() *MuxedAccount
}

func (pk SignerKey) ToSignerKey() SignerKey {
	return pk
}

func (pk PublicKey) ToSignerKey() SignerKey {
       switch pk.Type {
       case PUBLIC_KEY_TYPE_ED25519:
               ret := SignerKey { Type: SIGNER_KEY_TYPE_ED25519 }
               *ret.Ed25519() = *pk.Ed25519()
               return ret
       }
       panic(StrKeyError("Invalid public key type"))
}

func (ma MuxedAccount) ToSignerKey() (ret SignerKey) {
	switch ma.Type {
	case KEY_TYPE_ED25519:
		ret.Type = SIGNER_KEY_TYPE_ED25519
		*ret.Ed25519() = *ma.Ed25519()
	case KEY_TYPE_MUXED_ED25519:
		ret.Type = SIGNER_KEY_TYPE_ED25519
		*ret.Ed25519() = ma.Med25519().Ed25519
	default:
		panic(StrKeyError("Invalid MuxedAccount type"))
	}
	return
}

func (body XdrAnon_Operation_Body) To_Operation_Body() XdrAnon_Operation_Body {
	return body
}

func (memo Memo) ToMemo() Memo {
	return memo
}

func (asset Asset) ToAsset() Asset {
	return asset
}

func (code AssetCode) ToAssetCode() AssetCode {
	return code
}

// Types that can be hashed
type Signable interface {
    // Writes the signature payload *without* the network ID.  Be sure
    // to write the SHA256 hash of the network ID before calling this.
	WriteTaggedTx(io.Writer)
}

func (t *TransactionSignaturePayload) WriteTaggedTx(w io.Writer) {
	t.XdrMarshal(&xdr.XdrOut{ Out: w }, "")
}

func (tx *Transaction) WriteTaggedTx(w io.Writer) {
	out := xdr.XdrOut{ Out: w }
	tp := ENVELOPE_TYPE_TX
	tp.XdrMarshal(out, "")
	tx.XdrMarshal(out, "")
}

func (tx *TransactionV0) WriteTaggedTx(w io.Writer) {
	out := xdr.XdrOut{ Out: w }
	tp := ENVELOPE_TYPE_TX
	tp.XdrMarshal(out, "")
	ktp := KEY_TYPE_ED25519
	ktp.XdrMarshal(out, "")
	tx.XdrMarshal(out, "")
}

func (tx *FeeBumpTransaction) WriteTaggedTx(w io.Writer) {
	out := xdr.XdrOut{ Out: w }
	tp := ENVELOPE_TYPE_TX_FEE_BUMP
	tp.XdrMarshal(out, "")
	tx.XdrMarshal(out, "")
}

func (tx *TransactionV0Envelope) WriteTaggedTx(w io.Writer) {
	tx.Tx.WriteTaggedTx(w)
}

func (tx *TransactionV1Envelope) WriteTaggedTx(w io.Writer) {
	tx.Tx.WriteTaggedTx(w)
}

func (tx *FeeBumpTransactionEnvelope) WriteTaggedTx(w io.Writer) {
	tx.Tx.WriteTaggedTx(w)
}

func (tx *TransactionEnvelope) WriteTaggedTx(w io.Writer) {
	if body, ok := tx.XdrUnionBody().(Signable); ok {
		body.WriteTaggedTx(w)
	} else {
		xdr.XdrPanic("TransactionEnvelope unsupported type %s", tx.Type)
	}
}

func (tx *TransactionEnvelope) Signatures() *[]DecoratedSignature {
	switch (tx.Type) {
	case ENVELOPE_TYPE_TX_V0:
		return &tx.V0().Signatures
	case ENVELOPE_TYPE_TX:
		return &tx.V1().Signatures
	case ENVELOPE_TYPE_TX_FEE_BUMP:
		return &tx.FeeBump().Signatures
	}
	return nil
}

func (tx *TransactionEnvelope) Operations() *[]Operation {
	switch (tx.Type) {
	case ENVELOPE_TYPE_TX_V0:
		return &tx.V0().Tx.Operations
	case ENVELOPE_TYPE_TX:
		return &tx.V1().Tx.Operations
	}
	return nil
}
