package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
)

// SphereLiftAgreement is continuous approximate model/pcurve evidence, not
// acceptance of a face or material. A false tolerance flag is inconclusive.
type SphereLiftAgreement struct {
	Source                               CylinderPCurveEvidence
	SphereID, PCurveID, ParameterCurveID int64
	CoordinateBounds                     [3]*big.Rat
	DistanceSquaredUpper, Tolerance      *big.Rat
	WithinTolerance                      bool
	Spans                                int
}

// BoundSpherePCurveLift supports the sphere owner of the selected two-owner
// sphere/cylinder edge graph. Exact degree/knot agreement, explicit radian units
// and signed-coordinate primitive frames are required. Degree-six Taylor
// lifts and exact Bernstein products enclose the entire same-parameter error.
// Latitude control hulls must remain strictly inside (-pi/2,pi/2).
// Input budgets match the residual codec; product coefficients permit32768bits.
// Tolerance and output distances are in unconverted model units.
func BoundSpherePCurveLift(r io.Reader, l Limits, edgeID int64, tolerance float64) (out SphereLiftAgreement, err error) {
	fail := func() (SphereLiftAgreement, error) {
		return SphereLiftAgreement{}, fmt.Errorf("step: sphere lift unsupported or proof budget")
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
	sc, e := simple(f, source.SurfaceCurveID, "SURFACE_CURVE", 4)
	if e != nil {
		return fail()
	}
	pcs, e := listRefs(sc.Args[2], 2)
	if e != nil {
		return fail()
	}
	var sphere, pcID, param int64
	for _, id := range pcs {
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			return fail()
		}
		sid, e := ref(pc.Args[1])
		if e != nil {
			return fail()
		}
		if _, e = simple(f, sid, "SPHERICAL_SURFACE", 3); e != nil {
			continue
		}
		if sphere != 0 {
			return fail()
		}
		sphere, pcID = sid, id
		rid, e := ref(pc.Args[2])
		if e != nil {
			return fail()
		}
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ids, e := listRefs(rep.Args[1], 1)
		if e != nil || len(ids) != 1 {
			return fail()
		}
		param = ids[0]
		cid, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		ctx, ok := f.Entities[cid]
		if !ok {
			return fail()
		}
		dim, ok := component(ctx, "GEOMETRIC_REPRESENTATION_CONTEXT")
		if !ok || len(dim.Args) != 1 {
			return fail()
		}
		n, e := numeric(dim.Args[0])
		if e != nil || n != 2 {
			return fail()
		}
	}
	if sphere == 0 {
		return fail()
	}
	support, e := simple(f, sphere, "SPHERICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	radius, e := liftSourceRat(support.Args[2])
	if e != nil || radius.Sign() <= 0 {
		return fail()
	}
	pid, e := ref(support.Args[1])
	if e != nil {
		return fail()
	}
	origin, axes, e := sphereLiftFrame(f, pid)
	if e != nil {
		return fail()
	}
	p, model, knots, e := readResidualPolynomial(bytes.NewReader(raw), l, source.ModelCurveID)
	if e != nil {
		return fail()
	}
	q, uv, uvKnots, e := readResidualPolynomialDimension(bytes.NewReader(raw), l, param, 2)
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
	out.SphereID, out.PCurveID, out.ParameterCurveID = sphere, pcID, param
	out.Tolerance = new(big.Rat).SetFloat64(tolerance)
	for a := 0; a < 3; a++ {
		out.CoordinateBounds[a] = new(big.Rat)
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
		piLo, _ := pcurvePiBounds()
		latitudeLimit := new(big.Rat).Quo(piLo, big.NewRat(2, 1))
		for _, v := range pc[1] {
			if new(big.Rat).Abs(v).Cmp(latitudeLimit) >= 0 {
				return fail()
			}
		}
		su, cu, eu, e := liftTrigBernsteinOrder(pc[0], 6)
		if e != nil {
			return fail()
		}
		sv, cv, ev, e := liftTrigBernsteinOrder(pc[1], 6)
		if e != nil {
			return fail()
		}
		target := [3][]*big.Rat{liftProduct(cu, cv), liftProduct(su, cv), sv}
		// |ab-AB| <= eA+eB+eA*eB when |a|,|b|<=1 and A/B approximate them.
		productError := new(big.Rat).Add(new(big.Rat).Add(eu, ev), new(big.Rat).Mul(eu, ev))
		errors := [3]*big.Rat{productError, productError, ev}
		for a := 0; a < 3; a++ {
			local := make([]*big.Rat, p+1)
			for i := range local {
				local[i] = new(big.Rat)
				for c := 0; c < 3; c++ {
					local[i].Add(local[i], new(big.Rat).Mul(axes[a][c], new(big.Rat).Sub(mc[c][i], origin[c])))
				}
			}
			actual := liftElevate(local, len(target[a])-1)
			bound := new(big.Rat)
			for i, v := range target[a] {
				d := new(big.Rat).Abs(new(big.Rat).Sub(actual[i], new(big.Rat).Mul(radius, v)))
				if !sphereLiftBits(d) {
					return fail()
				}
				if d.Cmp(bound) > 0 {
					bound = d
				}
			}
			bound.Add(bound, new(big.Rat).Mul(radius, errors[a]))
			if !sphereLiftBits(bound) {
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
	if !sphereLiftBits(out.DistanceSquaredUpper) {
		return fail()
	}
	out.WithinTolerance = out.DistanceSquaredUpper.Cmp(new(big.Rat).Mul(out.Tolerance, out.Tolerance)) <= 0
	return out, nil
}
func sphereLiftBits(q *big.Rat) bool { return q.Num().BitLen() <= 32768 && q.Denom().BitLen() <= 32768 }
func sphereLiftFrame(f *File, pid int64) (origin [3]*big.Rat, axes [3][3]*big.Rat, err error) {
	rec, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return origin, axes, e
	}
	vector := func(v Value, kind string) ([3]*big.Rat, error) {
		var a [3]*big.Rat
		id, e := ref(v)
		if e != nil {
			return a, e
		}
		r, e := simple(f, id, kind, 2)
		if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
			return a, geom.ErrDomain
		}
		for i, v := range r.Args[1].Items {
			a[i], e = liftSourceRat(v)
			if e != nil {
				return a, e
			}
		}
		return a, nil
	}
	origin, e = vector(rec.Args[1], "CARTESIAN_POINT")
	if e != nil {
		return origin, axes, e
	}
	axes[2], e = vector(rec.Args[2], "DIRECTION")
	if e != nil {
		return origin, axes, e
	}
	axes[0], e = vector(rec.Args[3], "DIRECTION")
	if e != nil {
		return origin, axes, e
	}
	index := func(a [3]*big.Rat) int {
		at := -1
		for i, q := range a {
			if q.Sign() == 0 {
				continue
			}
			if at >= 0 || new(big.Rat).Abs(q).Cmp(big.NewRat(1, 1)) != 0 {
				return -1
			}
			at = i
		}
		return at
	}
	if index(axes[0]) < 0 || index(axes[2]) < 0 || index(axes[0]) == index(axes[2]) {
		return origin, axes, geom.ErrDomain
	}
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		axes[1][i] = new(big.Rat).Sub(new(big.Rat).Mul(axes[2][j], axes[0][k]), new(big.Rat).Mul(axes[2][k], axes[0][j]))
	}
	return origin, axes, nil
}
