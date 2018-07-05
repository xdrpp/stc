package main

import "os"

func main() {
	var t Transaction
	t.TimeBounds = &TimeBounds{MinTime: 12345}
	t.Memo.Type = MEMO_TEXT
	t.Memo.Text().SetString("Enjoy this transaction")
	t.Operations = append(t.Operations, Operation{})
	t.Operations[0].Body.Type = CREATE_ACCOUNT
	XdrPrint(os.Stdout, "", &t)
}
