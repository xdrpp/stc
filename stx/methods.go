
package stx

import "github.com/xdrpp/goxdr/xdr"
import "io"

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

func (body XdrAnon_Operation_Body) To_Operation_Body() XdrAnon_Operation_Body {
	return body
}

func (memo Memo) ToMemo() Memo {
	return memo
}

func (asset Asset) ToAsset() Asset {
	return asset
}

// Alias for the asset code required in AllowTrustOp.  Since the
// issuer is the operation source, an AllowTrustAsset only includes
// the code, not the issuer.
type AllowTrustAsset = XdrAnon_AllowTrustOp_Asset

func (code AllowTrustAsset) ToAllowTrustAsset() AllowTrustAsset {
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

func (txe *TransactionEnvelope) WriteTaggedTx(w io.Writer) {
	out := xdr.XdrOut{ Out: w }
	tp := ENVELOPE_TYPE_TX
	tp.XdrMarshal(out, "")
	txe.Tx.XdrMarshal(out, "")
}
