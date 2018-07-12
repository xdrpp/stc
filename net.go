
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var Horizon = "https://horizon-testnet.stellar.org/"

func get(query string) []byte {
	resp, err := http.Get(Horizon + query)
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

type HorizonAccountEntry struct {
	Sequence json.Number
	Thresholds struct {
		Low_threshold uint8
		Med_threshold uint8
		High_threshold uint8
	}
	Signers []struct {
		Key string
		Weight uint32
	}
}

func GetAccountEntry(acct string) *HorizonAccountEntry {
	if body := get("accounts/" + acct); body != nil {
		var ae HorizonAccountEntry
		if err := json.Unmarshal(body, &ae); err != nil {
			return nil
		}
		return &ae
	}
	return nil
}

func GetLedgerHeader() (ret *LedgerHeader) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			ret = nil
		}
	}()

	body := get("ledgers?limit=1&order=desc")
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
	if err := json.Unmarshal(body, &lhx);
	err != nil || len(lhx.Embedded.Records) == 0 {
		panic(err)
	}

	ret = &LedgerHeader{}
	b64i := base64.NewDecoder(base64.StdEncoding,
		strings.NewReader(lhx.Embedded.Records[0].Header_xdr))
	ret.XdrMarshal(&XdrIn{b64i}, "")
	return
}
