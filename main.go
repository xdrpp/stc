package main

import "fmt"
import "io"
import "os"

type XP struct {
	out io.Writer
}

func (xp *XP) marshal(name string, val interface{}) {
	fmt.Fprintf(xp.out, "%s: %v\n", name, val)
}

func main() {
	var t Transaction
	t.TimeBounds = &TimeBounds{MinTime: 12345}
	t.Memo.Type = MEMO_TEXT
	*t.Memo.Text() = "Enjoy this transaction"
	t.Operations = append(t.Operations, Operation{})
	t.Operations[0].Body.Type = CREATE_ACCOUNT
	XDR_Transaction(&XP{os.Stdout}, "t", &t)
	// XdrPrint(os.Stdout, "", &t)
}
