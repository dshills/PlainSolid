package step

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// CylinderLiftAgreement bounds same-parameter model minus lifted pcurve in the
// cylinder's orthonormal frame. Success of WithinTolerance is approximate
// agreement only, never exact equality or a material/face acceptance decision.
// A false flag is failure to certify the tolerance, not proof of disagreement.
type CylinderLiftAgreement struct {
	Source                          CylinderPCurveEvidence
	CoordinateBounds                [3]*big.Rat
	DistanceSquaredUpper, Tolerance *big.Rat
	WithinTolerance                 bool
	Spans                           int
}

// BoundCylinderPCurveLift supports equal-degree polynomial model/pcurve pairs
// with identical exact knot sequences and signed-coordinate cylinder frames.
// It uses degree-10 Taylor lifts about dyadic centers with exact Bernstein
// coefficients, rational sine/cosine center enclosures and remainder bounds.
// A span's U controls must lie within one radian of its expansion center.
// Source, coefficient and span budgets are inherited from the residual codec.
// Tolerance and returned coordinate bounds are in unconverted model units.
func BoundCylinderPCurveLift(r io.Reader, l Limits, edgeID int64, tolerance float64) (out CylinderLiftAgreement, err error) {
	fail := func() (CylinderLiftAgreement, error) {
		return CylinderLiftAgreement{}, fmt.Errorf("step: cylinder lift unsupported or proof budget")
	}
	if r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	source, e := InspectCylinderPCurve(bytes.NewReader(raw), l, edgeID)
	if e != nil {
		return out, e
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	if !liftRadianContext(f, source.BodyID) {
		return fail()
	}
	cyl, e := simple(f, source.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	radius, e := liftSourceRat(cyl.Args[2])
	if e != nil || radius.Sign() <= 0 {
		return fail()
	}
	pid, e := ref(cyl.Args[1])
	if e != nil {
		return fail()
	}
	placement, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return fail()
	}
	vector := func(v Value, kind string) ([3]*big.Rat, error) {
		var a [3]*big.Rat
		id, e := ref(v)
		if e != nil {
			return a, e
		}
		rec, e := simple(f, id, kind, 2)
		if e != nil || rec.Args[1].Kind != Aggregate || len(rec.Args[1].Items) != 3 {
			return a, geom.ErrDomain
		}
		for i, v := range rec.Args[1].Items {
			a[i], e = liftSourceRat(v)
			if e != nil {
				return a, e
			}
		}
		return a, nil
	}
	origin, e := vector(placement.Args[1], "CARTESIAN_POINT")
	if e != nil {
		return fail()
	}
	z, e := vector(placement.Args[2], "DIRECTION")
	if e != nil {
		return fail()
	}
	x, e := vector(placement.Args[3], "DIRECTION")
	if e != nil {
		return fail()
	}
	axisIndex := func(a [3]*big.Rat) int {
		index := -1
		for i, q := range a {
			if q.Sign() == 0 {
				continue
			}
			if index >= 0 || new(big.Rat).Abs(q).Cmp(big.NewRat(1, 1)) != 0 {
				return -1
			}
			index = i
		}
		return index
	}
	if axisIndex(x) < 0 || axisIndex(z) < 0 || axisIndex(x) == axisIndex(z) {
		return fail()
	}
	var y [3]*big.Rat
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		y[i] = new(big.Rat).Sub(new(big.Rat).Mul(z[j], x[k]), new(big.Rat).Mul(z[k], x[j]))
	}
	axes := [3][3]*big.Rat{x, y, z}
	p, model, knots, e := readResidualPolynomial(bytes.NewReader(raw), l, source.ModelCurveID)
	if e != nil {
		return fail()
	}
	q, uv, uvKnots, e := readResidualPolynomialDimension(bytes.NewReader(raw), l, source.ParameterCurveID, 2)
	if e != nil || p != q || len(knots) != len(uvKnots) {
		return fail()
	}
	for i, k := range knots {
		if k.Cmp(uvKnots[i]) != 0 {
			return fail()
		}
	}
	inverse, e := residualBernsteinInverse(p)
	if e != nil {
		return fail()
	}
	out.Source = source
	out.Tolerance = new(big.Rat).SetFloat64(tolerance)
	for i := 0; i < 3; i++ {
		out.CoordinateBounds[i] = new(big.Rat)
	}
	for k := p; k < len(model); k++ {
		if knots[k].Cmp(knots[k+1]) == 0 {
			continue
		}
		mc, e := liftSpanControls(p, model, knots, k, inverse)
		if e != nil {
			return fail()
		}
		pc, e := liftSpanControls(p, uv, knots, k, inverse)
		if e != nil {
			return fail()
		}
		local := [3][]*big.Rat{}
		for a := 0; a < 3; a++ {
			local[a] = make([]*big.Rat, p+1)
			for i := 0; i <= p; i++ {
				v := new(big.Rat)
				for c := 0; c < 3; c++ {
					v.Add(v, new(big.Rat).Mul(axes[a][c], new(big.Rat).Sub(mc[c][i], origin[c])))
				}
				local[a][i] = v
			}
		}
		sine, cosine, remainder, e := liftTrigBernstein(pc[0])
		if e != nil {
			return fail()
		}
		targets := [3][]*big.Rat{cosine, sine, pc[1]}
		for a := 0; a < 3; a++ {
			target := targets[a]
			actual := liftElevate(local[a], len(target)-1)
			bound := new(big.Rat)
			for i, v := range target {
				want := new(big.Rat).Set(v)
				if a < 2 {
					want.Mul(want, radius)
				}
				d := new(big.Rat).Abs(new(big.Rat).Sub(actual[i], want))
				if !residualBits(d) {
					return fail()
				}
				if d.Cmp(bound) > 0 {
					bound = d
				}
			}
			if a < 2 {
				bound.Add(bound, new(big.Rat).Mul(radius, remainder))
			}
			if !residualBits(bound) {
				return fail()
			}
			if bound.Cmp(out.CoordinateBounds[a]) > 0 {
				out.CoordinateBounds[a] = bound
			}
		}
		out.Spans++
	}
	out.DistanceSquaredUpper = new(big.Rat)
	for _, v := range out.CoordinateBounds {
		out.DistanceSquaredUpper.Add(out.DistanceSquaredUpper, new(big.Rat).Mul(v, v))
	}
	if !residualBits(out.DistanceSquaredUpper) {
		return fail()
	}
	out.WithinTolerance = out.DistanceSquaredUpper.Cmp(new(big.Rat).Mul(out.Tolerance, out.Tolerance)) <= 0
	return out, nil
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
func liftSpanControls(p int, points [][3]*big.Rat, knots []*big.Rat, k int, inverse [][]*big.Rat) ([3][]*big.Rat, error) {
	var out [3][]*big.Rat
	samples := make([][3]*big.Rat, p+1)
	for j := 0; j <= p; j++ {
		t := new(big.Rat).Add(knots[k], new(big.Rat).Mul(new(big.Rat).Sub(knots[k+1], knots[k]), big.NewRat(int64(j), int64(p))))
		v, e := residualDeBoor(p, points, knots, k, t)
		if e != nil {
			return out, e
		}
		samples[j] = v
	}
	for a := 0; a < 3; a++ {
		out[a] = make([]*big.Rat, p+1)
		for i := 0; i <= p; i++ {
			v := new(big.Rat)
			for j := 0; j <= p; j++ {
				v.Add(v, new(big.Rat).Mul(inverse[i][j], samples[j][a]))
			}
			if !residualBits(v) {
				return out, geom.ErrDomain
			}
			out[a][i] = v
		}
	}
	return out, nil
}
func liftProduct(a, b []*big.Rat) []*big.Rat {
	m, n := len(a)-1, len(b)-1
	out := make([]*big.Rat, m+n+1)
	for k := range out {
		out[k] = new(big.Rat)
		den := new(big.Int).Binomial(int64(m+n), int64(k))
		for i := max(0, k-n); i <= min(m, k); i++ {
			j := k - i
			weight := new(big.Rat).SetFrac(new(big.Int).Mul(new(big.Int).Binomial(int64(m), int64(i)), new(big.Int).Binomial(int64(n), int64(j))), den)
			out[k].Add(out[k], new(big.Rat).Mul(new(big.Rat).Mul(a[i], b[j]), weight))
		}
	}
	return out
}
func liftElevate(a []*big.Rat, n int) []*big.Rat {
	if n == len(a)-1 {
		return a
	}
	b := make([]*big.Rat, n-len(a)+2)
	for i := range b {
		b[i] = big.NewRat(1, 1)
	}
	return liftProduct(a, b)
}
func liftTrigBernstein(u []*big.Rat) (sine, cosine []*big.Rat, remainder *big.Rat, err error) {
	return liftTrigBernsteinOrder(u, 10)
}

func liftTrigBernsteinOrder(u []*big.Rat, order int) (sine, cosine []*big.Rat, remainder *big.Rat, err error) {
	if len(u) < 2 || order < 1 || order > 10 {
		return nil, nil, nil, geom.ErrDomain
	}
	lo, hi := new(big.Rat).Set(u[0]), new(big.Rat).Set(u[0])
	for _, v := range u {
		if v.Cmp(lo) < 0 {
			lo.Set(v)
		}
		if v.Cmp(hi) > 0 {
			hi.Set(v)
		}
	}
	middle := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
	mf, _ := middle.Float64()
	if !finiteNumber(mf) || math.Abs(mf) > 16 {
		return nil, nil, nil, geom.ErrDomain
	}
	center := big.NewRat(int64(math.Round(mf*16)), 16)
	delta := make([]*big.Rat, len(u))
	bound := new(big.Rat)
	for i, v := range u {
		delta[i] = new(big.Rat).Sub(v, center)
		abs := new(big.Rat).Abs(delta[i])
		if abs.Cmp(bound) > 0 {
			bound = abs
		}
	}
	if bound.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, nil, nil, geom.ErrDomain
	}
	sc, cc := new(big.Rat), new(big.Rat)
	term := big.NewRat(1, 1)
	for k := 0; k <= 64; k++ {
		switch k % 4 {
		case 0:
			cc.Add(cc, term)
		case 1:
			sc.Add(sc, term)
		case 2:
			cc.Sub(cc, term)
		case 3:
			sc.Sub(sc, term)
		}
		term.Mul(term, center)
		term.Quo(term, big.NewRat(int64(k+1), 1))
	}
	centerError := new(big.Rat).Abs(term)
	degree := order * (len(u) - 1)
	sine, cosine = make([]*big.Rat, degree+1), make([]*big.Rat, degree+1)
	for i := range sine {
		sine[i] = new(big.Rat)
		cosine[i] = new(big.Rat)
	}
	power := []*big.Rat{big.NewRat(1, 1)}
	factorial := big.NewInt(1)
	boundTerm := big.NewRat(1, 1)
	errorSum := new(big.Rat)
	for k := 0; k <= order; k++ {
		sv, cv := new(big.Rat), new(big.Rat)
		switch k % 4 {
		case 0:
			sv.Set(sc)
			cv.Set(cc)
		case 1:
			sv.Set(cc)
			cv.Neg(sc)
		case 2:
			sv.Neg(sc)
			cv.Neg(cc)
		case 3:
			sv.Neg(cc)
			cv.Set(sc)
		}
		sv.Quo(sv, new(big.Rat).SetInt(factorial))
		cv.Quo(cv, new(big.Rat).SetInt(factorial))
		elevated := liftElevate(power, degree)
		for i, v := range elevated {
			sine[i].Add(sine[i], new(big.Rat).Mul(v, sv))
			cosine[i].Add(cosine[i], new(big.Rat).Mul(v, cv))
			if !residualBits(sine[i]) || !residualBits(cosine[i]) {
				return nil, nil, nil, geom.ErrDomain
			}
		}
		errorSum.Add(errorSum, new(big.Rat).Mul(centerError, boundTerm))
		boundTerm.Mul(boundTerm, bound)
		boundTerm.Quo(boundTerm, big.NewRat(int64(k+1), 1))
		if k < order {
			power = liftProduct(power, delta)
			for _, v := range power {
				if !residualBits(v) {
					return nil, nil, nil, geom.ErrDomain
				}
			}
			factorial.Mul(factorial, big.NewInt(int64(k+1)))
		}
	}
	remainder = new(big.Rat).Add(errorSum, boundTerm)
	return sine, cosine, remainder, nil
}

// Angle scaling is not inferred: the body must have one explicit radian
// representation context before raw U is used in trigonometric functions.
func liftRadianContext(f *File, body int64) bool {
	var contextID int64
	for _, en := range f.Entities {
		if len(en.Records) != 1 || en.Records[0].Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		r := en.Records[0]
		if len(r.Args) != 3 {
			return false
		}
		items, e := listRefs(r.Args[1], 1)
		if e != nil {
			return false
		}
		for _, id := range items {
			if id == body {
				if contextID != 0 {
					return false
				}
				contextID, e = ref(r.Args[2])
				if e != nil {
					return false
				}
			}
		}
	}
	if contextID == 0 {
		return false
	}
	if _, e := contextUnits(f, contextID); e != nil {
		return false
	}
	record, ok := component(f.Entities[contextID], "GLOBAL_UNIT_ASSIGNED_CONTEXT")
	if !ok || len(record.Args) != 1 {
		return false
	}
	ids, e := listRefs(record.Args[0], 1)
	if e != nil {
		return false
	}
	count := 0
	for _, id := range ids {
		en := f.Entities[id]
		if _, ok := component(en, "PLANE_ANGLE_UNIT"); !ok {
			continue
		}
		count++
		si, ok := component(en, "SI_UNIT")
		if !ok || len(si.Args) != 2 || si.Args[0].Kind != Omitted || si.Args[1].Kind != Enumeration || si.Args[1].Text != "RADIAN" {
			return false
		}
		if _, ok := component(en, "CONVERSION_BASED_UNIT"); ok {
			return false
		}
	}
	return count == 1
}
