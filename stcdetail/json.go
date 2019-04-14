package stcdetail

import(
	"encoding/json"
	"fmt"
)

// Convert a json.Number to an int64 by scanning the textual
// representation of the number.  (The simpler approach of going
// through a double risks losing precision.)
func JsonNumberToI64(n json.Number) (int64, error) {
	val := int64(-1)
	if _, err := fmt.Sscan(n.String(), &val); err != nil {
		return -1, err
	}
	return val, nil
}
