package stcdetail_test

import (
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
)

type IniDumper struct {}
func (IniDumper) Value(sec *stcdetail.IniSection, k string, v string) error {
	if sec != nil {
		fmt.Printf("%s.", sec.Section)
		if sec.Subsection != nil {
			fmt.Printf("%s.", *sec.Subsection)
		}
	}
	fmt.Printf("%s = %s\n", k, v)
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
