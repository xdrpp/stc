package stc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/xdrpp/stc/detail"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func get(net *StellarNet, query string) []byte {
	if net.Horizon == "" {
		fmt.Fprintln(os.Stderr, "Missing or invalid horizon URL\n")
		return nil
	}
	resp, err := http.Get(net.Horizon + query)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	return body
}

type HorizonSigner struct {
	Key    string
	Weight uint32
}
type HorizonAccountEntry struct {
	Sequence   json.Number
	Thresholds struct {
		Low_threshold  uint8
		Med_threshold  uint8
		High_threshold uint8
	}
	Signers []HorizonSigner
}

// Fetch the sequence number and signers of an account over the
// network.
func (net *StellarNet) GetAccountEntry(acct string) *HorizonAccountEntry {
	if body := get(net, "accounts/"+acct); body != nil {
		var ae HorizonAccountEntry
		if err := json.Unmarshal(body, &ae); err != nil {
			return nil
		}
		return &ae
	}
	return nil
}

func (net *StellarNet) GetNetworkId() string {
	if net.NetworkId != "" {
		return net.NetworkId
	}
	body := get(net, "/")
	if body == nil {
		return ""
	}
	var np struct {
		Network_passphrase string
	}
	if err := json.Unmarshal(body, &np); err != nil {
		return ""
	}
	net.NetworkId = np.Network_passphrase
	return net.NetworkId
}

var feeSuffix string = "_accepted_fee"
type feePercentile = struct {
	Percentile int
	Fee uint32
}

// Go representation of the Horizon Fee Stats structure response.  The
// fees are per operation in a transaction, and the individual fields
// are documented here:
// https://www.stellar.org/developers/horizon/reference/endpoints/fee-stats.html
type FeeStats struct {
	Last_ledger uint64
	Last_ledger_base_fee uint32
	Ledger_capacity_usage float64
	Min_accepted_fee uint32
	Mode_accepted_fee uint32
	Percentiles []struct {
		Percentile int
		Fee uint32
	}
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
                return string(s[0] &^ 0x20) + s[1:]
        }
        return s
}

func getU32(i interface{}) (uint32, error) {
	// Annoyingly, Horizion aleays returns strings instead of numbers
	// for the /fee_stats endpoint.  Because this behavior is
	// annoying, we want to be prepared for it to change, which is why
	// we Sprint and then Parse.
	n, err := strconv.ParseUint(fmt.Sprint(i), 10, 32)
	return uint32(n), err
}

// Queries the network for the latest fee statistics.
func (net *StellarNet) GetFeeStats() *FeeStats {
	body := get(net, "fee_stats")
	if body == nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var obj map[string]interface{}
	if err := dec.Decode(&obj); err != nil {
		return nil
	}

	var fs FeeStats
	rv := reflect.ValueOf(&fs).Elem()
	for k := range obj {
		if strings.HasSuffix(k, feeSuffix) &&
			k[0] == 'p' && k[1] >= '0' || k[1] <= '9' {
			if p, err := getU32(k[1:len(k)-len(feeSuffix)]); err == nil {
				if fee, err := getU32(obj[k]); err == nil {
					fs.Percentiles = append(fs.Percentiles, feePercentile{
						Percentile: int(p),
						Fee: fee,
					})
				}
			}
			continue
		}
		capk := capitalize(k)
		if capk == "Percentiles" {
			continue // Server is messing with us
		}
		switch field, s := rv.FieldByName(capk), fmt.Sprint(obj[k]);
		field.Kind() {
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
		return nil
	}

	sort.Slice(fs.Percentiles, func(i, j int) bool {
		return fs.Percentiles[i].Percentile < fs.Percentiles[j].Percentile
	})
	return &fs
}

// Fetch the latest ledger header over the network.
func (net *StellarNet) GetLedgerHeader() *LedgerHeader {
	body := get(net, "ledgers?limit=1&order=desc")
	if body == nil {
		return nil
	}

	var lhx struct {
		Embedded struct {
			Records []struct {
				Header_xdr string
			}
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(body, &lhx); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	} else if len(lhx.Embedded.Records) == 0 {
		fmt.Fprintln(os.Stderr, "Horizon returned no ledgers")
		return nil
	}

	ret := &LedgerHeader{}
	if err := detail.XdrFromBase64(ret, lhx.Embedded.Records[0].Header_xdr);
	err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}
	return ret
}

// Post a new transaction to the network.
func (net *StellarNet) Post(e *TransactionEnvelope) *TransactionResult {
	if net.Horizon == "" {
		fmt.Fprintln(os.Stderr, "Missing or invalid horizon URL\n")
		return nil
	}
	tx := detail.XdrToBase64(e)
	resp, err := http.PostForm(net.Horizon+"/transactions",
		url.Values{"tx": {tx}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
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
		fmt.Fprintf(os.Stderr, "PostTransaction: %s\n", err.Error())
		return nil
	}
	if res.Result_xdr == "" {
		res.Result_xdr = res.Extras.Result_xdr
	}

	var ret TransactionResult
	if err = detail.XdrFromBase64(&ret, res.Result_xdr); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid result_xdr\n")
		return nil
	}
	return &ret
}
