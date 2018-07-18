package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"os"
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

func getAccounts(net *StellarNet, e *TransactionEnvelope, sc *SignerCache,
	usenet bool) {
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
		for _, signer := range infp.signers {
			var comment string
			if acs != signer.Key {
				comment = fmt.Sprintf("signer for account %s", acs)
			}
			sc.Add(signer.Key, comment)
		}
	}
}

func checkSigs(net *StellarNet, sc *SignerCache, e *TransactionEnvelope) bool {
	return false
}

func doKeyGen() {
	sk := KeyGen(PUBLIC_KEY_TYPE_ED25519)
	fmt.Println(sk)
	fmt.Println(sk.Public())
	fmt.Printf("%x\n", sk.Public().Hint())
}

func getSecKey() *PrivateKey {
	fmt.Print("Secret Key: ")
	bytePassword, err := terminal.ReadPassword(0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	var sk PrivateKey
	if n, err := fmt.Sscan(string(bytePassword), &sk); n != 1 {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	return &sk
}

func doSec2pub() {
	sk := getSecKey()
	fmt.Println(sk.Public().String())
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

var progname string

func main() {
	opt_compile := flag.Bool("c", false, "Compile output to binary XDR")
	opt_decompile := flag.Bool("d", false, "Decompile input from binary XDR")
	opt_keygen := flag.Bool("keygen", false, "Create a new signing keypair")
	opt_sec2pub := flag.Bool("sec2pub", false, "Get public key from private")
	opt_output := flag.String("o", "", "Output to file instead of stdout")
	opt_preauth := flag.Bool("preauth", false,
		"Hash transaction for pre-auth use")
	opt_inplace := flag.Bool("i", false,
		"Edit the input file (required) in place")
	opt_sign := flag.Bool("sign", false, "Sign the transaction")
	opt_netname := flag.String("net", "main", `Network ID "main" or "test"`)
	opt_update := flag.Bool("u", false,
		"Query network to update fee and sequence number")
	opt_learn := flag.Bool("l", false, "Learn new signers from network")
	opt_post := flag.Bool("post", false,
		"Post transaction instead of editing it")
	opt_verbose := flag.Bool("v", false, "Annotate output more verbosely")
	if pos := strings.LastIndexByte(os.Args[0], '/'); pos >= 0 {
		progname = os.Args[0][pos+1:]
	} else {
		progname = os.Args[0]
	}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
`Usage: %[1]s [-sign] [-net=ID] [-c|-v] [-d] [-u] [-i | -o FILE] [INPUT-FILE]
       %[1]s [-preauth] [-net=ID] [-d] [INPUT-FILE]
       %[1]s [-keygen]
       %[1]s [-sec2pub]
       %[1]s [-post [-net=ID]] [-sign] [-d] [-u] [INPUT-FILE]
`, progname)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *opt_preauth && *opt_output != "" ||
		(*opt_keygen || *opt_sec2pub) &&
		(*opt_compile || *opt_decompile || *opt_preauth || *opt_inplace) ||
		(*opt_keygen && *opt_sec2pub) {
		flag.Usage()
		os.Exit(1)
	}
	if *opt_inplace {
		if *opt_output != "" || len(flag.Args()) == 0 {
			flag.Usage()
			os.Exit(1)
		}
		*opt_output = flag.Args()[0]
	}

	if (*opt_keygen) {
		doKeyGen()
		return
	}
	if (*opt_sec2pub) {
		doSec2pub()
		return
	}

	var input []byte
	var err error
	switch (len(flag.Args())) {
	case 0:
		input, err = ioutil.ReadAll(os.Stdin)
	case 1:
		input, err = ioutil.ReadFile(flag.Args()[0])
	default:
		flag.Usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	sinput := string(input)

	if !*opt_decompile && len(sinput) != 0 &&
		strings.IndexByte(sinput, ':') == -1 {
		if bs, err := base64.StdEncoding.DecodeString(sinput);
		err == nil && len(bs) > 0 {
			*opt_decompile = true
		}
	}

	var e TransactionEnvelope
	if *opt_decompile {
		err = txIn(&e, sinput)
	} else {
		err = txScan(&e, sinput)
	}
	if err != nil {
		os.Exit(1)
	}

	net, ok := Networks[*opt_netname]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown network %q\n", *opt_netname)
		os.Exit(1)
	}
	if *opt_update {
		fixTx(&net, &e)
	}

	var sc SignerCache
	sc.Load(ConfigPath("signers"))
	getAccounts(&net, &e, &sc, *opt_learn)
	sc.Save(ConfigPath("signers"))

	checkSigs(&net, &sc, &e)

	if *opt_sign {
		sk := getSecKey()
		if sk == nil {
			os.Exit(1)
		}
		fmt.Println(sk.Public())
		if err = sk.SignTx(net.NetworkId, &e); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if (*opt_post) {
		res := PostTransaction(&net, &e)
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
		copy(sk.PreAuthTx()[:], TxPayloadHash(net.NetworkId, &e))
		fmt.Printf("%x\n", *sk.PreAuthTx())
		fmt.Println(&sk)
		return
	}

	var output string
	if *opt_compile {
		output = txOut(&e) + "\n"
	} else {
		buf := &strings.Builder{}
		TxStringCtx{ Out: buf, Env: &e, Signers: sc, Net: &net,
			Verbose: *opt_verbose }.Exec()
		output = buf.String()
	}

	if *opt_output == "" {
		fmt.Print(output)
	} else {
		if err = SafeWriteFile(*opt_output, output, 0666); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
}
