package stcdetail

import "fmt"
import "math/rand"
import "testing"

func ExampleScaleFmt() {
	fmt.Println(ScaleFmt(987654321, 7))
	// Output:
	// 98.7654321e7
}

func TestJsonInt64e7Conv(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 10000; i++ {
		j := JsonInt64e7(r.Uint64())
		var k JsonInt64e7
		if text, err := j.MarshalText(); err != nil {
			t.Errorf("error marshaling JsonInt64e7 %d: %s", int64(j), err)
		} else if err = k.UnmarshalText(text); err != nil {
			t.Errorf("error unmarshaling JsonInt64e7 %d: %s", int64(j), err)
		} else if k != j {
			t.Errorf("JsonInt64e7 %d (%s) round-trip marshal returns %d",
				int64(j), text, int64(k))
		}
	}
	j := JsonInt64e7(0x7fffffffffffffff)
	var k JsonInt64e7
	if text, err := j.MarshalText(); err != nil {
		t.Errorf("error marshaling JsonInt64e7 %d: %s", int64(j), err)
	} else if err = k.UnmarshalText(text); err != nil {
		t.Errorf("error unmarshaling JsonInt64e7 %d: %s", int64(j), err)
	} else if k != j {
		t.Errorf("JsonInt64e7 %d (%s) round-trip marshal returns %d",
			int64(j), text, int64(k))
	}
}

func TestJsonInt64Conv(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 10000; i++ {
		j := JsonInt64(r.Uint64())
		var k JsonInt64
		if text, err := j.MarshalText(); err != nil {
			t.Errorf("error marshaling JsonInt64 %d: %s", int64(j), err)
		} else if err = k.UnmarshalText(text); err != nil {
			t.Errorf("error unmarshaling JsonInt64 %d: %s", int64(j), err)
		} else if k != j {
			t.Errorf("JsonInt64 %d (%s) round-trip marshal returns %d",
				int64(j), text, int64(k))
		}
	}
}
