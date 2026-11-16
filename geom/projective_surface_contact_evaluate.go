package geom

import (
	"context"
	"math/big"
)

type projectiveContactGraphProof struct {
	arcs     []projectiveContactArcProof
	contours []ProjectiveContactContour
	overlap  ProjectiveSurfaceGraphOverlap
	holes    []ExactSurfaceRectangle
	sources  [2]*SurfacePatch
	cells    []ProjectiveSurfaceHeightCell
}

type projectiveContactArcProof struct {
	arc      SurfacePlaneArc
	common   ExactSurfaceRectangle
	saddle   *projectiveContactSaddleArcProof
	ends     [2][2]*ExactAlgebraicRoot
	vertices [2]int
}

type projectiveContactSaddleArcProof struct {
	axis   int
	center [2]*ExactAlgebraicRoot
	end    [2]*ExactAlgebraicRoot
}

func sealProjectiveContactGraph(cover ProjectiveSurfaceContactCover, arcs []ProjectiveContactArc,
	vertices []ProjectiveContactVertex, contours []ProjectiveContactContour, sources [2]*SurfacePatch) *projectiveContactGraphProof {
	clone := func(x ExactSurfaceRectangle) ExactSurfaceRectangle {
		return ExactSurfaceRectangle{
			ExactParameterInterval{new(big.Rat).Set(x.U.Lower), new(big.Rat).Set(x.U.Upper)},
			ExactParameterInterval{new(big.Rat).Set(x.V.Lower), new(big.Rat).Set(x.V.Upper)},
		}
	}
	p := &projectiveContactGraphProof{arcs: make([]projectiveContactArcProof, len(arcs)), overlap: cover.Atlas.Overlap, sources: sources}
	for i, use := range arcs {
		if use.Saddle {
			saddle := cover.Saddles[use.Cell]
			ray := saddle.Rays[use.Local/2][use.Local%2]
			axis := 0
			if ray.Side[0] == 0 {
				axis = 1
			}
			p.arcs[i].saddle = &projectiveContactSaddleArcProof{axis: axis, center: saddle.Center.Common, end: ray.End.Common}
		} else {
			p.arcs[i] = projectiveContactArcProof{arc: cover.Isolations[use.Cell].Arcs[use.Local], common: clone(cover.Atlas.Cells[use.Cell].Common)}
		}
		for j, v := range use.Vertices {
			p.arcs[i].ends[j] = vertices[v].Common
		}
		p.arcs[i].vertices = use.Vertices
	}
	for _, contour := range contours {
		p.contours = append(p.contours, ProjectiveContactContour{
			Arcs: append([]ProjectiveContactArcUse(nil), contour.Arcs...), Closed: contour.Closed,
		})
	}
	for i := range p.overlap.Areas {
		p.overlap.Areas[i] = clone(p.overlap.Areas[i])
		p.overlap.BStationRatios[i] = new(big.Rat).Set(p.overlap.BStationRatios[i])
	}
	for _, hole := range cover.CommonHoles {
		p.holes = append(p.holes, clone(hole))
	}
	for _, cell := range cover.Atlas.Cells {
		cell.Common = clone(cell.Common)
		p.cells = append(p.cells, cell)
	}
	return p
}

// ArcFreeAxis returns the independent common parameter axis for an original
// contact arc. Its values use A's normalized overlap clock, as do the two
// common coordinates of each vertex. The proof view is immutable.
func (g ProjectiveSurfaceContactGraph) ArcFreeAxis(index int) (int, error) {
	if g.proof == nil || index < 0 || index >= len(g.proof.arcs) {
		return -1, ErrInvalid
	}
	if saddle := g.proof.arcs[index].saddle; saddle != nil {
		return saddle.axis, nil
	}
	return 1 - g.proof.arcs[index].arc.Certificate.FixedParameterAxis, nil
}

// EvaluateArcRational binds an exact rational station to the unique point of
// an original intersection arc. It returns both owners' exact parameter
// identities, including an irrational dependent coordinate when needed. A
// station with no point on the requested arc returns ErrDomain. The query
// uses the private proof rather than mutable display or result fields.
func (g ProjectiveSurfaceContactGraph) EvaluateArcRational(ctx context.Context, index int, station *big.Rat,
	o SurfacePatchEvaluationOptions) (r ProjectiveContactVertex, operations int, err error) {
	if ctx == nil || station == nil || g.proof == nil || index < 0 || index >= len(g.proof.arcs) {
		return r, 0, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, 0, err
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, 0, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	if err := b.check(station); err != nil {
		return r, 0, err
	}
	proof := g.proof.arcs[index]
	if proof.saddle != nil {
		common, err := proof.saddle.evaluate(station, b)
		if err != nil {
			return r, 0, err
		}
		q, err := projectiveGraphContactVertex(ctx, common, g.proof.overlap, g.proof.holes, b)
		if err != nil {
			return r, 0, err
		}
		if err := ctx.Err(); err != nil {
			return r, 0, err
		}
		return q, 0, nil
	}
	arc := proof.arc
	dep := arc.Certificate.FixedParameterAxis
	free := 1 - dep
	section := [2]ExactParameterInterval{proof.common.U, proof.common.V}
	a := boundedRatArithmetic{b: b}
	commonWidth := a.sub(section[free].Upper, section[free].Lower)
	f := a.quo(a.sub(station, section[free].Lower), commonWidth)
	domain := projectiveParameterIntervals(arc.source.Domain())
	if a.err != nil {
		return r, 0, a.err
	}
	if f.Cmp(domain[free].lo) < 0 || f.Cmp(domain[free].hi) > 0 {
		return r, 0, ErrDomain
	}
	t := a.quo(a.sub(f, domain[free].lo), a.sub(domain[free].hi, domain[free].lo))
	if a.err != nil {
		return r, 0, a.err
	}
	p := arc.source
	n := [2]int{p.countU, p.countV}
	h := make([]*big.Rat, n[dep])
	equation := exactPlaneCoefficients(arc.plane)
	for i := range h {
		if err := ctx.Err(); err != nil {
			return r, 0, err
		}
		curve := make([]exactHomogeneous, n[free])
		for j := range curve {
			k := i*p.countV + j
			if dep == 1 {
				k = j*p.countV + i
			}
			curve[j] = p.net[k]
		}
		point, err := restrictExactBezier(curve, t, t, b)
		if err != nil {
			return r, 0, err
		}
		h[i] = exactPlaneHomogeneousValue(equation, point[0])
		if err := b.check(h[i]); err != nil {
			return r, 0, err
		}
	}
	power, err := bernsteinToPower(h, b)
	if err != nil {
		return r, 0, err
	}
	if len(power) == 0 {
		return r, 0, ErrConvergence
	}
	lo, hi := new(big.Rat), big.NewRat(1, 1)
	// The retained local arc proves strict monotonicity in dep. Opposed
	// signs then prove a unique root, including rational endpoint equality.
	switch first, last := h[0].Sign(), h[len(h)-1].Sign(); {
	case first == 0:
		hi = lo
	case last == 0:
		lo = hi
	case first == last:
		return r, 0, ErrDomain
	}
	dependent := &ExactGraphLineRoot{polynomial: power, bounds: [2]*big.Rat{lo, hi}}
	width := a.sub(section[dep].Upper, section[dep].Lower)
	offset := a.add(section[dep].Lower, a.mul(width, domain[dep].lo))
	scale := a.mul(width, a.sub(domain[dep].hi, domain[dep].lo))
	if a.err != nil {
		return r, 0, a.err
	}
	dependent, err = affineMapRoot(dependent, offset, scale, b)
	if err != nil {
		return r, 0, err
	}
	x := new(big.Rat).Set(station)
	constant := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}, bounds: [2]*big.Rat{x, x}}
	var common [2]*ExactAlgebraicRoot
	common[dep], common[free] = &ExactAlgebraicRoot{root: dependent}, &ExactAlgebraicRoot{root: constant}
	q, err := projectiveGraphContactVertex(ctx, common, g.proof.overlap, g.proof.holes, b)
	if err != nil {
		return ProjectiveContactVertex{}, 0, err
	}
	if err := ctx.Err(); err != nil {
		return ProjectiveContactVertex{}, 0, err
	}
	return q, 0, nil
}

// The free axis is the exact side coordinate that ends this ray. The
// secondary coordinate varies affinely with the same station fraction,
// even when the other boundary coordinate is an irrational algebraic root.
func (p *projectiveContactSaddleArcProof) evaluate(station *big.Rat, b *polynomialBudget) (out [2]*ExactAlgebraicRoot, err error) {
	base := p.center[p.axis].root.bounds[0]
	terminal := p.end[p.axis].root.bounds[0]
	if base.Cmp(terminal) < 0 && (station.Cmp(base) < 0 || station.Cmp(terminal) > 0) ||
		base.Cmp(terminal) > 0 && (station.Cmp(terminal) < 0 || station.Cmp(base) > 0) {
		return out, ErrDomain
	}
	a := boundedRatArithmetic{b: b}
	fraction := a.quo(a.sub(station, base), a.sub(terminal, base))
	complement := a.sub(big.NewRat(1, 1), fraction)
	for i, root := range p.end {
		offset := a.mul(complement, p.center[i].root.bounds[0])
		if a.err != nil {
			return out, a.err
		}
		q, err := affineMapRoot(root.root, offset, fraction, b)
		if err != nil {
			return out, err
		}
		out[i] = &ExactAlgebraicRoot{root: q}
	}
	return out, nil
}
