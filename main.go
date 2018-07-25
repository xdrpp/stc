package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

type acctInfo struct {
	field string
	name string
	signers []HorizonSigner
}
type xdrGetAccounts struct {
	accounts map[AccountID]*acctInfo
}

func (xp *xdrGetAccounts) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}
func (xp *xdrGetAccounts) Marshal(field string, i interface{}) {
	switch v := i.(type) {
	case *AccountID:
		if _, ok := xp.accounts[*v]; !ok {
			xp.accounts[*v] = &acctInfo{ field: field }
		}
	case XdrAggregate:
		v.XdrMarshal(xp, field)
	}
}

func getAccounts(net *StellarNet, e *TransactionEnvelope, usenet bool) {
	xga := xdrGetAccounts{ map[AccountID]*acctInfo{} }
	e.XdrMarshal(&xga, "")
	c := make(chan struct{})
	for ac, infp := range xga.accounts {
		go func(ac AccountID, infp *acctInfo) {
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
	sk := KeyGen(PUBLIC_KEY_TYPE_ED25519)
	if outfile == "" {
		fmt.Println(sk)
		fmt.Println(sk.Public())
		fmt.Printf("%x\n", sk.Public().Hint())
	} else {
		if FileExists(outfile) {
			fmt.Fprintf(os.Stderr, "%s: file already exists\n", outfile)
			return
		}
		fmt.Print("Passphrase: ")
		bytePassword, err := terminal.ReadPassword(0)
		fmt.Println("")
		if FileExists(outfile) {
			fmt.Fprintf(os.Stderr, "%s: file already exists\n", outfile)
			return
		}
		if err == nil { err = sk.Save(outfile, bytePassword) }
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Println(sk.Public())
			fmt.Printf("%x\n", sk.Public().Hint())
		}
	}
}

func getSecKey(file string) *PrivateKey {
	if file == "" {
		fmt.Print("Secret Key: ")
	} else if FileExists(file) {
		fmt.Printf("Passphrase for %s: ", file)
	} else {
		fmt.Fprintf(os.Stderr, "%s: no such file", file)
		return nil
	}
	bytePassword, err := terminal.ReadPassword(0)
	fmt.Println("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}

	if file == "" {
		var sk PrivateKey
		if n, err := fmt.Sscan(string(bytePassword), &sk); n != 1 {
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
		return &sk
	} else if sk, err := PrivateKeyFromFile(file, bytePassword); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	} else {
		return sk
	}
}

func doSec2pub(file string) {
	sk := getSecKey(file)
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

	seqchan := make(chan SequenceNumber)
	go func() {
		var val SequenceNumber
		var zero AccountID
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
	} else {
		input, err = ioutil.ReadFile(infile)
	}
	if err != nil {
		return
	}
	sinput := string(input)

	var e TransactionEnvelope
	if isCompiled(sinput) {
		err = txIn(&e, sinput)
	} else {
		help, err = txScan(&e, sinput)
	}
	if err == nil {
		txe = &e
	}
	return
}

func writeTx(outfile string, e *TransactionEnvelope, net *StellarNet,
	help XdrHelp) error {
	var output string
	if help == nil {
		output = txOut(e) + "\n"
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

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

var progname string

func main() {
	opt_compile := flag.Bool("c", false, "Compile output to base64 XDR")
	opt_keygen := flag.Bool("keygen", false, "Create a new signing keypair")
	opt_sec2pub := flag.Bool("sec2pub", false, "Get public key from private")
	opt_output := flag.String("o", "", "Output to file instead of stdout")
	opt_preauth := flag.Bool("preauth", false,
		"Hash transaction for pre-auth use")
	opt_inplace := flag.Bool("i", false, "Edit the input file in place")
	opt_sign := flag.Bool("sign", false, "Sign the transaction")
	opt_signwith := flag.String("key", "", "File containing signing key")
	opt_netname := flag.String("net", "default",
		`Network ID ("main" or "test")`)
	opt_update := flag.Bool("u", false,
		"Query network to update fee and sequence number")
	opt_learn := flag.Bool("l", false, "Learn new signers")
	opt_help := flag.Bool("help", false, "Print usage information")
	opt_post := flag.Bool("post", false,
		"Post transaction instead of editing it")
	opt_edit := flag.Bool("edit", false,
		"keep editing the file until it doesn't change")
	if pos := strings.LastIndexByte(os.Args[0], '/'); pos >= 0 {
		progname = os.Args[0][pos+1:]
	} else {
		progname = os.Args[0]
	}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			`Usage: %[1]s [-sign] [-net=ID] [-c|-v] [-l] [-u] [-i | -o FILE] INPUT-FILE
       %[1]s -preauth [-net=ID] INPUT-FILE
       %[1]s -post [-sign] [-net=ID] [-u] INPUT-FILE
       %[1]s -edit FILE
       %[1]s -keygen
       %[1]s -sec2pub
`, progname)
		flag.PrintDefaults()
	}
	flag.Parse()
	if (*opt_help) {
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		return
	}

	if *opt_signwith != "" && !*opt_sec2pub {
		*opt_sign = true
	}

	if len(flag.Args()) == 0 {
		if *opt_sign || *opt_compile || *opt_preauth ||
			*opt_post || *opt_learn || *opt_update || *opt_inplace ||
			b2i(*opt_sec2pub) + b2i(*opt_keygen) != 1 {
			flag.Usage()
			os.Exit(1)
		}
		if (*opt_keygen) {
			doKeyGen(*opt_output)
		} else if (*opt_sec2pub) {
			doSec2pub(*opt_signwith)
		}
		return
	}

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	infile := flag.Args()[0]

	if *opt_keygen || *opt_sec2pub ||
		(*opt_inplace && (*opt_preauth || *opt_output != "")) ||
		((*opt_inplace || *opt_edit) && infile == "-") ||
		(b2i(*opt_preauth) + b2i(*opt_post) + b2i(*opt_compile) +
		b2i(*opt_edit) > 1) ||
		*opt_edit && (*opt_sign || *opt_inplace || infile == "-") {
		flag.Usage()
		os.Exit(1)
	}

	if *opt_edit {
/*
		f, err := ioutil.TempFile("", progname)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		path := f.Name()
		defer os.Remove(path)
		*opt_output = path
*/
		*opt_inplace = true
	}
	if *opt_inplace {
		*opt_output = infile
	}

	net := GetStellarNet(*opt_netname)
	if net == nil {
		fmt.Fprintf(os.Stderr, "unknown network %q\n", *opt_netname)
		os.Exit(1)
	}

edit_loop:
	e, help, err := readTx(infile)

	var pause bool
	if err != nil {
		if *opt_edit && os.IsNotExist(err) {
			e = &TransactionEnvelope{}
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
			if (*opt_edit) {
				pause = true
			} else {
				os.Exit(1)
			}
		}
	}

	if *opt_update {
		fixTx(net, e)
	}

	getAccounts(net, e, *opt_learn)

	if *opt_sign {
		sk := getSecKey(*opt_signwith)
		if sk == nil {
			os.Exit(1)
		}
		net.Signers.Add(sk.Public().String(), "")
		if sk == nil {
			os.Exit(1)
		}
		fmt.Println(sk.Public())
		if err = sk.SignTx(net.NetworkId, e); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if *opt_learn {
		net.SaveSigners()
	}

	if (*opt_post) {
		res := PostTransaction(net, e)
		if res != nil {
			fmt.Print(XdrToString(res))
		}
		if res == nil || res.Result.Code != TxSUCCESS {
			fmt.Fprint(os.Stderr, "Post transaction failed\n")
			os.Exit(1)
		}
		return
	}

	if (*opt_preauth) {
		sk := SignerKey{ Type: SIGNER_KEY_TYPE_PRE_AUTH_TX }
		copy(sk.PreAuthTx()[:], TxPayloadHash(net.NetworkId, e))
		fmt.Printf("%x\n", *sk.PreAuthTx())
		fmt.Println(&sk)
		return
	}


	if *opt_compile {
		help = nil
	} else if help == nil {
		help = make(XdrHelp)
	}
	if err = writeTx(*opt_output, e, net, help); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if *opt_edit {
		fi, err := os.Stat(*opt_output)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		ed, ok := os.LookupEnv("EDITOR")
		if !ok {
			ed = "vi"
		}
		if path, err := exec.LookPath(ed); err == nil {
			ed = path
		}

		if pause {
			fmt.Printf("Press return to run editor.")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		}

		proc, err := os.StartProcess(ed,
			[]string{ed, *opt_output}, &os.ProcAttr{
				Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
			})
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		proc.Wait()

		fi2, err := os.Stat(*opt_output)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if fi.Size() != fi2.Size() || fi.ModTime() != fi2.ModTime() {
			goto edit_loop
		}
	}
}
