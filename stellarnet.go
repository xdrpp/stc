package stc

import (
	"bufio"
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
	"os"
	"strings"
)

type StellarNet struct {
	// Short name for network (used only in error messages).
	Name string

	// Network password used for hashing and signing transactions.
	NetworkId string

	// Name to use for native asset
	NativeAsset string

	// Base URL of horizon (including trailing slash).
	Horizon string

	// Set of signers to recognize when checking signatures on
	// transactions and annotations to show when printing signers.
	Signers SignerCache

	// Annotations to show on particular accounts when rendering them
	// in human-readable txrep format.
	Accounts AccountHints

	// Changes will be saved to this file.
	SavePath string
}

// Default parameters for the Stellar main net (including the address
// of a Horizon instance hosted by SDF).
var StellarMainNet = StellarNet{
	Name:        "main",
	NetworkId:   "Public Global Stellar Network ; September 2015",
	NativeAsset: "XLM",
	Horizon:     "https://horizon.stellar.org/",
}

// Default parameters for the Stellar test network (including the
// address of a Horizon instance hosted by SDF).
var StellarTestNet = StellarNet{
	Name:        "test",
	NetworkId:   "",
	NativeAsset: "TestXLM",
	Horizon:     "https://horizon-testnet.stellar.org/",
}

func (net *StellarNet) GetNativeAsset() string {
	return net.NativeAsset
}

// Returns true only if sig is a valid signature on e for public key
// pk.
func (net *StellarNet) VerifySig(
	pk *SignerKey, tx stx.IsTransaction, sig Signature) bool {
	return stcdetail.VerifyTx(pk, net.GetNetworkId(), tx.ToTransaction(), sig)
}

// Return a transaction hash (which in Stellar is defined as the hash
// of the constant ENVELOPE_TYPE_TX, the NetworkID, and the marshaled
// XDR of the Transaction).
func (net *StellarNet) HashTx(tx stx.IsTransaction) *stx.Hash {
	return stcdetail.TxPayloadHash(net.GetNetworkId(), tx.ToTransaction())
}

// Sign a transaction and append the signature to the
// TransactionEnvelope.
func (net *StellarNet) SignTx(sk *PrivateKey, e *TransactionEnvelope) error {
	sig, err := sk.Sign(net.HashTx(e)[:])
	if err != nil {
		return err
	}
	e.Signatures = append(e.Signatures, stx.DecoratedSignature{
		Hint:      sk.Public().Hint(),
		Signature: sig,
	})
	return nil
}

// An annotated SignerKey that can be used to authenticate
// transactions.  Prints and Scans as a StrKey-format SignerKey, a
// space, and then the comment.
type SignerKeyInfo struct {
	Key     stx.SignerKey
	Comment string
}

func (ski SignerKeyInfo) String() string {
	if ski.Comment != "" {
		return fmt.Sprintf("%s %s", ski.Key, ski.Comment)
	}
	return ski.Key.String()
}

func (ski *SignerKeyInfo) Scan(ss fmt.ScanState, c rune) error {
	if err := ski.Key.Scan(ss, c); err != nil {
		return err
	}
	if t, err := ss.Token(true, func(r rune) bool {
		return !strings.ContainsRune("\r\n", r)
	}); err != nil {
		return err
	} else {
		ski.Comment = string(t)
		return nil
	}
}

// A SignerCache contains a set of possible Stellar signers.  Because
// a TransactionEnvelope contains an array of signatures without
// public keys, it is not possible to verify the signatures without
// having the Signers.  The signatures in a TransactionEnvelope
// envelope are, however, accompanied by a 4-byte SignatureHint,
// making it efficient to look up signers if they are in a SignerCache.
type SignerCache map[stx.SignatureHint][]SignerKeyInfo

// Renders SignerCache as a a set of SignerKeyInfo structures, one per
// line, suitable for saving to a file.
func (c SignerCache) String() string {
	out := &strings.Builder{}
	for _, ski := range c {
		for i := range ski {
			fmt.Fprintf(out, "%s\n", ski[i])
		}
	}
	return out.String()
}

// Reads a SignerCache from a file, discarding the previous contents
// of the cache.
func (c *SignerCache) Load(filename string) error {
	*c = make(SignerCache)
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for lineno := 1; scanner.Scan(); lineno++ {
		var ski SignerKeyInfo
		_, e := fmt.Sscanf(scanner.Text(), "%v", &ski)
		if e == nil {
			c.Add(ski.Key.String(), ski.Comment)
		} else if _, ok := e.(stx.StrKeyError); ok {
			fmt.Fprintf(os.Stderr, "%s:%d: invalid signer key\n",
				filename, lineno)
		} else {
			fmt.Fprintf(os.Stderr, "%s:%d: %s\n", filename, lineno, e.Error())
			err = e
		}
	}
	return err
}

// Saves a SignerCache to a file.
func (c SignerCache) Save(filename string) error {
	return stcdetail.SafeWriteFile(filename, c.String(), 0666)
}

// Finds the signer in a SignerCache that corresponds to a particular
// signature on a transaction.
func (c SignerCache) Lookup(networkID string, e *stx.TransactionEnvelope,
	ds *stx.DecoratedSignature) *SignerKeyInfo {
	skis := c[ds.Hint]
	for i := range skis {
		if stcdetail.VerifyTx(&skis[i].Key, networkID, &e.Tx, ds.Signature) {
			return &skis[i]
		}
	}
	return nil
}

// Adds a signer to a SignerCache if the signer is not already in the
// cache.  If the signer is already in the cache, the comment is left
// unchanged.
func (c SignerCache) Add(strkey, comment string) error {
	var signer stx.SignerKey
	_, err := fmt.Sscan(strkey, &signer)
	if err != nil {
		return err
	}
	hint := signer.Hint()
	skis, ok := c[hint]
	if ok {
		for i := range skis {
			if strkey == skis[i].Key.String() {
				return nil
			}
		}
		c[hint] = append(c[hint], SignerKeyInfo{Key: signer, Comment: comment})
	} else {
		c[hint] = []SignerKeyInfo{{Key: signer, Comment: comment}}
	}
	return nil
}

// Adds a signer to a SignerCache if the signer is not already in the
// cache.  If the signer is already in the cache, the comment is left
// unchanged.
func (c SignerCache) Del(strkey string) error {
	var signer stx.SignerKey
	_, err := fmt.Sscan(strkey, &signer)
	if err != nil {
		return err
	}
	hint := signer.Hint()
	skis, ok := c[hint]
	if !ok {
		return nil
	}
	for i := 0; i < len(skis); i++ {
		if strkey == skis[i].Key.String() {
			if i == len(skis) - 1 {
				skis = skis[:i]
			} else {
				skis = append(skis[:i], skis[i+1:]...)
				i--
			}
		}
	}
	if len(skis) == 0 {
		delete(c, hint)
	} else {
		c[hint] = skis
	}
	return nil
}

// Set of annotations to show as comments when showing Stellar
// AccountID values.
type AccountHints map[string]string

// Renders an account hint as the AccountID in StrKey format, a space,
// and the comment (if any).
func (h AccountHints) String() string {
	out := &strings.Builder{}
	for k, v := range h {
		fmt.Fprintf(out, "%s %s\n", k, v)
	}
	return out.String()
}

// Loads a set of account hints from a file, discarding any current
// hints.
func (h *AccountHints) Load(filename string) error {
	*h = make(AccountHints)
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for lineno := 1; scanner.Scan(); lineno++ {
		v := strings.SplitN(scanner.Text(), " ", 2)
		if len(v) == 0 || len(v[0]) == 0 {
			continue
		}
		var ac stx.AccountID
		if _, err := fmt.Sscan(v[0], &ac); err != nil {
			fmt.Fprintf(os.Stderr, "%s:%d: %s\n", filename, lineno, err.Error())
			continue
		}
		(*h)[ac.String()] = strings.Trim(v[1], " ")
	}
	return nil
}

// Saves the set of account hints from a file.
func (h *AccountHints) Save(filename string) error {
	return stcdetail.SafeWriteFile(filename, h.String(), 0666)
}
