package ini_test

import (
	"fmt"
	"github.com/xdrpp/stc/ini"
)

func ExampleIniEdit() {
	bini := []byte(
`; Here's a comment
[sec1]
	key1 = val1
	key2 = val2
; Here's another comment
[sec2]
	key3 = val3
`)
	ie, _ := ini.NewIniEdit("", bini)
	sec1 := ini.IniSection{Section: "sec1"}
	sec2 := ini.IniSection{Section: "sec2"}
	sec3 := ini.IniSection{Section: "sec3"}
	ie.Add(&sec1, "key4", "val4")
	ie.Del(&sec1, "key2")
	ie.Set(&sec1, "key1", "second version of val1")
	ie.Add(&sec1, "key2", "second version of key2")
	ie.Set(&sec1, "key5", "val5")
	ie.Set(&sec2, "key6", "val6")
	ie.Set(&sec2, "key3", "second version of key3")
	ie.Set(&sec3, "key7", "val7")

	fmt.Print(ie.String())
	// Output:
	// ; Here's a comment
	// [sec1]
	// 	key1 = second version of val1
	// 	key4 = val4
	// 	key2 = second version of key2
	// 	key5 = val5
	// ; Here's another comment
	// [sec2]
	// 	key3 = second version of key3
	// 	key6 = val6
	// [sec3]
	//	key7 = val7
}
