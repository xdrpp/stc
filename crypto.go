
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
	"stc/stx"
)

// Computes the SHA-256 hash of an arbitrary XDR data structure.
func xdrSHA256(t stx.XdrAggregate) []byte {
	sha := sha256.New()
	t.XdrMarshal(&stx.XdrOut{sha}, "")
	return sha.Sum(nil)
}

// Returns the transaction hash for a transaction.  The first
// argument, network, is the network name, since the transaction hash
// depends on the particular instantiation of the Stellar network.
func TxPayloadHash(network string, e *stx.TransactionEnvelope) []byte {
	payload := stx.TransactionSignaturePayload{
		NetworkId: sha256.Sum256(([]byte)(network)),
	}
	payload.TaggedTransaction.Type = stx.ENVELOPE_TYPE_TX
	*payload.TaggedTransaction.Tx() = e.Tx
	return xdrSHA256(&payload)
}

/*
func verify(pk *PublicKey, message []byte, sig []byte) bool {
	switch pk.Type {
	case stx.PUBLIC_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], message, sig)
	default:
		return false
	}
}
*/

// Verify the signature on a transaction.
func VerifyTx(pk *stx.SignerKey, network string, e *stx.TransactionEnvelope,
	sig []byte) bool {
	switch pk.Type {
	case stx.SIGNER_KEY_TYPE_ED25519:
		return ed25519.Verify(pk.Ed25519()[:], TxPayloadHash(network, e), sig)
	case stx.SIGNER_KEY_TYPE_PRE_AUTH_TX:
		return bytes.Equal(TxPayloadHash(network, e), pk.PreAuthTx()[:])
	case stx.SIGNER_KEY_TYPE_HASH_X:
		x := sha256.Sum256(sig)
		return bytes.Equal(x[:], pk.HashX()[:])
	default:
		return false
	}
}

type ed25519Priv ed25519.PrivateKey

func (sk ed25519Priv) String() string {
	return stx.ToStrKey(stx.STRKEY_SEED_ED25519, ed25519.PrivateKey(sk).Seed())
}

func (sk ed25519Priv) Sign(msg []byte) ([]byte, error) {
	return ed25519.PrivateKey(sk).Sign(rand.Reader, msg, crypto.Hash(0))
}

func (sk ed25519Priv) Public() *PublicKey {
	ret := stx.PublicKey{ Type: stx.PUBLIC_KEY_TYPE_ED25519 }
	copy(ret.Ed25519()[:], ed25519.PrivateKey(sk).Public().(ed25519.PublicKey))
	return &ret
}

// Abstract type representing a Stellar private key.  Prints and scans
// in StrKey format.
type PrivateKey struct {
	k interface {
		String() string
		Sign([]byte) ([]byte, error)
		Public() *PublicKey
	}
}
func (sk PrivateKey) String() string { return sk.k.String() }
func (sk *PrivateKey) sign(msg []byte) ([]byte, error) { return sk.k.Sign(msg) }
func (sk *PrivateKey) Public() *PublicKey { return sk.k.Public() }

func (sec *PrivateKey) Scan(ss fmt.ScanState, _ rune) error {
	bs, err := ss.Token(true, stx.IsStrKeyChar)
	if err != nil {
		return err
	}
	key, vers := stx.FromStrKey(string(bs))
	switch vers {
	case stx.STRKEY_SEED_ED25519:
		sec.k = ed25519Priv(ed25519.NewKeyFromSeed(key))
		return nil
	default:
		return stx.StrKeyError("Invalid private key")
	}
}

// Signs a transaction and appends the signature to the Signatures
// list in the TransactionEnvelope.
func (sec *PrivateKey) SignTx(network string, e *TransactionEnvelope) error {
	sig, err := sec.sign(TxPayloadHash(network, e.TransactionEnvelope))
	if err != nil {
		return err
	}

	e.Signatures = append(e.Signatures, stx.DecoratedSignature{
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
	return PrivateKey{ ed25519Priv(sk) }
}

// Generates a new Stellar keypair and returns the PrivateKey.
// Currently the only valid value for pkt is
// stx.PUBLIC_KEY_TYPE_ED25519.
func KeyGen(pkt stx.PublicKeyType) PrivateKey {
	switch pkt {
	case stx.PUBLIC_KEY_TYPE_ED25519:
		return genEd25519()
	default:
		panic(fmt.Sprintf("KeyGen: unsupported PublicKeyType %v", pkt))
	}
}

type InputLine = stx.InputLine

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
		var line InputLine
		fmt.Fscanln(PassphraseFile, &line)
		return []byte(line)
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
		if f != nil { f.Close() }
		if tmp != "" { os.Remove(tmp) }
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
	os.Link(filename, filename + "~")
	if err = os.Rename(tmp, filename); err == nil {
		tmp = ""
	}
	return err
}

var InvalidPassphrase = errors.New("Invalid passphrase")
var InvalidKeyFile = errors.New("Invalid private key file")

// Reads a private key from a file, prompting for a passphrase if the
// key is in ASCII-armored symmetrically-encrypted GPG format.
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

// Reads a private key from PassphraseFile (default os.Stdin).  If
// PassphraseFile is a terminal, then prints prompt and disables echo.
func PrivateKeyFromInput(prompt string) (*PrivateKey, error) {
	key := GetPass(prompt)
	var sk PrivateKey
	if _, err := fmt.Fscan(bytes.NewBuffer(key), &sk); err != nil {
		return nil, err
	}
	return &sk, nil
}
