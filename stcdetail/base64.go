package stcdetail

import (
	"encoding/base64"
	"github.com/xdrpp/goxdr/xdr"
	"strings"
)

// Convert an XDR aggregate to base64-encoded binary format.  Calls
// panic() with an XdrError if any field contains illegal values
// (e.g., if a slice exceeds its bounds or a union discriminant has an
// invalid value).
func XdrToBase64(es ...xdr.XdrType) string {
	out := &strings.Builder{}
	b64o := base64.NewEncoder(base64.StdEncoding, out)
	for i := range es {
		es[i].XdrMarshal(&xdr.XdrOut{b64o}, "")
	}
	b64o.Close()
	return out.String()
}

// Parse base64-encoded binary XDR into an XDR aggregate structure.
func XdrFromBase64(e xdr.XdrType, input string) (err error) {
	defer func() {
		if i := recover(); i != nil {
			var ok bool
			if err, ok = i.(error); !ok {
				panic(i)
			}
			return
		}
	}()
	in := strings.NewReader(input)
	b64i := base64.NewDecoder(base64.StdEncoding, in)
	e.XdrMarshal(&xdr.XdrIn{b64i}, "")
	return nil
}
