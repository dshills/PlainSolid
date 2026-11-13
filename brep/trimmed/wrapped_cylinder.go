package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// NewWrappedCylinder builds the proved canonical rational extrusion with an
// exact planar disk at each end. Face order is lateral, lower cap, upper cap.
// Each rim is split into its four exact rational quadrant edges and shared by
// both incident faces. The two seam uses identify the same vertical edge.
func NewWrappedCylinder(ctx context.Context, w *geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, geom.WrappedCylinderEmbedding, error) {
	proof, e := geom.CertifyWrappedCylinderEmbedding(ctx, w, o)
	if e != nil {
		return Solid{}, proof, e
	}
	result, e := buildCanonicalWrappedFrustum(ctx, w, tolerance, o,
		[2]*big.Rat{proof.LowerZ, proof.UpperZ}, proof.QuarterParameters)
	return result, proof, e
}

// NewWrappedFrustum caps a certified homothetic rational frustum with the
// exact lower and upper profiles. It preserves the original lateral chart.
func NewWrappedFrustum(ctx context.Context, w *geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, geom.WrappedFrustumEmbedding, error) {
	proof, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return Solid{}, proof, e
	}
	result, e := buildCanonicalWrappedFrustum(ctx, w, tolerance, o,
		[2]*big.Rat{proof.LowerZ, proof.UpperZ}, proof.QuarterParameters)
	return result, proof, e
}

func buildCanonicalWrappedFrustum(ctx context.Context, w *geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions,
	heights [2]*big.Rat, cuts [5]float64) (Solid, error) {
	d := w.Domain()
	v := [2]float64{d.VMin, d.VMax}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	faces := []Face{{Surface: w}, {Reversed: true}, {}}
	for j, height := range v {
		z := heights[j]
		h, e := geom.RepresentedExactScalar(z)
		if e != nil {
			return Solid{}, e
		}
		frame, e := geom.NewFrame(geom.Vec3{Z: h}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return Solid{}, e
		}
		plane, e := geom.NewPlaneSurface(frame)
		if e != nil {
			return Solid{}, e
		}
		faces[j+1].Surface = plane
		rim, e := w.FundamentalPatch().IsoCurve(1, height, o)
		if e != nil {
			return Solid{}, e
		}
		var cap []Use
		for i := range 4 {
			a, b := cuts[i], cuts[i+1]
			c, e := rim.Restrict(a, b, o)
			if e != nil {
				return Solid{}, e
			}
			model, e := c.ToBSpline()
			if e != nil {
				return Solid{}, e
			}
			poles := model.Poles()
			vertices = append(vertices, poles[0])
			p, q := geom.Vec2{X: poles[0].X, Y: poles[0].Y}, geom.Vec2{X: poles[2].X, Y: poles[2].Y}
			for k := range poles {
				poles[k].Z = 0
			}
			uv, e := geom.NewBSpline(2, poles, model.Knots(), model.Weights())
			if e != nil {
				return Solid{}, e
			}
			id := len(edges)
			edges = append(edges, parametric.Edge{Start: 4*j + i, End: 4*j + (i+1)%4, Curve: model,
				Range: parametric.Interval{Min: a, Max: b}})
			cap = append(cap, Use{Edge: id, UVStart: p, UVEnd: q, Curve: SplineUV{Curve: uv, Range: UVInterval{Min: a, Max: b}}})
		}
		faces[j+1].Outer = cap
		p, e := ValidateFaceTrimLoops(ctx, faces[j+1], trimvalidate.Options{})
		if e != nil || p.Status != trimvalidate.Certified {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, e
		}
	}
	uv := func(i, j int) geom.Vec2 { return geom.Vec2{X: cuts[i], Y: v[j]} }
	for i := range 4 {
		faces[0].Outer = append(faces[0].Outer, Use{Edge: i, UVStart: uv(i, 0), UVEnd: uv(i+1, 0)})
	}
	seam := len(edges)
	edges = append(edges, parametric.Edge{Start: 0, End: 4,
		Curve: surfaceTrimCurve{w, uv(0, 0), uv(0, 1), nil}, Range: parametric.Interval{Max: 1}})
	faces[0].Outer = append(faces[0].Outer, Use{Edge: seam, UVStart: uv(4, 0), UVEnd: uv(4, 1)})
	for i := 3; i >= 0; i-- {
		faces[0].Outer = append(faces[0].Outer, Use{Edge: i + 4, Reversed: true, UVStart: uv(i+1, 1), UVEnd: uv(i, 1)})
	}
	faces[0].Outer = append(faces[0].Outer, Use{Edge: seam, Reversed: true, UVStart: uv(0, 1), UVEnd: uv(0, 0)})
	out, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, e
	}
	agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil || !agreement.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, e
	}
	return out, nil
}

// ValidateCanonicalWrappedCylinder verifies a complete constant-radius
// wrapped cylinder with its canonical, exact cap and rim topology.
func (s Solid) ValidateCanonicalWrappedCylinder(ctx context.Context, o geom.SurfacePatchOptions) (*geom.WrappedSurfacePatch, geom.WrappedCylinderEmbedding, error) {
	w, e := s.validateCanonicalWrappedFrustumTopology(ctx, o)
	if e != nil {
		return nil, geom.WrappedCylinderEmbedding{}, e
	}
	p, e := geom.CertifyWrappedCylinderEmbedding(ctx, w, o)
	if e != nil {
		return nil, p, e
	}
	return w, p, nil
}

// ValidateCanonicalWrappedFrustum proves the existing solid retains the
// complete canonical lateral rectangle and its two exact planar caps.
// Reindexing is allowed; edited or independently repartitioned rim curves
// are outside this proof. The returned support is the original evaluator.
func (s Solid) ValidateCanonicalWrappedFrustum(ctx context.Context, o geom.SurfacePatchOptions) (*geom.WrappedSurfacePatch, geom.WrappedFrustumEmbedding, error) {
	w, e := s.validateCanonicalWrappedFrustumTopology(ctx, o)
	if e != nil {
		return nil, geom.WrappedFrustumEmbedding{}, e
	}
	p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return nil, p, e
	}
	return w, p, nil
}

func (s Solid) validateCanonicalWrappedFrustumTopology(ctx context.Context, o geom.SurfacePatchOptions) (*geom.WrappedSurfacePatch, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	t := s.Topology()
	if t.Vertices != 8 || t.Edges != 9 || t.Faces != 3 || len(t.Shells) != 1 || t.InnerWires != 0 {
		return nil, geom.ErrConvergence
	}
	side := -1
	var w *geom.WrappedSurfacePatch
	for i, f := range s.faces {
		if x, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
			if side >= 0 || f.Reversed {
				return nil, geom.ErrConvergence
			}
			side, w = i, x
		}
	}
	if w == nil {
		return nil, geom.ErrConvergence
	}
	_, full := straightFullWrappedRectangle(w, s.faces[side])
	if !full || len(s.faces[side].Outer) != 10 {
		return nil, geom.ErrConvergence
	}
	canonical, _, e := NewWrappedFrustum(ctx, w, s.tolerance, o)
	if e != nil {
		return nil, e
	}
	match := func(a, b Face, as, bs Solid) (bool, error) {
		if a.Reversed != b.Reversed || len(a.Outer) != len(b.Outer) {
			return false, nil
		}
		if ax, ok := a.Surface.(geom.PlaneSurface); ok {
			bx, ok := b.Surface.(geom.PlaneSurface)
			if !ok || ax != bx {
				return false, nil
			}
		}
		for shift := range b.Outer {
			if a.Outer[0].UVStart != b.Outer[shift].UVStart {
				continue
			}
			valid := true
			for i, u := range a.Outer {
				v := b.Outer[(i+shift)%len(b.Outer)]
				if u.UVStart != v.UVStart || u.UVEnd != v.UVEnd || (u.Curve == nil) != (v.Curve == nil) {
					valid = false
					break
				}
				p, rp, e := exactEdgeSplineWithOptions(as.edges[u.Edge], o)
				if e != nil {
					return false, e
				}
				q, rq, e := exactEdgeSplineWithOptions(bs.edges[v.Edge], o)
				if e != nil {
					return false, e
				}
				if p == nil || q == nil {
					return false, nil
				}
				same, e := geom.CompareExactCurvesContext(ctx, p, q, geom.ExactCurveComparisonOptions{
					NormalizeDomains: true, ReverseB: (rp != rq) != (u.Reversed != v.Reversed),
					MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
				})
				if e != nil || !same.Complete || !same.Equal {
					return false, e
				}
				x, y := as.edges[u.Edge].Start, bs.edges[v.Edge].Start
				if u.Reversed {
					x = as.edges[u.Edge].End
				}
				if v.Reversed {
					y = bs.edges[v.Edge].End
				}
				if as.vertices[x] != bs.vertices[y] {
					return false, nil
				}
			}
			if valid {
				return true, nil
			}
		}
		return false, nil
	}
	used := [3]bool{}
	for _, f := range s.faces {
		found := false
		for j, candidate := range canonical.faces {
			if used[j] || (f.Surface == w) != (j == 0) {
				continue
			}
			if j != 0 {
				if _, ok := f.Surface.(geom.PlaneSurface); !ok {
					continue
				}
			}
			ok, e := match(f, candidate, s, canonical)
			if e != nil {
				return nil, e
			}
			if ok {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return nil, geom.ErrConvergence
		}
	}
	agreement, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil || !agreement.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return nil, e
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Cmp(new(big.Rat)) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	return w, nil
}

// WrappedCylinderSplitSide lists each original face and edge's retained output
// identity (-1 means absent) and identifies the generated cut cap and rim.
type WrappedCylinderSplitSide struct {
	Faces  [3]int
	Edges  [9]int
	CutCap int
	CutRim [4]int
}

// WrappedCylinderSplitHistory uses local output IDs in lower and upper parts.
type WrappedCylinderSplitHistory struct{ Lower, Upper WrappedCylinderSplitSide }

// SplitWrappedCylinderAtZ partitions a canonical rational extrusion by a
// strictly interior horizontal plane. The new cap geometry on both parts is
// identical with opposite outward orientations. Original cap and rim ancestry
// is retained; the vertical seam is split into one edge on each result.
func SplitWrappedCylinderAtZ(ctx context.Context, w *geom.WrappedSurfacePatch, z, tolerance float64, o geom.SurfacePatchOptions) (Solid, Solid, WrappedCylinderSplitHistory, error) {
	var h WrappedCylinderSplitHistory
	a, b, e := geom.SplitWrappedCylinderAtZ(ctx, w, z, o)
	if e != nil {
		return Solid{}, Solid{}, h, e
	}
	x, _, e := NewWrappedCylinder(ctx, a, tolerance, o)
	if e != nil {
		return Solid{}, Solid{}, h, e
	}
	y, _, e := NewWrappedCylinder(ctx, b, tolerance, o)
	if e != nil {
		return Solid{}, Solid{}, h, e
	}
	return x, y, wrappedCylinderCutHistory(), nil
}

func wrappedCylinderCutHistory() (h WrappedCylinderSplitHistory) {
	h.Lower = WrappedCylinderSplitSide{
		Faces: [3]int{0, 1, -1}, Edges: [9]int{0, 1, 2, 3, -1, -1, -1, -1, 8}, CutCap: 2, CutRim: [4]int{4, 5, 6, 7},
	}
	h.Upper = WrappedCylinderSplitSide{
		Faces: [3]int{0, -1, 2}, Edges: [9]int{-1, -1, -1, -1, 4, 5, 6, 7, 8}, CutCap: 1, CutRim: [4]int{0, 1, 2, 3},
	}
	return h
}
