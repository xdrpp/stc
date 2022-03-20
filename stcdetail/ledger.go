package stcdetail

import (
	"fmt"
	"github.com/xdrpp/goxdr/xdr"
	"github.com/xdrpp/stc/stx"
	"strings"
)

// Takes two inputs in txrep format, and returns a string describing
// all the lines that have changed in the form:
//
//     field: old_value -> new_value
//
// Either old_value or new_value may be empty for a field that appears
// only in a or only in b.
func RepDiff(prefix, arep, brep string) string {
	out := &strings.Builder{}
	amap := make(map[string]string)
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
			fmt.Fprintf(out, "%s%s: -> %s\n", prefix, kv[0], kv[1])
		} else if av != kv[1] {
			fmt.Fprintf(out, "%s%s: %s -> %s\n", prefix, kv[0], av, kv[1])
		}
	}
	return out.String()
}

func GetLedgerEntryKey(e *stx.LedgerEntry) stx.LedgerKey {
	k := stx.LedgerKey{Type: e.Data.Type}
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

// Return the first AccountID found when traversing a data structure
// (or nil if none).
func getAccountID(a xdr.XdrAggregate) (ret *stx.AccountID) {
	XdrExtract(a, &ret)
	return
}

type acctChecker struct {
	target string
	result bool
}

func (_ acctChecker) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

func (x *acctChecker) Marshal(name string, t xdr.XdrType) {
	if x.result {
		return
	}
	switch v := t.XdrPointer().(type) {
	case *stx.AccountID:
		if XdrToBin(v) == x.target {
			x.result = true
		}
	case *stx.Asset, *stx.TrustLineAsset:
	default:
		if ag, ok := t.(xdr.XdrAggregate); ok {
			ag.XdrRecurse(x, name)
		}
	}
}

// Returns true if structure contains AccountID in something other
// than an Asset issuer field.
func HasAccountID(a *stx.AccountID, t xdr.XdrType) bool {
	if a == nil {
		return false
	}
	x := acctChecker{target: XdrToBin(a)}
	t.XdrMarshal(&x, "")
	return x.result
}

func changeInfo(c *stx.LedgerEntryChange) (key stx.LedgerKey,
	entry *stx.LedgerEntry) {
	switch v := c.XdrUnionBody().(type) {
	case *stx.LedgerKey:
		return *v, nil
	case *stx.LedgerEntry:
		k := GetLedgerEntryKey(v)
		return k, v
	default:
		panic("ChangeInfo: invalid LedgerEntryChange")
	}
}

// Structure representing the value of a specific ledger entry before
// and after execution of a transaction.
type MetaDelta struct {
	Key      stx.LedgerKey
	Old, New *stx.LedgerEntry
}

// The account that owns the ledger entry.
func (md MetaDelta) AccountID() *stx.AccountID {
	return getAccountID(&md.Key)
}

// Extract the before/after ledger entry values from XDR structures
// containing LedgerEntryChanges.
func GetMetaDeltas(ms ...xdr.XdrType) (ret []MetaDelta) {
	kmap := make(map[string]int)
	for _, m := range ms {
		ForEachXdrType(m, func(c *stx.LedgerEntryChange) {
			k, e := changeInfo(c)
			kk := XdrToBin(&k)
			var md *MetaDelta
			first := false
			if i, ok := kmap[kk]; ok {
				md = &ret[i]
			} else {
				i = len(ret)
				first = true
				ret = append(ret, MetaDelta{Key: k})
				kmap[kk] = i
				md = &ret[i]
			}
			if c.Type == stx.LEDGER_ENTRY_STATE {
				if first {
					md.Old = e
				}
			} else {
				md.New = e
			}
		})
	}
	return
}
