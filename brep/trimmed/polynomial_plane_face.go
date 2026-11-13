package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PolynomialPlaneFaceSection certifies a regular single-span polynomial graph
// section of degree at most four. The original XY coordinates must be affine;
// Z is a polynomial of the free coordinate plus a nonzero constant multiple of
// the dependent coordinate. Native trim controls must export losslessly.
type PolynomialPlaneFaceSection struct {
	source     Solid
	face       int
	patch      *geom.SurfacePatch
	trim       SplineUV
	height     *big.Rat
	sign       int
	freeAxis   int
	edges      [2]int
	parameters [2]*big.Rat
}

func (p *PolynomialPlaneFaceSection) Trim() SplineUV { return p.trim }
func (p *PolynomialPlaneFaceSection) Face() int      { return p.face }
func (p *PolynomialPlaneFaceSection) Edges() [2]int  { return p.edges }
func (p *PolynomialPlaneFaceSection) Parameters() [2]*big.Rat {
	return [2]*big.Rat{new(big.Rat).Set(p.parameters[0]), new(big.Rat).Set(p.parameters[1])}
}

// FreeAxis is the original UV coordinate traversed from minimum to maximum.
func (p *PolynomialPlaneFaceSection) FreeAxis() int { return p.freeAxis }

// DependentAxis identifies the original coordinate expressed by the polynomial.
func (p *PolynomialPlaneFaceSection) DependentAxis() int { return 1 - p.freeAxis }

// AboveSign is the exact sign of Z-height when the dependent coordinate is
// increased above the trim. The legacy U-free wrapper always uses original V.
func (p *PolynomialPlaneFaceSection) AboveSign() int { return p.sign }

// CertifyPolynomialPlaneFaceSection selects the free-U convention.
func CertifyPolynomialPlaneFaceSection(ctx context.Context, s Solid, face int, height float64, o SplineRectangleEmbeddingOptions) (*PolynomialPlaneFaceSection, error) {
	return CertifyPolynomialPlaneFaceSectionAxis(ctx, s, face, height, 0, o)
}

// CertifyPolynomialPlaneFaceSectionAxis chooses free U(0) or free V(1) without
// reparameterizing the original support. The unique trim crosses the two
// original edges of that free coordinate; Parameters retains each model edge's
// own declared range and direction. Knot crossings remain unsupported.
func CertifyPolynomialPlaneFaceSectionAxis(ctx context.Context, s Solid, face int, height float64, freeAxis int, o SplineRectangleEmbeddingOptions) (*PolynomialPlaneFaceSection, error) {
	if !finite(height) {
		return nil, geom.ErrInvalid
	}
	return certifyPolynomialPlaneFaceSection(ctx, s, face, horizontalPolynomialPlane(ratInset(height)), freeAxis, o)
}

func certifyPolynomialPlaneFaceSection(ctx context.Context, s Solid, face int, plane polynomialPlaneEquation, freeAxis int, o SplineRectangleEmbeddingOptions) (*PolynomialPlaneFaceSection, error) {
	if freeAxis < 0 || freeAxis > 1 {
		return nil, geom.ErrInvalid
	}
	if ctx == nil || face < 0 || face >= len(s.faces) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f, _, e := s.splineEmbeddingFace(ctx, s.faces[face], o)
	if e != nil {
		return nil, e
	}
	q, sign, e := polynomialPlaneTrimEquation(f.patch, plane, freeAxis)
	if e != nil {
		return nil, e
	}
	r := &PolynomialPlaneFaceSection{source: s, face: face, patch: f.patch, trim: q, height: new(big.Rat).Set(plane.offset), sign: sign, freeAxis: freeAxis, edges: [2]int{f.edge[geom.SurfaceEdge(2*freeAxis)], f.edge[geom.SurfaceEdge(2*freeAxis+1)]}}
	ends, _, e := polynomialPlaneEndpoints(q)
	if e != nil {
		return nil, e
	}
	for i, id := range r.edges {
		var use *Use
		for j := range s.faces[face].Outer {
			if s.faces[face].Outer[j].Edge == id {
				use = &s.faces[face].Outer[j]
			}
		}
		if use == nil {
			return nil, geom.ErrConvergence
		}
		start, end := use.UVStart.Y, use.UVEnd.Y
		if freeAxis == 1 {
			start, end = use.UVStart.X, use.UVEnd.X
		}
		if start == end {
			return nil, geom.ErrConvergence
		}
		alpha := new(big.Rat).Quo(new(big.Rat).Sub(ends[i][1-freeAxis], ratInset(start)), new(big.Rat).Sub(ratInset(end), ratInset(start)))
		if alpha.Sign() <= 0 || alpha.Cmp(big.NewRat(1, 1)) >= 0 {
			return nil, geom.ErrConvergence
		}
		if use.Reversed {
			alpha.Sub(big.NewRat(1, 1), alpha)
		}
		edge := s.edges[id]
		r.parameters[i] = new(big.Rat).Add(new(big.Rat).SetFloat64(edge.Range.Min), new(big.Rat).Mul(alpha, new(big.Rat).Sub(new(big.Rat).SetFloat64(edge.Range.Max), new(big.Rat).SetFloat64(edge.Range.Min))))
	}
	return r, nil
}
func polynomialChoose(n, k int) int64 {
	if k < 0 || k > n {
		return 0
	}
	x := int64(1)
	for i := 1; i <= k; i++ {
		x = x * int64(n-i+1) / int64(i)
	}
	return x
}
func polynomialPlaneTrimAxis(p *geom.SurfacePatch, height *big.Rat, freeAxis int) (SplineUV, int, error) {
	return polynomialPlaneTrimEquation(p, horizontalPolynomialPlane(height), freeAxis)
}

func polynomialPlaneTrimEquation(p *geom.SurfacePatch, plane polynomialPlaneEquation, freeAxis int) (SplineUV, int, error) {
	var zero SplineUV
	if freeAxis < 0 || freeAxis > 1 {
		return zero, 0, geom.ErrInvalid
	}
	h, _, ok := p.RationalBezierCoefficients(4)
	if !ok {
		return zero, 0, geom.ErrConvergence
	}
	if freeAxis == 1 {
		transpose := make([][][4]*big.Rat, len(h[0]))
		for i := range transpose {
			transpose[i] = make([][4]*big.Rat, len(h))
			for j := range h {
				transpose[i][j] = h[j][i]
			}
		}
		h = transpose
	}
	w := h[0][0][3]
	if w.Sign() <= 0 {
		return zero, 0, geom.ErrConvergence
	}
	for i, row := range h {
		for j, c := range row {
			if i+j > 0 && c[3].Sign() != 0 {
				return zero, 0, geom.ErrConvergence
			}
			for _, axis := range plane.axes {
				if i+j > 1 && c[axis].Sign() != 0 {
					return zero, 0, geom.ErrConvergence
				}
			}
			if !(j == 0 || i == 0 && j == 1) && c[plane.dropped].Sign() != 0 {
				return zero, 0, geom.ErrConvergence
			}
		}
	}
	c := func(i, j int) *big.Rat {
		if i >= len(h) || j >= len(h[i]) {
			return new(big.Rat)
		}
		x := h[i][j]
		return new(big.Rat).Quo(plane.dot([3]*big.Rat{x[0], x[1], x[2]}), w)
	}
	b := c(0, 1)
	if b.Sign() == 0 {
		return zero, 0, geom.ErrConvergence
	}
	power := make([]*big.Rat, 5)
	degree := 0
	for i := range power {
		a := new(big.Rat).Neg(c(i, 0))
		if i == 0 {
			a.Add(a, plane.offset)
		}
		power[i] = a.Quo(a, b)
		if power[i].Sign() != 0 {
			degree = i
		}
	}
	// Degree three is elevated to four so the affine free coordinate can be
	// represented by binary64 poles at quarters instead of nonbinary thirds.
	n := 2
	if degree > 2 {
		n = 4
	}
	d := p.Domain()
	if freeAxis == 1 {
		d.UMin, d.VMin = d.VMin, d.UMin
		d.UMax, d.VMax = d.VMax, d.UMax
	}
	controls := make([][3]*big.Rat, n+1)
	du := new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin))
	dv := new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))
	for i := range controls {
		value := new(big.Rat)
		for j := 0; j <= i; j++ {
			value.Add(value, new(big.Rat).Mul(power[j], big.NewRat(polynomialChoose(i, j), polynomialChoose(n, j))))
		}
		if value.Sign() <= 0 || value.Cmp(big.NewRat(1, 1)) >= 0 {
			return zero, 0, geom.ErrConvergence
		}
		controls[i] = [3]*big.Rat{new(big.Rat).Add(ratInset(d.UMin), new(big.Rat).Mul(du, big.NewRat(int64(i), int64(n)))), new(big.Rat).Add(ratInset(d.VMin), new(big.Rat).Mul(dv, value)), big.NewRat(1, 1)}
		if freeAxis == 1 {
			controls[i][0], controls[i][1] = controls[i][1], controls[i][0]
		}
	}
	q, e := polynomialNativeTrim(controls)
	return q, b.Sign(), e
}
func polynomialNativeTrim(h [][3]*big.Rat) (SplineUV, error) {
	var zero SplineUV
	if len(h) < 2 || len(h) > 5 {
		return zero, geom.ErrConvergence
	}
	poles := make([]geom.Vec3, len(h))
	for i, p := range h {
		if p[2].Cmp(big.NewRat(1, 1)) != 0 {
			return zero, geom.ErrConvergence
		}
		x, e := geom.RepresentedExactScalar(p[0])
		if e != nil {
			return zero, e
		}
		y, e := geom.RepresentedExactScalar(p[1])
		if e != nil {
			return zero, e
		}
		poles[i] = geom.Vec3{X: x, Y: y}
	}
	knots := make([]float64, 2*len(h))
	for i := len(h); i < len(knots); i++ {
		knots[i] = 1
	}
	curve, e := geom.NewBSpline(len(h)-1, poles, knots, nil)
	if e != nil {
		return zero, e
	}
	return SplineUV{Curve: curve, Range: UVInterval{Max: 1}}, nil
}
func polynomialPlaneControls(c PCurve) ([][3]*big.Rat, error) {
	switch q := c.(type) {
	case ExactQuadraticUV:
		h := q.HomogeneousControls()
		for _, p := range h {
			if p[2].Cmp(big.NewRat(1, 1)) != 0 {
				return nil, geom.ErrConvergence
			}
		}
		return h[:], nil
	case SplineUV:
		if q.Curve == nil || q.Range != (UVInterval{Max: 1}) || q.Curve.Degree() > 4 || len(q.Curve.Poles()) != q.Curve.Degree()+1 {
			return nil, geom.ErrConvergence
		}
		for _, w := range q.Curve.Weights() {
			if w != 1 {
				return nil, geom.ErrConvergence
			}
		}
		poles := q.Curve.Poles()
		h := make([][3]*big.Rat, len(poles))
		for i, p := range poles {
			if p.Z != 0 {
				return nil, geom.ErrConvergence
			}
			j := i
			if q.Reversed {
				j = len(poles) - 1 - i
			}
			h[j] = [3]*big.Rat{ratInset(p.X), ratInset(p.Y), big.NewRat(1, 1)}
		}
		return h, nil
	}
	return nil, geom.ErrConvergence
}
func polynomialPlaneEndpoints(c PCurve) ([2][2]*big.Rat, [2]geom.Vec2, error) {
	var exact [2][2]*big.Rat
	var display [2]geom.Vec2
	h, e := polynomialPlaneControls(c)
	if e != nil {
		return exact, display, e
	}
	for i, j := range []int{0, len(h) - 1} {
		exact[i] = [2]*big.Rat{new(big.Rat).Set(h[j][0]), new(big.Rat).Set(h[j][1])}
		display[i].X, e = geom.RepresentedExactScalar(exact[i][0])
		if e != nil {
			return exact, display, e
		}
		display[i].Y, e = geom.RepresentedExactScalar(exact[i][1])
		if e != nil {
			return exact, display, e
		}
	}
	return exact, display, nil
}
