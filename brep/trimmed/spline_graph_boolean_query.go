package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SharedBaseSplineGraphMaterial represents two certified closed graph solids
// between the same base and two top charts. Exact point queries work even
// when a nonbinary or singular contact prevents boundary shell assembly.
type SharedBaseSplineGraphMaterial struct {
	base, a, b *geom.SurfacePatch
}

// NewSharedBaseSplineGraphMaterial proves a unit identity XY chart and
// strictly positive thickness for each input solid independently.
func NewSharedBaseSplineGraphMaterial(ctx context.Context, base, a, b *geom.BSplineSurface,
	o intersect.SplinePlaneOptions) (SharedBaseSplineGraphMaterial, error) {
	var empty SharedBaseSplineGraphMaterial
	if ctx == nil || base == nil || a == nil || b == nil || o.Domain != (geom.ParameterRectangle{}) {
		return empty, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return empty, e
	}
	bottom, e := geom.NewSurfacePatch(base, o.Patch)
	if e != nil {
		return empty, e
	}
	return SharedBaseSplineGraphMaterial{bottom, p[0], p[1]}, nil
}

// LocateEnvelopeRational exactly classifies a point against the union or
// intersection of these two solids by evaluating their original rational
// graphs. At a coincidence or crossing the common top remains boundary.
// Subtraction needs closure evidence at zero-thickness contacts and is
// outside this query.
func (p SharedBaseSplineGraphMaterial) LocateEnvelopeRational(ctx context.Context, point [3]*big.Rat,
	op parametric.BooleanOp, o geom.SurfacePatchEvaluationOptions) (parametric.Location, error) {
	if point[2] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	q, e := p.SectionEnvelopeVertical(ctx, [2]*big.Rat{point[0], point[1]}, op, o)
	if errors.Is(e, geom.ErrDomain) {
		return parametric.Outside, nil
	}
	if e != nil {
		return parametric.Outside, e
	}
	a, b := point[2].Cmp(q.Lower), point[2].Cmp(q.Upper)
	if a < 0 || b > 0 {
		return parametric.Outside, nil
	}
	if a == 0 || b == 0 || q.SideWall {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

// LocateEnvelope is the represented-point form of LocateEnvelopeRational.
func (p SharedBaseSplineGraphMaterial) LocateEnvelope(ctx context.Context, point geom.Vec3,
	op parametric.BooleanOp, o geom.SurfacePatchEvaluationOptions) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	return p.LocateEnvelopeRational(ctx, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)}, op, o)
}

// SharedBaseGraphVerticalSection gives the closed material interval on a
// vertical line at an exact XY location. Faces lists original top ownership
// (0 for A, 1 for B) at the selected envelope height.
type SharedBaseGraphVerticalSection struct {
	Lower, Upper *big.Rat
	Faces        []int
	SideWall     bool
}

// SectionDifferenceVertical proves the exact vertical interval of the
// regularized A-minus-B material when A is strictly higher at this XY.
// Present=false means there is no positive-thickness interval; it does
// not rule out a zero-thickness boundary limit at that point.
func (p SharedBaseSplineGraphMaterial) SectionDifferenceVertical(ctx context.Context, xy [2]*big.Rat,
	o geom.SurfacePatchEvaluationOptions) (section SharedBaseGraphVerticalSection, present bool, err error) {
	z, wall, e := p.graphHeightsAt(ctx, xy, o)
	if e != nil {
		return section, false, e
	}
	if z[1].Cmp(z[2]) <= 0 {
		return section, false, nil
	}
	return SharedBaseGraphVerticalSection{Lower: z[2], Upper: z[1], Faces: []int{0}, SideWall: wall}, true, nil
}

// SectionEnvelopeVertical returns the exact base and selected top heights
// of a shared-base union or intersection. It does not require a trimmed
// result shell; a nonbinary chart position is accepted directly.
func (p SharedBaseSplineGraphMaterial) SectionEnvelopeVertical(ctx context.Context, xy [2]*big.Rat,
	op parametric.BooleanOp, o geom.SurfacePatchEvaluationOptions) (SharedBaseGraphVerticalSection, error) {
	var r SharedBaseGraphVerticalSection
	if op != parametric.Union && op != parametric.Intersection {
		return r, geom.ErrInvalid
	}
	z, wall, e := p.graphHeightsAt(ctx, xy, o)
	if e != nil {
		return r, e
	}
	r.SideWall = wall
	r.Lower = z[0]
	c := z[1].Cmp(z[2])
	switch {
	case c == 0:
		r.Upper, r.Faces = z[1], []int{0, 1}
	case (c > 0) == (op == parametric.Union):
		r.Upper, r.Faces = z[1], []int{0}
	default:
		r.Upper, r.Faces = z[2], []int{1}
	}
	return r, nil
}

func (p SharedBaseSplineGraphMaterial) graphHeightsAt(ctx context.Context, xy [2]*big.Rat,
	o geom.SurfacePatchEvaluationOptions) (z [3]*big.Rat, wall bool, err error) {
	if ctx == nil || p.base == nil || p.a == nil || p.b == nil || xy[0] == nil || xy[1] == nil {
		return z, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return z, false, e
	}
	one := big.NewRat(1, 1)
	for _, q := range xy {
		c := q.Cmp(one)
		if q.Sign() < 0 || c > 0 {
			return z, false, geom.ErrDomain
		}
		wall = wall || q.Sign() == 0 || c == 0
	}
	for i, s := range [3]*geom.SurfacePatch{p.base, p.a, p.b} {
		x, e := s.EvaluateRational(ctx, xy[0], xy[1], o)
		if e != nil {
			return [3]*big.Rat{}, false, e
		}
		z[i] = x.Point[2]
	}
	return z, wall, nil
}

// SharedBaseGraphVerticalHit is an exact point in the original vertical line
// parameter. Base is set on the common lower face; TopFaces use the same
// source convention as SharedBaseGraphVerticalSection. SideWall indicates
// that the line itself lies along the material's vertical side.
type SharedBaseGraphVerticalHit struct {
	Parameter *big.Rat
	Base      bool
	TopFaces  []int
	SideWall  bool
}

// SharedBaseGraphVerticalLine describes boundary contacts on a finite
// vertical line. Coincident is nonnil when the line runs on a side wall;
// its endpoints are clipped to the original interval.
type SharedBaseGraphVerticalLine struct {
	Hits       []SharedBaseGraphVerticalHit
	Coincident *[2]*big.Rat
}

// IntersectEnvelopeVerticalLine returns exact contacts in origin+directionZ*t,
// including finite intervals lying on a side wall.
func (p SharedBaseSplineGraphMaterial) IntersectEnvelopeVerticalLine(ctx context.Context, origin geom.Vec3,
	directionZ, minT, maxT float64, op parametric.BooleanOp, o geom.SurfacePatchEvaluationOptions) (SharedBaseGraphVerticalLine, error) {
	var r SharedBaseGraphVerticalLine
	if !origin.IsFinite() || !finite(directionZ) || directionZ == 0 || !finite(minT) || !finite(maxT) || minT > maxT {
		return r, geom.ErrInvalid
	}
	s, e := p.SectionEnvelopeVertical(ctx, [2]*big.Rat{ratInset(origin.X), ratInset(origin.Y)}, op, o)
	if errors.Is(e, geom.ErrDomain) {
		return r, nil
	}
	if e != nil {
		return r, e
	}
	z, d, lo, hi := ratInset(origin.Z), ratInset(directionZ), ratInset(minT), ratInset(maxT)
	param := func(h *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Sub(h, z), d) }
	a, b := SharedBaseGraphVerticalHit{Parameter: param(s.Lower), Base: true, SideWall: s.SideWall},
		SharedBaseGraphVerticalHit{Parameter: param(s.Upper), TopFaces: s.Faces, SideWall: s.SideWall}
	if a.Parameter.Cmp(b.Parameter) > 0 {
		a, b = b, a
	}
	contains := func(t *big.Rat) bool { return t.Cmp(lo) >= 0 && t.Cmp(hi) <= 0 }
	for _, x := range [2]SharedBaseGraphVerticalHit{a, b} {
		if contains(x.Parameter) {
			r.Hits = append(r.Hits, x)
		}
	}
	if s.SideWall {
		u, v := new(big.Rat).Set(a.Parameter), new(big.Rat).Set(b.Parameter)
		if u.Cmp(lo) < 0 {
			u.Set(lo)
		}
		if v.Cmp(hi) > 0 {
			v.Set(hi)
		}
		if u.Cmp(v) <= 0 {
			r.Coincident = &[2]*big.Rat{u, v}
		}
	}
	return r, nil
}

// SharedBaseGraphLineSections reports the contacts with all three original
// graph supports, in the caller's unnormalized line parameter. Top events
// have not been classified as retained envelope boundary. The line is first
// clipped to the exact XY chart, so every reported event belongs to a graph.
type SharedBaseGraphLineSections struct {
	Graph      [3]geom.PiecewiseGraphLineIntersection // common base, top A, top B
	Clip       *geom.ExactParameterInterval
	pieces     [3]geom.SurfaceAffineCurvePieces
	clipHeight [2]*big.Rat
}

// SectionOriginalGraphsLine finds all finite line contacts with the original
// graph supports. Tangencies and coincident spans are explicit. The XY line
// may cross multiple knot cells at nonbinary parameters. Domain walls and
// Boolean face selection are outside this original-support query.
func (p SharedBaseSplineGraphMaterial) SectionOriginalGraphsLine(ctx context.Context, origin, direction geom.Vec3,
	minT, maxT float64, o geom.ToroidLineOptions) (SharedBaseGraphLineSections, error) {
	var empty SharedBaseGraphLineSections
	if ctx == nil || p.base == nil || p.a == nil || p.b == nil || !origin.IsFinite() || !direction.IsFinite() ||
		direction == (geom.Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT ||
		!finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	a, b := ratInset(minT), ratInset(maxT)
	x := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}
	d := [3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	one := big.NewRat(1, 1)
	for i := range 2 {
		if d[i].Sign() == 0 {
			if x[i].Sign() < 0 || x[i].Cmp(one) > 0 {
				return empty, nil
			}
			continue
		}
		lo := new(big.Rat).Quo(new(big.Rat).Neg(x[i]), d[i])
		hi := new(big.Rat).Quo(new(big.Rat).Sub(one, x[i]), d[i])
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if lo.Cmp(a) > 0 {
			a = lo
		}
		if hi.Cmp(b) < 0 {
			b = hi
		}
	}
	if a.Cmp(b) > 0 {
		return empty, nil
	}
	r := SharedBaseGraphLineSections{Clip: &geom.ExactParameterInterval{Lower: a, Upper: b}}
	width := new(big.Rat).Sub(b, a)
	value := func(t *big.Rat, i int) *big.Rat { return new(big.Rat).Add(x[i], new(big.Rat).Mul(t, d[i])) }
	uvA, uvB := [2]*big.Rat{value(a, 0), value(a, 1)}, [2]*big.Rat{value(b, 0), value(b, 1)}
	z0, z1 := value(a, 2), value(b, 2)
	r.clipHeight = [2]*big.Rat{z0, z1}
	eval := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits}
	if width.Sign() == 0 || d[0].Sign() == 0 && d[1].Sign() == 0 {
		for i, s := range [3]*geom.SurfacePatch{p.base, p.a, p.b} {
			j, e := s.EvaluateRational(ctx, uvA[0], uvA[1], eval)
			if e != nil {
				return empty, e
			}
			t := a
			if width.Sign() == 0 {
				if z0.Cmp(j.Point[2]) != 0 {
					continue
				}
			} else {
				t = new(big.Rat).Quo(new(big.Rat).Sub(j.Point[2], x[2]), d[2])
				if t.Cmp(a) < 0 || t.Cmp(b) > 0 {
					continue
				}
			}
			r.Graph[i].Hits = []geom.PiecewiseGraphLineHit{{Parameter: geom.ToroidLineRoot{Min: t, Max: new(big.Rat).Set(t), Multiplicity: 1}}}
		}
		return r, nil
	}
	tolerance := new(big.Rat).Quo(ratInset(o.ParameterTolerance), width)
	if tolerance.Cmp(one) > 0 {
		tolerance.Set(one)
	}
	f, exact := tolerance.Float64()
	if !exact && f > 0 && ratInset(f).Cmp(tolerance) > 0 {
		f = math.Nextafter(f, 0)
	}
	if f <= 0 {
		return empty, geom.ErrConvergence
	}
	o.ParameterTolerance = f
	for i, s := range [3]*geom.SurfacePatch{p.base, p.a, p.b} {
		curves, e := s.AffineParameterCurvePieces(ctx, uvA, uvB, geom.SurfaceAffineCurveOptions{TubeAffineLineOptions: geom.TubeAffineLineOptions{
			MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits, MaxPoles: o.Surface.MaxPoles,
		}})
		if e != nil {
			return empty, e
		}
		r.pieces[i] = curves
		q, e := geom.IntersectAffineGraphCurveHeight(ctx, curves, z0, z1, o)
		if e != nil {
			return empty, e
		}
		remap := func(t *big.Rat) *big.Rat { return new(big.Rat).Add(a, new(big.Rat).Mul(width, t)) }
		for j := range q.Hits {
			x := &q.Hits[j].Parameter
			x.Min, x.Max = remap(x.Min), remap(x.Max)
		}
		for j := range q.Coincident {
			x := &q.Coincident[j]
			x.Lower, x.Upper = remap(x.Lower), remap(x.Upper)
		}
		r.Graph[i] = q
	}
	return r, nil
}
