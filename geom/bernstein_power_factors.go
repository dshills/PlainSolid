package geom

// These source-independent exact integers cover the degree<=8 residual API.
// The largest entry is 560. Read-only after initialization, they replace only
// integer factor construction; rational accumulation and all proof budget
// checks keep the same order and values as the general big.Int formula.
var smallBernsteinPowerFactors = func() (out [9][9][9]int64) {
	choose := func(n, k int) int64 {
		value := int64(1)
		for i := 1; i <= k; i++ {
			value = value * int64(n-i+1) / int64(i)
		}
		return value
	}
	for n := 0; n < len(out); n++ {
		for k := 0; k <= n; k++ {
			for i := 0; i <= k; i++ {
				out[n][k][i] = choose(n, i) * choose(n-i, k-i)
			}
		}
	}
	return
}()
