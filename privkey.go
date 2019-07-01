package stc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"io"
	"io/ioutil"
	"strings"
)

// Abstract type representing a Stellar private key.  Prints and scans
// in StrKey format.
type PrivateKey struct {
	stcdetail.PrivateKeyInterface
}

func (sec *PrivateKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, stx.IsStrKeyChar)
	if err != nil {
		return err
	}
	key, vers := stx.FromStrKey(bs)
	switch vers {
	case stx.STRKEY_SEED_ED25519:
		sec.PrivateKeyInterface =
			stcdetail.Ed25519Priv(ed25519.NewKeyFromSeed(key))
		return nil
	default:
		return stx.StrKeyError("Invalid private key")
	}
}

// Generates a new Stellar keypair and returns the PrivateKey.
// Currently the only valid value for pkt is
// stx.PUBLIC_KEY_TYPE_ED25519.
func NewPrivateKey(pkt stx.PublicKeyType) PrivateKey {
	switch pkt {
	case stx.PUBLIC_KEY_TYPE_ED25519:
		return PrivateKey{stcdetail.NewEd25519Priv()}
	default:
		panic(fmt.Sprintf("KeyGen: unsupported PublicKeyType %v", pkt))
	}
}

// Writes the a private key to a file in strkey format.  If passphrase
// has non-zero length, then the key is symmetrically encrypted in
// ASCII-armored GPG format.
func (sk *PrivateKey) Save(file string, passphrase []byte) error {
	out := &strings.Builder{}
	if len(passphrase) == 0 {
		fmt.Fprintln(out, sk.String())
	} else {
		w0, err := armor.Encode(out, "PGP MESSAGE", nil)
		if err != nil {
			return err
		}
		w, err := openpgp.SymmetricallyEncrypt(w0, passphrase, nil,
			&packet.Config{
				DefaultCipher:          packet.CipherAES256,
				DefaultCompressionAlgo: packet.CompressionNone,
				S2KCount:               65011712,
			})
		if err != nil {
			w0.Close()
			return err
		}
		fmt.Fprintln(w, sk.String())
		w.Close()
		w0.Close()
		out.WriteString("\n")
	}
	return stcdetail.SafeCreateFile(file, out.String(), 0400)
}

var InvalidPassphrase = errors.New("Invalid passphrase")
var InvalidKeyFile = errors.New("Invalid private key file")

// Reads a private key from a file, prompting for a passphrase if the
// key is in ASCII-armored symmetrically-encrypted GPG format.
func LoadPrivateKey(file string) (*PrivateKey, error) {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	ret := &PrivateKey{}
	if _, err = fmt.Fscan(bytes.NewBuffer(input), ret); err == nil {
		return ret, nil
	}

	block, err := armor.Decode(bytes.NewBuffer(input))
	if err != nil {
		return nil, InvalidKeyFile
	}
	md, err := openpgp.ReadMessage(block.Body, nil,
		func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
			passphrase :=
				stcdetail.GetPass(fmt.Sprintf("Passphrase for %s: ", file))
			if len(passphrase) > 0 {
				return passphrase, nil
			}
			return nil, InvalidPassphrase
		}, nil)
	if err != nil {
		return nil, err
	} else if _, err = fmt.Fscan(md.UnverifiedBody, ret); err != nil {
		return nil, err
	} else if io.Copy(ioutil.Discard, md.UnverifiedBody); md.SignatureError != nil {
		return nil, md.SignatureError
	}
	return ret, nil
}

// Reads a private key from standard input.  If standard input is a
// terminal, disables echo and prints prompt to standard error.
func InputPrivateKey(prompt string) (*PrivateKey, error) {
	key := stcdetail.GetPass(prompt)
	var sk PrivateKey
	if _, err := fmt.Fscan(bytes.NewBuffer(key), &sk); err != nil {
		return nil, err
	}
	return &sk, nil
}
