
package stx

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

func (body XdrAnon_Operation_Body) ToXdrAnon_Operation_Body(
) XdrAnon_Operation_Body {
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
