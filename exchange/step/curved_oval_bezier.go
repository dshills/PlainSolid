package step

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"strings"
)

func decodeCurvedOvalBezier(f *File, id int64, two bool, o CurvedOvalOptions) (*geom.BSpline, *geom.ExactSplineCurve, error) {
	var c *geom.BSpline
	var e error
	if two {
		c, e = decodeOvalUV(f, id, 17)
	} else {
		var q SplineCurve
		q, e = DecodeBSplineCurve(f, id, 17)
		c = q.Geometry
		if e == nil && (q.Closed == "T" || q.SelfIntersect == "T") {
			return nil, nil, geom.ErrInvalid
		}
	}
	if e != nil {
		return nil, nil, e
	}
	base, ok := component(f.Entities[id], "B_SPLINE_CURVE")
	if !ok {
		base, ok = component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS")
		if !ok || len(base.Args) < 6 {
			return nil, nil, geom.ErrInvalid
		}
		base.Args = base.Args[1:]
	}
	if len(base.Args) < 5 || base.Args[3].Text == "T" || base.Args[4].Text == "T" {
		return nil, nil, geom.ErrInvalid
	}
	n := len(c.Poles())
	if n != c.Degree()+1 || n < 2 || n > 17 {
		return nil, nil, geom.ErrConvergence
	}
	knots := c.Knots()
	if len(knots) != 2*n {
		return nil, nil, geom.ErrInvalid
	}
	for i, k := range knots {
		want := 0.
		if i >= n {
			want = 1
		}
		if k != want {
			return nil, nil, geom.ErrDomain
		}
	}
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = 1
	}
	if wr, ok := component(f.Entities[id], "RATIONAL_B_SPLINE_CURVE"); ok {
		if len(wr.Args) != 1 || wr.Args[0].Kind != Aggregate || len(wr.Args[0].Items) != n {
			return nil, nil, geom.ErrInvalid
		}
		for i, v := range wr.Args[0].Items {
			weights[i], e = numeric(v)
			if e != nil || weights[i] <= 0 {
				return nil, nil, geom.ErrInvalid
			}
		}
	}
	controls := make([][4]*big.Rat, n)
	for i, p := range c.Poles() {
		w := new(big.Rat).SetFloat64(weights[i])
		controls[i] = [4]*big.Rat{new(big.Rat).Mul(new(big.Rat).SetFloat64(p.X), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Y), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Z), w), w}
	}
	exact, e := geom.NewExactHomogeneousBezier(context.Background(), controls, o.patch())
	return c, exact, e
}
func emitCurvedOvalBezier(emit func(string) int64, poles []geom.Vec3, weights []float64, two bool) int64 {
	ids := []int64{}
	ws := []string{}
	for i, p := range poles {
		values := real(p.X) + "," + real(p.Y)
		if !two {
			values += "," + real(p.Z)
		}
		ids = append(ids, emit("CARTESIAN_POINT('',("+values+"))"))
		ws = append(ws, real(weights[i]))
	}
	n := len(poles)
	return emit(fmt.Sprintf("(BOUNDED_CURVE() B_SPLINE_CURVE(%d,%s,.UNSPECIFIED.,.F.,.F.) B_SPLINE_CURVE_WITH_KNOTS((%d,%d),(0.,1.),.UNSPECIFIED.) CURVE() GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_CURVE((%s)) REPRESENTATION_ITEM(''))", n-1, references(ids), n, n, strings.Join(ws, ",")))
}
