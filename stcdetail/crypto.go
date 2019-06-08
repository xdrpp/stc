package stcdetail

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/xdrpp/stc/stx"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"os"
)

// Computes the SHA-256 hash of an arbitrary XDR data structure.
func xdrSHA256(t stx.XdrType) (ret stx.Hash) {
	sha := sha256.New()
	t.XdrMarshal(&stx.XdrOut{sha}, "")
	copy(ret[:], sha.Sum(nil))
	return
}

// Returns the transaction hash for a transaction.  The first
// argument, network, is the network name, since the transaction hash
// depends on the particular instantiation of the Stellar network.
func TxPayloadHash(network string, tx *stx.Transaction) *stx.Hash {
	payload := stx.TransactionSignaturePayload{
		NetworkId: sha256.Sum256(([]byte)(network)),
	}
	payload.TaggedTransaction.Type = stx.ENVELOPE_TYPE_TX
	*payload.TaggedTransaction.Tx() = *tx
	ret := xdrSHA256(&payload)
	return &ret
}

/*
func Verify(pk *stx.PublicKey, message []byte, sig []byte) bool {
	switch pk.Type {
	case stx.PUBLIC_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], message, sig)
	default:
		return false
	}
}
*/

// Verify the signature on a transaction.
func VerifyTx(pk *stx.SignerKey, network string, tx *stx.Transaction,
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
	return stx.ToStrKey(stx.STRKEY_SEED_ED25519, ed25519.PrivateKey(sk).Seed())
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

// Writes data tile filename in a safe way, similarly to how emacs
// saves files.  Specifically, if filename is "foo", then data is
// first written to a file called "foo#PID#" (where PID is the current
// process ID) and that file is flushed to disk.  Then, if a file
// called "foo" already exists, "foo" is linked to "foo~" to keep a
// backup.  Finally, "foo#PID#" is renamed to "foo".
func SafeWriteFile(filename string, data string, perm os.FileMode) error {
	tmp := fmt.Sprintf("%s#%d#", filename, os.Getpid())
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
		if tmp != "" {
			os.Remove(tmp)
		}
	}()

	n, err := f.WriteString(data)
	if err != nil {
		return err
	} else if n < len(data) {
		return io.ErrShortWrite
	}
	if err = f.Sync(); err != nil {
		return err
	}
	err = f.Close()
	f = nil
	if err != nil {
		return err
	}

	os.Remove(filename + "~")
	os.Link(filename, filename+"~")
	if err = os.Rename(tmp, filename); err == nil {
		tmp = ""
	}
	return err
}
