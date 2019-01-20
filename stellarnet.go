
package stc

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"stc/stx"
)

type StellarNet struct {
	// Short name for network (used only in error messages).
	Name string

	// Network password used for hashing and signing transactions.
	NetworkId string

	// Base URL of horizon (including trailing slash).
	Horizon string

	// Set of signers to recognize when checking signatures on
	// transactions and annotations to show when printing signers.
	Signers SignerCache

	// Annotations to show on particular accounts when rendering them
	// in human-readable txrep format.
	Accounts AccountHints
}

// Default parameters for the Stellar main net (including the address
// of a Horizon instance hosted by SDF).
var StellarMainNet = StellarNet{
	Name: "main",
	NetworkId: "Public Global Stellar Network ; September 2015",
	Horizon: "https://horizon.stellar.org/",
}

// Default parameters for the Stellar test network (including the
// address of a Horizon instance hosted by SDF).
var StellarTestNet = StellarNet{
	Name: "test",
	NetworkId: "Test SDF Network ; September 2015",
	Horizon: "https://horizon-testnet.stellar.org/",
}

// An annotated SignerKey that can be used to authenticate
// transactions.  Prints and Scans as a StrKey-format SignerKey, a
// space, and then the comment.
type SignerKeyInfo struct {
	Key stx.SignerKey
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
	if t, err := ss.Token(true, func (r rune) bool {
		return !strings.ContainsRune("\r\n", r)
	}); err != nil {
		return err
	} else {
		ski.Comment = string(t)
		return nil
	}
}

// A SignerCache maps 4-byte SignatureHint values to annotated
// SignerKeys.
type SignerCache map[stx.SignatureHint][]SignerKeyInfo

// Renders SignerCache as a a set of SignerKeyInfo structures, one per
// line.
func (c SignerCache) String() string {
	out := &strings.Builder{}
	for _, ski := range c {
		for i := range ski {
			fmt.Fprintf(out, "%s\n", ski[i])
		}
	}
	return out.String()
}

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

func (c SignerCache) Save(filename string) error {
	return SafeWriteFile(filename, c.String(), 0666)
}

func (c SignerCache) Lookup(net *StellarNet, e *TransactionEnvelope,
	ds *stx.DecoratedSignature) *SignerKeyInfo {
	skis := c[ds.Hint]
	for i := range skis {
		if VerifyTx(&skis[i].Key, net.NetworkId, e,  ds.Signature) {
			return &skis[i]
		}
	}
	return nil
}

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
				skis[i].Comment = comment
				return nil
			}
		}
		c[hint] = append(c[hint], SignerKeyInfo{Key: signer, Comment: comment})
	} else {
		c[hint] = []SignerKeyInfo{{Key: signer, Comment: comment}}
	}
	return nil
}

// Set of annotations to show as comments when showing Stellar
// AccountID values.
type AccountHints map[string]string

func (h AccountHints) String() string {
	out := &strings.Builder{}
	for k, v := range h {
		fmt.Fprintf(out, "%s %s\n", k, v)
	}
	return out.String()
}

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

func (h *AccountHints) Save(filename string) error {
	return SafeWriteFile(filename, h.String(), 0666)
}
