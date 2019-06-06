package stc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/xdrpp/stc/stcdetail"
	"github.com/xdrpp/stc/stx"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

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

var badCb error = fmt.Errorf(
	"StreamJSON cb argument must be of type func(*T) or func(*T)error")

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

	return stcdetail.Stream(ctx, query, func(evtype string, data []byte) error {
		switch evtype {
		case "error":
			return stcdetail.HTTPerror(data)
		case "message":
			v := reflect.New(tp)
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

	backoff := time.Second
	for url := net.Horizon + query; !stcdetail.IsDone(ctx); url =
		j.Links.Next.Href {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		} else if ctx != nil {
			req = req.WithContext(ctx)
		}
		cleanup()
		resp, err = http.DefaultClient.Do(req)
		if err != nil || stcdetail.IsDone(ctx) {
			return err
		} else if resp.StatusCode != 200 {
			if resp.StatusCode != 429 {
				return stcdetail.HTTPerror(resp.Status)
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
		Asset_type string
		Asset_code string
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

func (hs *HorizonAccountEntry) String() string {
	return stcdetail.PrettyPrint(hs)
}

// Return the next sequence number (1 + Sequence) as an int64 (or 0 if
// an invalid sequence number was returned by horizon).
func (ae *HorizonAccountEntry) NextSeq() int64 {
	val := int64(ae.Sequence)
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
	var ret HorizonAccountEntry
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
		if err := net.GetJSON("/", &np); err == nil {
			net.NetworkId = np.Network_passphrase
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
	default:
		return stcdetail.XdrToBase64(&k)
	}
}

func (net *StellarNet) AccountDelta(
	m *stcdetail.StellarMetas, acct *AccountID, prefix string) string {
	pprefix := prefix + "  "
	out := &strings.Builder{}
	mds := stcdetail.GetMetaDeltas(m)
	target := ""
	if acct != nil {
		target = stcdetail.XdrToBin(acct)
	}
	for i := range mds {
		if target != "" && stcdetail.XdrToBin(mds[i].AccountID()) != target {
			continue
		}
		ks := showLedgerKey(mds[i].Key)
		if mds[i].Old != nil && mds[i].New != nil {
			fmt.Fprintf(out, "%supdated %s\n%s", prefix, ks,
				stcdetail.RepDiff(pprefix,
				net.ToRep(mds[i].Old.Data.XdrUnionBody().(stx.XdrAggregate)),
				net.ToRep(mds[i].New.Data.XdrUnionBody().(stx.XdrAggregate))))
		} else if mds[i].New != nil {
			fmt.Fprintf(out, "%screated %s\n%s", prefix, ks, stcdetail.RepDiff(
				pprefix, "",
				net.ToRep(mds[i].New.Data.XdrUnionBody().(stx.XdrAggregate))))
		} else {
			fmt.Fprintf(out, "%sdeleted %s\n%s", prefix, ks,
				stcdetail.RepDiff(pprefix,
				net.ToRep(mds[i].Old.Data.XdrUnionBody().(stx.XdrAggregate)),
				""))
		}
	}
	return out.String()
}

type HorizonTxResult struct {
	Txhash stx.Hash
	Env stx.TransactionEnvelope
	Result stx.TransactionResult
	stcdetail.StellarMetas
	PagingToken string
}

func (r HorizonTxResult) String() string {
	out := strings.Builder{}
	stcdetail.XdrToTxrep(&out, "", &r.Env)
	stcdetail.XdrToTxrep(&out, "", &r.Result)
	stcdetail.XdrToTxrep(&out, "", &r.StellarMetas)
	fmt.Fprintf(&out, "paging_token: %s\n", r.PagingToken)
	return out.String()
}

func (r *HorizonTxResult) UnmarshalJSON(data []byte) error {
	var j struct {
		Envelope_xdr string
		Result_xdr string
		Result_meta_xdr string
		Fee_meta_xdr string
		Paging_token string
		Hash string
	}
	hash := stx.XdrArrayOpaque(r.Txhash[:])
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64(&r.Env,
		j.Envelope_xdr); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64(&r.Result,
		j.Result_xdr); err != nil {
		return err
	} else if err = stcdetail.XdrFromBase64s(&r.StellarMetas,
		j.Fee_meta_xdr, j.Result_meta_xdr); err != nil {
			return err
	} else if _, err := fmt.Sscanf(j.Hash, "%x", &hash); err != nil {
		return err
	}
	r.PagingToken = j.Paging_token
	return nil
}

func (net *StellarNet) GetTxResult(txid string) (*HorizonTxResult, error) {
	var ret HorizonTxResult
	if err := net.GetJSON("transactions/"+txid, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

var feeSuffix string = "_accepted_fee"

type feePercentile = struct {
	Percentile int
	Fee        uint32
}

// Go representation of the Horizon Fee Stats structure response.  The
// fees are per operation in a transaction, and the individual fields
// are documented here:
// https://www.stellar.org/developers/horizon/reference/endpoints/fee-stats.html
type FeeStats struct {
	Last_ledger           uint64
	Last_ledger_base_fee  uint32
	Ledger_capacity_usage float64
	Min_accepted_fee      uint32
	Mode_accepted_fee     uint32
	Percentiles           []struct {
		Percentile int
		Fee        uint32
	}
}

func (fs FeeStats) String() string {
	out := strings.Builder{}
	rv := reflect.ValueOf(&fs).Elem()
	tp := rv.Type()
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i).Name
		if field != "Percentiles" {
			fmt.Fprintf(&out, "%24s: %v\n", strings.ToLower(field),
				rv.Field(i).Interface())
		}
	}
	for i := range fs.Percentiles {
		fmt.Fprintf(&out, "%9d_percentile_fee: %d\n",
			fs.Percentiles[i].Percentile,
			fs.Percentiles[i].Fee)
	}
	return out.String()
}

// Conservatively returns a fee that is a known fee for the target or
// the closest higher known percentile.  Does not interpolate--e.g.,
// if you ask for the 51st percentile but only the 50th and 60th are
// known, returns the 60th percentile.  Never returns a value less
// than the base fee.
func (fs *FeeStats) Percentile(target int) uint32 {
	var fee uint32
	if len(fs.Percentiles) > 0 {
		fee = 1 + fs.Percentiles[len(fs.Percentiles)-1].Fee
	}
	for lo, hi := 0, len(fs.Percentiles); lo < hi; {
		n := (lo + hi) / 2
		p := &fs.Percentiles[n]
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
	if fee < fs.Last_ledger_base_fee {
		fee = fs.Last_ledger_base_fee
	}
	return fee
}

func capitalize(s string) string {
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]&^0x20) + s[1:]
	}
	return s
}

func parseU32(i interface{}) (uint32, error) {
	// Annoyingly, Horizion always returns strings instead of numbers
	// for the /fee_stats endpoint.  Because this behavior is
	// annoying, we want to be prepared for it to change, which is why
	// we Sprint and then Parse.
	n, err := strconv.ParseUint(fmt.Sprint(i), 10, 32)
	return uint32(n), err
}

func (fs *FeeStats) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]interface{}
	if err := dec.Decode(&obj); err != nil {
		return err
	}

	rv := reflect.ValueOf(fs).Elem()
	for k := range obj {
		if strings.HasSuffix(k, feeSuffix) &&
			k[0] == 'p' && k[1] >= '0' || k[1] <= '9' {
			if p, err := parseU32(k[1 : len(k)-len(feeSuffix)]); err == nil {
				if fee, err := parseU32(obj[k]); err == nil {
					fs.Percentiles = append(fs.Percentiles, feePercentile{
						Percentile: int(p),
						Fee:        fee,
					})
				}
			}
			continue
		}
		capk := capitalize(k)
		if capk == "Percentiles" {
			continue // Server is messing with us
		}
		switch field, s :=
			rv.FieldByName(capk), fmt.Sprint(obj[k]); field.Kind() {
		case reflect.Uint32:
			if v, err := strconv.ParseUint(s, 10, 32); err == nil {
				field.SetUint(v)
			}
		case reflect.Uint64:
			if v, err := strconv.ParseUint(s, 10, 64); err == nil {
				field.SetUint(v)
			}
		case reflect.Float64:
			if v, err := strconv.ParseFloat(s, 64); err == nil {
				field.SetFloat(v)
			}
		}
	}
	if fs.Min_accepted_fee == 0 || fs.Last_ledger_base_fee == 0 ||
		len(fs.Percentiles) == 0 {
		// Something's wrong; don't return garbage
		return horizonFailure("Garbled fee_stats")
	}

	sort.Slice(fs.Percentiles, func(i, j int) bool {
		return fs.Percentiles[i].Percentile < fs.Percentiles[j].Percentile
	})
	return nil
}

// Queries the network for the latest fee statistics.
func (net *StellarNet) GetFeeStats() (*FeeStats, error) {
	var ret FeeStats
	if err := net.GetJSON("fee_stats", &ret); err != nil {
		return nil, err
	}
	return &ret, nil
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

func enumDesc(e stx.XdrEnum) string {
	if ec, ok := e.(enumComments); ok {
		if c, ok := ec.XdrEnumComments()[int32(e.GetU32())]; ok {
			return c
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

func (e TxFailure) Error() string {
	msg := enumDesc(&e.Result.Code)
	switch e.Result.Code {
	case stx.TxFAILED:
		out := strings.Builder{}
		out.WriteString(msg)
		for i := range *e.Result.Results() {
			fmt.Fprintf(&out, "\noperation %d: %s", i,
				enumDesc(&(*e.Result.Results())[i].Code))
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
	resp, err := http.PostForm(net.Horizon+"/transactions",
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
