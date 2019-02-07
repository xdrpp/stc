package main

import (
	"bytes"
	"fmt"
	. "github.com/xdrpp/stc"
	"github.com/xdrpp/stc/detail"
	"io/ioutil"
	"os"
	"path/filepath"
)

var defaultNets = []StellarNet{
	StellarMainNet,
	StellarTestNet,
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

func ConfigPath(net *StellarNet, names ...string) string {
	args := make([]string, 3, 3+len(names))
	args[0] = ConfigRoot
	args[1] = "networks"
	args[2] = net.Name
	args = append(args, names...)
	return filepath.Join(args...)
}

func EnsureDir(filename string) error {
	return os.MkdirAll(filepath.Dir(filename), 0777)
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

func printErr() bool {
	i := recover()
	if err, ok := i.(error); ok {
		fmt.Fprintln(os.Stderr, err.Error())
		return true
	} else if i != nil {
		panic(i)
	}
	return false
}

func CreateIfMissing(path string, contents string) {
	defer printErr()
	if !FileExists(path) {
		detail.SafeWriteFile(path, contents, 0666)
	}
}

func netInit() {
	for i := range defaultNets {
		os.MkdirAll(ConfigPath(&defaultNets[i]), 0777)
		if path := ConfigPath(&defaultNets[i], "network_id");
		!FileExists(path) {
			if id := defaultNets[i].GetNetworkId(); id != "" {
				CreateIfMissing(path, id+"\n")
			}
		}
	}
	os.Symlink(defaultNets[0].Name,
		filepath.Join(ConfigRoot, "networks", "default"))
}

func head(path string) (string, error) {
	input, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	if pos := bytes.IndexByte(input, '\n'); pos >= 0 {
		input = input[:pos]
	}
	return string(input), nil
}

func GetStellarNet(name string) *StellarNet {
	netInit()
	net := &StellarNet{Name: name}
	var err error
	net.NetworkId, err = head(ConfigPath(net, "network_id"))
	if err != nil {
		return nil
	}
	net.Horizon, _ = head(ConfigPath(net, "horizon"))
	net.Signers.Load(ConfigPath(net, "signers"))
	net.Accounts.Load(ConfigPath(net, "accounts"))
	return net
}

func AdjustKeyName(key string) string {
	if key == "" {
		fmt.Fprintln(os.Stderr, "missing private key name")
		os.Exit(1)
	}
	if dir, _ := filepath.Split(key); dir != "" {
		return key
	}
	keydir := filepath.Join(ConfigRoot, "keys")
	os.MkdirAll(keydir, 0700)
	return filepath.Join(keydir, key)
}

func GetKeyNames() []string {
	d, err := os.Open(filepath.Join(ConfigRoot, "keys"))
	if err != nil {
		return nil
	}
	names, _ := d.Readdirnames(-1)
	return names
}

func SaveSigners(net *StellarNet) error {
	target := ConfigPath(net, "signers")
	EnsureDir(target)
	return net.Signers.Save(target)
}
