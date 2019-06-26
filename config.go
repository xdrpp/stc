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
`# This file specifies the default network configurations for the stc
# library and command-line tool.

[global]
default-net = main

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
	defaultIni := path.Join(stcDir, configFileName)
	if _, err := os.Stat(defaultIni); os.IsNotExist(err) {
		os.MkdirAll(stcDir, 0777)
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

type GlobalConfig struct {
	DefaultNet string
	Nets map[string]*StellarNet
}

var globalConfig *GlobalConfig
func GetGlobalConfig() *GlobalConfig {
	if globalConfig == nil {
		globalConfig = &GlobalConfig{}
		stcdetail.IniParseContents(globalConfig, "", getGlobalConfigContents())
	}
	return globalConfig
}

func ValidNetName(name string) bool {
	return len(name) > 0 && name[0] != '.' &&
		stcdetail.ValidIniSubsection(name) &&
		strings.IndexByte(name, '/') == -1
}

func (gc *GlobalConfig) Init() {
	if !ValidNetName(gc.DefaultNet) {
		gc.DefaultNet = "default"
	}
	if gc.Nets == nil {
		gc.Nets = make(map[string]*StellarNet)
	}
}

func loadNetItem(net *StellarNet, ii stcdetail.IniItem, nameOK bool) {
	switch ii.Key {
	case "name":
		if nameOK && ValidNetName(ii.Val()) {
			net.Name = ii.Val()
		}
	case "horizon":
		net.Horizon = ii.Val()
	case "native-asset":
		net.NativeAsset = ii.Val()
	case "network-id":
		net.NetworkId = ii.Val()
	}
}

func (gc *GlobalConfig) Item(ii stcdetail.IniItem) error {
	if ii.IniSection == nil {
		return nil
	} else if ii.IniSection.Section == "global" &&
		ii.IniSection.Subsection == nil {
		switch ii.Key {
		case "default-net":
			if name := ii.Val(); ValidNetName(name) {
				gc.DefaultNet = name
			}
		}
	} else if ii.IniSection.Section == "net" &&
		ii.IniSection.Subsection != nil &&
		ValidNetName(*ii.IniSection.Subsection) {
		name := *ii.IniSection.Subsection
		net, ok := gc.Nets[name]
		if !ok {
			net = &StellarNet{
				Name: name,
			}
			gc.Nets[name] = net
		}
		loadNetItem(net, ii, false)
	}
	return nil
}

type stellarNetParser struct {
	*StellarNet
	ItemCB func(stcdetail.IniItem)error
	NameOK bool
}

func (snp *stellarNetParser) Init() {
	snp.ItemCB = func(stcdetail.IniItem) error { return nil }
}

func (snp *stellarNetParser) Item(ii stcdetail.IniItem) error {
	return snp.ItemCB(ii)
}

func (snp *stellarNetParser) doNet(ii stcdetail.IniItem) error {
	loadNetItem(snp.StellarNet, ii, snp.NameOK)
	return nil
}

func (snp *stellarNetParser) doAccounts(ii stcdetail.IniItem) error {
	var acct AccountID
	if _, err := fmt.Sscan(ii.Key, &acct); err != nil {
		return stcdetail.BadKey(err.Error())
	}
	if ii.Value == nil {
		delete(snp.Accounts, ii.Key)
	} else {
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
	snp.ItemCB = nil
	if iss.Subsection == nil {
		switch iss.Section {
		case "net":
			snp.ItemCB = snp.doNet
		case "accounts":
			snp.ItemCB = snp.doAccounts
		case "signers":
			snp.ItemCB = snp.doSigners
		}
	}
	return nil
}

// Load a Stellar network from an INI file in path.  If the network
// does not exist, it will be named name and default parameters based
// on name will be looked up in the stc.conf configuration file.
func LoadStellarNet(path, name string) *StellarNet {
	ret := StellarNet{
		SavePath: path,
		Signers: make(SignerCache),
		Accounts: make(AccountHints),
	}
	snp := stellarNetParser{
		StellarNet: &ret,
		NameOK: true,
	}
	if err := stcdetail.IniParse(&snp, path); err != nil &&
		!os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
	}
	if ret.Name == "" && name != "" {
		ret.Name = name
		ret.Edits.Set("net", "name", ret.Name)
	}
	if proto, ok := GetGlobalConfig().Nets[ret.Name]; ok {
		if ret.NetworkId == "" && proto.NetworkId != "" {
			ret.NetworkId = proto.NetworkId
			ret.Edits.Set("net", "network-id", ret.NetworkId)
		}
		if ret.NativeAsset == "" && proto.NativeAsset != "" {
			ret.NativeAsset = proto.NativeAsset
			ret.Edits.Set("net", "native-asset", ret.NativeAsset)
		}
		if ret.Horizon == "" && proto.Horizon != "" {
			ret.Horizon = proto.Horizon
			ret.Edits.Set("net", "horizon", ret.Horizon)
		}
	}
	if ret.NetworkId == "" && ret.GetNetworkId() != "" {
		ret.Edits.Set("net", "network-id", ret.NetworkId)
	}
	if ret.NetworkId == "" {
		return nil
	}
	ret.Save()
	return &ret
}

// Load a network from under the ConfigPath() directory.  If name is
// "", then it will look at the $STCNET environment variable and if
// that is unset load a default network.  Returns nil if the network
// name does not exist.
//
// Two pre-defined names are "main" and "test", with "main" being the
// default.  Other networks can be created under ConfigPath(), or can
// be pre-specified (and created on demand) in stc.conf.
func DefaultStellarNet(name string) *StellarNet {
	if !ValidNetName(name) {
		name = os.Getenv("STCNET")
		if !ValidNetName(name) {
			name = GetGlobalConfig().DefaultNet
		}
	}
	return LoadStellarNet(ConfigPath(name) + ".net", name)
}

func (net *StellarNet) Save() error {
	if len(net.Edits) == 0 {
		return nil
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
