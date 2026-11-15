package step

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"plainsolid/geom"
)

type mappedChainStepSpline struct {
	degree  int
	poles   [][]*big.Rat
	weights []*big.Rat
	knots   []*big.Rat
}

func mappedChainStepRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }

func mappedChainStepDecimal(q *big.Rat) (string, error) {
	out, err := mappedTerminatingDecimal(q)
	if err != nil {
		return "", geom.ErrUnrepresentable
	}
	if !strings.Contains(out, ".") {
		out += "."
	}
	return out, nil
}

func mappedChainStepDecimals(q []*big.Rat) (string, error) {
	out := make([]string, len(q))
	for i, v := range q {
		var err error
		out[i], err = mappedChainStepDecimal(v)
		if err != nil {
			return "", err
		}
	}
	return "(" + strings.Join(out, ",") + ")", nil
}

func mappedChainStepFloats(in []float64) []*big.Rat {
	out := make([]*big.Rat, len(in))
	for i, v := range in {
		out[i] = mappedChainStepRat(v)
	}
	return out
}

func mappedChainStepKnots(knots []*big.Rat) (string, string, error) {
	var stations []*big.Rat
	var mult []string
	for i := 0; i < len(knots); {
		j := i + 1
		for j < len(knots) && knots[i].Cmp(knots[j]) == 0 {
			j++
		}
		stations = append(stations, knots[i])
		mult = append(mult, strconv.Itoa(j-i))
		i = j
	}
	r, err := mappedChainStepDecimals(stations)
	return "(" + strings.Join(mult, ",") + ")", r, err
}

func mappedChainStepSplineFromSource(c *geom.BSpline) mappedChainStepSpline {
	s := mappedChainStepSpline{degree: c.Degree(), weights: mappedChainStepFloats(c.Weights()), knots: mappedChainStepFloats(c.Knots())}
	for _, p := range c.Poles() {
		s.poles = append(s.poles, mappedChainStepFloats([]float64{p.X, p.Y}))
	}
	return s
}

func mappedChainStepSplineFromBezier(h [][4]*big.Rat, dim int, start, end *big.Rat) mappedChainStepSpline {
	s := mappedChainStepSpline{degree: len(h) - 1}
	for _, row := range h {
		p := make([]*big.Rat, dim)
		for i := range p {
			p[i] = new(big.Rat).Quo(row[i], row[3])
		}
		s.poles = append(s.poles, p)
		s.weights = append(s.weights, row[3])
		s.knots = append(s.knots, start)
	}
	for range h {
		s.knots = append(s.knots, end)
	}
	// A derived homogeneous model or UV net can choose its common scale.
	// Remove non-decimal global weights without changing any pole or curve.
	equal := true
	for _, w := range s.weights[1:] {
		equal = equal && w.Cmp(s.weights[0]) == 0
	}
	if equal {
		for i := range s.weights {
			s.weights[i] = big.NewRat(1, 1)
		}
	} else {
		denominator := big.NewInt(1)
		for _, w := range s.weights {
			d := w.Denom()
			gcd := new(big.Int).GCD(nil, nil, denominator, d)
			denominator.Mul(denominator.Quo(denominator, gcd), d)
		}
		scale := new(big.Rat).SetInt(denominator)
		for i, w := range s.weights {
			s.weights[i] = new(big.Rat).Mul(w, scale)
		}
	}
	return s
}

func (s mappedChainStepSpline) emit(emit func(string) int64) (int64, error) {
	ms, ks, err := mappedChainStepKnots(s.knots)
	if err != nil {
		return 0, err
	}
	ws, err := mappedChainStepDecimals(s.weights)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, len(s.poles))
	for i, p := range s.poles {
		v, err := mappedChainStepDecimals(p)
		if err != nil {
			return 0, err
		}
		ids[i] = emit("CARTESIAN_POINT(''," + v + ")")
	}
	if mappedChainUnitWeights(s.weights) {
		return emit(fmt.Sprintf("B_SPLINE_CURVE_WITH_KNOTS('',%d,%s,.UNSPECIFIED.,.F.,.F.,%s,%s,.UNSPECIFIED.)",
			s.degree, references(ids), ms, ks)), nil
	}
	return emit(fmt.Sprintf("(BOUNDED_CURVE() B_SPLINE_CURVE(%d,%s,.UNSPECIFIED.,.F.,.F.) B_SPLINE_CURVE_WITH_KNOTS(%s,%s,.UNSPECIFIED.) CURVE() GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_CURVE(%s) REPRESENTATION_ITEM(''))",
		s.degree, references(ids), ms, ks, ws)), nil
}

func mappedChainUnitWeights(w []*big.Rat) bool {
	for _, v := range w {
		if !v.IsInt() || v.Num().Cmp(big.NewInt(1)) != 0 {
			return false
		}
	}
	return true
}

func emitMappedChainStepSurface(emit func(string) int64, s *geom.BSplineSurface) (int64, error) {
	d := s.Data()
	mu, ku, err := mappedChainStepKnots(mappedChainStepFloats(d.KnotsU))
	if err != nil {
		return 0, err
	}
	mv, kv, err := mappedChainStepKnots(mappedChainStepFloats(d.KnotsV))
	if err != nil {
		return 0, err
	}
	rows := make([]string, d.CountU)
	wrows := make([]string, d.CountU)
	w := mappedChainStepFloats(d.Weights)
	for i := range rows {
		ids := make([]int64, d.CountV)
		for j := range ids {
			p := d.Poles[i*d.CountV+j]
			data, err := mappedChainStepDecimals(mappedChainStepFloats([]float64{p.X, p.Y, p.Z}))
			if err != nil {
				return 0, err
			}
			ids[j] = emit("CARTESIAN_POINT(''," + data + ")")
		}
		rows[i] = references(ids)
		wrows[i], err = mappedChainStepDecimals(w[i*d.CountV : (i+1)*d.CountV])
		if err != nil {
			return 0, err
		}
	}
	if mappedChainUnitWeights(w) {
		return emit(fmt.Sprintf("B_SPLINE_SURFACE_WITH_KNOTS('',%d,%d,(%s),.UNSPECIFIED.,.F.,.F.,.F.,%s,%s,%s,%s,.UNSPECIFIED.)",
			d.DegreeU, d.DegreeV, strings.Join(rows, ","), mu, mv, ku, kv)), nil
	}
	return emit(fmt.Sprintf("(BOUNDED_SURFACE() B_SPLINE_SURFACE(%d,%d,(%s),.UNSPECIFIED.,.F.,.F.,.F.) B_SPLINE_SURFACE_WITH_KNOTS(%s,%s,%s,%s,.UNSPECIFIED.) GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_SURFACE((%s)) REPRESENTATION_ITEM('') SURFACE())",
		d.DegreeU, d.DegreeV, strings.Join(rows, ","), mu, mv, ku, kv, strings.Join(wrows, ","))), nil
}

func mappedChainStepSplineParts(f *File, id int64) ([]Value, []Value, []Value, error) {
	e := f.Entities[id]
	if len(e.Records) == 1 {
		r, err := simple(f, id, "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if err != nil || r.Args[0].Kind != String {
			return nil, nil, nil, geom.ErrInvalid
		}
		return r.Args[1:6], r.Args[6:], nil, nil
	}
	parts := map[string]int{"BOUNDED_CURVE": 0, "B_SPLINE_CURVE": 5, "B_SPLINE_CURVE_WITH_KNOTS": 3, "CURVE": 0, "GEOMETRIC_REPRESENTATION_ITEM": 0, "RATIONAL_B_SPLINE_CURVE": 1, "REPRESENTATION_ITEM": 1}
	if len(e.Records) != len(parts) {
		return nil, nil, nil, geom.ErrInvalid
	}
	var base, knot, weight []Value
	for _, r := range e.Records {
		length, ok := parts[r.Type]
		if !ok || len(r.Args) != length {
			return nil, nil, nil, geom.ErrInvalid
		}
		delete(parts, r.Type)
		switch r.Type {
		case "B_SPLINE_CURVE":
			base = r.Args
		case "B_SPLINE_CURVE_WITH_KNOTS":
			knot = r.Args
		case "RATIONAL_B_SPLINE_CURVE":
			if r.Args[0].Kind != Aggregate {
				return nil, nil, nil, geom.ErrInvalid
			}
			weight = r.Args[0].Items
		case "REPRESENTATION_ITEM":
			if r.Args[0].Kind != String {
				return nil, nil, nil, geom.ErrInvalid
			}
		}
	}
	if len(weight) == 0 {
		return nil, nil, nil, geom.ErrInvalid
	}
	return base, knot, weight, nil
}

func readMappedChainStepSpline(f *File, id int64, dim, maxDegree, maxPoles int, foreign bool) (mappedChainStepSpline, error) {
	var zero mappedChainStepSpline
	base, knot, weight, err := mappedChainStepSplineParts(f, id)
	if err != nil {
		return zero, err
	}
	d, err := integer(base[0])
	if err != nil || d < 1 || d > maxDegree || base[2].Kind != Enumeration || base[2].Text != "UNSPECIFIED" ||
		knot[2].Kind != Enumeration || (knot[2].Text != "UNSPECIFIED" && (!foreign || knot[2].Text != "PIECEWISE_BEZIER_KNOTS")) {
		return zero, geom.ErrInvalid
	}
	for _, v := range base[3:] {
		if x, err := boolean(v); err != nil || x {
			return zero, geom.ErrInvalid
		}
	}
	poles, err := listRefs(base[1], d+1)
	if err != nil || len(poles) > maxPoles || (weight != nil && len(weight) != len(poles)) {
		return zero, geom.ErrInvalid
	}
	s := mappedChainStepSpline{degree: d}
	for i, id := range poles {
		point, err := mappedChainStepPoint(f, id, dim)
		if err != nil {
			return zero, err
		}
		w := big.NewRat(1, 1)
		if weight != nil {
			w, err = affineSplineSourceRat(weight[i])
			if err != nil || w.Sign() <= 0 {
				return zero, geom.ErrInvalid
			}
		}
		s.poles = append(s.poles, point)
		s.weights = append(s.weights, w)
	}
	if knot[0].Kind != Aggregate || knot[1].Kind != Aggregate || len(knot[0].Items) != len(knot[1].Items) ||
		len(knot[0].Items) < 2 || len(knot[0].Items) > len(poles)+1 {
		return zero, geom.ErrInvalid
	}
	for i, k := range knot[1].Items {
		count, err := integer(knot[0].Items[i])
		end := i == 0 || i == len(knot[1].Items)-1
		if err != nil || count < 1 || end && count != d+1 || !end && count > d {
			return zero, geom.ErrInvalid
		}
		q, err := affineSplineSourceRat(k)
		if err != nil || (len(s.knots) > 0 && q.Cmp(s.knots[len(s.knots)-1]) <= 0) {
			return zero, geom.ErrInvalid
		}
		for range count {
			s.knots = append(s.knots, q)
		}
	}
	if len(s.knots) != len(poles)+d+1 {
		return zero, geom.ErrInvalid
	}
	return s, nil
}

func mappedChainStepPoint(f *File, id int64, dim int) ([]*big.Rat, error) {
	p, err := simple(f, id, "CARTESIAN_POINT", 2)
	if err != nil || p.Args[0].Kind != String || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != dim {
		return nil, geom.ErrInvalid
	}
	q := make([]*big.Rat, dim)
	for i, v := range p.Args[1].Items {
		q[i], err = affineSplineSourceRat(v)
		if err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (s mappedChainStepSpline) bezierControls() ([][4]*big.Rat, error) {
	if len(s.poles) != s.degree+1 {
		return nil, geom.ErrConvergence
	}
	return s.homogeneousControls(), nil
}

func (s mappedChainStepSpline) homogeneousControls() [][4]*big.Rat {
	h := make([][4]*big.Rat, len(s.poles))
	for i, p := range s.poles {
		h[i] = [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), s.weights[i]}
		for j, q := range p {
			h[i][j].Mul(q, s.weights[i])
		}
	}
	return h
}

// Degree multiplicity at every interior knot gives exact Bézier nets
// meeting at one original shared pole. Keep every original knot ratio.
func (s mappedChainStepSpline) exactBezierPieces(ctx context.Context, o geom.SurfacePatchOptions, maxPieces int) ([]*geom.ExactSplineCurve, []*big.Rat, error) {
	d := s.degree
	n := (len(s.poles) - 1) / d
	if n < 2 || n > maxPieces || len(s.poles) != n*d+1 {
		return nil, nil, geom.ErrConvergence
	}
	lo, hi := s.knots[0], s.knots[len(s.knots)-1]
	width := new(big.Rat).Sub(hi, lo)
	joins := make([]*big.Rat, n-1)
	last := lo
	for i := range joins {
		start := (i+1)*d + 1
		mid := s.knots[start]
		if mid.Cmp(last) <= 0 || mid.Cmp(hi) >= 0 || s.knots[start+d-1].Cmp(mid) != 0 {
			return nil, nil, fmt.Errorf("invalid model spline junction multiplicity or order: %w", geom.ErrConvergence)
		}
		joins[i] = new(big.Rat).Quo(new(big.Rat).Sub(mid, lo), width)
		last = mid
	}
	pieces := make([]*geom.ExactSplineCurve, n)
	h := s.homogeneousControls()
	for i := range pieces {
		var err error
		pieces[i], err = geom.NewExactHomogeneousBezier(ctx, h[i*d:i*d+d+1], o)
		if err != nil {
			return nil, nil, err
		}
	}
	return pieces, joins, nil
}

func (s mappedChainStepSpline) exactBezier(ctx context.Context, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	h, err := s.bezierControls()
	if err != nil {
		return nil, err
	}
	// The exact source station interval is recorded separately. The control
	// net is the exact restriction in its local normalized parameter.
	return geom.NewExactHomogeneousBezier(ctx, h, o)
}

func (s mappedChainStepSpline) ownerBasis() (*geom.BSpline, error) {
	knots, err := mappedChainRepresentedScalars(s.knots)
	if err != nil {
		return nil, err
	}
	w, err := mappedChainRepresentedScalars(s.weights)
	if err != nil {
		return nil, err
	}
	poles := make([]geom.Vec3, len(s.poles))
	for i, p := range s.poles {
		q, err := mappedChainRepresentedScalars(p)
		if err != nil {
			return nil, err
		}
		poles[i] = geom.Vec3{X: q[0], Y: q[1]}
	}
	return geom.NewBSpline(s.degree, poles, knots, w)
}

func mappedChainRepresentedScalars(input []*big.Rat) ([]float64, error) {
	out := make([]float64, len(input))
	for i, q := range input {
		x, err := geom.RepresentedExactScalar(q)
		if err != nil {
			return nil, err
		}
		out[i] = x
	}
	return out, nil
}
