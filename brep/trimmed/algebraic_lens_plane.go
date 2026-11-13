package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicLensCapChart is the exact affine vertical section surface (t,z). t is U or
// V in the original identity graph chart; the two model edges bound z.
type AlgebraicLensCapChart struct {
	Axis        int
	Line        [3]*big.Rat
	Parameter   [2]*geom.ExactAlgebraicRoot
	Edges       [2]int
	Orientation int // outward normal's sign relative to A*x+B*y+C
}

// EvaluateRational evaluates the supporting affine plane at exact coordinates.
func (c AlgebraicLensCapChart) EvaluateRational(t, z *big.Rat) ([3]*big.Rat, error) {
	var p [3]*big.Rat
	if t == nil || z == nil || c.Axis < 0 || c.Axis > 1 || c.Line[0] == nil || c.Line[1] == nil || c.Line[2] == nil || c.Line[c.Axis^1].Sign() == 0 {
		return p, geom.ErrInvalid
	}
	for _, x := range []*big.Rat{t, z} {
		if x.Num().BitLen() > 4096 || x.Denom().BitLen() > 4096 {
			return p, geom.ErrConvergence
		}
	}
	p[c.Axis], p[2] = new(big.Rat).Set(t), new(big.Rat).Set(z)
	p[c.Axis^1] = new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Add(c.Line[2], new(big.Rat).Mul(c.Line[c.Axis], t)), c.Line[c.Axis^1]))
	return p, nil
}

// CapChart returns the exact supporting chart, algebraic span and both
// authoritative boundary curves' edge IDs. EvaluateRational evaluates the
// support; CapVerticalSection classifies the filled domain at a rational t.
func (s AlgebraicLensPlaneSplit) CapChart(side int) (AlgebraicLensCapChart, error) {
	var empty AlgebraicLensCapChart
	h, e := s.Half(side)
	if e != nil || len(s.source.curves) != 2 {
		return empty, geom.ErrInvalid
	}
	c := s.source.curves[0]
	r := AlgebraicLensCapChart{Axis: c.Axis, Parameter: c.Endpoints, Edges: h.CapEdges, Orientation: -side}
	for i, q := range c.Partition.Line {
		r.Line[i] = new(big.Rat).Set(q)
	}
	return r, nil
}

// AlgebraicLensPlaneSplit divides an exactly certified shared-chart conic
// lens by a vertical rational line in XY. Both results retain the original
// upper and lower spline supports and an exact affine section cap. Negative
// and Positive refer to the original A*x+B*y+C plane expression.
type AlgebraicLensPlaneSplit struct {
	Negative, Positive AlgebraicCappedLensHalf
	Source             AlgebraicFaceArrangement
	source             AlgebraicFaceArrangement
	Contact            geom.SplineGraphQuadraticEllipse
	contact            [6]*big.Rat
	half               [2]AlgebraicCappedLensHalf
	base               *geom.BSplineSurface
	point              [2]*big.Rat
}

type AlgebraicCappedLensHalf struct {
	Side                int
	Fragments           [2]int
	CapEdges            [2]int  // source upper and lower cut curves
	CapReversed         [2]bool // traversals opposite to the retained source fragments
	EulerCharacteristic int
}

// VerticalSection returns the exact lower and upper heights at a rational
// XY point in a selected half. A zero-length section is allowed on its rim.
// It also works on the generated planar cap, without approximating either
// original spline or the algebraic terminals.
func (s AlgebraicLensPlaneSplit) VerticalSection(ctx context.Context, side int, xy [2]*big.Rat,
	o geom.SurfacePatchOptions) ([2]*big.Rat, bool, error) {
	var empty [2]*big.Rat
	if ctx == nil || side != 1 && side != -1 || xy[0] == nil || xy[1] == nil || len(s.source.curves) != 2 || s.contact[0] == nil {
		return empty, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, false, e
	}
	for _, v := range xy {
		if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return empty, false, geom.ErrConvergence
		}
	}
	conic, plane := s.sectionSigns(xy)
	if conic > 0 || plane*side < 0 {
		return empty, false, nil
	}
	var height [2]*big.Rat
	for i, f := range s.source.subdivision.source.faces {
		p, e := exactFaceSupportPatch(f, o)
		if e != nil {
			return empty, false, e
		}
		j, e := p.EvaluateRational(ctx, xy[0], xy[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return empty, false, e
		}
		height[1-i] = j.Point[2]
	}
	if height[0].Cmp(height[1]) > 0 {
		return empty, false, geom.ErrConvergence
	}
	return height, true, nil
}

func (s AlgebraicLensPlaneSplit) sectionSigns(xy [2]*big.Rat) (conic, plane int) {
	x, y := xy[0], xy[1]
	line := s.source.curves[0].Partition.Line
	plane = new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(line[0], x), new(big.Rat).Mul(line[1], y)), line[2]).Sign()
	q := s.contact
	v := new(big.Rat).Set(q[0])
	for i, p := range []*big.Rat{x, new(big.Rat).Mul(x, x), y, new(big.Rat).Mul(y, y), new(big.Rat).Mul(x, y)} {
		v.Add(v, new(big.Rat).Mul(q[i+1], p))
	}
	return v.Sign(), plane
}

// CapVerticalSection evaluates the original lower and upper supports along
// the cap chart (t,z), where t is the selected varying U or V coordinate.
func (s AlgebraicLensPlaneSplit) CapVerticalSection(ctx context.Context, t *big.Rat, o geom.SurfacePatchOptions) ([2]*big.Rat, bool, error) {
	if t == nil || len(s.source.curves) != 2 || s.contact[0] == nil {
		return [2]*big.Rat{}, false, geom.ErrInvalid
	}
	if t.Num().BitLen() > 4096 || t.Denom().BitLen() > 4096 {
		return [2]*big.Rat{}, false, geom.ErrConvergence
	}
	c := s.source.curves[0]
	a, b := c.Axis, c.Axis^1
	x := [2]*big.Rat{}
	x[a] = t
	x[b] = new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Add(c.Partition.Line[2], new(big.Rat).Mul(c.Partition.Line[a], t)), c.Partition.Line[b]))
	return s.VerticalSection(ctx, 1, x, o)
}

// Half returns the verified topology and orientation for the selected side.
func (s AlgebraicLensPlaneSplit) Half(side int) (AlgebraicCappedLensHalf, error) {
	if s.contact[0] == nil || side != 1 && side != -1 {
		return AlgebraicCappedLensHalf{}, geom.ErrInvalid
	}
	if side < 0 {
		return s.half[0], nil
	}
	return s.half[1], nil
}

// LocateRational classifies an exact represented or nonbinary model point
// in either closed half. The original graph supports are evaluated directly;
// the conic and cut-plane classifications use rational arithmetic.
func (s AlgebraicLensPlaneSplit) LocateRational(ctx context.Context, side int, point [3]*big.Rat,
	o geom.SurfacePatchOptions) (parametric.Location, error) {
	if ctx == nil || side != 1 && side != -1 || point[0] == nil || point[1] == nil || point[2] == nil ||
		len(s.source.curves) != 2 || s.contact[0] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, e
	}
	for _, p := range point {
		if p.Num().BitLen() > 4096 || p.Denom().BitLen() > 4096 {
			return parametric.Outside, geom.ErrConvergence
		}
	}
	xy := [2]*big.Rat{point[0], point[1]}
	height, ok, e := s.VerticalSection(ctx, side, xy, o)
	if e != nil || !ok {
		return parametric.Outside, e
	}
	conic, sign := s.sectionSigns(xy)
	l, h := point[2].Cmp(height[0]), point[2].Cmp(height[1])
	if l < 0 || h > 0 {
		return parametric.Outside, nil
	}
	if l == 0 || h == 0 || sign == 0 || conic == 0 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

// SplitRationalEllipseLensByVerticalPlane constructs and proves the lens
// before classification. The interior conic, opposite graph height signs,
// and simple chart cuts prove both retained fragment sides and the cap
// between their original curves. Tangencies, corner cuts, and other source
// shell families remain unresolved.
func SplitRationalEllipseLensByVerticalPlane(ctx context.Context, base, upper, lower *geom.BSplineSurface,
	hint geom.GraphQuadraticEllipseHint, point [2]*big.Rat, line [3]*big.Rat, tolerance float64,
	proof geom.GraphPointContactOptions, o AlgebraicSeamOptions) (AlgebraicLensPlaneSplit, error) {
	var empty AlgebraicLensPlaneSplit
	if ctx == nil || line[0] == nil || line[1] == nil || line[2] == nil || line[0].Sign() == 0 && line[1].Sign() == 0 {
		return empty, geom.ErrInvalid
	}
	if e := requireIdentitySplineChart(ctx, upper, o.Composition.SurfacePatchOptions); e != nil {
		return empty, e
	}
	if e := requireIdentitySplineChart(ctx, lower, o.Composition.SurfacePatchOptions); e != nil {
		return empty, e
	}
	s, h, e := CombineRationalEllipseContactSplineGraphs(ctx, base, upper, lower, parametric.Difference, tolerance,
		intersect.SplinePlaneOptions{Patch: o.Composition.SurfacePatchOptions}, hint, point, proof)
	if e != nil {
		return empty, e
	}
	if h.QuadraticEllipse == nil || !h.QuadraticEllipse.Proven || h.QuadraticEllipse.InsideSide != 1 || len(s.faces) != 2 || h.TopA != 0 || h.TopB != 1 ||
		len(h.ContourEdges) != 4 || s.Topology().EulerCharacteristic != 2 {
		return empty, geom.ErrConvergence
	}
	plane := [4]*big.Rat{line[0], line[1], nil, line[2]}
	var recipes []AlgebraicSeamRecipe
	for _, edge := range h.ContourEdges {
		model, _, e := algebraicSeamSource(ctx, s, edge, o.Composition)
		if e != nil {
			return empty, e
		}
		q, e := model.curve.IntersectModelRawPlane(ctx, model.patch, plane, o.Composition, o.Root.ExactCurvePlaneEventOptions)
		if e != nil {
			return empty, e
		}
		if len(q.Coincident) > 0 {
			return empty, geom.ErrConvergence
		}
		for _, hit := range q.Hits {
			if len(recipes) == 2 || hit.Multiplicity != 1 {
				return empty, geom.ErrConvergence
			}
			r, _, e := s.PrepareAlgebraicSeamRecipe(ctx, edge, plane, hit.Event, o)
			if e != nil {
				return empty, e
			}
			recipes = append(recipes, r)
		}
	}
	if len(recipes) != 2 {
		return empty, geom.ErrConvergence
	}
	sub, e := s.SubdivideAlgebraicSeams(ctx, recipes, o)
	if e != nil {
		return empty, e
	}
	a, e := sub.ArrangeFacesByAlgebraicUVLines(ctx, []AlgebraicFaceCut{{0, line}, {1, line}}, o)
	if e != nil {
		return empty, e
	}
	if a.curves[0].Partition.Vertices != a.curves[1].Partition.Vertices {
		return empty, geom.ErrConvergence
	}
	result := AlgebraicLensPlaneSplit{Source: a, source: a, Contact: *h.QuadraticEllipse, base: base,
		point: [2]*big.Rat{new(big.Rat).Set(point[0]), new(big.Rat).Set(point[1])}}
	for i, c := range h.QuadraticEllipse.Ellipse.Coefficients {
		result.contact[i] = new(big.Rat).Set(c)
	}
	for _, side := range []int{-1, 1} {
		q := AlgebraicCappedLensHalf{Side: side, CapEdges: [2]int{len(sub.edges), len(sub.edges) + 1}}
		for i, c := range a.curves {
			found := false
			for k, v := range c.Partition.FragmentSigns {
				if v != side {
					continue
				}
				q.Fragments[i] = a.facesBySource[i][k]
				f := a.faces[q.Fragments[i]]
				u := f.Outer[len(f.Outer)-1]
				if u.Edge != q.CapEdges[i] {
					return empty, geom.ErrConvergence
				}
				q.CapReversed[i] = u.Reversed == f.Reversed
				found = true
			}
			if !found {
				return empty, geom.ErrConvergence
			}
		}
		if q.CapReversed[0] == q.CapReversed[1] {
			return empty, geom.ErrConvergence
		}
		// The oriented cap starts along the upper curve. With U as
		// the varying coordinate, the interior is below it, so forward
		// traversal gives a normal with the sign of the V coefficient.
		forwardNormal := line[1].Sign()
		if a.curves[0].Axis == 1 {
			forwardNormal = -line[0].Sign()
		}
		if q.CapReversed[0] {
			forwardNormal = -forwardNormal
		}
		if forwardNormal != -side {
			return empty, geom.ErrConvergence
		}
		v, e := validateCappedLensHalf(a, q)
		if e != nil {
			return empty, e
		}
		q.EulerCharacteristic = v
		if side < 0 {
			result.Negative = q
			result.half[0] = q
		} else {
			result.Positive = q
			result.half[1] = q
		}
	}
	return result, nil
}

func validateCappedLensHalf(a AlgebraicFaceArrangement, h AlgebraicCappedLensHalf) (int, error) {
	type use struct {
		edge    int
		reverse bool
	}
	counts := map[int][2]int{}
	vertices := map[int]bool{}
	appendUse := func(u use) {
		count := counts[u.edge]
		i := 0
		if u.reverse {
			i = 1
		}
		count[i]++
		counts[u.edge] = count
		x := a.edges[u.edge]
		vertices[x.Start], vertices[x.End] = true, true
	}
	for _, id := range h.Fragments {
		f := a.faces[id]
		if len(f.Holes) > 0 {
			return 0, geom.ErrConvergence
		}
		for _, u := range f.Outer {
			appendUse(use{u.Edge, u.Reversed != f.Reversed})
		}
	}
	for i, e := range h.CapEdges {
		appendUse(use{e, h.CapReversed[i]})
	}
	for _, n := range counts {
		if n != ([2]int{1, 1}) {
			return 0, geom.ErrConvergence
		}
	}
	x := len(vertices) - len(counts) + 3
	if x != 2 {
		return 0, geom.ErrConvergence
	}
	return x, nil
}
