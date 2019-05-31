package stcdetail

import (
	"bytes"
	"fmt"
	"github.com/xdrpp/stc/stx"
	"strings"
)

func XdrBin(t stx.XdrAggregate) []byte {
	out := bytes.Buffer{}
	t.XdrMarshal(&stx.XdrOut{&out}, "")
	return out.Bytes()
}

func RepDiff(arep, brep string) string {
	out := &strings.Builder{}
	amap := make(map [string]string)
	for _, a := range strings.Split(arep, "\n") {
		kv := strings.SplitN(a, ": ", 2)
		if len(kv) != 2 {
			continue
		}
		amap[kv[0]] = kv[1]
	}
	for _, b := range strings.Split(brep, "\n") {
		kv := strings.SplitN(b, ": ", 2)
		if len(kv) != 2 {
			continue
		}
		av, ok := amap[kv[0]]
		if !ok {
			fmt.Fprintf(out, "    %s: -> %s\n", kv[0], kv[1])
		} else if av != kv[1] {
			fmt.Fprintf(out, "    %s: %s -> %s\n", kv[0], av, kv[1])
		}
	}
	return out.String()
}

func GetLedgerEntryKey(e stx.LedgerEntry) stx.LedgerKey {
	k := stx.LedgerKey{ Type: e.Data.Type }
	switch k.Type {
	case stx.ACCOUNT:
		k.Account().AccountID = e.Data.Account().AccountID
	case stx.TRUSTLINE:
		k.TrustLine().AccountID = e.Data.TrustLine().AccountID
		k.TrustLine().Asset = e.Data.TrustLine().Asset
	case stx.OFFER:
		k.Offer().SellerID = e.Data.Offer().SellerID
		k.Offer().OfferID = e.Data.Offer().OfferID
	case stx.DATA:
		k.Data().AccountID = e.Data.Data().AccountID
		k.Data().DataName = e.Data.Data().DataName
	}
	return k
}

type aex struct {
	tp stx.LedgerEntryChangeType
	fn func(stx.LedgerEntryChangeType, *stx.AccountID, *stx.AccountEntry)
}
func (*aex) Sprintf(format string, args ...interface{}) string {
	return ""
}
func (x *aex) Marshal(_ string, t stx.XdrType) {
	switch v := t.(type) {
	case *stx.LedgerEntryChangeType:
		x.tp = *v
	case *stx.LedgerEntry:
		if v.Data.Type == stx.ACCOUNT {
			x.fn(x.tp, &v.Data.Account().AccountID, v.Data.Account())
		}
	case *stx.LedgerKey:
		if v.Type == stx.ACCOUNT {
			x.fn(x.tp, &v.Account().AccountID, nil)
		}
	case stx.XdrAggregate:
		v.XdrMarshal(x, "")
	}
}

func ForEachAccountEntry(m *stx.TransactionMeta,
	fn func(stx.LedgerEntryChangeType, *stx.AccountID, *stx.AccountEntry)) {
	m.XdrMarshal(&aex{fn: fn}, "")
}
