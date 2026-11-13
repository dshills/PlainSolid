package step

import (
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strconv"
	"strings"
)

func readResidualPolynomial(r io.Reader, l Limits, curveID int64) (int, [][3]*big.Rat, []*big.Rat, error) {
	f, e := Parse(r, l)
	if e != nil {
		return 0, nil, nil, e
	}
	rec, e := simple(f, curveID, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if e != nil {
		return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
	}
	p64, e := numeric(rec.Args[1])
	p := int(p64)
	if e != nil || p64 != float64(p) || p < 1 || p > 8 {
		return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
	}
	ids, e := listRefs(rec.Args[2], p+1)
	if e != nil || len(ids) > 512 {
		return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
	}
	rat := func(v Value) (*big.Rat, error) {
		if v.Kind != Number || len(v.Text) > 64 {
			return nil, geom.ErrDomain
		}
		if i := strings.IndexAny(v.Text, "eE"); i >= 0 {
			exp, e := strconv.Atoi(v.Text[i+1:])
			if e != nil || exp < -256 || exp > 256 {
				return nil, geom.ErrDomain
			}
		}
		q, ok := new(big.Rat).SetString(v.Text)
		if !ok || q.Num().BitLen() > 1024 || q.Denom().BitLen() > 1024 {
			return nil, geom.ErrDomain
		}
		return q, nil
	}
	poles := make([][3]*big.Rat, len(ids))
	for i, id := range ids {
		point, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil || point.Args[1].Kind != Aggregate || len(point.Args[1].Items) != 3 {
			return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
		}
		for j, v := range point.Args[1].Items {
			poles[i][j], e = rat(v)
			if e != nil {
				return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
			}
		}
	}
	mult, distinct := rec.Args[6], rec.Args[7]
	if mult.Kind != Aggregate || distinct.Kind != Aggregate || len(mult.Items) != len(distinct.Items) || len(distinct.Items) < 2 || len(distinct.Items) > 65 {
		return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
	}
	knots := []*big.Rat{}
	for i, v := range distinct.Items {
		q, e := rat(v)
		if e != nil {
			return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
		}
		if i > 0 && q.Cmp(knots[len(knots)-1]) <= 0 {
			return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
		}
		n, e := numeric(mult.Items[i])
		m := int(n)
		if e != nil || n != float64(m) || m < 1 || m > p+1 || (i == 0 || i == len(distinct.Items)-1) && m != p+1 || i > 0 && i < len(distinct.Items)-1 && m > p {
			return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
		}
		for j := 0; j < m; j++ {
			knots = append(knots, new(big.Rat).Set(q))
		}
	}
	if len(knots) != len(poles)+p+1 {
		return 0, nil, nil, fmt.Errorf("step: unsupported polynomial spline input or budget")
	}

	return p, poles, knots, nil
}
