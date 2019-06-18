package stc

import (
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

var defaultDefaultConfigContents = []byte(
`# This file specifies the default configuration for the stc library
# and command-line tool.  You can delete this file and it will be
# re-created and reset to defaults.  The contents of the file comes
# from /etc/stc.conf if that file exists, otherwise from
# ../share/stc.conf relative to the executable if that exists,
# otherwise from a simple default hard-coded into the library.

[net "main"]
network-id = "Public Global Stellar Network ; September 2015"
horizon = https://horizon.stellar.org/
native-asset = XLM

[net "test"]
horizon = https://horizon-testnet.stellar.org/
native-asset = TestXLM
`)

var DefaultConfigContents []byte

func getDefaultConfigContents() []byte {
	if DefaultConfigContents != nil {
		return DefaultConfigContents
	}
	confs := []string{ "/etc/stc.conf" }
	if exe, err := os.Executable(); err == nil {
		confs = append(confs,
			path.Join(path.Dir(path.Dir(exe)), "share", "stc.conf"))
	}
	for _, conf := range confs {
		if contents, err := ioutil.ReadFile(conf); err == nil {
			DefaultConfigContents = contents
			break
		}
	}
	if DefaultConfigContents == nil {
		DefaultConfigContents = defaultDefaultConfigContents
	}
	return DefaultConfigContents
}

const defaultConf = "default.conf"
const keyDir = "keys"

var STCdir string

func GetConfigDir() string {
	if STCdir != "" {
		return STCdir
	} else if d, ok := os.LookupEnv("STCDIR"); ok {
		STCdir = d
	} else if d, ok = os.LookupEnv("XDG_CONFIG_HOME"); ok {
		STCdir = filepath.Join(d, "stc")
	} else if d, ok = os.LookupEnv("HOME"); ok {
		STCdir = filepath.Join(d, ".config", "stc")
	} else {
		STCdir = ".stc"
	}
	if len(STCdir) > 0 && STCdir[0] != '/' {
		if d, err := filepath.Abs(STCdir); err == nil {
			STCdir = d
		}
	}
	defaultIni := path.Join(STCdir, defaultConf)
	if _, err := os.Stat(defaultIni); os.IsNotExist(err) {
		os.MkdirAll(STCdir, 0777)
		stcdetail.SafeWriteFile(defaultIni,
			string(getDefaultConfigContents()),
			0666)
	}
	return STCdir
}

func DefaultConfigFile() string {
	return path.Join(GetConfigDir(), "stc.conf")
}

type StellarNetParser struct {
	*StellarNet
	ItemCB func(stcdetail.IniItem)error
}

func (snp *StellarNetParser) Item(ii stcdetail.IniItem) error {
	if snp.ItemCB != nil {
		return snp.ItemCB(ii)
	}
	return nil
}

func (snp *StellarNetParser) doNet(ii stcdetail.IniItem) error {
	switch ii.Key {
	case "horizon":
		snp.Horizon = ii.Val()
	case "native-asset":
		snp.NativeAsset = ii.Val()
	case "network-id":
		snp.NetworkId = ii.Val()
	}
	return nil
}

func (snp *StellarNetParser) doAccounts(ii stcdetail.IniItem) error {
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

func (snp *StellarNetParser) doSigners(ii stcdetail.IniItem) error {
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

func (snp *StellarNetParser) Section(iss stcdetail.IniSecStart) error {
	snp.ItemCB = nil
	switch iss.Section {
	case "net":
		if iss.Subsection != nil {
			if *iss.Subsection == snp.Name {
			}
			return nil
		}
	case "accounts":
		if iss.Subsection != nil {
			if *iss.Subsection == snp.Name {
			}
			return nil
		}
	case "signers":
		if iss.Subsection == nil {
			return nil
		}
	}
	// return fmt.Errorf("unrecognized section %s", iss.IniSection.String())
	return nil
}

func LoatStellarNet(name, configPath string) *StellarNet {
	if configPath == "" {
		configPath = DefaultConfigFile()
	}
	ret := StellarNet{
		Name: name,
		SavePath: configPath,
	}
	snp := StellarNetParser{
		StellarNet: &ret,
	}
	if err := stcdetail.IniParse(&snp,
		path.Join(GetConfigDir(), defaultConf)); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := stcdetail.IniParse(&snp, configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return &ret
}
