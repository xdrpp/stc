// The stx package provides a compiled go representation of Stellar's
// XDR data structures.
package stx

import (
	"bytes"
	"fmt"
	"encoding/base32"
	"io"
	"strings"
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
func FromStrKey(in []byte) ([]byte, StrKeyVersionByte) {
	bin := make([]byte, base32.StdEncoding.DecodedLen(len(in)))
	n, err := base32.StdEncoding.Decode(bin, in)
	if err != nil || n != len(bin) || n < 3 || bin[0]&7 != 0 {
		return nil, STRKEY_ERROR
	}
	want := uint16(bin[len(bin)-2]) | uint16(bin[len(bin)-1])<<8
	if want != crc16(bin[:len(bin)-2]) {
		return nil, STRKEY_ERROR
	}
	targetlen := -1
	switch StrKeyVersionByte(bin[0] >> 3) {
		case STRKEY_PUBKEY_ED25519, STRKEY_SEED_ED25519,
		STRKEY_PRE_AUTH_TX, STRKEY_HASH_X:
		targetlen = 32
	}
	if n - 3 != targetlen {
		return nil, STRKEY_ERROR
	}
	return bin[1 : len(bin)-2], StrKeyVersionByte(bin[0] >> 3)
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

func renderByte(b byte) string {
	if b <= ' ' || b >= '\x7f' {
		return fmt.Sprintf("\\x%02x", b)
	} else if b == '\\' || b == ':' {
		return "\\" + string(b)
	}
	return string(b)
}

func RenderAssetCode(bs []byte) string {
	var n int
	for n = len(bs); n > 0 && bs[n-1] == 0; n-- {
	}
	if len(bs) > 4 && n <= 4 {
		n = 5
	}
	out := &strings.Builder{}
	for i := 0; i < n; i++ {
		out.WriteString(renderByte(bs[i]))
	}
	return out.String()
}

// Renders an Asset as Code:AccountID.
func (a Asset) String() string {
	var code []byte
	var issuer *AccountID
	switch a.Type {
	case ASSET_TYPE_NATIVE:
		return "NATIVE"
	case ASSET_TYPE_CREDIT_ALPHANUM4:
		code = a.AlphaNum4().AssetCode[:]
		issuer = &a.AlphaNum4().Issuer
	case ASSET_TYPE_CREDIT_ALPHANUM12:
		code = a.AlphaNum12().AssetCode[:]
		issuer = &a.AlphaNum12().Issuer
	default:
		return fmt.Sprintf("Asset.Type#%d", int32(a.Type))
	}
	return fmt.Sprintf("%s:%s", RenderAssetCode(code), issuer.String())
}

func (a XdrAnon_AllowTrustOp_Asset) String() string {
	switch a.Type {
	case ASSET_TYPE_CREDIT_ALPHANUM4:
		return RenderAssetCode(a.AssetCode4()[:])
	case ASSET_TYPE_CREDIT_ALPHANUM12:
		return RenderAssetCode(a.AssetCode12()[:])
	default:
		return fmt.Sprintf("AllowTrustOp_Asset.Type#%d", int32(a.Type))
	}
}

func ScanAssetCode(input []byte) ([]byte, error) {
	out := make([]byte, 12)
	ss := bytes.NewReader(input)
	var i int
	r := byte(' ')
	var err error
	for i = 0; i < len(out); i++ {
		r, err = ss.ReadByte()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if r <= 32 || r >= 127 {
			return nil, StrKeyError("Invalid character in AssetCode")
		} else if r != '\\' {
			out[i] = byte(r)
			continue
		}
		r, err = ss.ReadByte()
		if err != nil {
			return nil, err
		} else if r != 'x' {
			out[i] = byte(r)
		} else if _, err = fmt.Fscanf(ss, "%02x", &out[i]); err != nil {
			return nil, err
		}
	}
	if ss.Len() > 0 {
		return nil, StrKeyError("AssetCode too long")
	}
	if i <= 4 {
		return out[:4], nil
	}
	return out, nil
}

func (a *Asset) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, nil)
	if err != nil {
		return err
	}
	colon := bytes.LastIndexByte(bs, ':')
	if colon == -1 {
		if len(bs) > 12 {
			return StrKeyError("Asset should be Code:AccountID or NATIVE")
		}
		a.Type = ASSET_TYPE_NATIVE
		return nil
	}
	var issuer AccountID
	if _, err = fmt.Fscan(bytes.NewReader(bs[colon+1:]), &issuer); err != nil {
		return err
	}
	code, err := ScanAssetCode(bs[:colon])
	if err != nil {
		return err
	}
	if len(code) <= 4 {
		a.Type = ASSET_TYPE_CREDIT_ALPHANUM4
		copy(a.AlphaNum4().AssetCode[:], code)
		a.AlphaNum4().Issuer = issuer
	} else {
		a.Type = ASSET_TYPE_CREDIT_ALPHANUM12
		copy(a.AlphaNum12().AssetCode[:], code)
		a.AlphaNum12().Issuer = issuer
	}
	return nil
}

func (a *XdrAnon_AllowTrustOp_Asset) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, nil)
	code, err := ScanAssetCode(bs)
	if err != nil {
		return err
	}
	if len(code) <= 4 {
		a.Type = ASSET_TYPE_CREDIT_ALPHANUM4
		copy(a.AssetCode4()[:], code)
	} else {
		a.Type = ASSET_TYPE_CREDIT_ALPHANUM12
		copy(a.AssetCode12()[:], code)
	}
	return nil
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
	return pk.UnmarshalText(bs)
}

// Parses a signer in strkey format.
func (pk *SignerKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, IsStrKeyChar)
	if err != nil {
		return err
	}
	return pk.UnmarshalText(bs)
}

// Parses a public key in strkey format.
func (pk *PublicKey) UnmarshalText(bs []byte) error {
	key, vers := FromStrKey(bs)
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
func (pk *SignerKey) UnmarshalText(bs []byte) error {
	key, vers := FromStrKey(bs)
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
func (pk PublicKey) Hint() SignatureHint {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return signerHint(pk.Ed25519()[:])
	default:
		panic(StrKeyError("Invalid public key type"))
	}
}

// Returns the last 4 bytes of a SignerKey, as required for the Hint
// field in a DecoratedSignature.
func (pk SignerKey) Hint() SignatureHint {
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
