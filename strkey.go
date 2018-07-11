package main

import (
	"bytes"
	"fmt"
	"os"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"golang.org/x/crypto/ed25519"
)

type StrKeyError string
func (e StrKeyError) Error() string { return string(e) }

const (
	MainNet = "Public Global Stellar Network ; September 2015"
	TestNet = "Test SDF Network ; September 2015"
)

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

func ToStrKey(ver StrKeyVersionByte, bin []byte) string {
	var out bytes.Buffer
	out.WriteByte(byte(ver) << 3)
	out.Write(bin)
	sum := crc16(out.Bytes())
	out.WriteByte(byte(sum))
	out.WriteByte(byte(sum >> 8))
	return base32.StdEncoding.EncodeToString(out.Bytes())
}

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

func MustFromStrKey(want StrKeyVersionByte, in string) []byte {
	bin, ver := FromStrKey(in)
	if bin == nil || ver != want {
		panic("invalid StrKey")
	}
	return bin
}

func (pk *PublicKey) String() string {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ToStrKey(STRKEY_PUBKEY_ED25519, pk.Ed25519()[:])
	default:
		return fmt.Sprintf("PublicKey.Type#%d", int32(pk.Type))
	}
}

func (pk *SignerKey) String() string {
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

func isKeyChar(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func (pk *PublicKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, isKeyChar)
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

func (pk *SignerKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, isKeyChar)
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

func (pk *PublicKey) Verify(message, sig []byte) bool {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], message, sig)
	default:
		return false
	}
}

func signerHint(bs []byte) (ret SignatureHint) {
	if len(bs) < 4 {
		panic("SignerHint invalid input")
	}
	copy(ret[:], bs[len(bs)-4:])
	return
}

func (pk *PublicKey) Hint() SignatureHint {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return signerHint(pk.Ed25519()[:])
	default:
		panic(StrKeyError("Invalid public key type"))
	}
}

type Ed25519Priv ed25519.PrivateKey

func (sk Ed25519Priv) String() string {
	return ToStrKey(STRKEY_SEED_ED25519, ed25519.PrivateKey(sk).Seed())
}

func (sk Ed25519Priv) Sign(msg []byte) ([]byte, error) {
	return ed25519.PrivateKey(sk).Sign(rand.Reader, msg, crypto.Hash(0))
}

func (sk Ed25519Priv) Public() *PublicKey {
	ret := PublicKey{ Type: PUBLIC_KEY_TYPE_ED25519 }
	copy(ret.Ed25519()[:], ed25519.PrivateKey(sk).Public().(ed25519.PublicKey))
	return &ret
}

// Use struct instead of interface so we can have Scan method
type PrivateKey struct {
	k interface {
		String() string
		Sign([]byte) ([]byte, error)
		Public() *PublicKey
	}
}
func (sk PrivateKey) String() string { return sk.k.String() }
func (sk PrivateKey) Sign(msg []byte) ([]byte, error) { return sk.k.Sign(msg) }
func (sk PrivateKey) Public() *PublicKey { return sk.k.Public() }

func (sec *PrivateKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, isKeyChar)
	if err != nil {
		return err
	}
	key, vers := FromStrKey(string(bs))
	switch vers {
	case STRKEY_SEED_ED25519:
		sec.k = Ed25519Priv(ed25519.NewKeyFromSeed(key))
		return nil
	default:
		return StrKeyError("Invalid private key")
	}
}

func (sec *PrivateKey) SignTx(network string, e *TransactionEnvelope) error {
	var payload TransactionSignaturePayload
	{
		sum := sha256.Sum256(([]byte)(network))
		copy(payload.NetworkId[:], sum[:])
	}
	payload.TaggedTransaction.Type = ENVELOPE_TYPE_TX
	*payload.TaggedTransaction.Tx() = e.Tx

	sum := sha256.New()
	payload.XdrMarshal(&XdrOut{sum}, "")
	msg := sum.Sum(nil)

	sig, err := sec.Sign(msg)
	if err != nil {
		return err
	}

	e.Signatures = append(e.Signatures, DecoratedSignature{
		Hint: sec.Public().Hint(),
		Signature: sig,
	})
	return nil
}

func genEd25519() PrivateKey {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	return PrivateKey{ Ed25519Priv(sk) }
}

func KeyGen(pkt PublicKeyType) PrivateKey {
	switch pkt {
	case PUBLIC_KEY_TYPE_ED25519:
		return genEd25519()
	default:
		panic(fmt.Sprintf("KeyGen: unsupported PublicKeyType %v", pkt))
	}
}
