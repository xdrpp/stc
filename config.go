package stc

import (
	"errors"
	"fmt"
	"github.com/xdrpp/stc/ini"
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

[net "standalone"]
network-id = "Standalone Network ; February 2017"
horizon = http://localhost:8000/

`)

var globalConfigContents []byte

func getGlobalConfigContents() []byte {
	if globalConfigContents != nil {
		return globalConfigContents
	}
	confs := []string{
		path.Join(getConfigDir(false), configFileName),
		filepath.FromSlash("/etc/" + configFileName),
	}
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

func getConfigDir(create bool) string {
	if stcDir != "" {
		return stcDir
	} else if d, ok := os.LookupEnv("STCDIR"); ok {
		stcDir = d
	} else if d, err := os.UserConfigDir(); err == nil {
		stcDir = filepath.Join(d, "stc")
	} else {
		stcDir = ".stc"
	}
	if len(stcDir) > 0 && stcDir[0] != '/' {
		if d, err := filepath.Abs(stcDir); err == nil {
			stcDir = d
		}
	}
	if _, err := os.Stat(stcDir); os.IsNotExist(err) && create &&
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
// UserConfigDir() (i.e., on Unix $XDG_CONFIG_HOME/.stc or
// $HOME/.config/stc), or ./.stc, using the first one with for which
// the environment variable exists.  If the configuration directory
// doesn't exist, it gets created, but the underlying path requested
// will not be created.
func ConfigPath(components...string) string {
	return path.Join(append([]string{getConfigDir(true)}, components...)...)
}

// Parse a series of INI configuration files specified by paths,
// followed by the global or built-in stc.conf file.
func ParseConfigFiles(sink ini.IniSink, paths...string) error {
	for _, path := range paths {
		contents, _, err := stcdetail.ReadFile(path)
		if err == nil {
			err = ini.IniParseContents(sink, path, contents)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Finish with global configuration
	err := ini.IniParseContents(sink, "", getGlobalConfigContents())
	if err != nil {
		return err
	}
	return nil
}

func ValidNetName(name string) bool {
	return len(name) > 0 && name[0] != '.' &&
		ini.ValidIniSubsection(name) &&
		strings.IndexByte(name, '/') == -1
}

type stellarNetParser struct {
	*StellarNet

	// How to handle items in the current section
	itemCB func(ini.IniItem)error

	// This is intended to be initialized to true, and then gets set
	// to false whenever Name gets set on StellarNet.  The reason is
	// that initially Name may be set from a source other than the
	// configuration file, such as a command-line argument or the
	// STCNET environment variable.  If the configuration file does
	// not set Name, then setName will never be set to false, which
	// tells us we need to save it to the configuration file.
	// (setName means set it in the configuration file.)
	setName bool
}

func (snp *stellarNetParser) Item(ii ini.IniItem) error {
	if snp.itemCB != nil {
		return snp.itemCB(ii)
	}
	return nil
}

func (snp *stellarNetParser) doNet(ii ini.IniItem) error {
	var target *string
	switch ii.Key {
	case "name":
		if (snp.Name == "" || snp.setName) &&
			ii.Val() != "" && ii.Subsection == nil {
			if !ValidNetName(ii.Val()) {
				return ErrInvalidNetName
			}
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

func (snp *stellarNetParser) doAccounts(ii ini.IniItem) error {
	var acct MuxedAccount
	if _, err := fmt.Sscan(ii.Key, &acct); err != nil {
		return ini.BadKey(err.Error())
	}
	if ii.Value == nil {
		delete(snp.Accounts, ii.Key)
	} else if _, ok := snp.Accounts[ii.Key]; !ok {
		snp.Accounts[ii.Key] = *ii.Value
	}
	return nil
}

func (snp *stellarNetParser) doSigners(ii ini.IniItem) error {
	var signer SignerKey
	if _, err := fmt.Sscan(ii.Key, &signer); err != nil {
		return ini.BadKey(err.Error())
	}
	if ii.Value == nil {
		snp.Signers.Del(ii.Key)
	} else {
		snp.Signers.Add(ii.Key, *ii.Value)
	}
	return nil
}

func (snp *stellarNetParser) Section(iss ini.IniSecStart) error {
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

func (snp *stellarNetParser) Done(ini.IniRange) {
	if snp.setName {
		snp.Edits.Set("net", "name", snp.Name)
		snp.setName = false
	}
}

var ErrNoNetworkId = errors.New("Cannot obtain Stellar network-id")
var ErrInvalidNetName = errors.New("Invalid or missing Stellar network name")

func (net *StellarNet) Validate() error {
	if !ValidNetName(net.Name) {
		return ErrInvalidNetName
	}
	if net.GetNetworkId()  == "" {
		return ErrNoNetworkId
	}
	return nil
}

func (net *StellarNet) IniSink() ini.IniSink {
	if net.Signers == nil {
		net.Signers = make(SignerCache)
	}
	if net.Accounts == nil {
		net.Accounts = make(AccountHints)
	}
	return &stellarNetParser{
		StellarNet: net,
		setName: true,
	}
}

// Load a Stellar network from an INI files.  If path[0] does not
// exist but name is valid, the path will be created and net.name will
// be set to name.  Otherwise the name argument is ignored.  After all
// files in paths are parsed, the global stc.conf file will be parsed.
// After that, there must be a valid NetworkId or the function will
// return nil.
func LoadStellarNet(name string, paths...string) (*StellarNet, error) {
	ret := StellarNet{ Name: name }
	if len(paths) > 0 {
		ret.SavePath = paths[0]
	}
	if err := ParseConfigFiles(ret.IniSink(), paths...); err != nil {
		return nil, err
	} else if err = ret.Validate(); err != nil {
		return nil, err
	}
	ret.Save()
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

// Save any changes to SavePath.  If SavePath does not exist, then
// create it with permissions Perm (subject to umask, of course).
func (net *StellarNet) SavePerm(perm os.FileMode) error {
	if len(net.Edits) == 0 {
		return nil
	}
	if net.SavePath == "" {
		return os.ErrInvalid
	}
	var lf stcdetail.LockedFile
	var err error
	lf, err = stcdetail.LockFile(net.SavePath, perm)
	if err != nil {
		return err
	}
	defer lf.Abort()

	contents, err := lf.ReadFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	ie, _ := ini.NewIniEdit(net.SavePath, contents)
	net.Edits.Apply(ie)
	ie.WriteTo(lf)
	return lf.Commit()
}

// Save any changes to to SavePath.  Equivalent to SavePerm(0666).
func (net *StellarNet) Save() error {
	return net.SavePerm(0666)
}
