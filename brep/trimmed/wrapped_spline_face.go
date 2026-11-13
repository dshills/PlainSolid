package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// NewWrappedToroid proves global embedding and outward orientation of the
// canonical tensor rational toroid, then sews its certified fundamental
// face into one material shell. Other wrapped supports may use
// WrappedSplineFace directly but do not inherit an embedding guarantee.
func NewWrappedToroid(ctx context.Context, surface *geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, geom.WrappedToroidEmbedding, error) {
	proof, e := geom.CertifyWrappedToroidEmbedding(ctx, surface, o)
	if e != nil {
		return Solid{}, proof, e
	}
	face, e := WrappedSplineFace(ctx, surface, false, o)
	if e != nil {
		return Solid{}, proof, e
	}
	out, _, e := Sew(ctx, []SewFace{face}, SewOptions{Tolerance: tolerance})
	if e != nil {
		return Solid{}, proof, e
	}
	t := out.Topology()
	if t.Vertices != 1 || t.Edges != 2 || t.Faces != 1 || len(t.Shells) != 1 || t.EulerCharacteristic != 0 {
		return Solid{}, proof, geom.ErrDegenerate
	}
	agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil || !agreement.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, proof, e
	}
	return out, proof, nil
}

// WrappedSplineFace makes an independent face on a seam-certified spline
// patch. The full fundamental rectangle is used. Its coincident corners
// are joined only where the supplied wrap certificate identifies them.
// A doubly wrapped, regular embedded support can be sewn as a one-face
// shell. A singly wrapped patch needs the remaining boundary closed by
// other faces. Sewing proves curve agreement and topology, not embedding.
func WrappedSplineFace(ctx context.Context, surface *geom.WrappedSurfacePatch, reversed bool, o geom.SurfacePatchOptions) (SewFace, error) {
	if ctx == nil || surface == nil || surface.FundamentalPatch() == nil {
		return SewFace{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SewFace{}, e
	}
	d := surface.Domain()
	corners := insetCorners(d)
	periods := surface.Periods()
	var out SewFace
	out.Face.Surface, out.Face.Reversed = surface, reversed
	ids := map[geom.Vec2]int{}
	positions := map[geom.Vec3]bool{}
	var vertex [4]int
	for i, p := range corners {
		key := p
		if periods[0] != nil && key.X == d.UMax {
			key.X = d.UMin
		}
		if periods[1] != nil && key.Y == d.VMax {
			key.Y = d.VMin
		}
		if id, ok := ids[key]; ok {
			vertex[i] = id
			continue
		}
		r, e := surface.EvaluateRational(ctx, ratInset(key.X), ratInset(key.Y), geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if e != nil {
			return SewFace{}, e
		}
		point, e := geom.RepresentedExactPoint(r.Point)
		if e != nil {
			return SewFace{}, e
		}
		if positions[point] {
			return SewFace{}, geom.ErrDegenerate
		}
		positions[point] = true
		vertex[i] = len(out.Vertices)
		ids[key] = vertex[i]
		out.Vertices = append(out.Vertices, point)
	}
	for i, a := range corners {
		j := (i + 1) % 4
		b := corners[j]
		out.Edges = append(out.Edges, parametric.Edge{Start: vertex[i], End: vertex[j],
			Curve: surfaceTrimCurve{surface, a, b, nil}, Range: parametric.Interval{Max: 1}})
		out.Face.Outer = append(out.Face.Outer, Use{Edge: i, UVStart: a, UVEnd: b})
	}
	return out, nil
}

func wrappedSurfaceModel(s geom.Surface) (*geom.WrappedSurfacePatch, geom.Affine) {
	base, placement := parametric.SurfaceMapping(s)
	w, _ := base.(*geom.WrappedSurfacePatch)
	return w, placement
}

func numericalWrappedSupport(s geom.Surface) geom.Surface {
	w, m := wrappedSurfaceModel(s)
	if w == nil {
		return s
	}
	return parametric.MapSurface(w.NumericalSurface(), m)
}

// Fundamental seam equality was proved at wrapper construction. Keep this
// shortcut restricted to the same immutable support and placement. Distinct
// exact supports can still match through independent spline-edge identity.
func sameWrappedUVLines(a, b analyticUVLine) bool {
	x, mx := wrappedSurfaceModel(a.surface)
	y, my := wrappedSurfaceModel(b.surface)
	if x == nil || y == nil || mx != my {
		return false
	}
	if x != y && !geom.SameWrappedSurfaceRecipe(x, y) {
		return false
	}
	d := x.Domain()
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	for i, p := range x.Periods() {
		for _, line := range [2]analyticUVLine{a, b} {
			for _, uv := range line.uv {
				if uv[i].Cmp(ratInset(lo[i])) < 0 || uv[i].Cmp(ratInset(hi[i])) > 0 {
					return false
				}
			}
		}
		if equalAnalyticParameterLine(a, b, i, false) {
			continue
		}
		if p == nil || a.uv[0][i].Cmp(a.uv[1][i]) != 0 || b.uv[0][i].Cmp(b.uv[1][i]) != 0 {
			return false
		}
		x, y := a.uv[0][i], b.uv[0][i]
		if x.Cmp(y) > 0 {
			x, y = y, x
		}
		if x.Cmp(ratInset(lo[i])) != 0 || y.Cmp(ratInset(hi[i])) != 0 {
			return false
		}
	}
	return true
}

func exactWrappedModelIso(e parametric.Edge, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, bool, error) {
	x, ok := analyticModelLine(e)
	if !ok {
		return nil, false, nil
	}
	return exactWrappedIso(x, o)
}

func exactWrappedIso(x analyticUVLine, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, bool, error) {
	w, m := wrappedSurfaceModel(x.surface)
	if w == nil {
		return nil, false, nil
	}
	axis := 0
	if x.uv[0][0].Cmp(x.uv[1][0]) != 0 {
		if x.uv[0][1].Cmp(x.uv[1][1]) != 0 {
			return nil, false, nil
		}
		axis = 1
	}
	d := w.Domain()
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	represented := func(r *big.Rat, axis int) (float64, error) {
		f, e := geom.RepresentedExactScalar(r)
		if e == nil && (f < lo[axis] || f > hi[axis]) {
			e = geom.ErrDomain
		}
		return f, e
	}
	fixed, e := represented(x.uv[0][axis], axis)
	if e != nil {
		return nil, false, e
	}
	a, e := represented(x.uv[0][axis^1], axis^1)
	if e != nil {
		return nil, false, e
	}
	b, e := represented(x.uv[1][axis^1], axis^1)
	if e != nil || a == b {
		return nil, false, e
	}
	c, e := w.FundamentalPatch().IsoCurve(axis, fixed, o)
	if e != nil {
		return nil, false, e
	}
	c, e = c.Restrict(min(a, b), max(a, b), o)
	if e != nil {
		return nil, false, e
	}
	if m != geom.Identity() {
		c, e = c.Transform(m, o)
	}
	return c, a > b, e
}

func isCanonicalWrappedLoop(e parametric.Edge) bool {
	x, ok := analyticModelLine(e)
	if !ok {
		return false
	}
	w, _ := wrappedSurfaceModel(x.surface)
	if w == nil {
		return false
	}
	d := w.Domain()
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	for i, p := range w.Periods() {
		if p == nil || x.uv[0][i^1].Cmp(x.uv[1][i^1]) != 0 {
			continue
		}
		a, b := x.uv[0][i], x.uv[1][i]
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		if a.Cmp(ratInset(lo[i])) == 0 && b.Cmp(ratInset(hi[i])) == 0 {
			return true
		}
	}
	return false
}
