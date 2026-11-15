package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// GroupedRationalCapEvidence describes full source spline domains, not selected
// EDGE_CURVE trims. RadialDistanceUpper bounds radial projection onto the model
// circle; it is not a same-parameter bound. WitnessDistanceSquaredLower proves
// a discrepancy at WitnessParameter against that circle's angular parameter.
type GroupedRationalCapEvidence struct {
	Owner                                         ConvexArcPCurveAgreement
	CircleID                                      int64
	Domain                                        [2]*big.Rat
	Knots                                         []*big.Rat
	PoleIDs                                       []int64
	Weights                                       []*big.Rat
	RadialDistanceUpper                           *big.Rat
	PositiveAngularVelocity, ClosedOneTurn        bool
	WitnessParameter, WitnessDistanceSquaredLower *big.Rat
}

// GroupedPCurveEvidence authenticates the model-only repaired topology and all
// original owner associations. It intentionally exposes no promoted solid:
// rational-cap finite trim/reparameterization remains unresolved.
type GroupedPCurveEvidence struct {
	Caps        []GroupedRationalCapEvidence
	OtherOwners []ConvexArcPCurveAgreement
	Reversals   []ConvexArcRepairReversal
	Repairs     []ConvexArcRepairSite
}

func groupedRatBounds(q []*big.Rat) (lo, hi *big.Rat) {
	for _, v := range q {
		if lo == nil || v.Cmp(lo) < 0 {
			lo = new(big.Rat).Set(v)
		}
		if hi == nil || v.Cmp(hi) > 0 {
			hi = new(big.Rat).Set(v)
		}
	}
	return
}
func groupedDerivative(q []*big.Rat) []*big.Rat {
	out := make([]*big.Rat, len(q)-1)
	for i := range out {
		out[i] = new(big.Rat).Mul(big.NewRat(int64(len(q)-1), 1), new(big.Rat).Sub(q[i+1], q[i]))
	}
	return out
}
func groupedEval(q []*big.Rat, t *big.Rat) *big.Rat {
	v := make([]*big.Rat, len(q))
	for i := range q {
		v[i] = new(big.Rat).Set(q[i])
	}
	one := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for n := len(v) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			v[i].Add(new(big.Rat).Mul(one, v[i]), new(big.Rat).Mul(t, v[i+1]))
		}
	}
	return v[0]
}

func groupedRationalCap(f *File, q ConvexArcPCurveAgreement, model int64) (out GroupedRationalCapEvidence, err error) {
	bad := func() (GroupedRationalCapEvidence, error) { return GroupedRationalCapEvidence{}, geom.ErrDomain }
	en := f.Entities[q.GeometryID]
	if len(en.Records) != 7 {
		return bad()
	}
	rec := map[string]Record{}
	for _, r := range en.Records {
		if _, ok := rec[r.Type]; ok {
			return bad()
		}
		rec[r.Type] = r
	}
	for _, name := range []string{"BOUNDED_CURVE", "CURVE", "GEOMETRIC_REPRESENTATION_ITEM"} {
		r, ok := rec[name]
		if !ok || len(r.Args) != 0 {
			return bad()
		}
	}
	ri, ok := rec["REPRESENTATION_ITEM"]
	if !ok || len(ri.Args) != 1 || ri.Args[0].Kind != String {
		return bad()
	}
	bs, ok := rec["B_SPLINE_CURVE"]
	if !ok || len(bs.Args) != 5 {
		return bad()
	}
	degree, e := integer(bs.Args[0])
	if e != nil || degree != 2 || bs.Args[2].Kind != Enumeration || bs.Args[2].Text != "UNSPECIFIED" {
		return bad()
	}
	closed, e := boolean(bs.Args[3])
	if e != nil || !closed {
		return bad()
	}
	self, e := boolean(bs.Args[4])
	if e != nil || self {
		return bad()
	}
	ids, e := listRefs(bs.Args[1], 7)
	if e != nil || len(ids) != 7 {
		return bad()
	}
	wr, ok := rec["RATIONAL_B_SPLINE_CURVE"]
	if !ok || len(wr.Args) != 1 || wr.Args[0].Kind != Aggregate || len(wr.Args[0].Items) != 7 {
		return bad()
	}
	kr, ok := rec["B_SPLINE_CURVE_WITH_KNOTS"]
	if !ok || len(kr.Args) != 3 || kr.Args[0].Kind != Aggregate || kr.Args[1].Kind != Aggregate || len(kr.Args[0].Items) != 6 || len(kr.Args[1].Items) != 6 || kr.Args[2].Kind != Enumeration || kr.Args[2].Text != "UNSPECIFIED" {
		return bad()
	}
	var knots []*big.Rat
	unique := make([]*big.Rat, 6)
	for i, v := range kr.Args[1].Items {
		unique[i], e = liftSourceRat(v)
		if e != nil || !residualBits(unique[i]) || (i > 0 && unique[i].Cmp(unique[i-1]) <= 0) {
			return bad()
		}
		m, e := integer(kr.Args[0].Items[i])
		want := 2
		if i == 0 || i == 5 {
			want = 1
		}
		if e != nil || m != want {
			return bad()
		}
		for j := 0; j < m; j++ {
			knots = append(knots, new(big.Rat).Set(unique[i]))
		}
	}
	plane, e := simple(f, q.SupportID, "PLANE", 2)
	if e != nil {
		return bad()
	}
	po, px, py, _, e := convexUVFrame(f, plane.Args[1].Ref)
	if e != nil {
		return bad()
	}
	circle, e := simple(f, model, "CIRCLE", 3)
	if e != nil {
		return bad()
	}
	co, cx, cy, cz, e := convexUVFrame(f, circle.Args[1].Ref)
	if e != nil {
		return bad()
	}
	r, e := liftSourceRat(circle.Args[2])
	if e != nil || r.Sign() <= 0 {
		return bad()
	}
	h := make([][3]*big.Rat, 7)
	weights := make([]*big.Rat, 7)
	for i, id := range ids {
		uv, e := convexUVVector(f, id, "CARTESIAN_POINT", 2)
		if e != nil {
			return bad()
		}
		w, e := liftSourceRat(wr.Args[0].Items[i])
		if e != nil || w.Sign() <= 0 || w.Num().BitLen() > 256 || w.Denom().BitLen() > 256 {
			return bad()
		}
		weights[i] = w
		p := occtLoopSub(convexUVLift(po, px, py, uv[0], uv[1]), co)
		if occtLoopDot(p, cz).Sign() != 0 {
			return bad()
		}
		h[i] = [3]*big.Rat{new(big.Rat).Mul(w, occtLoopDot(p, cx)), new(big.Rat).Mul(w, occtLoopDot(p, cy)), new(big.Rat).Set(w)}
	}
	inv, e := residualBernsteinInverse(2)
	if e != nil {
		return bad()
	}
	var spans [3][3][]*big.Rat
	radial := new(big.Rat)
	for s, k := range []int{2, 4, 6} {
		v, e := liftSpanControls(2, h, knots, k, inv)
		if e != nil {
			return bad()
		}
		spans[s] = v
		wmin, _ := groupedRatBounds(v[2])
		if wmin.Sign() <= 0 {
			return bad()
		}
		xx, yy, ww := liftProduct(v[0], v[0]), liftProduct(v[1], v[1]), liftProduct(v[2], v[2])
		maxres := new(big.Rat)
		for i := range xx {
			res := new(big.Rat).Sub(new(big.Rat).Add(xx[i], yy[i]), new(big.Rat).Mul(new(big.Rat).Mul(r, r), ww[i]))
			res.Abs(res)
			if !residualBits(res) {
				return bad()
			}
			if res.Cmp(maxres) > 0 {
				maxres.Set(res)
			}
		}
		bound := new(big.Rat).Quo(maxres, new(big.Rat).Mul(r, new(big.Rat).Mul(wmin, wmin)))
		if bound.Cmp(radial) > 0 {
			radial.Set(bound)
		}
		xy, yx := liftProduct(v[0], groupedDerivative(v[1])), liftProduct(v[1], groupedDerivative(v[0]))
		for i := range xy {
			if new(big.Rat).Sub(xy[i], yx[i]).Sign() <= 0 {
				return bad()
			}
		}
	}
	// Exactly one positive-ray crossing, at the shared closed endpoint: first
	// span is in the upper half-plane, middle strictly left, final lower.
	for i := 0; i < 3; i++ {
		if spans[0][1][i].Sign() < 0 || spans[1][0][i].Sign() >= 0 || spans[2][1][i].Sign() > 0 {
			return bad()
		}
	}
	if spans[0][1][0].Sign() != 0 || spans[2][1][2].Sign() != 0 || spans[0][1][2].Sign() <= 0 || spans[2][1][0].Sign() >= 0 || spans[0][0][0].Sign() <= 0 {
		return bad()
	}
	for c := 0; c < 2; c++ {
		a := new(big.Rat).Quo(spans[0][c][0], spans[0][2][0])
		b := new(big.Rat).Quo(spans[2][c][2], spans[2][2][2])
		if a.Cmp(b) != 0 {
			return bad()
		}
	}
	t := new(big.Rat).Add(unique[1], new(big.Rat).Mul(new(big.Rat).Sub(unique[2], unique[1]), big.NewRat(1, 4)))
	sine, cosine, rem, e := liftTrigBernstein([]*big.Rat{t, t})
	if e != nil {
		return bad()
	}
	lower := new(big.Rat)
	den := groupedEval(spans[0][2], big.NewRat(1, 4))
	for i, trig := range []*big.Rat{cosine[0], sine[0]} {
		actual := new(big.Rat).Quo(groupedEval(spans[0][i], big.NewRat(1, 4)), den)
		d := new(big.Rat).Sub(new(big.Rat).Abs(new(big.Rat).Sub(actual, new(big.Rat).Mul(r, trig))), new(big.Rat).Mul(r, rem))
		if d.Sign() > 0 {
			lower.Add(lower, new(big.Rat).Mul(d, d))
		}
	}
	if lower.Sign() <= 0 {
		return bad()
	}
	return GroupedRationalCapEvidence{Owner: q, CircleID: model, Domain: [2]*big.Rat{new(big.Rat).Set(unique[1]), new(big.Rat).Set(unique[4])}, Knots: unique, PoleIDs: ids, Weights: weights, RadialDistanceUpper: radial, PositiveAngularVelocity: true, ClosedOneTurn: true, WitnessParameter: t, WitnessDistanceSquaredLower: lower}, nil
}

// InspectGroupedConvexArcPCurves proves original owner bindings and a closed
// one-turn radial projection for both rational cap curves. It does not repair
// or import the full source: same-parameter correspondence is disproved and
// finite edge trim selection under a nonlinear reparameterization is unknown.
func InspectGroupedConvexArcPCurves(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (GroupedPCurveEvidence, error) {
	var zero GroupedPCurveEvidence
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	view, owners, e := convexPCurveModelView(f, 15, true)
	if e != nil {
		return zero, e
	}
	b, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	m, e := RepairGroupedOCCTConvexArcExtrusion(ctx, bytes.NewReader(b), l, o)
	if e != nil {
		return zero, e
	}
	fixed, e := Parse(bytes.NewBufferString(m.CorrectedSTEP()), l)
	if e != nil {
		return zero, e
	}
	out := GroupedPCurveEvidence{Reversals: m.Reversals(), Repairs: m.Repairs()}
	tol := new(big.Rat).SetFloat64(o.MaxDistance)
	tol.Mul(tol, tol)
	for _, q := range owners {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		sc, _ := simple(f, q.SurfaceCurveID, "SURFACE_CURVE", 4)
		model := sc.Args[1].Ref
		if len(f.Entities[q.GeometryID].Records) > 1 {
			cap, e := groupedRationalCap(f, q, model)
			if e != nil {
				return zero, e
			}
			if new(big.Rat).Mul(cap.RadialDistanceUpper, cap.RadialDistanceUpper).Cmp(tol) > 0 {
				return zero, geom.ErrDomain
			}
			out.Caps = append(out.Caps, cap)
			continue
		}
		// An original reversed circle has the same cosine axis and opposite sine
		// axis to its cylinder. U=-t is therefore the exact owner correspondence.
		q.Kind, q.DistanceSquaredUpper, e = boundGroupedSimplePCurve(f, fixed, model, q.GeometryID, q.SupportID)
		if e != nil || q.DistanceSquaredUpper.Cmp(tol) > 0 {
			return zero, geom.ErrDomain
		}
		out.OtherOwners = append(out.OtherOwners, q)
	}
	if len(out.Caps) != 2 || len(out.OtherOwners) != 28 || len(out.Reversals) != 2 {
		return zero, geom.ErrDomain
	}
	for _, c := range out.Caps {
		found := false
		for _, v := range out.Reversals {
			if v.EdgeID == c.Owner.EdgeID && v.CircleID == c.CircleID {
				found = true
			}
		}
		if !found {
			return zero, geom.ErrDomain
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func boundGroupedSimplePCurve(f, fixed *File, model, geometry, support int64) (string, *big.Rat, error) {
	circle, e := simple(f, model, "CIRCLE", 3)
	if e == nil {
		cyl, e := simple(f, support, "CYLINDRICAL_SURFACE", 3)
		if e == nil {
			co, cx, cy, _, e := convexUVFrame(f, circle.Args[1].Ref)
			if e != nil {
				return "", nil, e
			}
			o, x, y, z, e := convexUVFrame(f, cyl.Args[1].Ref)
			if e != nil {
				return "", nil, e
			}
			uv, d, e := convexUVLine(f, geometry, 2)
			if e != nil {
				return "", nil, e
			}
			r, e := liftSourceRat(circle.Args[2])
			if e != nil {
				return "", nil, e
			}
			rr, e := liftSourceRat(cyl.Args[2])
			if e != nil {
				return "", nil, e
			}
			sign := d[0]
			if uv[0].Sign() != 0 || d[1].Sign() != 0 || new(big.Rat).Abs(sign).Cmp(big.NewRat(1, 1)) != 0 || r.Cmp(rr) != 0 || !convexUVSame(cx, x) {
				return "", nil, geom.ErrDomain
			}
			for i := range y {
				y[i] = new(big.Rat).Mul(y[i], sign)
				o[i].Add(o[i], new(big.Rat).Mul(z[i], uv[1]))
			}
			if !convexUVSame(cy, y) {
				return "", nil, geom.ErrDomain
			}
			delta := occtLoopSub(o, co)
			return "cylinder-circle", occtLoopDot(delta, delta), nil
		}
	}
	return boundConvexOwnerPCurve(f, fixed, model, geometry, support)
}
