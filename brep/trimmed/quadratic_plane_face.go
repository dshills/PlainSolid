package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// QuadraticPlaneFaceSection proves the unique horizontal-plane branch on one
// original rectangular spline face. The represented support is polynomial,
// X/Y are affine, Z=a0+a1*s+a2*s²+b*t in normalized free/dependent chart
// coordinates, b!=0.
// The quadratic graph t=q(s) stays strictly inside (0,1) by its exact Bernstein
// controls. Thus it crosses precisely the two original free-axis edges.
// Unsupported rational weights, other powers, knot breaks, and vertex contacts
// remain unresolved. It is a face certificate, not by itself shell permission.
type QuadraticPlaneFaceSection struct {
	source     Solid
	face       int
	patch      *geom.SurfacePatch
	trim       ExactQuadraticUV
	height     *big.Rat
	sign       int
	freeAxis   int
	edges      [2]int
	parameters [2]*big.Rat
}

func (p *QuadraticPlaneFaceSection) Trim() ExactQuadraticUV { return p.trim }
func (p *QuadraticPlaneFaceSection) Face() int              { return p.face }
func (p *QuadraticPlaneFaceSection) Edges() [2]int          { return p.edges }
func (p *QuadraticPlaneFaceSection) Parameters() [2]*big.Rat {
	return [2]*big.Rat{new(big.Rat).Set(p.parameters[0]), new(big.Rat).Set(p.parameters[1])}
}

// FreeAxis is the original UV coordinate traversed from minimum to maximum.
func (p *QuadraticPlaneFaceSection) FreeAxis() int { return p.freeAxis }

// DependentAxis identifies the original coordinate expressed by the quadratic.
func (p *QuadraticPlaneFaceSection) DependentAxis() int { return 1 - p.freeAxis }

// AboveSign is the exact sign of Z-height when the dependent coordinate is
// increased above the trim. The legacy U-free wrapper always uses original V.
func (p *QuadraticPlaneFaceSection) AboveSign() int { return p.sign }

// CertifyQuadraticPlaneFaceSection preserves the legacy free-U convention.
func CertifyQuadraticPlaneFaceSection(ctx context.Context, s Solid, face int, height float64, o SplineRectangleEmbeddingOptions) (*QuadraticPlaneFaceSection, error) {
	return CertifyQuadraticPlaneFaceSectionAxis(ctx, s, face, height, 0, o)
}

// CertifyQuadraticPlaneFaceSectionAxis chooses free U(0) or free V(1) without
// reparameterizing the original support. The unique trim crosses the two
// original edges of that free coordinate; Parameters retains each model edge's
// own declared range and direction. Knot crossings remain unsupported.
func CertifyQuadraticPlaneFaceSectionAxis(ctx context.Context, s Solid, face int, height float64, freeAxis int, o SplineRectangleEmbeddingOptions) (*QuadraticPlaneFaceSection, error) {
	if freeAxis < 0 || freeAxis > 1 {
		return nil, geom.ErrInvalid
	}
	if ctx == nil || face < 0 || face >= len(s.faces) || !finite(height) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f, _, e := s.splineEmbeddingFace(ctx, s.faces[face], o)
	if e != nil {
		return nil, e
	}
	q, sign, e := quadraticPlaneTrimAxis(f.patch, new(big.Rat).SetFloat64(height), freeAxis)
	if e != nil {
		return nil, e
	}
	r := &QuadraticPlaneFaceSection{source: s, face: face, patch: f.patch, trim: q, height: new(big.Rat).SetFloat64(height), sign: sign, freeAxis: freeAxis, edges: [2]int{f.edge[geom.SurfaceEdge(2*freeAxis)], f.edge[geom.SurfaceEdge(2*freeAxis+1)]}}
	ends, _ := q.Endpoints()
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
func quadraticPlaneTrim(p *geom.SurfacePatch, height *big.Rat) (ExactQuadraticUV, int, error) {
	return quadraticPlaneTrimAxis(p, height, 0)
}
func quadraticPlaneTrimAxis(p *geom.SurfacePatch, height *big.Rat, freeAxis int) (ExactQuadraticUV, int, error) {
	var zero ExactQuadraticUV
	if freeAxis < 0 || freeAxis > 1 {
		return zero, 0, geom.ErrInvalid
	}
	h, _, ok := p.RationalBezierCoefficients(2)
	if !ok {
		return zero, 0, geom.ErrConvergence
	}
	if freeAxis == 1 {
		transposed := make([][][4]*big.Rat, len(h[0]))
		for i := range transposed {
			transposed[i] = make([][4]*big.Rat, len(h))
			for j := range h {
				transposed[i][j] = h[j][i]
			}
		}
		h = transposed
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
			for axis := 0; axis < 2; axis++ {
				if i+j > 1 && c[axis].Sign() != 0 {
					return zero, 0, geom.ErrConvergence
				}
			}
			if !((j == 0 && i <= 2) || (i == 0 && j == 1)) && c[2].Sign() != 0 {
				return zero, 0, geom.ErrConvergence
			}
		}
	}
	c := func(i, j int) *big.Rat {
		if i >= len(h) || j >= len(h[i]) {
			return new(big.Rat)
		}
		return new(big.Rat).Quo(h[i][j][2], w)
	}
	b := c(0, 1)
	if b.Sign() == 0 {
		return zero, 0, geom.ErrConvergence
	}
	power := [3]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(height, c(0, 0)), b), new(big.Rat).Quo(new(big.Rat).Neg(c(1, 0)), b), new(big.Rat).Quo(new(big.Rat).Neg(c(2, 0)), b)}
	values := [3]*big.Rat{new(big.Rat).Set(power[0]), new(big.Rat).Add(power[0], new(big.Rat).Quo(power[1], big.NewRat(2, 1))), new(big.Rat).Add(new(big.Rat).Add(power[0], power[1]), power[2])}
	d := p.Domain()
	if freeAxis == 1 {
		d.UMin, d.VMin = d.VMin, d.UMin
		d.UMax, d.VMax = d.VMax, d.UMax
	}
	u0, v0 := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)
	du := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), u0)
	dv := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), v0)
	var control [3][3]*big.Rat
	for i, v := range values {
		if v.Sign() <= 0 || v.Cmp(big.NewRat(1, 1)) >= 0 {
			return zero, 0, geom.ErrConvergence
		}
		control[i] = [3]*big.Rat{new(big.Rat).Add(u0, new(big.Rat).Mul(du, big.NewRat(int64(i), 2))), new(big.Rat).Add(v0, new(big.Rat).Mul(dv, v)), big.NewRat(1, 1)}
	}
	if freeAxis == 1 {
		for i := range control {
			control[i][0], control[i][1] = control[i][1], control[i][0]
		}
	}
	q, e := NewExactQuadraticUV(control)
	return q, b.Sign(), e
}
