package stcdetail_test

import (
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
)

type IniDumper struct {}
func (IniDumper) Consume(item stcdetail.IniItem) error {
	if item.IniSection != nil {
		fmt.Printf("%s.", item.Section)
		if item.Subsection != nil {
			fmt.Printf("%s.", *item.Subsection)
		}
	}
	fmt.Printf("%s = %s\n", item.Key, item.Value)
	return nil
}

var contents = []byte(`
bare-key = bar value
[section]
key1 = value1
[another-section "with-subsection"]
key2 = value2
`)

func ExampleIniParseContents() {
	stcdetail.IniParseContents(IniDumper{}, "(test)", contents)
	// Output:
	// bare-key = bar value
	// section.key1 = value1
	// another-section.with-subsection.key2 = value2
}
