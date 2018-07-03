package main

type XDR interface {
	b(*bool, string)
	i32(*int32, string)
	u32(*uint32, string)
	i64(*int64, string)
	u64(*uint64, string)
	f32(*float32, string)
	f64(*float64, string)
}
