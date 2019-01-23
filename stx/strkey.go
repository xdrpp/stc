// The stx package provides a compiled go representation of Stellar's
// XDR data structures.
package stx

import (
	"bytes"
	"fmt"
	"encoding/base32"
)

type StrKeyError string
func (e StrKeyError) Error() string { return string(e) }

type StrKeyVersionByte byte

const (
	STRKEY_PUBKEY_ED25519 StrKeyVersionByte = 6  // 'G'
	STRKEY_SEED_ED25519   StrKeyVersionByte = 18 // 'S'
	STRKEY_PRE_AUTH_TX    StrKeyVersionByte = 19 // 'T',
	STRKEY_HASH_X         StrKeyVersionByte = 23 // 'X'
	STRKEY_ERROR          StrKeyVersionByte = 255
)

var crc16table [256]uint16

func init() {
	const poly = 0x1021
	for i := 0; i < 256; i++ {
		crc := uint16(i) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ poly
			} else {
				crc <<= 1
			}
		}
		crc16table[i] = crc
	}
}

func crc16(data []byte) (crc uint16) {
	for _, b := range data {
		temp := b ^ byte(crc>>8)
		crc = crc16table[temp] ^ (crc << 8)
	}
	return
}

// ToStrKey converts the raw bytes of a key to ASCII strkey format.
func ToStrKey(ver StrKeyVersionByte, bin []byte) string {
	var out bytes.Buffer
	out.WriteByte(byte(ver) << 3)
	out.Write(bin)
	sum := crc16(out.Bytes())
	out.WriteByte(byte(sum))
	out.WriteByte(byte(sum >> 8))
	return base32.StdEncoding.EncodeToString(out.Bytes())
}

// FromStrKey decodes a strkey-format string into the raw bytes of the
// key and the type of key.  Returns the reserved StrKeyVersionByte
// STRKEY_ERROR if it fails to decode the string.
func FromStrKey(in string) ([]byte, StrKeyVersionByte) {
	bin, err := base32.StdEncoding.DecodeString(in)
	if err != nil || len(bin) < 3 || bin[0]&7 != 0 {
		return nil, STRKEY_ERROR
	}
	want := uint16(bin[len(bin)-2]) | uint16(bin[len(bin)-1])<<8
	if want != crc16(bin[:len(bin)-2]) {
		return nil, STRKEY_ERROR
	}
	switch len(bin) - 3 {
	case 32:
	default:
		// Just so happens all three key types are currently 32 bytes
		return nil, STRKEY_ERROR
	}
	return bin[1 : len(bin)-2], StrKeyVersionByte(bin[0] >> 3)
}

// Like FromStrKey, but the caller specifies the desired
// StrKeyVersionByte.  Throws an error if the input is either invalid
// or is of a type other than tat designated by the specified
// StrKeyVersionByte.
func MustFromStrKey(want StrKeyVersionByte, in string) []byte {
	bin, ver := FromStrKey(in)
	if bin == nil || ver != want {
		panic(StrKeyError("invalid StrKey"))
	}
	return bin
}

// Renders a PublicKey in strkey format.
func (pk PublicKey) String() string {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ToStrKey(STRKEY_PUBKEY_ED25519, pk.Ed25519()[:])
	default:
		return fmt.Sprintf("PublicKey.Type#%d", int32(pk.Type))
	}
}

// Renders a SignerKey in strkey format.
func (pk SignerKey) String() string {
	switch pk.Type {
	case SIGNER_KEY_TYPE_ED25519:
		return ToStrKey(STRKEY_PUBKEY_ED25519, pk.Ed25519()[:])
	case SIGNER_KEY_TYPE_PRE_AUTH_TX:
		return ToStrKey(STRKEY_PRE_AUTH_TX, pk.PreAuthTx()[:])
	case SIGNER_KEY_TYPE_HASH_X:
		return ToStrKey(STRKEY_HASH_X, pk.HashX()[:])
	default:
		return fmt.Sprintf("SignerKey.Type#%d", int32(pk.Type))
	}
}

// Returns true if c is a valid character in a strkey formatted key.
func IsStrKeyChar(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// Parses a public key in strkey format.
func (pk *PublicKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, IsStrKeyChar)
	if err != nil {
		return err
	}
	key, vers := FromStrKey(string(bs))
	switch vers {
	case STRKEY_PUBKEY_ED25519:
		pk.Type = PUBLIC_KEY_TYPE_ED25519
		copy(pk.Ed25519()[:], key)
		return nil
	default:
		return StrKeyError("Invalid public key type")
	}
}

// Parses a signer in strkey format.
func (pk *SignerKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, IsStrKeyChar)
	if err != nil {
		return err
	}
	key, vers := FromStrKey(string(bs))
	switch vers {
	case STRKEY_PUBKEY_ED25519:
		pk.Type = SIGNER_KEY_TYPE_ED25519
		copy(pk.Ed25519()[:], key)
	case STRKEY_PRE_AUTH_TX:
		pk.Type = SIGNER_KEY_TYPE_PRE_AUTH_TX
		copy(pk.PreAuthTx()[:], key)
	case STRKEY_HASH_X:
		pk.Type = SIGNER_KEY_TYPE_HASH_X
		copy(pk.HashX()[:], key)
	default:
		return StrKeyError("Invalid signer key string")
	}
	return nil
}

func signerHint(bs []byte) (ret SignatureHint) {
	if len(bs) < 4 {
		panic(StrKeyError("signerHint insufficient signer length"))
	}
	copy(ret[:], bs[len(bs)-4:])
	return
}

// Returns the last 4 bytes of a PublicKey, as required for the Hint
// field in a DecoratedSignature.
func (pk *PublicKey) Hint() SignatureHint {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return signerHint(pk.Ed25519()[:])
	default:
		panic(StrKeyError("Invalid public key type"))
	}
}

// Returns the last 4 bytes of a SignerKey, as required for the Hint
// field in a DecoratedSignature.
func (pk *SignerKey) Hint() SignatureHint {
	switch pk.Type {
	case SIGNER_KEY_TYPE_ED25519:
		return signerHint(pk.Ed25519()[:])
	case SIGNER_KEY_TYPE_PRE_AUTH_TX:
		return signerHint(pk.PreAuthTx()[:])
	case SIGNER_KEY_TYPE_HASH_X:
		return signerHint(pk.HashX()[:])
	default:
		panic(StrKeyError("Invalid signer key type"))
	}
}

func (pk *SignerKey) ToSignerKey() *SignerKey {
	return pk
}

func (pk *PublicKey) ToSignerKey() *SignerKey {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		ret := SignerKey { Type: SIGNER_KEY_TYPE_ED25519 }
		*ret.Ed25519() = *pk.Ed25519()
		return &ret
	}
	panic(StrKeyError("Invalid public key type"))
}
