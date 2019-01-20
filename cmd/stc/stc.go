// Please see the stc.1 man page for complete documentation of this
// command.
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	. "stc"
	"stc/stx"
)

type acctInfo struct {
	field string
	name string
	signers []HorizonSigner
}
type xdrGetAccounts struct {
	accounts map[stx.AccountID]*acctInfo
}

func (xp *xdrGetAccounts) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (xp *xdrGetAccounts) Marshal(field string, i stx.XdrType) {
	switch v := i.(type) {
	case *stx.AccountID:
		if _, ok := xp.accounts[*v]; !ok {
			xp.accounts[*v] = &acctInfo{ field: field }
		}
	case stx.XdrAggregate:
		v.XdrMarshal(xp, field)
	}
}

func getAccounts(net *StellarNet, e *TransactionEnvelope, usenet bool) {
	xga := xdrGetAccounts{ map[stx.AccountID]*acctInfo{} }
	e.XdrMarshal(&xga, "")
	c := make(chan struct{})
	for ac, infp := range xga.accounts {
		go func(ac stx.AccountID, infp *acctInfo) {
			var ae *HorizonAccountEntry
			if usenet {
				ae = GetAccountEntry(net, ac.String())
			}
			if ae != nil {
				infp.signers = ae.Signers
			} else {
				infp.signers = []HorizonSigner{{Key: ac.String()}}
			}
			c <- struct{}{}
		}(ac, infp)
	}
	for i := 0; i < len(xga.accounts); i++ {
		<-c
	}

	for ac, infp := range xga.accounts {
		acs := ac.String()
		if acs == "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF" {
			continue
		}
		for _, signer := range infp.signers {
			var comment string
			if acs != signer.Key {
				comment = fmt.Sprintf("signer for account %s", acs)
			}
			net.Signers.Add(signer.Key, comment)
		}
	}
}

func doKeyGen(outfile string) {
	sk := KeyGen(stx.PUBLIC_KEY_TYPE_ED25519)
	if outfile == "" {
		fmt.Println(sk)
		fmt.Println(sk.Public())
		// fmt.Printf("%x\n", sk.Public().Hint())
	} else {
		if FileExists(outfile) {
			fmt.Fprintf(os.Stderr, "%s: file already exists\n", outfile)
			return
		}
		bytePassword := GetPass2("Passphrase: ")
		if FileExists(outfile) {
			fmt.Fprintf(os.Stderr, "%s: file already exists\n", outfile)
			return
		}
		err := sk.Save(outfile, bytePassword)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Println(sk.Public())
			//fmt.Printf("%x\n", sk.Public().Hint())
		}
	}
}

func getSecKey(file string) (*PrivateKey, error) {
	var sk *PrivateKey
	var err error
	if file == "" {
		sk, err = PrivateKeyFromInput("Secret key: ")
	} else {
		sk, err = PrivateKeyFromFile(file)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return sk, err
}

func doSec2pub(file string) {
	sk, _ := getSecKey(file)
	if sk != nil {
		fmt.Println(sk.Public().String())
	}
}

func fixTx(net *StellarNet, e *TransactionEnvelope) {
	feechan := make(chan uint32)
	go func() {
		if h := GetLedgerHeader(net); h != nil {
			feechan <- h.BaseFee
		} else {
			feechan <- 0
		}
	}()

	seqchan := make(chan stx.SequenceNumber)
	go func() {
		var val stx.SequenceNumber
		var zero stx.AccountID
		if e.Tx.SourceAccount != zero {
			if a := GetAccountEntry(net, e.Tx.SourceAccount.String());
			a != nil {
				if fmt.Sscan(a.Sequence.String(), &val); val != 0 {
					val++
				}
			}
		}
		seqchan <- val
	}()

	if newfee := uint32(len(e.Tx.Operations)) * <-feechan; newfee > e.Tx.Fee {
		e.Tx.Fee = newfee
	}
	if newseq := <-seqchan; newseq > e.Tx.SeqNum {
		e.Tx.SeqNum = newseq
	}
}

// Guess whether input is key: value lines or compiled base64
func isCompiled(content string) bool {
	if len(content) != 0 && strings.IndexByte(content, ':') == -1 {
		bs, err := base64.StdEncoding.DecodeString(content);
		if err == nil && len(bs) > 0 {
			return true
		}
	}
	return false
}

func readTx(infile string) (txe *TransactionEnvelope, help XdrHelp, err error) {
	var input []byte
	if infile == "-" {
		input, err = ioutil.ReadAll(os.Stdin)
		infile = "(stdin)"
	} else {
		input, err = ioutil.ReadFile(infile)
	}
	if err != nil {
		return
	}
	sinput := string(input)

	if isCompiled(sinput) {
		txe, err = TxFromBase64(sinput)
		return
	}

	var e TransactionEnvelope
	if help, err = TxScan(&e, sinput, infile); err == nil {
		txe = &e
	}
	return
}

func mustReadTx(infile string) (*TransactionEnvelope, XdrHelp) {
	e, help, err := readTx(infile)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	if help == nil {
		help = make(XdrHelp)
	}
	return e, help
}

func writeTx(outfile string, e *TransactionEnvelope, net *StellarNet,
	help XdrHelp) error {
	var output string
	if help == nil {
		output = TxToBase64(e) + "\n"
	} else {
		buf := &strings.Builder{}
		TxStringCtx{ Out: buf, Env: e, Net: net, Help: help }.Exec()
		output = buf.String()
	}

	if outfile == "" {
		fmt.Print(output)
	} else {
		if err := SafeWriteFile(outfile, output, 0666); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return err
		}
	}
	return nil
}

func mustWriteTx(outfile string, e *TransactionEnvelope, net *StellarNet,
	help XdrHelp) {
	if err := writeTx(outfile, e, net, help); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func signTx(net *StellarNet, key string, e *TransactionEnvelope) error {
	if key != "" { key = AdjustKeyName(key) }
	sk, err := getSecKey(key)
	if err != nil {
		return err
	}
	net.Signers.Add(sk.Public().String(), "")
	if err = sk.SignTx(net.NetworkId, e); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func editor(args ...string) {
	ed, ok := os.LookupEnv("EDITOR")
	if !ok {
		ed = "vi"
	}
	if path, err := exec.LookPath(ed); err == nil {
		ed = path
	}

	argv := append([]string{ed}, args...)
	proc, err := os.StartProcess(ed, argv, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	proc.Wait()
}

func firstDifferentLine(a []byte, b []byte) (lineno int) {
	n := len(a)
	if n > len(b) {
		n = len(b)
	}
	lineno = 1
	for i := 0; i < n && a[i] == b[i]; i++ {
		if a[i] == '\n' {
			lineno++
		}
	}
	return
}

func doEdit(net *StellarNet, arg string) {
	if arg == "" || arg == "-" {
		fmt.Fprintln(os.Stderr, "Must supply file name to edit")
		os.Exit(1)
	}

	e, help, err := readTx(arg)
	compiled := help == nil
	if os.IsNotExist(err) {
		e = &TransactionEnvelope{}
	} else if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	getAccounts(net, e, false)
	if help == nil {
		help = make(XdrHelp)
	}

	f, err := ioutil.TempFile("", progname)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path + "~")
	defer os.Remove(path)

	var contents, lastcontents []byte
	for {
		if err == nil {
			buf := &bytes.Buffer{}
			TxStringCtx{ Out: buf, Env: e, Net: net, Help: help }.Exec()
			lastcontents = buf.Bytes()
			ioutil.WriteFile(path, lastcontents, 0600)
		}

		fi1, staterr := os.Stat(path)
		if staterr != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			fmt.Printf("Press return to run editor.")
			var li InputLine
			fmt.Scanln(&li)
		}
		editor(fmt.Sprintf("+%d", firstDifferentLine(contents, lastcontents)),
			path)

		if err == nil {
			fi2, staterr := os.Stat(path)
			if staterr != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			if fi1.Size() == fi2.Size() && fi1.ModTime() == fi2.ModTime() {
				break
			}
		}

		contents, err = ioutil.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		e = &TransactionEnvelope{}
		help, err = TxScan(e, string(contents), path)
	}

	if compiled {
		help = nil
	}
	mustWriteTx(arg, e, net, help)
}

func b2i(bs ...bool) int {
	ret := 0
	for _, b := range bs {
		if b {
			ret++
		}
	}
	return ret
}

var progname string

func main() {
	opt_compile := flag.Bool("c", false, "Compile output to base64 XDR")
	opt_keygen := flag.Bool("keygen", false, "Create a new signing keypair")
	opt_sec2pub := flag.Bool("sec2pub", false, "Get public key from private")
	opt_output := flag.String("o", "", "Output to file instead of stdout")
	opt_preauth := flag.Bool("preauth", false,
		"Hash transaction to strkey for use as a pre-auth transaction signer")
	opt_txhash := flag.Bool("txhash", false, "Hash transaction to hex format")
	opt_inplace := flag.Bool("i", false, "Edit the input file in place")
	opt_sign := flag.Bool("sign", false, "Sign the transaction")
	opt_key := flag.String("key", "", "File containing signing key")
	opt_netname := flag.String("net", "",
		`Network ID (e.g., "test"); default: $STCNET, otherwise "main"`)
	opt_update := flag.Bool("u", false,
		"Query network to update fee and sequence number")
	opt_learn := flag.Bool("l", false, "Learn new signers")
	opt_help := flag.Bool("help", false, "Print usage information")
	opt_post := flag.Bool("post", false,
		"Post transaction instead of editing it")
	opt_nopass := flag.Bool("nopass", false, "Never prompt for passwords")
	opt_edit := flag.Bool("edit", false,
		"keep editing the file until it doesn't change")
	opt_import_key := flag.Bool("import-key", false,
		"Import signing key to your $STCDIR directory")
	opt_export_key := flag.Bool("export-key", false,
		"Export signing key from your $STCDIR directory")
	opt_list_keys := flag.Bool("list-keys", false,
		"List keys that have been stored in $STCDIR")
	if pos := strings.LastIndexByte(os.Args[0], '/'); pos >= 0 {
		progname = os.Args[0][pos+1:]
	} else {
		progname = os.Args[0]
	}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
`Usage: %[1]s [-net=ID] [-sign] [-c] [-l] [-u] [-i | -o FILE] INPUT-FILE
       %[1]s -edit [-net=ID] FILE
       %[1]s -post [-net=ID] INPUT-FILE
       %[1]s -preauth [-net=ID] INPUT-FILE
       %[1]s -keygen [NAME]
       %[1]s -sec2pub [NAME]
       %[1]s -import-key NAME
       %[1]s -export-key NAME
       %[1]s -list-keys
`, progname)
		flag.PrintDefaults()
	}
	flag.Parse()
	if (*opt_help) {
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		return
	}

	if n := b2i(*opt_preauth, *opt_txhash, *opt_post, *opt_edit, *opt_keygen,
		*opt_sec2pub, *opt_import_key, *opt_export_key, *opt_list_keys);
	n > 1 || len(flag.Args()) > 1 ||
		(len(flag.Args()) == 0 &&
		!(*opt_keygen || *opt_sec2pub || *opt_list_keys)) {
		flag.Usage()
		os.Exit(2)
	} else if n == 1 {
		bail := false
		if *opt_sign || *opt_key != "" {
			fmt.Fprintln(os.Stderr,
				"--sign and --key only availble in default mode")
			bail = true
		}
		if *opt_learn || *opt_update {
			fmt.Fprintln(os.Stderr, "-l and -u only availble in default mode")
			bail = true
		}
		if *opt_inplace || *opt_output != "" {
			fmt.Fprintln(os.Stderr, "-i and -o only availble in default mode")
			bail = true
		}
		if *opt_compile {
			fmt.Fprintln(os.Stderr, "-c o only availble in default mode")
			bail = true
		}
		if bail {
			os.Exit(2)
		}
	}

	var arg string
	if len(flag.Args()) == 1 {
		arg = flag.Args()[0]
	}

	if *opt_nopass {
		PassphraseFile = io.MultiReader()
	} else if arg == "-" {
		PassphraseFile = nil
	}

	switch {
	case *opt_keygen:
		if arg != "" { arg = AdjustKeyName(arg) }
		doKeyGen(arg)
		return
	case *opt_sec2pub:
		if arg != "" { arg = AdjustKeyName(arg) }
		doSec2pub(arg)
		return
	case *opt_import_key:
		arg = AdjustKeyName(arg)
		sk, err := PrivateKeyFromInput("Secret key: ")
		if err == nil { err = sk.Save(arg, GetPass2("Passphrase: ")) }
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case *opt_export_key:
		arg = AdjustKeyName(arg)
		sk, err := PrivateKeyFromFile(arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Println(sk)
		return
	case *opt_list_keys:
		for _, k := range GetKeyNames() {
			fmt.Println(k)
		}
		return
	}

	if *opt_netname == "" { *opt_netname = os.Getenv("STCNET") }
	if *opt_netname == "" { *opt_netname = "default" }
	net := GetStellarNet(*opt_netname)
	if net == nil {
		fmt.Fprintf(os.Stderr, "unknown network %q\n", *opt_netname)
		os.Exit(1)
	}

	if *opt_edit {
		doEdit(net, arg)
		return
	}

	e, help := mustReadTx(arg)
	switch {
	case *opt_post:
		res := PostTransaction(net, e)
		if res != nil {
			fmt.Print(stx.XdrToString(res))
		}
		if res == nil || res.Result.Code != stx.TxSUCCESS {
			fmt.Fprint(os.Stderr, "Post transaction failed\n")
			os.Exit(1)
		}
	case *opt_preauth, *opt_txhash:
		sk := stx.SignerKey{ Type: stx.SIGNER_KEY_TYPE_PRE_AUTH_TX }
		copy(sk.PreAuthTx()[:], TxPayloadHash(net.NetworkId, e))
		if *opt_txhash { fmt.Printf("%x\n", *sk.PreAuthTx()) }
		if *opt_preauth { fmt.Println(&sk) }
	default:
		getAccounts(net, e, *opt_learn)
		if *opt_update { fixTx(net, e) }
		if *opt_sign || *opt_key != "" {
			if err := signTx(net, *opt_key, e); err != nil {
				os.Exit(1)
			}
		}
		if *opt_learn { SaveSigners(net) }
		if *opt_compile { help = nil }
		if *opt_inplace { *opt_output = arg }
		mustWriteTx(*opt_output, e, net, help)
	}
}
