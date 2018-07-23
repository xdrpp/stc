
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type StellarNet struct {
	Name string
	NetworkId string
	Horizon string
	Signers SignerCache
	Accounts AccountHints
}

var defaultNets = []StellarNet{
	{Name: "main",
		NetworkId: "Public Global Stellar Network ; September 2015",
		Horizon: "https://horizon.stellar.org/"},
	{Name: "test",
		NetworkId: "Test SDF Network ; September 2015",
		Horizon: "https://horizon-testnet.stellar.org/"},
}

var ConfigRoot string

func init() {
	if d, ok := os.LookupEnv("STCDIR"); ok {
		ConfigRoot = d
	} else if d, ok = os.LookupEnv("XDG_CONFIG_HOME"); ok {
		ConfigRoot = filepath.Join(d, "stc")
	} else if d, ok = os.LookupEnv("HOME"); ok {
		ConfigRoot = filepath.Join(d, ".config", "stc")
	} else {
		ConfigRoot = ".stc"
	}
}

func (net *StellarNet) ConfigPath(names ...string) string {
	args := make([]string, 2, 2 + len(names))
	args[0] = ConfigRoot
	args[1] = net.Name
	args = append(args, names...)
	return filepath.Join(args...)
}

func SafeWriteFile(filename string, data string, perm os.FileMode) error {
	tmp := fmt.Sprintf("%s#%d#", filename, os.Getpid())
	f, err := os.Create(tmp)
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
	err = os.Rename(tmp, filename)
	tmp = ""
	return err
}

func EnsureDir(filename string) error {
	return os.MkdirAll(filepath.Dir(filename), 0777)
}

type SignerKeyInfo struct {
	Key SignerKey
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

type SignerCache map[SignatureHint][]SignerKeyInfo

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
		} else if _, ok := e.(StrKeyError); ok {
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
	EnsureDir(filename)
	return SafeWriteFile(filename, c.String(), 0666)
}

func (c SignerCache) Lookup(
	net *StellarNet, e *TransactionEnvelope, n int) *SignerKeyInfo {
	skis := c[e.Signatures[n].Hint]
	for i := range skis {
		if skis[i].Key.VerifyTx(net.NetworkId, e,  e.Signatures[n].Signature) {
			return &skis[i]
		}
	}
	return nil
}

func (c SignerCache) Add(strkey, comment string) error {
	var signer SignerKey
	_, err := fmt.Sscan(strkey, &signer)
	if err != nil {
		return err
	}
	hint := signer.Hint()
	skis, ok := c[hint]
	if ok {
		for _, k := range skis {
			if strkey == k.Key.String() {
				return nil
			}
		}
		c[hint] = append(c[hint], SignerKeyInfo{Key: signer, Comment: comment})
	} else {
		c[hint] = []SignerKeyInfo{{Key: signer, Comment: comment}}
	}
	return nil
}

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
		var ac AccountID
		if _, err := fmt.Sscan(v[0], &ac); err != nil {
			fmt.Fprintf(os.Stderr, "%s:%d: %s\n", filename, lineno, err.Error())
			continue
		}
		(*h)[ac.String()] = strings.Trim(v[1], " ")
	}
	return nil
}

func (h *AccountHints) Save(filename string) error {
	EnsureDir(filename)
	return SafeWriteFile(filename, h.String(), 0666)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		panic(err)
	}
}

func PrintErr() {
	i := recover()
	if err, ok := i.(error); ok {
		fmt.Fprintln(os.Stderr, err.Error())
	} else if i != nil {
		panic(i)
	}
}

func CreateIfMissing(path string, contents string) {
	defer PrintErr()
	if !FileExists(path) {
		SafeWriteFile(path, contents, 0666)
	}
}

func netInit() {
	for _, net := range defaultNets {
		os.MkdirAll(net.ConfigPath(), 0777)
		CreateIfMissing(net.ConfigPath("network_id"), net.NetworkId + "\n")
		CreateIfMissing(net.ConfigPath("horizon"), net.Horizon + "\n")
	}
	os.Symlink(defaultNets[0].Name, filepath.Join(ConfigRoot, "default"))
}

func head(path string) string {
	input, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ""
	}
	if pos := bytes.IndexByte(input, '\n'); pos >= 0 {
		input = input[:pos]
	}
	return string(input)
}

func GetStellarNet(name string) *StellarNet {
	netInit()
	net := &StellarNet{ Name: name }
	net.NetworkId = head(net.ConfigPath("network_id"))
	net.Horizon = head(net.ConfigPath("horizon"))
	net.Signers.Load(net.ConfigPath("signers"))
	net.Accounts.Load(net.ConfigPath("accounts"))
	return net
}

func (net *StellarNet) SaveSigners() error {
	return net.Signers.Save(net.ConfigPath("signers"))
}
