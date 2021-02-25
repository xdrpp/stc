package stcdetail

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/xdrpp/goxdr/xdr"
	"github.com/xdrpp/stc/stx"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"os"
)

// Computes the SHA-256 hash of an arbitrary XDR data structure.
func XdrSHA256(ts ...xdr.XdrType) (ret stx.Hash) {
	sha := sha256.New()
	out := xdr.XdrOut{sha}
	for _, t := range ts {
		t.XdrMarshal(out, "")
	}
	copy(ret[:], sha.Sum(nil))
	return
}

// Returns the transaction hash for a transaction.  The first
// argument, network, is the network name, since the transaction hash
// depends on the particular instantiation of the Stellar network.
func TxPayloadHash(network string, tx stx.Signable) *stx.Hash {
	sha := sha256.New()
	id := sha256.Sum256(([]byte)(network))
	sha.Write(id[:])
	tx.WriteTaggedTx(sha)
	var ret stx.Hash
	copy(ret[:], sha.Sum(nil))
	return &ret
}

// Verify a signature on an arbitrary raw message.  Stellar messages
// should be hashed with the NetworkID before signing or verifying, so
// you probably don't want to use this function.  See VerifyTx and the
// ToSignerKey() method of PublicKey, instead.
func Verify(pk *stx.PublicKey, message []byte, sig []byte) bool {
	switch pk.Type {
	case stx.PUBLIC_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], message, sig)
	default:
		return false
	}
}

// Verify the signature on a transaction.
func VerifyTx(pk *stx.SignerKey, network string, tx stx.Signable,
	sig []byte) bool {
	switch pk.Type {
	case stx.SIGNER_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:],
			TxPayloadHash(network, tx)[:], sig)
	case stx.SIGNER_KEY_TYPE_PRE_AUTH_TX:
		return bytes.Equal(TxPayloadHash(network, tx)[:], pk.PreAuthTx()[:])
	case stx.SIGNER_KEY_TYPE_HASH_X:
		x := sha256.Sum256(sig)
		return bytes.Equal(x[:], pk.HashX()[:])
	default:
		return false
	}
}

type PrivateKeyInterface interface {
	String() string
	Sign([]byte) ([]byte, error)
	Public() stx.PublicKey
}

type Ed25519Priv ed25519.PrivateKey

func (sk Ed25519Priv) String() string {
	return stx.ToStrKey(stx.STRKEY_PRIVKEY|stx.STRKEY_ALG_ED25519,
		ed25519.PrivateKey(sk).Seed())
}

func (sk Ed25519Priv) Sign(msg []byte) ([]byte, error) {
	return ed25519.PrivateKey(sk).Sign(rand.Reader, msg, crypto.Hash(0))
}

func (sk Ed25519Priv) Public() (ret stx.PublicKey) {
	ret.Type = stx.PUBLIC_KEY_TYPE_ED25519
	copy(ret.Ed25519()[:], ed25519.PrivateKey(sk).Public().(ed25519.PublicKey))
	return
}

func NewEd25519Priv() Ed25519Priv {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return Ed25519Priv(sk)
}

// PassphraseFile is the io.Reader from which passphrases should be
// read.  If set to a terminal, then a prompt will be displayed and
// echo will be disabled while the user types the passphrase.  The
// default is os.Stdin.  If set to nil, then GetPass will attempt to
// open /dev/tty.  Set it to io.MultiReader() (i.e., an io.Reader that
// always returns EOF) to assume an empty passphrase every time
// GetPass is called.
var PassphraseFile io.Reader = os.Stdin

// If PassphraseFile is a terminal, then the user will be prompted for
// a password, and this is the terminal to which the prompt should be
// written.  The default is os.Stderr.
var PassphrasePrompt io.Writer = os.Stderr

func getTtyFd(f interface{}) int {
	if file, ok := f.(*os.File); ok && terminal.IsTerminal(int(file.Fd())) {
		return int(file.Fd())
	}
	return -1
}

// Read a passphrase from PassphraseFile and return it as a byte
// array.  If PassphraseFile is nil, attempt to open "/dev/tty".  If
// PassphraseFile is a terminal, then write prompt to PassphrasePrompt
// before reading the passphrase and disable echo.
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
		line, _ := ReadTextLine(PassphraseFile)
		return line
	}
}

// Call GetPass twice until the user enters the same passphrase twice.
// Intended for when the user is selecting a new passphrase, to reduce
// the chances of the user mistyping the passphrase.
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
