// Please see the stc.1 man page for complete documentation of this
// command.  The man page is included in the release and available at
// https://xdrpp.github.io/stc/pkg/github.com/xdrpp/stc/cmd/stc/stc.1.html
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xdrpp/goxdr/xdr"
	. "github.com/xdrpp/stc"
	"github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
)

type format int

const (
	fmt_compiled = format(iota)
	fmt_txrep
	fmt_json
)

type isSignerKey interface {
	ToSignerKey() SignerKey
}

func getAccounts(net *StellarNet, e *TransactionEnvelope, usenet bool) {
	accounts := make(map[string][]HorizonSigner)
	record := func(ac isSignerKey) {
		k := ac.ToSignerKey()
		if !isZeroAccount(k) {
			accounts[k.String()] = []HorizonSigner{{Key: k}}
		}
	}
	record(e.SourceAccount())
	stcdetail.ForEachXdrType(e, func(ac isSignerKey) {
		record(ac)
	})

	if usenet {
		c := make(chan func())
		for ac := range accounts {
			go func(ac string) {
				if ae, err := net.GetAccountEntry(ac); err == nil {
					c <- func() { accounts[ac] = ae.Signers }
				} else {
					c <- func() {}
				}
			}(ac)
		}
		for i := len(accounts); i > 0; i-- {
			(<-c)()
		}
	}

	for ac, signers := range accounts {
		for _, signer := range signers {
			var comment string
			if ac != signer.Key.String() {
				comment = fmt.Sprintf("signer for account %s", ac)
			}
			net.AddSigner(signer.Key.String(), comment)
		}
	}
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		panic(err)
	}
}

func AdjustKeyName(key string) string {
	if key == "" {
		fmt.Fprintln(os.Stderr, "missing private key name")
		os.Exit(1)
	}
	if dir, _ := filepath.Split(key); dir != "" {
		return key
	}
	os.MkdirAll(ConfigPath("keys"), 0700)
	return ConfigPath("keys", key)
}

func GetKeyNames() []string {
	d, err := os.Open(ConfigPath("keys"))
	if err != nil {
		return nil
	}
	names, _ := d.Readdirnames(-1)
	return names
}

func doGenesisKey(outfile string, net *StellarNet) {
	hash := sha256.Sum256([]byte(net.NetworkId))
	sk := PrivateKey{
		stcdetail.Ed25519Priv(ed25519.NewKeyFromSeed(hash[:])),
	}
	storeKey(outfile, sk)
}

func doKeyGen(outfile string, genesis bool) {
	storeKey(outfile, NewPrivateKey(stx.PUBLIC_KEY_TYPE_ED25519))
}

func storeKey(outfile string, sk PrivateKey) {
	if outfile == "" {
		fmt.Println(sk)
		fmt.Println(sk.Public())
		// fmt.Printf("%x\n", sk.Public().Hint())
	} else {
		if FileExists(outfile) {
			fmt.Fprintf(os.Stderr, "%s: file already exists\n", outfile)
			return
		}
		bytePassword := stcdetail.GetPass2("Passphrase: ")
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

func getSecKey(file string) (PrivateKey, error) {
	var sk PrivateKey
	var err error
	if file == "" {
		sk, err = InputPrivateKey("Secret key: ")
	} else {
		sk, err = LoadPrivateKey(file)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return sk, err
}

func doSec2pub(file string) {
	sk, err := getSecKey(file)
	if err == nil {
		fmt.Println(sk.Public().String())
	}
}

var u256zero stx.Uint256

func isZeroAccount(ac isSignerKey) bool {
	k := ac.ToSignerKey()
	return k.Type == stx.SIGNER_KEY_TYPE_ED25519 &&
		bytes.Compare(k.Ed25519()[:], u256zero[:]) == 0
}

func fixTx(net *StellarNet, e *TransactionEnvelope) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h, err := net.GetFeeStats(); err == nil {
			// 20 should be a parameter
			e.SetFee(h.Percentile(20))
		}
	}()
	if !isZeroAccount(e.SourceAccount()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if a, _ := net.GetAccountEntry(
				e.SourceAccount().ToSignerKey().String()); a != nil {
				switch e.Type {
				case stx.ENVELOPE_TYPE_TX:
					e.V1().Tx.SeqNum = a.NextSeq()
				case stx.ENVELOPE_TYPE_TX_V0:
					e.V0().Tx.SeqNum = a.NextSeq()
				}
			}
		}()
	}
	wg.Wait()
}

// Guess whether input is key: value lines or compiled base64
func guessFormat(content string) format {
	if len(content) == 0 {
		return fmt_compiled
	}
	if strings.IndexAny(content, ":{") == -1 {
		bs, err := base64.StdEncoding.DecodeString(content)
		if err == nil && len(bs) > 0 {
			return fmt_compiled
		}
	}
	if content[0] == '{' {
		return fmt_json
	}
	return fmt_txrep
}

type ParseError struct {
	stcdetail.TxrepError
	Filename string
}

func (pe ParseError) Error() string {
	return pe.FileError(pe.Filename)
}

func readTx(infile string) (
	txe *TransactionEnvelope, f format, err error) {
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

	switch f = guessFormat(sinput); f {
	case fmt_txrep:
		if newe, pe := TxFromRep(sinput); pe != nil {
			err = ParseError{pe.(stcdetail.TxrepError), infile}
		} else {
			txe = newe
		}
	case fmt_compiled:
		txe, err = TxFromBase64(sinput)
	case fmt_json:
		e := NewTransactionEnvelope()
		if err = stcdetail.JsonToXdr(e, input); err == nil {
			txe = e
		}
	}
	return
}

func mustReadTx(infile string) (*TransactionEnvelope, format) {
	e, f, err := readTx(infile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return e, f
}

func writeTx(outfile string, e *TransactionEnvelope, net *StellarNet,
	f format) error {
	var output string
	switch f {
	case fmt_compiled:
		output = TxToBase64(e) + "\n"
	case fmt_txrep:
		output = net.TxToRep(e)
	case fmt_json:
		if boutput, err := stcdetail.XdrToJson(e); err != nil {
			panic(err)
		} else {
			output = string(boutput)
		}
	}

	if outfile == "" {
		fmt.Print(output)
	} else {
		if err := stcdetail.SafeWriteFile(outfile, output, 0666); err != nil {
			return err
		}
	}
	return nil
}

func mustWriteTx(outfile string, e *TransactionEnvelope, net *StellarNet,
	f format) {
	if err := writeTx(outfile, e, net, f); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func signTx(net *StellarNet, key string, e *TransactionEnvelope) error {
	if key != "" {
		key = AdjustKeyName(key)
	}
	sk, err := getSecKey(key)
	if err != nil {
		return err
	}
	net.AddSigner(sk.Public().String(), "")
	if err = net.SignTx(sk, e); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

var bad_payload_pk_type error =
	fmt.Errorf("invalid public key type for signed payload")

func signPayload(net *StellarNet, key string, e *TransactionEnvelope,
	hexpayload string) error {
	signer := SignerKey { Type: stx.SIGNER_KEY_TYPE_ED25519_SIGNED_PAYLOAD }
	if hexpayload != "" {
		if _, err := fmt.Sscanf(hexpayload, "%x",
			&signer.Ed25519SignedPayload().Payload); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	}
	if key != "" {
		key = AdjustKeyName(key)
	}
	sk, err := getSecKey(key)
	if err != nil {
		return err
	}
	pk := sk.Public()
	if pk.Type != stx.PUBLIC_KEY_TYPE_ED25519 {
		return bad_payload_pk_type
	}
	signer.Ed25519SignedPayload().Ed25519 = *pk.Ed25519()
	net.AddSigner(pk.String(), "")
	sig, err := sk.Sign(signer.Ed25519SignedPayload().Payload)
	if err != nil {
		return err
	}
	*e.Signatures() = append(*e.Signatures(), stx.DecoratedSignature{
		Hint: signer.Hint(),
		Signature: sig,
	})
	return nil
}

func editor(path string, line int) {
	ed, ok := os.LookupEnv("STCEDITOR")
	if !ok {
		ed, ok = os.LookupEnv("EDITOR")
	}
	if !ok {
		ed = "vi"
	}
	edArgv := strings.Split(ed, " ")
	var argv []string
	switch edArgv[0] {
	case "code": // Visual Studio Code
		argv = append(edArgv, path)
	default: // Vi, vim, emacs, ex, etc
		argv = append(edArgv, fmt.Sprintf("+%d", line), path)
	}
	if path, err := exec.LookPath(argv[0]); err == nil {
		argv[0] = path
	}
	proc, err := os.StartProcess(argv[0], argv, &os.ProcAttr{
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
	m := n
	if n > len(b) {
		n = len(b)
	} else {
		m = n
	}
	lineno = 1
	for i := 0; ; i++ {
		if i >= n {
			if i >= m {
				lineno = 0
			}
			break
		}
		if a[i] != b[i] {
			break
		}
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

	e, txfmt, err := readTx(arg)
	if os.IsNotExist(err) {
		e = NewTransactionEnvelope()
		txfmt = fmt_compiled
	} else if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	getAccounts(net, e, false)

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
			lastcontents = []byte(net.TxToRep(e))
			ioutil.WriteFile(path, lastcontents, 0600)
		}

		fi1, staterr := os.Stat(path)
		if staterr != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		line := firstDifferentLine(contents, lastcontents)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			fmt.Printf("Press return to run editor.")
			b := make([]byte, 1)
			for n, err := os.Stdin.Read(b); err != nil && n > 0 && b[0] != '\n'; {
				fmt.Printf("Read %c\n", b)
			}
			if pe, ok := err.(ParseError); ok {
				line = pe.TxrepError[0].Line
			}
		}
		editor(path, line)

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
		err = nil
		if newe, pe := TxFromRep(string(contents)); pe != nil {
			err = ParseError{pe.(stcdetail.TxrepError), path}
		} else {
			e = newe
		}
	}

	mustWriteTx(arg, e, net, txfmt)
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

var dateFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
	"20060102150405",
	"200601021504",
	"20060102",
}

func main() {
	opt_compile := flag.Bool("c", false, "Compile output to base64 XDR")
	opt_json := flag.Bool("json", false, "Output transaction in JSON format")
	opt_keygen := flag.Bool("keygen", false, "Create a new signing keypair")
	opt_genesis_key := flag.Bool("genesis-key", false,
		"Compute genesis key for network")
	opt_sec2pub := flag.Bool("pub", false, "Get public key from private")
	opt_output := flag.String("o", "", "Output to `FILE` instead of stdout")
	opt_preauth := flag.Bool("preauth", false,
		"Hash transaction to strkey for use as a pre-auth transaction signer")
	opt_txhash := flag.Bool("txhash", false, "Hash transaction to hex format")
	opt_inplace := flag.Bool("i", false, "Edit the input file in place")
	opt_sign := flag.Bool("sign", false, "Sign the transaction")
	opt_payload := flag.String("payload", "false",
		"Add signature on raw `HEX-STRING` instead of on this transaction")
	opt_key := flag.String("key", "", "Use secret signing key in `FILE`")
	opt_netname := flag.String("net", "",
		"Use Network `NET` (e.g., test); default: $STCNET or \"default\"")
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
	opt_fee_stats := flag.Bool("fee-stats", false,
		"Dump fee stats from network")
	opt_ledger_header := flag.Bool("ledger-header", false,
		"Dump ledger header from network")
	opt_acctinfo := flag.Bool("qa", false,
		"Query Horizon for information on account")
	opt_txinfo := flag.Bool("qt", false,
		"Query Horizon for information on transaction")
	opt_txacct := flag.Bool("qta", false,
		"Query Horizon for transactions on account")
	opt_mux := flag.Bool("mux", false,
		"Created a MuxedAccount from an AccountID and uint64")
	opt_demux := flag.Bool("demux", false,
		"Split a MuxedAccount into an AccountID and a uint64")
	opt_pack := flag.Bool("pack-payload", false,
		"Pack a public key and payload into a payload signer")
	opt_unpack := flag.Bool("unpack-payload", false,
		"Unpack a payload signer into a public key and payload")
	opt_friendbot := flag.Bool("create", false,
		"Create and fund account (on testnet only)")
	opt_date := flag.Bool("date", false,
		"Convert data to Unix time (for use in TimeBounds)")
	opt_verbose := flag.Bool("v", false,
		"Be more verbose for some operations")
	opt_hint := flag.Bool("hint", false,
		"Print signature hint for a public key")
	opt_print_default_config := flag.Bool("builtin-config", false,
		"Print the built-in stc.conf file used when none is found")
	opt_zerosig := flag.Bool("z", false, "Zero out the signatures vector")
	opt_opid := flag.Bool("opid", false, "Calculate a balance entry ID")
	if pos := strings.LastIndexByte(os.Args[0], '/'); pos >= 0 {
		progname = os.Args[0][pos+1:]
	} else {
		progname = os.Args[0]
	}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			`Usage: %[1]s [-net=ID] [-z] [-sign] [-c|-json] [-l] [-u] \
           [-i | -o OUTPUT-FILE] INPUT-FILE
       %[1]s -edit [-net=ID] FILE
       %[1]s -post [-net=ID] INPUT-FILE
       %[1]s -preauth [-net=ID] INPUT-FILE
       %[1]s -txhash [-net=ID] INPUT-FILE
       %[1]s -fee-stats
       %[1]s -ledger-header
       %[1]s -qa [-net=ID] ACCT
       %[1]s -qt [-net=ID] TXHASH
       %[1]s -qta [-net=ID] ACCT
       %[1]s -create [-net=ID] ACCT
       %[1]s -keygen [NAME]
       %[1]s -genesis-key [NAME]
       %[1]s -pub [NAME]
       %[1]s -import-key NAME
       %[1]s -export-key NAME
       %[1]s -list-keys
       %[1]s -date YYYY-MM-DD[Thh:mm:ss[Z]]
       %[1]s -hint PUBKEY
       %[1]s -mux ACCT U64
       %[1]s -demux ACCT
       %[1]s -payload HEX-PAYLOAD
       %[1]s -pack-payload KEY PAYLOAD
       %[1]s -unpack-payload PAYLOAD
       %[1]s -opid ACCT SEQNO OPNO
       %[1]s -builtin-config
`, progname)
		flag.PrintDefaults()
	}
	flag.Parse()
	if *opt_help {
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		return
	}
	if *opt_print_default_config {
		os.Stdout.Write(DefaultGlobalConfigContents)
		return
	}
	if *opt_payload != "false" {
		*opt_sign = true
	}

	nmode := b2i(*opt_preauth, *opt_txhash, *opt_post, *opt_edit,
		*opt_keygen, *opt_genesis_key, *opt_date, *opt_sec2pub,
		*opt_import_key, *opt_export_key, *opt_acctinfo, *opt_txinfo,
		*opt_txacct, *opt_friendbot, *opt_list_keys, *opt_fee_stats,
		*opt_ledger_header, *opt_print_default_config, *opt_mux,
		*opt_demux, *opt_pack, *opt_unpack, *opt_opid, *opt_hint)

	argsMin, argsMax := 1, 1
	switch {
	case *opt_fee_stats || *opt_ledger_header ||
		*opt_print_default_config || *opt_list_keys:
		argsMin, argsMax = 0, 0
	case *opt_keygen || *opt_sec2pub || *opt_genesis_key:
		argsMin = 0
	case *opt_mux, *opt_pack:
		argsMin, argsMax = 2, 2
	case *opt_opid:
		argsMax, argsMax = 3, 3
	}

	if na := len(flag.Args()); nmode > 1 || na < argsMin || na > argsMax {
		flag.Usage()
		os.Exit(2)
	}

	outfmt := fmt_txrep
	if *opt_compile {
		outfmt = fmt_compiled
		if *opt_json {
			fmt.Fprintln(os.Stderr, "-json and -c are mutually exclusive")
			os.Exit(2)
		}
	} else if *opt_json {
		outfmt = fmt_json
	}

	if nmode > 0 {
		bail := false
		if *opt_sign || *opt_key != "" {
			fmt.Fprintln(os.Stderr,
				"--sign, --key, and --payload only availble in default mode")
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
			fmt.Fprintln(os.Stderr, "-c only availble in default mode")
			bail = true
		}
		if *opt_json {
			fmt.Fprintln(os.Stderr, "-json only availble in default mode")
			bail = true
		}
		if *opt_zerosig {
			fmt.Fprintln(os.Stderr, "-z only availble in default mode")
			bail = true
		}
		if bail {
			os.Exit(2)
		}
	} else if *opt_inplace && *opt_output != "" {
		fmt.Fprintln(os.Stderr, "-i and -o are mutually exclusive")
		os.Exit(2)
	}

	var arg string
	if len(flag.Args()) >= 1 {
		arg = flag.Args()[0]
	}

	if *opt_nopass {
		stcdetail.PassphraseFile = io.MultiReader()
	} else if arg == "-" {
		stcdetail.PassphraseFile = nil
	}

	switch {
	case *opt_hint:
		var pk PublicKey
		if _, err := fmt.Sscan(arg, &pk); err != nil {
			fmt.Fprintf(os.Stderr, "invalid PublicKey %s\n", arg)
			os.Exit(2)
		}
		fmt.Printf("%x\n", pk.Hint())
		os.Exit(0)
	case *opt_opid:
		var opid stx.HashIDPreimage
		opid.Type = stx.ENVELOPE_TYPE_OP_ID
		if _, err := fmt.Sscan(arg, &opid.OperationID().SourceAccount);
		err != nil {
			fmt.Fprintf(os.Stderr, "invalid account ID %s\n", arg)
			os.Exit(2)
		}
		arg = flag.Args()[1]
		if _, err := fmt.Sscan(arg, &opid.OperationID().SeqNum); err != nil {
			fmt.Fprintf(os.Stderr, "invalid SequenceNumber %q (%s)\n",
				arg, err)
			os.Exit(2)
		}
		arg = flag.Args()[2]
		if _, err := fmt.Sscan(arg, &opid.OperationID().OpNum); err != nil {
			fmt.Fprintf(os.Stderr, "invalid operation number %q (%s)\n",
				arg, err)
			os.Exit(2)
		}
		var cbid stx.ClaimableBalanceID
		cbid.Type = stx.CLAIMABLE_BALANCE_ID_TYPE_V0
		*cbid.V0() = stcdetail.XdrSHA256(&opid)
		fmt.Printf("%x\n", []byte(stcdetail.XdrToBin(&cbid)))
		return
	case *opt_mux:
		var pk AccountID
		var id uint64
		if _, err := fmt.Sscan(arg, &pk); err != nil {
			fmt.Fprintf(os.Stderr, "invalid account ID %s\n", arg)
			os.Exit(2)
		}
		arg1 := flag.Args()[1]
		if _, err := fmt.Sscan(arg1, &id); err != nil {
			fmt.Fprintf(os.Stderr, "invalid uint64 %q (%s)\n", arg1, err)
			os.Exit(2)
		}
		m := MuxAcct(&pk, &id)
		if m == nil {
			fmt.Fprintf(os.Stderr, "cannot multiplex account\n")
			os.Exit(2)
		}
		fmt.Println(m.String())
		return
	case *opt_demux:
		var m MuxedAccount
		if _, err := fmt.Sscan(arg, &m); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		pk, id := DemuxAcct(&m)
		if pk == nil {
			fmt.Fprintf(os.Stderr, "cannot demultiplex account\n")
			os.Exit(2)
		}
		fmt.Print(pk)
		if id != nil {
			fmt.Print(" ", *id)
		}
		fmt.Println()
		return
	case *opt_pack:
		s := SignerKey { Type: stx.SIGNER_KEY_TYPE_ED25519_SIGNED_PAYLOAD }
		spl := s.Ed25519SignedPayload()
		var pk PublicKey
		if _, err := fmt.Sscan(arg, &pk); err != nil ||
			pk.Type != stx.PUBLIC_KEY_TYPE_ED25519 {
			fmt.Fprintf(os.Stderr, "invalid public key %s\n", arg)
			os.Exit(2)
		}
		copy(spl.Ed25519[:], pk.Ed25519()[:])
		arg1 := flag.Args()[1]
		if arg1 != "" {
			if _, err := fmt.Sscanf(arg1, "%x", &spl.Payload); err != nil {
				fmt.Fprintf(os.Stderr, "invalid hex payload %s\n", arg1)
				os.Exit(2)
			}
		}
		fmt.Println(s)
		return
	case *opt_unpack:
		var s SignerKey
		if _, err := fmt.Sscan(arg, &s); err != nil ||
			s.Type != stx.SIGNER_KEY_TYPE_ED25519_SIGNED_PAYLOAD {
			fmt.Fprintf(os.Stderr, "invalid payload signer %s\n", arg)
			os.Exit(2)
		}
		spl := s.Ed25519SignedPayload()
		pk := PublicKey { Type: stx.PUBLIC_KEY_TYPE_ED25519 }
		*pk.Ed25519() = spl.Ed25519
		fmt.Printf("%s\n%x\n", pk, spl.Payload)
		return
	case *opt_date:
		for _, f := range dateFormats {
			t, err := time.ParseInLocation(f, arg, time.Local)
			if err == nil {
				fmt.Printf("%d\n", t.Unix())
				return
			}
		}
		fmt.Fprintf(os.Stderr, "%s: cannot parse date %q\n", progname, arg)
		os.Exit(1)
	case *opt_keygen:
		if arg != "" {
			arg = AdjustKeyName(arg)
		}
		doKeyGen(arg, *opt_genesis_key)
		return
	case *opt_sec2pub:
		if arg != "" {
			arg = AdjustKeyName(arg)
		}
		doSec2pub(arg)
		return
	case *opt_import_key:
		arg = AdjustKeyName(arg)
		sk, err := InputPrivateKey("Secret key: ")
		if err == nil {
			err = sk.Save(arg, stcdetail.GetPass2("Passphrase: "))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case *opt_export_key:
		arg = AdjustKeyName(arg)
		sk, err := LoadPrivateKey(arg)
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

	net := DefaultStellarNet(*opt_netname)
	if net == nil {
		fmt.Fprintf(os.Stderr, "unknown network %q\n", *opt_netname)
		os.Exit(1)
	}

	if *opt_genesis_key {
		if arg != "" {
			arg = AdjustKeyName(arg)
		}
		doGenesisKey(arg, net)
		return
	}

	if *opt_acctinfo {
		var acct AccountID
		if _, err := fmt.Sscan(arg, &acct); err != nil {
			fmt.Fprintln(os.Stderr, "syntactically invalid account")
			os.Exit(1)
		}
		if ae, err := net.GetAccountEntry(arg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else {
			fmt.Print(ae)
		}
		return
	}

	if *opt_txinfo {
		var txid stx.Hash
		if _, err := fmt.Sscanf(arg, "%v", stx.XDR_Hash(&txid)); err != nil {
			fmt.Fprintln(os.Stderr, "syntactically invalid txid")
			os.Exit(1)
		} else if txr, err := net.GetTxResult(arg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else if *opt_verbose {
			fmt.Print(txr)
		} else {
			fmt.Printf("created_at: %s\n", txr.Time)
			fmt.Print("==== TRANSACTION ====\n", net.ToRep(&txr.Env),
				"==== RESULT ====\n", net.ToRep(&txr.Result),
				"==== EFFECTS ====\n",
				net.AccountDelta(&txr.StellarMetas, nil, ""))
		}
		return
	}

	if *opt_txacct {
		var acct AccountID
		if _, err := fmt.Sscan(arg, &acct); err != nil {
			fmt.Fprintln(os.Stderr, "syntactically invalid account")
			os.Exit(1)
		}

		nl := false
		err := net.IterateJSON(nil, "accounts/"+arg+
			"/transactions?order=desc&limit=200",
			func(r *HorizonTxResult) {
				if *opt_verbose {
					if !nl {
						nl = true
					} else {
						fmt.Println()
					}
					fmt.Print(r)
				} else {
					fmt.Printf("%x\n  time %s\n", r.Txhash, r.Time)
					fmt.Printf(net.AccountDelta(&r.StellarMetas, &acct, "  "))
				}
			})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if *opt_friendbot {
		var acct AccountID
		if _, err := fmt.Sscan(arg, &acct); err != nil {
			fmt.Fprintln(os.Stderr, "syntactically invalid account")
			os.Exit(1)
		}
		if _, err := net.Get("friendbot?addr=" + arg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if *opt_fee_stats {
		fs, err := net.GetFeeStats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching fee stats: %s\n",
				err.Error())
			os.Exit(1)
		}
		fmt.Print(fs)
		return
	}

	if *opt_ledger_header {
		lh, err := net.GetLedgerHeader()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching fee stats: %s\n",
				err.Error())
			os.Exit(1)
		}
		fmt.Print(net.ToRep(lh))
		return
	}

	if *opt_edit {
		doEdit(net, arg)
		return
	}

	e, infmt := mustReadTx(arg)
	switch {
	case *opt_post:
		res, err := net.Post(e)
		if err == nil {
			fmt.Print(xdr.XdrToString(res))
		} else {
			fmt.Fprintf(os.Stderr, "Post transaction failed: %s\n", err)
			os.Exit(1)
		}
	case *opt_txhash:
		fmt.Printf("%x\n", *net.HashTx(e))
	case *opt_preauth:
		sk := stx.SignerKey{Type: stx.SIGNER_KEY_TYPE_PRE_AUTH_TX}
		*sk.PreAuthTx() = *net.HashTx(e)
		fmt.Println(&sk)
	default:
		getAccounts(net, e, *opt_learn)
		if *opt_zerosig {
			*e.Signatures() = nil
		}
		if *opt_update {
			fixTx(net, e)
		}
		if *opt_sign || *opt_key != "" {
			var err error
			if *opt_payload == "false" {
				err = signTx(net, *opt_key, e)
			} else {
				err = signPayload(net, *opt_key, e, *opt_payload)
			}
			if err != nil {
				os.Exit(1)
			}
		}
		if *opt_learn {
			net.Save()
		}
		if *opt_inplace {
			*opt_output = arg
			if infmt == fmt_compiled && outfmt == fmt_txrep {
				outfmt = infmt
			}
		}
		mustWriteTx(*opt_output, e, net, outfmt)
	}
}
