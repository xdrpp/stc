package main

import "bytes"
// import "fmt"
// import "os"
// import "crypto/rand"
import "encoding/base32"
// import "golang.org/x/crypto/ed25519"

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
	return bin[1 : len(bin)-2], StrKeyVersionByte(bin[0] >> 3)
}

func MustFromStrKey(want StrKeyVersionByte, in string) []byte {
	bin, ver := FromStrKey(in)
	if bin == nil || ver != want {
		panic("invalid StrKey")
	}
	return bin
}

/*
func main() {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	pks := ToStrKey(STRKEY_PUBKEY_ED25519, pk)
	sks := ToStrKey(STRKEY_SEED_ED25519, sk.Seed())

	pk1 := MustFromStrKey(STRKEY_PUBKEY_ED25519, pks)
	sk1 := MustFromStrKey(STRKEY_SEED_ED25519, sks)

	fmt.Println(pks, sks)
	if bytes.Compare(pk1, pk) != 0 {
		fmt.Println("pk borked", pk, pk1)
	}
	if bytes.Compare(sk1, sk.Seed()) != 0 {
		fmt.Println("sk borked", sk.Seed(), sk1)
	}
}
*/
