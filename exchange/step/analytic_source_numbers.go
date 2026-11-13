package step

import (
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"strconv"
	"strings"
)

func cylinderSupportUnit(v geom.Vec3) bool {
	n := 0
	for _, x := range []float64{v.X, v.Y, v.Z} {
		if x != 0 {
			if x != 1 && x != -1 {
				return false
			}
			n++
		}
	}
	return n == 1
}

func cylinderSupportNumber(v Value) (float64, error) {
	r, e := liftSourceRat(v)
	if e != nil {
		return 0, e
	}
	x, exact := r.Float64()
	if !exact || !finiteNumber(x) {
		return 0, errors.New("step: cylinder support scalar would round")
	}
	return x, nil
}

func liftSourceRat(v Value) (*big.Rat, error) { // bounded exact source decimal
	if v.Kind != Number || len(v.Text) > 64 {
		return nil, geom.ErrDomain
	}
	// ParseFloat bounds the exponent before big.Rat can allocate a huge integer.
	if i := strings.IndexAny(v.Text, "eE"); i >= 0 {
		ex, e := strconv.Atoi(v.Text[i+1:])
		if e != nil || ex < -256 || ex > 256 {
			return nil, geom.ErrDomain
		}
	}
	f, e := numeric(v)
	if e != nil || !finiteNumber(f) || math.Abs(f) > 1e100 {
		return nil, geom.ErrDomain
	}
	q, ok := new(big.Rat).SetString(v.Text)
	if !ok || q.Num().BitLen() > 1024 || q.Denom().BitLen() > 1024 {
		return nil, geom.ErrDomain
	}
	return q, nil
}
