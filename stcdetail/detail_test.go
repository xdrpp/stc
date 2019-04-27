package stcdetail

import "fmt"
// import "strings"
// import "testing"

func ExampleScaleFmt() {
	fmt.Println(ScalePrint(987654321, 7))
	// Output:
	// 98.7654321e7
}
