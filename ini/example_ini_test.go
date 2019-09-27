package ini_test

import (
	"fmt"
	"github.com/xdrpp/stc/ini"
)

type IniDumper struct{}

func (IniDumper) Item(item ini.IniItem) error {
	if item.Value == nil {
		fmt.Printf("%s\n", item.QKey())
	} else {
		fmt.Printf("%s = %s\n", item.QKey(), *item.Value)
	}
	return nil
}

var contents = []byte(`
# discouraged (like git-config, you can't edit keys outside of sections)
bare-key = bare value
[section]
key1 = value1
[other "sub"]
key2 = value2
key3 # this one has no value
key4 = " value4"   ; this one started with a space
`)

func ExampleIniParseContents() {
	ini.IniParseContents(IniDumper{}, "(test)", contents)
	// Output:
	// bare-key = bare value
	// section.key1 = value1
	// other.sub.key2 = value2
	// other.sub.key3
	// other.sub.key4 =  value4
}
