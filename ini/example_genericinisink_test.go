package ini_test

import (
	"fmt"
	"github.com/xdrpp/stc/ini"
)

type Foo struct {
	A_field int
	AnotherField string `ini:"the-field"`
	StillAnotherField bool
	StrVec []string
	IntVec []int
}

func ExampleGenericIniSink() {
	var contents = []byte(`
[foo]
	A-field = 44
	the-field
	the-field = hello world
	StillAnotherField = true
    StrVec = a string
    StrVec = another string
    IntVec = 101
    IntVec = 102
`)

	foo := Foo{}
	gs := &ini.GenericIniSink{ Sec: &ini.IniSection{ Section: "foo" } }
	gs.AddStruct(&foo)
	fmt.Println("=== before:")
	fmt.Print(gs)
	fmt.Println("=== after:")
	err := ini.IniParseContents(gs, "(test)", contents)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Print(gs)
	// [foo]
	// 	A-field = 0
	// 	the-field = 
	// 	StillAnotherField = false
	// === after:
	// [foo]
	// 	A-field = 44
	// 	the-field = hello world
	// 	StillAnotherField = true
	// 
}
