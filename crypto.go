
package stc

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func XdrSHA256(t XdrAggregate) []byte {
	sha := sha256.New()
	t.XdrMarshal(&XdrOut{sha}, "")
	return sha.Sum(nil)
}

func TxPayloadHash(network string, e *TransactionEnvelope) []byte {
	payload := TransactionSignaturePayload{
		NetworkId: sha256.Sum256(([]byte)(network)),
	}
	payload.TaggedTransaction.Type = ENVELOPE_TYPE_TX
	*payload.TaggedTransaction.Tx() = e.Tx
	return XdrSHA256(&payload)
}

func (pk *PublicKey) Verify(message, sig []byte) bool {
	switch pk.Type {
	case PUBLIC_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], message, sig)
	default:
		return false
	}
}

func (pk *SignerKey) VerifyTx(network string, e *TransactionEnvelope,
	sig []byte) bool {
	switch pk.Type {
	case SIGNER_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], TxPayloadHash(network, e), sig)
	case SIGNER_KEY_TYPE_PRE_AUTH_TX:
		return bytes.Equal(TxPayloadHash(network, e), pk.PreAuthTx()[:])
	case SIGNER_KEY_TYPE_HASH_X:
		x := sha256.Sum256(sig)
		return bytes.Equal(x[:], pk.HashX()[:])
	default:
		return false
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
	sig, err := sec.Sign(TxPayloadHash(network, e))
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

type InputLine []byte
func (il *InputLine) Scan(ss fmt.ScanState, _ rune) error {
	if line, err := ss.Token(false, func (r rune) bool {
		return r != '\n'
	}); err != nil {
		return err
	} else {
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		*il = InputLine(line)
		return nil
	}
}

var PassphraseFile io.Reader = os.Stdin
var PassphrasePrompt io.Writer = os.Stderr

func getTtyFd(f interface{}) int {
	if file, ok := f.(*os.File); ok && terminal.IsTerminal(int(file.Fd())) {
		return int(file.Fd())
	}
	return -1
}

func GetPass(prompt string) []byte {
	if PassphraseFile == nil {
		var err error
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err == nil {
			PassphraseFile = tty
			PassphrasePrompt = tty
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
			PassphraseFile = io.MultiReader()
			PassphrasePrompt = ioutil.Discard
		}
	}

	if fd := getTtyFd(PassphraseFile); fd >= 0 {
		fmt.Fprint(PassphrasePrompt, prompt)
		bytePassword, _ := terminal.ReadPassword(fd)
		fmt.Fprintln(PassphrasePrompt, "")
		return bytePassword
	} else {
		var line InputLine
		fmt.Fscanln(PassphraseFile, &line)
		return []byte(line)
	}
}

func GetPass2(prompt string) []byte {
	for {
		pw1 := GetPass(prompt)
		if len(pw1) == 0 || getTtyFd(PassphraseFile) < 0 {
			return pw1
		}
		pw2 := GetPass("Again: ")
		if bytes.Compare(pw1, pw2) == 0 {
			return pw1
		}
		fmt.Fprintln(PassphrasePrompt, "The two do not match.")
	}
}

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
				DefaultCipher: packet.CipherAES256,
				DefaultCompressionAlgo: packet.CompressionNone,
				S2KCount: 65011712,
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
	return SafeWriteFile(file, out.String(), 0600)
}

var InvalidPassphrase = errors.New("Invalid passphrase")
var InvalidKeyFile = errors.New("Invalid private key file")

func PrivateKeyFromFile(file string) (*PrivateKey, error) {
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
			passphrase := GetPass(fmt.Sprintf("Passphrase for %s: ", file))
			if len(passphrase) > 0 {
				return passphrase, nil
			}
			return nil, InvalidPassphrase
		}, nil)
	if err != nil {
		return nil, err
	} else if _, err = fmt.Fscan(md.UnverifiedBody, ret); err != nil {
		return nil, err
	} else if io.Copy(ioutil.Discard, md.UnverifiedBody);
	md.SignatureError != nil {
		return nil, md.SignatureError
	}
	return ret, nil
}

func PrivateKeyFromInput(prompt string) (*PrivateKey, error) {
	key := GetPass(prompt)
	var sk PrivateKey
	if _, err := fmt.Fscan(bytes.NewBuffer(key), &sk); err != nil {
		return nil, err
	}
	return &sk, nil
}
