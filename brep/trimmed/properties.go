package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type differentiableTrim interface {
	PCurve
	DerivativeUV(float64) (geom.Vec2, error)
}

// MassProperties integrates the original surfaces over their UV trim
// domains, with straight or differentiable curved trims. The method integrates
// the polygon of trim chords and adds oriented strips between each chord and
// its true curve. Signed chart area cancels overlapping or folded strips.
// The caller must provide an embedded boundary and valid surface charts on
// all evaluated UV points.
// No model-space tessellation is used. A wrapped support may use its
// lossless-coefficient numerical evaluator for quadrature. Exactly certified affine paired UV
// paths skip zero-width strips only when their endpoints equal the face chord.
// Optional proofs share a four-million-operation/256-segment limit; exhausted
// or unproved cases retain ordinary quadrature. A private bounded 16384-sample
// cache avoids reevaluating trim points/derivatives within this call. Neither
// optimization changes topology or replaces curved trims with chord geometry.
// Spline-supported chord triangles are clipped at U/V knot lines before
// quadrature. Exact Bézier trims and spline wall trims can also split their
// correction strips at exactly represented knot events. The integration
// preprocessing is bounded to 8192 pieces.
func (s Solid) MassProperties(ctx context.Context, o parametric.IntegrationOptions) (parametric.Properties, error) {
	if len(s.vertices) == 0 {
		return parametric.Properties{}, geom.ErrDegenerate
	}
	if ctx == nil {
		return parametric.Properties{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Properties{}, e
	}
	if !finite(o.RelativeTolerance) || o.RelativeTolerance < 0 || o.RelativeTolerance >= 1 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 512 || o.MaxSamples < 0 {
		return parametric.Properties{}, fmt.Errorf("trimmed: invalid integration options")
	}
	proofs := massAffineProofs{left: 4000000, values: map[massProofKey]geom.TubeAffineLineCertificate{}}
	stripProofs := massKnotStripProofs{left: 4000000}
	memo := massTrimMemo{values: map[massTrimKey]massTrimSample{}}
	trimID := 0
	var faces []parametric.Face
	var maps []*triangleSurface
	appendTriangle := func(f Face, t triangleSurface) {
		faces = append(faces, parametric.Face{Surface: t, U: parametric.Interval{Max: 1}, V: parametric.Interval{Max: 1}, Reversed: f.Reversed})
		maps = append(maps, &t)
	}
	for i, f := range s.faces {
		uKnots, vKnots := massSplineKnotLines(f.Surface)
		w, _ := wrappedSurfaceModel(f.Surface)
		f.Surface = numericalWrappedSupport(f.Surface)
		if w == nil {
			base, placement := parametric.SurfaceMapping(f.Surface)
			if exact, ok := base.(*geom.ExactBezierSurface); ok {
				f.Surface = parametric.MapSurface(exact.NumericalSurface(), placement)
			}
		}
		if d, ok := straightFullWrappedRectangle(w, f); ok {
			// The original UV rectangle needs no Duffy map or trim strips.
			// Split directly at spline knots so quadrature never straddles
			// a change in the rational polynomial piece.
			u, v := w.FundamentalPatch().KnotBreaks()
			for j := 0; j+1 < len(u); j++ {
				for k := 0; k+1 < len(v); k++ {
					if e := ctx.Err(); e != nil {
						return parametric.Properties{}, e
					}
					if len(faces) >= 8192 {
						return parametric.Properties{}, parametric.ErrBudget
					}
					faces = append(faces, parametric.Face{Surface: f.Surface,
						U: parametric.Interval{Min: max(d.UMin, u[j]), Max: min(d.UMax, u[j+1])},
						V: parametric.Interval{Min: max(d.VMin, v[k]), Max: min(d.VMax, v[k+1])}, Reversed: f.Reversed})
					maps = append(maps, nil)
				}
			}
			continue
		}
		if len(s.seeds[i]) == 0 && (s.sampledAnnularCaps || s.sampledAnalyticCaps) {
			// Signed planar polygon fans of the original chord rings
			// cancel in the overlap. The corrections below use the actual
			// original curved trims, irrespective of display subdivision.
			for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
				for j := 1; j+1 < len(loop); j++ {
					p := [3]geom.Vec2{loop[0].UVStart, loop[j].UVStart, loop[j+1].UVStart}
					appendTriangle(f, triangleSurface{surface: f.Surface, p: p})
				}
			}
		}
		for _, tri := range s.seeds[i] {
			pieces, e := splitMassTriangleAtKnots(ctx, tri.uv, uKnots, vKnots, 8192-len(faces))
			if e != nil {
				return parametric.Properties{}, e
			}
			for _, piece := range pieces {
				appendTriangle(f, triangleSurface{surface: f.Surface, p: piece})
			}
		}
		// Add the oriented strips between each curved edge and its chord.
		// This chain depends only on the original wire; it avoids integrating
		// mixed boundary terms from a triangle blending multiple p-curves.
		for _, l := range append([][]Use{f.Outer}, f.Holes...) {
			for _, side := range l {
				if side.Curve == nil {
					continue
				}
				zero, e := proofs.zeroStrip(ctx, side)
				if e != nil {
					return parametric.Properties{}, e
				}
				if zero {
					continue
				}
				c, ok := side.Curve.(differentiableTrim)
				if !ok {
					return parametric.Properties{}, fmt.Errorf("trimmed: direct integration needs trim derivatives")
				}
				strips, e := stripProofs.split(ctx, f.Surface, side, c, [2][]float64{uKnots, vKnots})
				if e != nil {
					return parametric.Properties{}, e
				}
				for j, strip := range strips {
					if j > 0 {
						fan := [3]geom.Vec2{side.UVStart, strip.p[0], strip.p[1]}
						sign, err := geom.Orient2D(fan[0], fan[1], fan[2])
						if err != nil {
							return parametric.Properties{}, err
						}
						if sign != 0 {
							pieces, err := splitMassTriangleAtKnots(ctx, fan, uKnots, vKnots, 8192-len(faces))
							if err != nil {
								return parametric.Properties{}, err
							}
							for _, p := range pieces {
								appendTriangle(f, triangleSurface{surface: f.Surface, p: p})
							}
						}
					}
					if len(faces) >= 8192 {
						return parametric.Properties{}, parametric.ErrBudget
					}
					strip.surface = f.Surface
					strip.strip = massMemoTrim{source: strip.strip, cache: &memo, id: trimID}
					trimID++
					appendTriangle(f, strip)
				}
			}
		}
	}
	return parametric.IntegrateOrientedBoundary(ctx, faces, s.vertices[0], o, func(i int, u, v float64) (int, error) {
		if maps[i] == nil {
			return 1, nil
		}
		_, a, b, e := maps[i].uv(u, v, true)
		d := a.X*b.Y - a.Y*b.X
		if !finite(d) {
			return 0, geom.ErrInvalid
		}
		if d < 0 {
			return -1, e
		}
		if d > 0 {
			return 1, e
		}
		return 0, e
	})
}

func straightFullWrappedRectangle(w *geom.WrappedSurfacePatch, f Face) (geom.ParameterRectangle, bool) {
	if w == nil || len(f.Holes) != 0 || len(f.Outer) < 4 {
		return geom.ParameterRectangle{}, false
	}
	d := w.Domain()
	corners := insetCorners(d)
	for start, u := range f.Outer {
		if u.UVStart != corners[0] {
			continue
		}
		side := 0
		for j := range f.Outer {
			use := f.Outer[(start+j)%len(f.Outer)]
			if side >= 4 || use.Curve != nil {
				return geom.ParameterRectangle{}, false
			}
			a, b := corners[side], corners[(side+1)%4]
			p, q := use.UVStart, use.UVEnd
			if a.X == b.X {
				if p.X != a.X || q.X != a.X || !directedSidePiece(a.Y, b.Y, p.Y, q.Y) {
					return geom.ParameterRectangle{}, false
				}
			} else if p.Y != a.Y || q.Y != a.Y || !directedSidePiece(a.X, b.X, p.X, q.X) {
				return geom.ParameterRectangle{}, false
			}
			if q == b {
				side++
			}
		}
		return d, side == 4
	}
	return geom.ParameterRectangle{}, false
}

func directedSidePiece(a, b, p, q float64) bool {
	return a < b && a <= p && p < q && q <= b || a > b && a >= p && p > q && q >= b
}

// Map a square to a UV triangle, or to a signed strip between a boundary
// curve (u=1) and its chord (u=0), with v increasing along the curve.
type triangleSurface struct {
	surface geom.Surface
	p       [3]geom.Vec2
	strip   differentiableTrim
}

func add2(a, b geom.Vec2) geom.Vec2           { return geom.Vec2{X: a.X + b.X, Y: a.Y + b.Y} }
func sub2(a, b geom.Vec2) geom.Vec2           { return geom.Vec2{X: a.X - b.X, Y: a.Y - b.Y} }
func scale2(a geom.Vec2, k float64) geom.Vec2 { return geom.Vec2{X: k * a.X, Y: k * a.Y} }

func (s triangleSurface) uv(u, v float64, derive bool) (p, du, dv geom.Vec2, err error) {
	if s.strip != nil {
		t := v
		line := mix(s.p[0], s.p[1], t)
		curve, e := s.strip.EvalUV(t)
		if e != nil {
			return p, du, dv, e
		}
		p = mix(line, curve, u)
		if derive {
			d, e := s.strip.DerivativeUV(t)
			if e != nil {
				return p, du, dv, e
			}
			du = sub2(curve, line)
			dv = mix(sub2(s.p[1], s.p[0]), d, u)
		}
		if !uvFinite(p) || !uvFinite(du) || !uvFinite(dv) {
			return p, du, dv, geom.ErrInvalid
		}
		return p, du, dv, nil
	}
	l, a, b := [3]float64{1 - u, u * (1 - v), u * v}, [3]float64{-1, 1 - v, v}, [3]float64{0, -u, u}
	for i, w := range l {
		p = add2(p, scale2(s.p[i], w))
		if derive {
			du = add2(du, scale2(s.p[i], a[i]))
			dv = add2(dv, scale2(s.p[i], b[i]))
		}
	}
	if !uvFinite(p) || !uvFinite(du) || !uvFinite(dv) {
		return p, du, dv, geom.ErrInvalid
	}
	return p, du, dv, nil
}
func (s triangleSurface) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.uv(u, v, false)
	if e != nil {
		return geom.Vec3{}, e
	}
	return s.surface.Eval(p.X, p.Y)
}
func (s triangleSurface) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	p, x, y, e := s.uv(u, v, true)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	a, b, e := s.surface.Partials(p.X, p.Y)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	if x.X*y.Y-x.Y*y.X == 0 {
		// The chart strip contributes zero area at this sample. Applying
		// the surface Jacobian separately to two parallel chart vectors
		// can otherwise introduce a spurious normal through roundoff.
		return geom.Vec3{}, geom.Vec3{}, nil
	}
	return a.Scale(x.X).Add(b.Scale(x.Y)), a.Scale(y.X).Add(b.Scale(y.Y)), nil
}
