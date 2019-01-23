package stc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/xdrpp/stc/detail"
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
	K interface {
		String() string
		Sign([]byte) ([]byte, error)
		Public() *PublicKey
	}
}

// Renders a private key in StrKey format.
func (sk PrivateKey) String() string { return sk.K.String() }

// Signs a raw stream of bytes.  You should generally use StellarNet's
// SignTx() method instead, as Stellar inserts a network id into all
// signed messages for the purposes of domain separation.
func (sk *PrivateKey) Sign(msg []byte) ([]byte, error) { return sk.K.Sign(msg) }

// Returns the public key corresponding to the private key.
func (sk *PrivateKey) Public() *PublicKey { return sk.K.Public() }

func (sec *PrivateKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, stx.IsStrKeyChar)
	if err != nil {
		return err
	}
	key, vers := stx.FromStrKey(string(bs))
	switch vers {
	case stx.STRKEY_SEED_ED25519:
		sec.K = detail.Ed25519Priv(ed25519.NewKeyFromSeed(key))
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
		return PrivateKey{detail.NewEd25519Priv()}
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
	return detail.SafeWriteFile(file, out.String(), 0600)
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
				detail.GetPass(fmt.Sprintf("Passphrase for %s: ", file))
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
	key := detail.GetPass(prompt)
	var sk PrivateKey
	if _, err := fmt.Fscan(bytes.NewBuffer(key), &sk); err != nil {
		return nil, err
	}
	return &sk, nil
}
