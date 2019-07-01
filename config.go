package stc

import (
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const configFileName = "stc.conf"

// When a user does not have an stc.conf configuration file, the
// library searches for one in $STCDIR/stc.conf, then /etc/stc.conf,
// then ../share/stc.conf (relative to the executable path).  If none
// of those paths exists, then it uses the built-in contents specified
// by this variable.
var DefaultGlobalConfigContents = []byte(
`# Default Stellar network configurations for stc.

[net "main"]
network-id = "Public Global Stellar Network ; September 2015"
horizon = https://horizon.stellar.org/
native-asset = XLM

[net "test"]
horizon = https://horizon-testnet.stellar.org/
native-asset = TestXLM

`)

var globalConfigContents []byte

func getGlobalConfigContents() []byte {
	if globalConfigContents != nil {
		return globalConfigContents
	}
	confs := []string{ filepath.FromSlash("/etc/" + configFileName) }
	if exe, err := os.Executable(); err == nil {
		confs = append(confs,
			path.Join(path.Dir(path.Dir(exe)), "share", configFileName))
	}
	for _, conf := range confs {
		if contents, err := ioutil.ReadFile(conf); err == nil {
			globalConfigContents = contents
			break
		}
	}
	if globalConfigContents == nil {
		globalConfigContents = DefaultGlobalConfigContents
	}
	return globalConfigContents
}

var stcDir string

func getConfigDir() string {
	if stcDir != "" {
		return stcDir
	} else if d, ok := os.LookupEnv("STCDIR"); ok {
		stcDir = d
	} else if d, ok = os.LookupEnv("XDG_CONFIG_HOME"); ok {
		stcDir = filepath.Join(d, "stc")
	} else if d, ok = os.LookupEnv("HOME"); ok {
		stcDir = filepath.Join(d, ".config", "stc")
	} else {
		stcDir = ".stc"
	}
	if len(stcDir) > 0 && stcDir[0] != '/' {
		if d, err := filepath.Abs(stcDir); err == nil {
			stcDir = d
		}
	}
	if _, err := os.Stat(stcDir); os.IsNotExist(err) &&
		os.MkdirAll(stcDir, 0777) == nil {
		if _, err = LoadStellarNet("main",
			path.Join(stcDir, "main.net")); err == nil {
				os.Symlink("main.net", path.Join(stcDir, "default.net"))
			}
	}
	return stcDir
}

// Return the path to a file under the user's configuration directory.
// The configuration directory is found based on environment
// variables.  From highest to lowest precedence tries $STCDIR,
// $XDG_CONFIG_HOME/.stc, $HOME/.config/stc, or ./.stc, using the
// first one with for which the environment variable exists.  If the
// configuration directory doesn't exist, it gets created, but the
// underlying path requested will not be created.
func ConfigPath(components...string) string {
	return path.Join(append([]string{getConfigDir()}, components...)...)
}

func ValidNetName(name string) bool {
	return len(name) > 0 && name[0] != '.' &&
		stcdetail.ValidIniSubsection(name) &&
		strings.IndexByte(name, '/') == -1
}

type stellarNetParser struct {
	*StellarNet
	itemCB func(stcdetail.IniItem)error
	setName bool
}

func (snp *stellarNetParser) Init() {
	if snp.Signers == nil {
		snp.Signers = make(SignerCache)
	}
	if snp.Accounts == nil {
		snp.Accounts = make(AccountHints)
	}
}

func (snp *stellarNetParser) Item(ii stcdetail.IniItem) error {
	if snp.itemCB != nil {
		return snp.itemCB(ii)
	}
	return nil
}

func (snp *stellarNetParser) doNet(ii stcdetail.IniItem) error {
	var target *string
	switch ii.Key {
	case "name":
		if (snp.Name == "" || snp.setName) &&
			ii.Subsection == nil && ValidNetName(ii.Val()) {
			snp.Name = ii.Val()
			snp.setName = false
		}
	case "horizon":
		target = &snp.Horizon
	case "native-asset":
		target = &snp.NativeAsset
	case "network-id":
		target = &snp.NetworkId
	}
	if target != nil {
		if ii.Value == nil {
			*target = ""
		} else if *target == "" {
			*target = ii.Val()
		}
	}
	return nil
}

func (snp *stellarNetParser) doAccounts(ii stcdetail.IniItem) error {
	var acct AccountID
	if _, err := fmt.Sscan(ii.Key, &acct); err != nil {
		return stcdetail.BadKey(err.Error())
	}
	if ii.Value == nil {
		delete(snp.Accounts, ii.Key)
	} else if _, ok := snp.Accounts[ii.Key]; !ok {
		snp.Accounts[ii.Key] = *ii.Value
	}
	return nil
}

func (snp *stellarNetParser) doSigners(ii stcdetail.IniItem) error {
	var signer SignerKey
	if _, err := fmt.Sscan(ii.Key, &signer); err != nil {
		return stcdetail.BadKey(err.Error())
	}
	if ii.Value == nil {
		snp.Signers.Del(ii.Key)
	} else {
		snp.Signers.Add(ii.Key, *ii.Value)
	}
	return nil
}

func (snp *stellarNetParser) Section(iss stcdetail.IniSecStart) error {
	snp.itemCB = nil
	if iss.Subsection == nil ||
		(*iss.Subsection == snp.Name && ValidNetName(snp.Name)) {
		switch iss.Section {
		case "net":
			snp.itemCB = snp.doNet
		case "accounts":
			snp.itemCB = snp.doAccounts
		case "signers":
			snp.itemCB = snp.doSigners
		}
	}
	return nil
}

// Load a Stellar network from an INI files.  If path[0] does not
// exist but name is valid, the path will be created and net.name will
// be set to name.  Otherwise the name argument is ignored.  After all
// files in paths are parsed, the global stc.conf file will be parsed.
// After that, there must be a valid NetworkId or the function will
// return nil.
func LoadStellarNet(name string, paths...string) (*StellarNet, error) {
	ret := StellarNet{
		Name: name,
	}
	if len(paths) > 0 {
		ret.SavePath = paths[0]
	}
	snp := stellarNetParser{
		StellarNet: &ret,
		setName: true,
	}

	for _, path := range paths {
		if err := stcdetail.IniParse(&snp, path); err != nil &&
			!os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			return nil, err
		} else if !ValidNetName(ret.Name) {
			return nil, fmt.Errorf("%s: invalid or missing net.name", path)
		} else if snp.setName {
			ret.Edits.Set("net", "name", ret.Name)
			snp.setName = false
		}
	}

	// Finish with global configuration
	stcdetail.IniParseContents(&snp, "", getGlobalConfigContents())
	if ret.GetNetworkId() == "" {
		return nil, fmt.Errorf("could not determine network-id for %s",
			ret.Name)
	} else if ret.SavePath != "" {
		if err := ret.Save(); err != nil {
			return nil, err
		}
	}
	return &ret, nil
}

var netCache map[string]*StellarNet

// Load a network from under the ConfigPath() ($STCDIR) directory.  If
// name is "", then it will look at the $STCNET environment variable
// and if that is unset load a default network.  Returns nil if the
// network name does not exist.  After loading the netname.net file,
// also parses $STCDIR/global.conf.
//
// Two pre-defined names are "main" and "test", with "main" being the
// default.  Other networks can be created under ConfigPath(), or can
// be pre-specified (and created on demand) in stc.conf.
func DefaultStellarNet(name string) *StellarNet {
	if !ValidNetName(name) {
		name = os.Getenv("STCNET")
		if !ValidNetName(name) {
			name = "default"
		}
	}
	if netCache == nil {
		netCache = make(map[string]*StellarNet)
	} else if net, ok := netCache[name]; ok {
		return net
	}
	ret, err := LoadStellarNet(name, ConfigPath(name + ".net"),
		ConfigPath("global.conf"))
	if ret == nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		netCache[name] = ret
	}
	return ret
}

func (net *StellarNet) Save() error {
	if len(net.Edits) == 0 {
		return nil
	}
	if net.SavePath == "" {
		return os.ErrInvalid
	}
	lf, err := stcdetail.LockFileIfUnchanged(net.SavePath, net.Status)
	if err != nil {
		return err
	}
	defer lf.Abort()

	contents, err := lf.ReadFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	ie, _ := stcdetail.NewIniEdit(net.SavePath, contents)
	net.Edits.Apply(ie)
	ie.WriteTo(lf)
	return lf.Commit()
}
