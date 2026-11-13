package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombineInsetJoinedSplineShells joins B to a rectangle strictly inside one
// face of A. A's interface must have an affine chart and the bounded face of B
// must match its subrectangle exactly, up to chart rescaling or reversal.
// Each input shell must lie on its own side of the supporting plane. Union
// creates an inner wire on A's interface and removes B's interface. The other
// operations have the same regularized behavior as a full-face join.
func CombineInsetJoinedSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellJoinOptions) (Solid, SplineShellJoinHistory, error) {
	h := SplineShellJoinHistory{InterfaceA: -1, InterfaceB: -1}
	if ctx == nil || op > parametric.Difference || len(a.faces) == 0 || len(b.faces) == 0 {
		return Solid{}, h, geom.ErrInvalid
	}
	var ap, bp []splineEmbeddingFace
	pa, e := a.validateSplineRectangleEmbedding(ctx, o.Embedding, &ap)
	if e != nil {
		return Solid{}, h, e
	}
	if _, e := b.validateSplineRectangleEmbedding(ctx, o.Embedding, &bp); e != nil {
		return Solid{}, h, e
	}
	var inset geom.ParameterRectangle
	var corners [4]geom.Vec3
	for i, x := range ap {
		if e := ctx.Err(); e != nil {
			return Solid{}, h, e
		}
		chart, e := splineAffineChart(ctx, x.patch, pa.Faces[i].ProjectionAxes, o.Coincidence)
		if e != nil {
			return Solid{}, h, e
		}
		if chart == nil {
			continue
		}
		as, e := proveSplineShellOnInterfaceSide(ctx, ap, i, x.plane, o.Embedding.Plane)
		if e != nil {
			return Solid{}, h, e
		}
		if as == 0 {
			continue
		}
		for j, y := range bp {
			var from [4]geom.Vec3
			for k, id := range y.vertex {
				from[k] = b.vertices[id]
			}
			d, points, ok := chart.containedRectangle(from)
			if !ok {
				continue
			}
			restricted, e := x.patch.Trim(d)
			if e != nil {
				return Solid{}, h, e
			}
			q, e := intersect.FindExactSplineCoincidence(ctx, restricted, y.patch, o.Coincidence)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return Solid{}, h, e
			}
			if !q.Found {
				continue
			}
			bs, e := proveSplineShellOnInterfaceSide(ctx, bp, j, x.plane, o.Embedding.Plane)
			if e != nil {
				return Solid{}, h, e
			}
			if bs != -as {
				continue
			}
			h.InterfaceA, h.InterfaceB, h.Map = i, j, q.Map
			inset, corners = d, points
			break
		}
		if h.InterfaceA >= 0 {
			break
		}
	}
	if h.InterfaceA < 0 {
		return Solid{}, h, fmt.Errorf("%w: no exact inset interface between the spline shells", geom.ErrConvergence)
	}
	return joinProvedSplineShells(ctx, a, b, op, h, &inset, corners, o.Sew)
}

type affineInsetChart struct {
	domain geom.ParameterRectangle
	exact  *geom.AffineSurfaceChart
}

func splineAffineChart(ctx context.Context, patch *geom.SurfacePatch, axes [2]int, o intersect.SplineCoincidenceOptions) (*affineInsetChart, error) {
	c, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{ProjectionAxes: axes,
		MaxCells: o.MaxCells, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &affineInsetChart{domain: c.Domain(), exact: c}, nil
}

func (c affineInsetChart) containedRectangle(from [4]geom.Vec3) (geom.ParameterRectangle, [4]geom.Vec3, bool) {
	d, ok := c.rectangularProjection(from)
	var corners [4]geom.Vec3
	if !ok || d.UMin <= c.domain.UMin || d.UMax >= c.domain.UMax || d.VMin <= c.domain.VMin || d.VMax >= c.domain.VMax {
		return geom.ParameterRectangle{}, corners, false
	}
	for i, q := range insetCorners(d) {
		corners[i], ok = c.point(q)
		if !ok {
			return geom.ParameterRectangle{}, corners, false
		}
	}
	return d, corners, true
}

func (c affineInsetChart) rectangularProjection(from [4]geom.Vec3) (geom.ParameterRectangle, bool) {
	var uv [4]geom.Vec2
	for i, p := range from {
		q, ok := c.project(p)
		if !ok {
			return geom.ParameterRectangle{}, false
		}
		uv[i] = q
	}
	d := geom.ParameterRectangle{UMin: uv[0].X, UMax: uv[0].X, VMin: uv[0].Y, VMax: uv[0].Y}
	for _, p := range uv[1:] {
		d.UMin, d.UMax = min(d.UMin, p.X), max(d.UMax, p.X)
		d.VMin, d.VMax = min(d.VMin, p.Y), max(d.VMax, p.Y)
	}
	if !d.Valid() {
		return d, false
	}
	for _, q := range insetCorners(d) {
		found := false
		for _, p := range uv {
			if p == q {
				found = true
				break
			}
		}
		if !found {
			return d, false
		}
	}
	return d, true
}

func (c affineInsetChart) point(uv geom.Vec2) (geom.Vec3, bool) {
	r, e := c.exact.EvalExact(ratInset(uv.X), ratInset(uv.Y))
	if e != nil {
		return geom.Vec3{}, false
	}
	p, e := geom.RepresentedExactPoint(r)
	return p, e == nil
}

func (c affineInsetChart) project(p geom.Vec3) (geom.Vec2, bool) {
	q, e := c.exact.Invert(p)
	if e != nil || !q.OnPlane {
		return geom.Vec2{}, false
	}
	pv, e := q.RepresentedUV()
	return pv, e == nil
}

func ratInset(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }

func exactRectEdgeFraction(x, lo, hi float64) *big.Rat {
	l := ratInset(lo)
	return new(big.Rat).Quo(new(big.Rat).Sub(ratInset(x), l), new(big.Rat).Sub(ratInset(hi), l))
}

func exactRectEdgeMix(t *big.Rat, lo, hi float64) (float64, bool) {
	l := ratInset(lo)
	v := new(big.Rat).Mul(t, new(big.Rat).Sub(ratInset(hi), l))
	f, e := geom.RepresentedExactScalar(v.Add(v, l))
	return f, e == nil
}

func insetCorners(d geom.ParameterRectangle) [4]geom.Vec2 {
	return [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
}

func addSplineSewHole(f *SewFace, d geom.ParameterRectangle, corners [4]geom.Vec3) {
	vs, es := len(f.Vertices), len(f.Edges)
	f.Vertices = append(f.Vertices, corners[:]...)
	uv := insetCorners(d)
	loop := make([]Use, 4)
	for i, a := range uv {
		j := (i + 1) % 4
		b := uv[j]
		f.Edges = append(f.Edges, parametric.Edge{Start: vs + i, End: vs + j,
			Curve: surfaceTrimCurve{f.Face.Surface, a, b, nil}, Range: parametric.Interval{Max: 1}})
		loop[3-i] = Use{Edge: es + i, Reversed: true, UVStart: b, UVEnd: a}
	}
	f.Face.Holes = append(f.Face.Holes, loop)
}
