package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombineEllipticalContactSplineGraphs combines strictly positive shared-base
// graphs after proving that the complete common-chart section is an interior
// ellipse. Four rational quadratic p-curves are shared by the original graph
// supports. Ellipse controls must be exactly representable; squared radii
// that are not representable rational squares still appear in history.
func CombineEllipticalContactSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions,
	hint geom.GraphEllipseHint, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	ellipse, e := geom.CertifySplineGraphEllipse(ctx, p[0], p[1], hint, proof)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleEllipticalContactGraphBoolean(ctx, base, a, b, op, tolerance, ellipse, proof)
}

func assembleEllipticalContactGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, ellipse geom.SplineGraphEllipse, proof geom.GraphPointContactOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if !ellipse.Proven || ellipse.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.Ellipse = &ellipse
	loop, e := ellipseGraphContactLoop(ellipse.Ellipse)
	if e != nil {
		return Solid{}, h, e
	}
	out, r, e := assembleGraphClosedContact(ctx, base, a, b, op, tolerance, ellipse.Domain, ellipse.InsideSide, ellipse.OutsideSide, loop, proof)
	if e != nil {
		return Solid{}, h, e
	}
	r.Ellipse = &ellipse
	return out, r, nil
}

func assembleGraphClosedContact(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp,
	tolerance float64, domain geom.ParameterRectangle, inside, outside int, loop []openSectionPiece, proof geom.GraphPointContactOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	reverse := make([]openSectionPiece, len(loop))
	for i, x := range loop {
		reverse[len(loop)-1-i] = openSectionPiece{a: x.b, b: x.a, c: reversePCurve(x.c), side: -1}
	}
	var out Solid
	var r SplineGraphBooleanHistory
	var e error
	if op == parametric.Difference && inside > 0 {
		out, r, e = assembleEllipticalGraphLens(ctx, a, b, loop, tolerance)
	} else {
		outer := partitionGraphRectangleLoop(domain)
		regions := []openGraphRegion{
			{loop: outer, holes: [][]openSectionPiece{reverse}, sign: outside},
			{loop: loop, sign: inside},
		}
		out, r, e = combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	}
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof); e != nil {
		return Solid{}, h, e
	}
	for _, f := range out.faces {
		if _, ok := f.Surface.(*geom.BSplineSurface); !ok {
			continue // exact axis chart walls have separately certified graph edges
		}
		tr, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if e != nil {
			return Solid{}, h, e
		}
		if tr.Status != trimvalidate.Certified {
			return Solid{}, h, geom.ErrConvergence
		}
	}
	return out, r, nil
}

func ellipseGraphContactLoop(q geom.GraphEllipseHint) ([]openSectionPiece, error) {
	var basis [2][2]*big.Rat
	for i, s := range q.RadiusSquared {
		r, e := exactRationalSquareRoot(s)
		if e != nil {
			return nil, e
		}
		basis[i][i], basis[i][1-i] = r, new(big.Rat)
	}
	return ellipseGraphAffineLoop(q.Center, basis)
}

func exactRationalSquareRoot(s *big.Rat) (*big.Rat, error) {
	if s == nil || s.Sign() < 0 {
		return nil, geom.ErrInvalid
	}
	a, b := new(big.Int).Sqrt(s.Num()), new(big.Int).Sqrt(s.Denom())
	if new(big.Int).Mul(a, a).Cmp(s.Num()) != 0 || new(big.Int).Mul(b, b).Cmp(s.Denom()) != 0 {
		return nil, geom.ErrUnrepresentable
	}
	return new(big.Rat).SetFrac(a, b), nil
}

func ellipseGraphAffineLoop(center [2]*big.Rat, basis [2][2]*big.Rat) ([]openSectionPiece, error) {
	directions := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var points [9]geom.Vec2
	for i, d := range directions {
		var uv [2]*big.Rat
		for j := range uv {
			uv[j] = new(big.Rat).Set(center[j])
			for k := range d {
				uv[j].Add(uv[j], new(big.Rat).Mul(basis[k][j], big.NewRat(d[k], 1)))
			}
		}
		p, e := exactGraphPartitionUV(uv)
		if e != nil {
			return nil, e
		}
		points[i] = p
	}
	k := []float64{0, 0, 0, 1, 1, 1}
	var loop []openSectionPiece
	for i := range 4 {
		a, c, b := points[2*i], points[2*i+1], points[2*i+2]
		// Endpoint weight product = twice the squared middle weight;
		// the represented curve lies on the exact ellipse.
		s, e := geom.NewBSpline(2, []geom.Vec3{{X: a.X, Y: a.Y}, {X: c.X, Y: c.Y}, {X: b.X, Y: b.Y}},
			k, []float64{1, 1, 2})
		if e != nil {
			return nil, e
		}
		loop = append(loop, openSectionPiece{a: a, b: b, c: SplineUV{Curve: s, Range: UVInterval{Max: 1}}, side: -1})
	}
	return loop, nil
}

func assembleEllipticalGraphLens(ctx context.Context, a, b *geom.BSplineSurface, loop []openSectionPiece, tolerance float64) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: 0, TopB: 1, Base: -1, FacesA: []int{0}, FacesB: []int{1}}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var uses []Use
	for i, x := range loop {
		p, e := graphTrimStartDisplay(ctx, a, x)
		if e != nil {
			return Solid{}, h, e
		}
		vertices = append(vertices, p)
		edges = append(edges, parametric.Edge{Start: i, End: (i + 1) % len(loop),
			Curve: surfaceTrimCurve{a, x.a, x.b, x.c}, Range: parametric.Interval{Max: 1}})
		uses = append(uses, Use{Edge: i, UVStart: x.a, UVEnd: x.b, Curve: x.c})
		h.ContourEdges = append(h.ContourEdges, i)
	}
	s, e := NewSolidContext(ctx, vertices, edges, []Face{{Surface: a, Outer: uses}, {Surface: b, Outer: uses, Reversed: true}}, tolerance)
	return s, h, e
}

// CombineQuadraticEllipseContactSplineGraphs handles a rotated or sheared
// ellipse when the exact positive quadratic form has a rational triangular
// factorization with representable trim controls. Other positive forms still
// return the exact contact proof in history with ErrUnrepresentable.
func CombineQuadraticEllipseContactSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions,
	hint geom.GraphQuadraticEllipseHint, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	ellipse, e := geom.CertifySplineGraphQuadraticEllipse(ctx, p[0], p[1], hint, proof)
	if e != nil {
		return Solid{}, h, e
	}
	return assembleQuadraticEllipseGraphBoolean(ctx, base, a, b, op, tolerance, ellipse, proof)
}

func assembleQuadraticEllipseGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, p geom.SplineGraphQuadraticEllipse, proof geom.GraphPointContactOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if !p.Proven || p.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.QuadraticEllipse = &p
	q := p.Ellipse.Coefficients
	f := new(big.Rat).Add(q[0], new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(q[1], p.Center[0]),
		new(big.Rat).Mul(q[3], p.Center[1])), big.NewRat(2, 1)))
	f.Neg(f)
	m := new(big.Rat).Quo(q[5], new(big.Rat).Mul(big.NewRat(2, 1), q[2]))
	// Q = qU*(u+m*v)^2 + schur*v² in centered coordinates.
	schur := new(big.Rat).Sub(q[4], new(big.Rat).Mul(q[2], new(big.Rat).Mul(m, m)))
	ru, e := exactRationalSquareRoot(new(big.Rat).Quo(f, q[2]))
	if e != nil {
		return Solid{}, h, e
	}
	rv, e := exactRationalSquareRoot(new(big.Rat).Quo(f, schur))
	if e != nil {
		return Solid{}, h, e
	}
	loop, e := ellipseGraphAffineLoop(p.Center, [2][2]*big.Rat{{ru, new(big.Rat)}, {new(big.Rat).Neg(new(big.Rat).Mul(m, rv)), rv}})
	if e != nil {
		return Solid{}, h, e
	}
	out, r, e := assembleGraphClosedContact(ctx, base, a, b, op, tolerance, p.Domain, p.InsideSide, p.OutsideSide, loop, proof)
	if e != nil {
		return Solid{}, h, e
	}
	r.QuadraticEllipse = &p
	return out, r, nil
}

// CombineRationalEllipseContactSplineGraphs accepts one exact rational point
// on a certified quadratic ellipse. Rational parametrization then constructs
// four exact positive-weight UV arcs, without requiring square roots of the
// form's principal or conjugate norms. Native assembly retains exact
// rational endpoints even when the displayed vertices need rounding.
// Display topology must still be nondegenerate. The exact conic proof is
// retained on failure.
func CombineRationalEllipseContactSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions,
	hint geom.GraphQuadraticEllipseHint, point [2]*big.Rat, proof geom.GraphPointContactOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	arcs, e := geom.BuildSplineGraphQuadraticEllipseArcs(ctx, p[0], p[1], hint, point, proof)
	if e != nil {
		return Solid{}, h, e
	}
	if !arcs.Complete || !arcs.Certificate.Proven || arcs.Certificate.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return Solid{}, h, geom.ErrConvergence
	}
	h.QuadraticEllipse, h.RationalEllipseArcs = &arcs.Certificate, &arcs
	var loop []openSectionPiece
	for i, c := range arcs.Arcs {
		a, ea := exactGraphPartitionUV(arcs.Junctions[i])
		b, eb := exactGraphPartitionUV(arcs.Junctions[(i+1)%len(arcs.Junctions)])
		rationalEnds := ea != nil || eb != nil
		var uv PCurve
		if p, e := c.ToBSpline(); e == nil && !rationalEnds {
			uv = SplineUV{Curve: p, Range: UVInterval{Max: 1}}
		} else {
			data, _, e := c.BezierControlNet(ctx, geom.SurfacePatchOptions{
				MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits,
			})
			if e != nil {
				return Solid{}, h, e
			}
			var controls [3][3]*big.Rat
			for j, x := range data.Control {
				controls[j] = [3]*big.Rat{x[0], x[1], x[3]}
			}
			q, e := newExactQuadraticUV(controls, rationalEnds)
			if e != nil {
				return Solid{}, h, e
			}
			a, b, uv = q.ends[0], q.ends[1], q
		}
		loop = append(loop, openSectionPiece{a: a, b: b, c: uv, side: -1})
	}
	r, report, e := assembleGraphClosedContact(ctx, base, a, b, op, tolerance, arcs.Certificate.Domain,
		arcs.Certificate.InsideSide, arcs.Certificate.OutsideSide, loop, proof)
	if e != nil {
		return Solid{}, h, e
	}
	report.QuadraticEllipse, report.RationalEllipseArcs = &arcs.Certificate, &arcs
	return r, report, nil
}
