package ini_test

import (
	"fmt"
	"strings"
	"github.com/xdrpp/stc/ini"
)

type Foo struct {
	A_field int
	AnotherField string `ini:"the-field"`
	StillAnotherField bool
	StrVec []string
	IntVec []int
}

func trimSpace(i interface{}) string {
	s := fmt.Sprint(i)
	return strings.ReplaceAll(s, " \n", "\n")
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
	IntVec	 # This erases previous entries
	IntVec = 100
`)

	foo := Foo{}
	gs := ini.NewGenericSink("foo")
	gs.AddStruct(&foo)
	fmt.Println("=== before:")
	fmt.Print(trimSpace(gs))
	fmt.Println("=== after:")
	err := ini.IniParseContents(gs, "(test)", contents)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Print(trimSpace(gs))
	// Unordered output:
	// [foo]
	// === before:
	// 	A-field = 0
	// 	the-field =
	// 	StillAnotherField = false
	// === after:
	// [foo]
	// 	A-field = 44
	// 	the-field = hello world
	// 	StillAnotherField = true
	//	StrVec = a string
	//	StrVec = another string
	//	IntVec = 100
	// 
}
