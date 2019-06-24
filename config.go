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
				snp.ItemCB = snp.doNet
			}
			return nil
		}
	case "accounts":
		if iss.Subsection != nil {
			if *iss.Subsection == snp.Name {
				snp.ItemCB = snp.doAccounts
			}
			return nil
		}
	case "signers":
		if iss.Subsection == nil {
			snp.ItemCB = snp.doSigners
			return nil
		}
	}
	return nil
}

func LoadStellarNet(name, configPath string) *StellarNet {
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

	if contents, fi, err := stcdetail.ReadFile(configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		ret.Status = fi
		err = stcdetail.IniParseContents(&snp, configPath, contents)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

	}
	return &ret
}

func (net *StellarNet) Save() error {
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

	// XXX below isn't great because it blows away comments on keys
	// that haven't changed.

	sec := stcdetail.IniSection{
		Section: "net",
		Subsection: &net.Name,
	}
	ie.Set(&sec, "horizon", net.Horizon)
	ie.Set(&sec, "native-asset", net.NativeAsset)
	ie.Set(&sec, "network-id", net.GetNetworkId())

	sec.Section = "accounts"
	for k, v := range net.Accounts {
		ie.Set(&sec, k, v)
	}

	sec.Section = "signers"
	sec.Subsection = nil
	for _, ski := range net.Signers {
		for i := range ski {
			ie.Set(&sec, ski[i].Key.String(), ski[i].Comment)
		}
	}

	ie.WriteTo(lf)
	return lf.Commit()
}
