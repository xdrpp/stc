package stc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xdrpp/goxdr/xdr"
	"github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Try to determine whether a request to Horizon indicates the
// operation is worth retrying.  Specifically, this function
// repeatedly unwraps errors and returns true if either A) one of the
// errors has a Temporary() method that returns true, or B) one of the
// errors is a net.OpError for Op "dial" and that is not wrapping a
// DNS error.  The logic here is that if the DNS name of a horizon
// server does not exist (permanent DNS error), there is likely some
// misconfiguration.  However, if the horizon server is refusing TCP
// connections, it may be undergoing maintenance.
func IsTemporary(err error) bool {
	dial_not_dns := false
	for ; err != nil; err = errors.Unwrap(err) {
		if t, ok := err.(interface{ Temporary() bool }); ok && t.Temporary() {
			return true
		} else if operr, ok := err.(*net.OpError); ok && operr.Op == "dial" {
			dial_not_dns = true
		} else if _, ok := err.(*net.DNSError); ok {
			dial_not_dns = false
		}
	}
	return dial_not_dns
}

// A communication error with horizon
type horizonFailure string

func (e horizonFailure) Error() string {
	return string(e)
}

const badHorizonURL horizonFailure = "Missing or invalid horizon URL"

func getURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, horizonFailure(body)
	}
	return body, nil
}

// Send an HTTP request to horizon
func (net *StellarNet) Get(query string) ([]byte, error) {
	if net.Horizon == "" {
		return nil, badHorizonURL
	}
	return getURL(net.Horizon + query)
}

// Send an HTTP request to horizon and perse the result as JSON
func (net *StellarNet) GetJSON(query string, out interface{}) error {
	if body, err := net.Get(query); err != nil {
		return err
	} else {
		return json.Unmarshal(body, out)
	}
}

var badCb error = errors.New(
	"StreamJSON cb argument must be of type func(*T) or func(*T)error")

type ErrEventStream string

func (e ErrEventStream) Error() string {
	return string(e)
}

func setField(v reflect.Value, field string, val reflect.Value) {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		if f := v.FieldByName(field); (f != reflect.Value{} &&
			f.Type() == val.Type()) {
			f.Set(val)
			return
		}
	}
}

// Stream a series of events.  cb is a callback function which must
// have type func(obj *T)error or func(obj *T), where *T is a type
// into which JSON can be unmarshalled.  Returns if there is an error
// or the ctx argument is Done.  You likely want to call this in a
// goroutine, and might want to call it in a loop to try again after
// errors.
func (net *StellarNet) StreamJSON(
	ctx context.Context, query string, cb interface{}) error {
	cbv := reflect.ValueOf(cb)
	tp := cbv.Type()
	if tp.Kind() != reflect.Func ||
		tp.NumIn() != 1 || tp.In(0).Kind() != reflect.Ptr ||
		tp.NumOut() > 1 ||
		(tp.NumOut() == 1 && tp.Out(0).String() != "error") {
		panic(badCb)
	}
	tp = tp.In(0).Elem()

	if net.Horizon == "" {
		return badHorizonURL
	}
	query = net.Horizon + query

	netval := reflect.ValueOf(net)
	return stcdetail.Stream(ctx, query, func(evtype string, data []byte) error {
		switch evtype {
		case "error":
			return ErrEventStream(data)
		case "message":
			v := reflect.New(tp)
			setField(v, "Net", netval)
			if err := json.Unmarshal(data, v.Interface()); err != nil {
				return err
			}
			errs := cbv.Call([]reflect.Value{v})
			if len(errs) != 0 {
				if err, ok := errs[0].Interface().(error); ok && err != nil {
					return err
				}
			}
		}
		return nil
	})
}

type jsonInterface struct {
	i interface{}
}

func (ji *jsonInterface) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, ji.i)
}

// Send a request to horizon and iterate through a series of embedded
// records in the response, continuing to fetch more records until
// zero records are returned.  cb is a callback function which must
// have type func(obj *T)error or func(obj *T), where *T is a type
// into which JSON can be unmarshalled.  Returns if there is an error
// or the ctx argument is Done.
func (net *StellarNet) IterateJSON(
	ctx context.Context, query string, cb interface{}) error {
	if net.Horizon == "" {
		return badHorizonURL
	}

	var resp *http.Response
	cleanup := func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
	defer cleanup()

	cbv := reflect.ValueOf(cb)
	tp := cbv.Type()
	if tp.Kind() != reflect.Func ||
		tp.NumIn() != 1 || tp.In(0).Kind() != reflect.Ptr ||
		tp.NumOut() > 1 ||
		(tp.NumOut() == 1 && tp.Out(0).String() != "error") {
		panic(badCb)
	}
	tp = tp.In(0).Elem()

	var j struct {
		Links struct {
			Next struct {
				Href string
			}
		} `json:"_links"`
		Embedded struct {
			Records jsonInterface
		} `json:"_embedded"`
	}
	j.Embedded.Records.i = reflect.New(reflect.SliceOf(tp)).Interface()

	netval := reflect.ValueOf(net)

	backoff := time.Second
	for url := net.Horizon + query; ctx == nil || ctx.Err() == nil; url =
		j.Links.Next.Href {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		} else if ctx != nil {
			req = req.WithContext(ctx)
		}
		cleanup()
		resp, err = http.DefaultClient.Do(req)
		if err != nil || ctx != nil && ctx.Err() != nil {
			return err
		} else if resp.StatusCode != 200 {
			if resp.StatusCode != 429 {
				return stcdetail.NewHTTPerror(resp)
			}
			if ctx != nil {
				select {
				case <-ctx.Done():
				case <-time.After(backoff):
				}
			} else {
				time.Sleep(backoff)
			}
			backoff *= 2
			continue
		}
		backoff = time.Second
		dec := json.NewDecoder(resp.Body)
		if err = dec.Decode(&j); err != nil {
			return err
		}
		v := reflect.ValueOf(j.Embedded.Records.i).Elem()
		n := v.Len()
		if n == 0 {
			break
		}
		for i := 0; i < n; i++ {
			setField(v.Index(i), "Net", netval)
			errs := cbv.Call([]reflect.Value{v.Index(i).Addr()})
			if len(errs) != 0 {
				if err, ok := errs[0].Interface().(error); ok && err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type HorizonThresholds struct {
	Low_threshold  uint8
	Med_threshold  uint8
	High_threshold uint8
}
type HorizonFlags struct {
	Auth_required  bool
	Auth_revocable bool
	Auth_immutable bool
}
type HorizonSigner struct {
	Key    SignerKey
	Weight uint32
}

type HorizonBalance struct {
	Balance             stcdetail.JsonInt64e7
	Buying_liabilities  stcdetail.JsonInt64e7
	Selling_liabilities stcdetail.JsonInt64e7
	Limit               stcdetail.JsonInt64e7
	Asset               stx.Asset `json:"-"`
}

func (hb *HorizonBalance) UnmarshalJSON(data []byte) error {
	type jhb HorizonBalance
	var jasset struct {
		Asset_type   string
		Asset_code   string
		Asset_issuer AccountID
	}
	if err := json.Unmarshal(data, (*jhb)(hb)); err != nil {
		return err
	} else if err = json.Unmarshal(data, &jasset); err != nil {
		return err
	}
	var code []byte
	switch jasset.Asset_type {
	case "native":
		hb.Asset.Type = stx.ASSET_TYPE_NATIVE
		return nil
	case "credit_alphanum4":
		hb.Asset.Type = stx.ASSET_TYPE_CREDIT_ALPHANUM4
		a := hb.Asset.AlphaNum4()
		a.Issuer = jasset.Asset_issuer
		code = a.AssetCode[:]
	case "credit_alphanum12":
		hb.Asset.Type = stx.ASSET_TYPE_CREDIT_ALPHANUM12
		a := hb.Asset.AlphaNum12()
		a.Issuer = jasset.Asset_issuer
		code = a.AssetCode[:]
	default:
		return horizonFailure("unknown asset type " + jasset.Asset_type)
	}
	for i := range code {
		code[i] = 0
	}
	copy(code, jasset.Asset_code)
	return nil
}

// Structure into which you can unmarshal JSON returned by a query to
// horizon for an account endpoint
type HorizonAccountEntry struct {
	Net                   *StellarNet `json:"-"`
	Sequence              stcdetail.JsonInt64
	Balance               stcdetail.JsonInt64e7
	Subentry_count        uint32
	Inflation_destination *AccountID
	Home_domain           string
	Last_modified_ledger  uint32
	Flags                 HorizonFlags
	Thresholds            HorizonThresholds
	Balances              []HorizonBalance
	Signers               []HorizonSigner
	Data                  map[string]string
}

func (net *StellarNet) prettyPrintAux(i interface{}) (string, bool) {
	if _, ok := i.(StellarNet); ok {
		return "", true
	} else if net == nil {
		return "", false
	}
	switch v := i.(type) {
	case stx.IsAccount:
		if note := net.AccountIDNote(v.String()); note != "" {
			return fmt.Sprintf("%s (%s)", v, note), true
		}
	case stx.SignerKey:
		b := stcdetail.XdrToBin(&v)
		if skis, ok := net.Signers[v.Hint()]; ok {
			for j := range skis {
				if stcdetail.XdrToBin(&skis[j].Key) == b {
					return fmt.Sprintf("%s (%s)", v, skis[j].Comment), true
				}
			}
		}
	}
	return "", false
}

func (hs *HorizonAccountEntry) String() string {
	return stcdetail.PrettyPrintAux(hs.Net.prettyPrintAux, hs)
}

// Return the next sequence number (1 + Sequence) as an int64 (or 0 if
// an invalid sequence number was returned by horizon).
func (ae *HorizonAccountEntry) NextSeq() stx.SequenceNumber {
	val := stx.SequenceNumber(ae.Sequence)
	if val <= 0 {
		return 0
	} else {
		return val + 1
	}
}

func (ae *HorizonAccountEntry) UnmarshalJSON(data []byte) error {
	type hae HorizonAccountEntry
	if err := json.Unmarshal(data, (*hae)(ae)); err != nil {
		return err
	}
	for i := range ae.Balances {
		if ae.Balances[i].Asset.Type == stx.ASSET_TYPE_NATIVE {
			ae.Balance = ae.Balances[i].Balance
			ae.Balances = append(ae.Balances[:i], ae.Balances[i+1:]...)
			break
		}
	}
	return nil
}

// Fetch the sequence number and signers of an account over the
// network.
func (net *StellarNet) GetAccountEntry(acct string) (
	*HorizonAccountEntry, error) {
	ret := HorizonAccountEntry{Net: net}
	if err := net.GetJSON("accounts/"+acct, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// Returns the network ID, a string that is hashed into transaction
// IDs to ensure that signature are not valid across networks (e.g., a
// testnet signature cannot work on the public network).  If the
// network ID is not cached in the StellarNet structure itself, then
// this function fetches it from the network.
//
// Note StellarMainNet already contains the network ID, while
// StellarTestNet requires fetching the network ID since the Stellar
// test network is periodically reset.
func (net *StellarNet) GetNetworkId() string {
	if net.NetworkId == "" {
		var np struct{ Network_passphrase string }
		if err := net.GetJSON("/", &np); err == nil &&
			np.Network_passphrase != "" {
			net.NetworkId = np.Network_passphrase
			net.Edits.Set("net", "network-id", net.NetworkId)
		}
	}
	return net.NetworkId
}

func showLedgerKey(k stx.LedgerKey) string {
	switch k.Type {
	case stx.ACCOUNT:
		return fmt.Sprintf("account %s", k.Account().AccountID)
	case stx.TRUSTLINE:
		return fmt.Sprintf("trustline %s[%s]", k.TrustLine().AccountID,
			k.TrustLine().Asset)
	case stx.OFFER:
		return fmt.Sprintf("offer %d", k.Offer().OfferID)
	case stx.DATA:
		return fmt.Sprintf("data %s[%q]", k.Data().AccountID, k.Data().DataName)
	case stx.CLAIMABLE_BALANCE:
		return fmt.Sprintf("claimable_balance %v %v",
			k.ClaimableBalance().BalanceID.Type,
			k.ClaimableBalance().BalanceID.XdrUnionBody())
	case stx.LIQUIDITY_POOL:
		return fmt.Sprintf("liquidity_pool %v",
			k.LiquidityPool().LiquidityPoolID)
	default:
		return stcdetail.XdrToBase64(&k)
	}
}

func (net *StellarNet) AccountDelta(
	m *StellarMetas, acct *AccountID, prefix string) string {
	pprefix := prefix + "  "
	out := &strings.Builder{}
	mds := stcdetail.GetMetaDeltas(stx.XDR_LedgerEntryChanges(&m.FeeMeta),
		&m.ResultMeta)
	target := ""
	if acct != nil {
		target = stcdetail.XdrToBin(acct)
	}
	for i := range mds {
			// Note AccountID will return nil for LedgerKeys of type
			// CLAIMABLE_BALANCE and LIQUIDITY_POOL
		if acct != nil {
			if k := mds[i].AccountID(); (k == nil ||
				stcdetail.XdrToBin(k) != target) &&
				(mds[i].Old == nil ||
					!stcdetail.HasAccountID(acct, mds[i].Old)) &&
				(mds[i].New == nil ||
					!stcdetail.HasAccountID(acct, mds[i].New)) {
			continue
			}
		}
		ks := showLedgerKey(mds[i].Key)
		if mds[i].Old != nil && mds[i].New != nil {
			fmt.Fprintf(out, "%supdated %s\n%s", prefix, ks,
				stcdetail.RepDiff(pprefix,
					net.ToRep(mds[i].Old.Data.XdrUnionBody().(xdr.XdrType)),
					net.ToRep(mds[i].New.Data.XdrUnionBody().(xdr.XdrType))))
		} else if mds[i].New != nil {
			fmt.Fprintf(out, "%screated %s\n%s", prefix, ks, stcdetail.RepDiff(
				pprefix, "",
				net.ToRep(mds[i].New.Data.XdrUnionBody().(xdr.XdrType))))
		} else {
			fmt.Fprintf(out, "%sdeleted %s\n%s", prefix, ks,
				stcdetail.RepDiff(pprefix,
					net.ToRep(mds[i].Old.Data.XdrUnionBody().(xdr.XdrType)),
					""))
		}
	}
	return out.String()
}

// Ledger entries changed by a transaction.
type StellarMetas struct {
	FeeMeta    stx.LedgerEntryChanges
	ResultMeta stx.TransactionMeta
}

type HorizonTxResult struct {
	Net    *StellarNet
	Txhash stx.Hash
	Ledger uint32
	Time   time.Time
	Env    stx.TransactionEnvelope
	Result stx.TransactionResult
	StellarMetas
	PagingToken string
}

func (r *HorizonTxResult) Success() bool {
	return r.Result.Result.Code == stx.TxSUCCESS &&
		len(*r.Result.Result.Results()) > 0
}

func (r HorizonTxResult) String() string {
	out := strings.Builder{}
	fmt.Fprintf(&out, "txhash: %x\n", r.Txhash)
	fmt.Fprintf(&out, "ledger: %d\ncreated_at: %d (%s)\n",
		r.Ledger, r.Time.Unix(), r.Time.Format(time.UnixDate))
	r.Net.WriteRep(&out, "", &r.Env)
	r.Net.WriteRep(&out, "", &r.Result)
	r.Net.WriteRep(&out, "feeMeta", stx.XDR_LedgerEntryChanges(&r.FeeMeta))
	r.Net.WriteRep(&out, "resultMeta", &r.ResultMeta)
	fmt.Fprintf(&out, "paging_token: %s\n", r.PagingToken)
	return out.String()
}

func (r *HorizonTxResult) UnmarshalJSON(data []byte) error {
	var j struct {
		Envelope_xdr    string
		Result_xdr      string
		Result_meta_xdr string
		Fee_meta_xdr    string
		Paging_token    string
		Hash            string
		Ledger          uint32
		Created_at      string
	}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64(&r.Env,
		j.Envelope_xdr); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64(&r.Result,
		j.Result_xdr); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64(
		stx.XDR_LedgerEntryChanges(&r.FeeMeta), j.Fee_meta_xdr); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64(&r.ResultMeta,
		j.Result_meta_xdr); err != nil {
		return err
	} else if _, err := fmt.Sscanf(j.Hash, "%v",
		stx.XDR_Hash(&r.Txhash)); err != nil {
		return err
	} else if r.Time, err = time.ParseInLocation("2006-01-02T15:04:05Z",
		j.Created_at, time.UTC); err != nil {
		return err
	}
	r.Time = r.Time.Local()
	r.Ledger = j.Ledger
	r.PagingToken = j.Paging_token
	return nil
}

func (net *StellarNet) GetTxResult(txid string) (*HorizonTxResult, error) {
	ret := HorizonTxResult{Net: net}
	if err := net.GetJSON("transactions/"+txid, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// A Fee Value is currently 32 bits, but could become 64 bits if
// CAP-0015 is adopted.
type FeeVal = uint32

const feeValSize = 32

func parseFeeVal(i interface{}) (FeeVal, error) {
	// Annoyingly, Horizion always returns strings instead of numbers
	// for the /fee_stats endpoint.  Because this behavior is
	// annoying, we want to be prepared for it to change, which is why
	// we Sprint and then Parse.
	n, err := strconv.ParseUint(fmt.Sprint(i), 10, feeValSize)
	return uint32(n), err
}

type FeePercentile = struct {
	Percentile int
	Fee        FeeVal
}

// Distribution of offered or charged fees.
type FeeDist struct {
	Max         FeeVal
	Min         FeeVal
	Mode        FeeVal
	Percentiles []FeePercentile
}

func getPercentage(k string) (bool, int) {
	if len(k) < 2 || k[0] != 'p' || len(k) > 4 {
		return false, -1
	}
	r := 0
	for i := 1; i < len(k); i++ {
		if k[i] < '0' || k[i] > '9' {
			return false, -1
		}
		r = r*10 + int(k[i]-'0')
	}
	return true, r
}

func setVal(v reflect.Value, s string) bool {
	if !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Uint32:
		if n, err := strconv.ParseUint(s, 10, 32); err == nil {
			v.SetUint(n)
			return true
		}
	case reflect.Uint64:
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			v.SetUint(n)
			return true
		}
	case reflect.Float64:
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			v.SetFloat(n)
			return true
		}
	}
	return false
}

func (fd *FeeDist) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]interface{}
	if err := dec.Decode(&obj); err != nil {
		return err
	}

	rv := reflect.ValueOf(fd).Elem()
	for k := range obj {
		if ok, p := getPercentage(k); ok {
			if fee, err := parseFeeVal(obj[k]); err == nil {
				fd.Percentiles = append(fd.Percentiles, FeePercentile{
					Percentile: int(p),
					Fee:        fee,
				})
				continue
			}
		}
		capk := capitalize(k)
		if capk == "Percentiles" {
			continue // Server is trolling us
		}
		setVal(rv.FieldByName(capk), fmt.Sprint(obj[k]))
	}
	if fd.Min == 0 || fd.Max == 0 || len(fd.Percentiles) == 0 {
		// Something's wrong; don't return garbage
		return horizonFailure("Garbled fee_stats")
	}

	sort.Slice(fd.Percentiles, func(i, j int) bool {
		return fd.Percentiles[i].Percentile < fd.Percentiles[j].Percentile
	})
	return nil
}

// Conservatively returns a fee that is a known fee for the target or
// the closest higher known percentile.  Does not interpolate--e.g.,
// if you ask for the 51st percentile but only the 50th and 60th are
// known, returns the 60th percentile.  Never returns a value less
// than the base fee.
func (fd *FeeDist) Percentile(target int) FeeVal {
	var fee FeeVal
	if len(fd.Percentiles) > 0 {
		max := fd.Percentiles[len(fd.Percentiles)-1].Fee
		fee = max + 1
		if fee < max {
			fee = max
		}
	}
	for lo, hi := 0, len(fd.Percentiles); lo < hi; {
		n := (lo + hi) / 2
		p := &fd.Percentiles[n]
		if p.Percentile == target {
			fee = p.Fee
			break
		} else if p.Percentile > target {
			if fee > p.Fee {
				fee = p.Fee
			}
			hi = n
		} else {
			lo = n + 1
		}
	}
	return fee
}

func printFsField(out io.Writer, field string, v interface{}) {
	fmt.Fprintf(out, "%24s: %v\n", field, v)
}

func (fd *FeeDist) withPrefix(out io.Writer, prefix string) {
	printFsField(out, prefix+"max", fd.Max)
	printFsField(out, prefix+"min", fd.Min)
	printFsField(out, prefix+"mode", fd.Mode)
	for i := range fd.Percentiles {
		printFsField(out,
			fmt.Sprintf("%sp%d", prefix, fd.Percentiles[i].Percentile),
			fd.Percentiles[i].Fee)
	}
}

// Go representation of the Horizon Fee Stats structure response.  The
// fees are per operation in a transaction, and the individual fields
// are documented here:
// https://www.stellar.org/developers/horizon/reference/endpoints/fee-stats.html
type FeeStats struct {
	Last_ledger           uint64
	Last_ledger_base_fee  uint32
	Ledger_capacity_usage float64
	Charged               FeeDist
	Offered               FeeDist
}

func (fs *FeeStats) UnmarshalJSON(data []byte) error {
	type feeNumbers struct {
		Last_ledger           json.Number
		Last_ledger_base_fee  json.Number
		Ledger_capacity_usage json.Number
		Fee_charged           FeeDist
		Max_fee               FeeDist
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj feeNumbers
	if err := dec.Decode(&obj); err != nil {
		return err
	}
	if n, err := obj.Last_ledger.Int64(); err != nil {
		return err
	} else {
		fs.Last_ledger = uint64(n)
	}
	if n, err := obj.Last_ledger_base_fee.Int64(); err != nil {
		return err
	} else {
		fs.Last_ledger_base_fee = uint32(n)
	}
	if n, err := obj.Ledger_capacity_usage.Float64(); err != nil {
		return err
	} else {
		fs.Ledger_capacity_usage = n
	}
	fs.Charged = obj.Fee_charged
	fs.Offered = obj.Max_fee
	return nil
}

// Conservatively a known offered fee for the target or a higher
// percentile.  Never returns a value less than the base fee.
func (fs *FeeStats) Percentile(target int) FeeVal {
	fee := fs.Offered.Percentile(target)
	if fee < fs.Last_ledger_base_fee {
		fee = fs.Last_ledger_base_fee
	}
	return fee
}

func (fs FeeStats) String() string {
	out := &strings.Builder{}
	printFsField(out, "last_ledger", fs.Last_ledger)
	printFsField(out, "last_ledger_base_fee", fs.Last_ledger_base_fee)
	printFsField(out, "ledger_capacity_usage", fs.Ledger_capacity_usage)
	fs.Charged.withPrefix(out, "fee_charged.")
	fs.Offered.withPrefix(out, "max_fee.")
	return out.String()
}

func capitalize(s string) string {
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]&^0x20) + s[1:]
	}
	return s
}

// Queries the network for the latest fee statistics.
func (net *StellarNet) GetFeeStats() (*FeeStats, error) {
	var ret FeeStats
	now := time.Now()
	if err := net.GetJSON("fee_stats", &ret); err != nil {
		return nil, err
	}
	net.FeeCache = &ret
	net.FeeCacheTime = now
	return &ret, nil
}

// Like GetFeeStats but a version cached for 1 minute
func (net *StellarNet) GetFeeCache() (*FeeStats, error) {
	now := time.Now()
	if net.FeeCache != nil && now.Sub(net.FeeCacheTime) < 60*time.Second {
		return net.FeeCache, nil
	}
	return net.GetFeeStats()
}

// Fetch the latest ledger header over the network.
func (net *StellarNet) GetLedgerHeader() (*LedgerHeader, error) {
	body, err := net.Get("ledgers?limit=1&order=desc")
	if err != nil {
		return nil, err
	}

	var lhx struct {
		Embedded struct {
			Records []struct {
				Header_xdr string
			}
		} `json:"_embedded"`
	}
	if err = json.Unmarshal(body, &lhx); err != nil {
		return nil, err
	} else if len(lhx.Embedded.Records) == 0 {
		return nil, horizonFailure("Horizon returned no ledgers")
	}

	ret := &LedgerHeader{}
	if err = stcdetail.XdrFromBase64(ret, lhx.Embedded.Records[0].Header_xdr); err != nil {
		return nil, err
	}
	return ret, nil
}

type enumComments interface {
	XdrEnumComments() map[int32]string
}

func enumDesc(e xdr.XdrEnum) string {
	if ec, ok := e.(enumComments); ok {
		if c, ok := ec.XdrEnumComments()[int32(e.GetU32())]; ok {
			return e.String() + " (" + c + ")"
		}
	}
	return e.String()
}

// An error representing the failure of a transaction submitted to the
// Stellar network, and from which you can extract the full
// TransactionResult.
type TxFailure struct {
	*TransactionResult
}

type codeExtractor struct {
	msg string
}

func (x *codeExtractor) Sprintf(string, ...interface{}) string {
	return ""
}
func (x *codeExtractor) Marshal(name string, val xdr.XdrType) {
	if x.msg != "" {
		return
	}
	switch t := val.(type) {
	case xdr.XdrEnum:
		x.msg = enumDesc(t)
	case xdr.XdrAggregate:
		t.XdrRecurse(x, "")
	}
}

func extractCode(t xdr.XdrType) string {
	e := codeExtractor{}
	e.Marshal("", t)
	if e.msg != "" {
		return e.msg
	}

	out := strings.Builder{}
	stcdetail.XdrToTxrep(&out, "", t)
	return strings.TrimSuffix(out.String(), "\n")
}

func (e TxFailure) Error() string {
	msg := enumDesc(&e.Result.Code)
	switch e.Result.Code {
	case stx.TxFAILED:
		out := strings.Builder{}
		out.WriteString(msg)
		for i := range *e.Result.Results() {
			fmt.Fprintf(&out, "\noperation %d: ", i)
			if code := (*e.Result.Results())[i].Code; code != stx.OpINNER {
				out.WriteString(enumDesc(&code))
			} else {
				out.WriteString(extractCode(
					(*e.Result.Results())[i].Tr().XdrUnionBody()))
			}
		}
		return out.String()
	default:
		return msg
	}
}

// Post a new transaction to the network.  In the event that the
// transaction is successfully submitted to horizon but rejected by
// the Stellar network, the error will be of type TxFailure, which
// contains the transaction result.
func (net *StellarNet) Post(e *TransactionEnvelope) (
	*TransactionResult, error) {
	if net.Horizon == "" {
		return nil, badHorizonURL
	}
	tx := stcdetail.XdrToBase64(e)
	resp, err := http.PostForm(net.Horizon+"transactions/",
		url.Values{"tx": {tx}})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	js := json.NewDecoder(resp.Body)
	var res struct {
		Result_xdr string
		Extras     struct {
			Result_xdr string
		}
	}
	if err = js.Decode(&res); err != nil {
		return nil, err
	}
	if res.Result_xdr == "" {
		res.Result_xdr = res.Extras.Result_xdr
	}

	var ret TransactionResult
	if err = stcdetail.XdrFromBase64(&ret, res.Result_xdr); err != nil {
		return nil, err
	}
	if ret.Result.Code != stx.TxSUCCESS {
		return nil, TxFailure{&ret}
	}
	return &ret, nil
}
