package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedKnotPatchMesh retains the actual knot-aligned parameter grid.
// Subdivisions is the dyadic refinement count per initial knot interval,
// not the total row count. Triangle certificates and direction flags retain
// CertifiedPatchMesh semantics. Each axis is capped at4096 intervals and the
// grid at1M cells, in addition to the cumulative triangle-check budget.
type CertifiedKnotPatchMesh struct {
	CertifiedPatchMesh
	UParameters, VParameters []float64
}

// TessellateKnotPatchCertified starts every row at an actual knot break and
// refines all spans uniformly. Exact local restrictions allow supported
// rational Bezier cells to certify both directions. No default mesh changes.
func TessellateKnotPatchCertified(ctx context.Context, p *geom.SurfacePatch, o CertifiedPatchMeshOptions) (CertifiedKnotPatchMesh, error) {
	return tessellateCertifiedKnotGrid(ctx, p, [2]bool{}, o)
}

// TessellateWrappedKnotPatchCertified synchronizes the exact wrapped seams;
// per-cell UV coordinates remain in the original fundamental rectangle.
func TessellateWrappedKnotPatchCertified(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedPatchMeshOptions) (CertifiedKnotPatchMesh, error) {
	if w == nil {
		return CertifiedKnotPatchMesh{}, geom.ErrInvalid
	}
	periods := w.Periods()
	return tessellateCertifiedKnotGrid(ctx, w.FundamentalPatch(), [2]bool{periods[0] != nil, periods[1] != nil}, o)
}
func tessellateCertifiedKnotGrid(ctx context.Context, p *geom.SurfacePatch, wrap [2]bool, o CertifiedPatchMeshOptions) (CertifiedKnotPatchMesh, error) {
	return tessellateCertifiedKnotGridFrom(ctx, p, wrap, o, 1)
}

func tessellateCertifiedKnotGridFrom(ctx context.Context, p *geom.SurfacePatch, wrap [2]bool, o CertifiedPatchMeshOptions, minimum int) (CertifiedKnotPatchMesh, error) {
	empty := CertifiedKnotPatchMesh{}
	if ctx == nil || p == nil || !finite(o.Deflection) || o.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 32
	}
	if o.MaxTriangleChecks == 0 {
		o.MaxTriangleChecks = 100000
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 128 || o.MaxSubdivisions&(o.MaxSubdivisions-1) != 0 || o.MaxTriangleChecks < 1 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	tolerance := ratInset(o.Deflection)
	tolerance.Mul(tolerance, tolerance)
	uCuts, vCuts := p.KnotBreaks()
	checks := 0
	var rectangleMemo triangleRectangleMemo
	for n := minimum; n <= o.MaxSubdivisions; n *= 2 {
		nu, nv := n*(len(uCuts)-1), n*(len(vCuts)-1)
		if wrap[0] && nu < 4 || wrap[1] && nv < 4 {
			continue
		}
		if nu < 1 || nv < 1 || nu > 4096 || nv > 4096 || nu*nv > 1<<20 || 2*nu*nv > o.MaxTriangleChecks-checks {
			return empty, geom.ErrConvergence
		}
		coordinates := func(cuts []float64) ([]float64, error) {
			r := make([]float64, 0, n*(len(cuts)-1)+1)
			for span := 0; span < len(cuts)-1; span++ {
				a, b := ratInset(cuts[span]), ratInset(cuts[span+1])
				width := new(big.Rat).Sub(b, a)
				for i := 0; i < n; i++ {
					x := new(big.Rat).Add(a, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(n))))
					f, _ := x.Float64()
					if !finite(f) || ratInset(f).Cmp(x) != 0 {
						return nil, geom.ErrConvergence
					}
					r = append(r, f)
				}
			}
			return append(r, cuts[len(cuts)-1]), nil
		}
		us, e := coordinates(uCuts)
		if e != nil {
			return empty, e
		}
		vs, e := coordinates(vCuts)
		if e != nil {
			return empty, e
		}
		result := CertifiedKnotPatchMesh{CertifiedPatchMesh: CertifiedPatchMesh{Subdivisions: n, BothDirections: true}, UParameters: us, VParameters: vs}
		ids := make([]int, (nu+1)*(nv+1))
		canonical := map[[2]int]int{}
		for i := 0; i <= nu; i++ {
			for j := 0; j <= nv; j++ {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				ci, cj := i, j
				if wrap[0] && i == nu {
					ci = 0
				}
				if wrap[1] && j == nv {
					cj = 0
				}
				key := [2]int{ci, cj}
				if id, ok := canonical[key]; ok {
					ids[i*(nv+1)+j] = id
					continue
				}
				jet, e := p.EvaluateRational(ctx, ratInset(us[ci]), ratInset(vs[cj]), o.Evaluation)
				if e != nil {
					return empty, e
				}
				x, _ := jet.Point[0].Float64()
				y, _ := jet.Point[1].Float64()
				z, _ := jet.Point[2].Float64()
				v := geom.Vec3{X: x, Y: y, Z: z}
				if !v.IsFinite() {
					return empty, geom.ErrConvergence
				}
				id := len(result.Mesh.Vertices)
				result.Mesh.Vertices = append(result.Mesh.Vertices, v)
				canonical[key] = id
				ids[i*(nv+1)+j] = id
			}
		}
		accepted := true
		for i := 0; i < nu; i++ {
			for j := 0; j < nv; j++ {
				corners := [4][2]int{{i, j}, {i + 1, j}, {i + 1, j + 1}, {i, j + 1}}
				for _, tri := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					var uv [3]geom.Vec2
					var model [3]geom.Vec3
					var index [3]int
					for k, c := range tri {
						a, b := corners[c][0], corners[c][1]
						uv[k] = geom.Vec2{X: us[a], Y: vs[b]}
						index[k] = ids[a*(nv+1)+b]
						model[k] = result.Mesh.Vertices[index[k]]
					}
					bound, e := boundSplineTriangleDeviationPrepared(ctx, p, uv, model, &rectangleMemo)
					checks++
					result.BothDirections = result.BothDirections && bound.BothDirections
					if e != nil {
						return empty, e
					}
					if bound.SquaredDistance.Cmp(tolerance) > 0 {
						accepted = false
					}
					result.Mesh.Triangles = append(result.Mesh.Triangles, index)
					result.UVTriangles = append(result.UVTriangles, uv)
					result.SquaredErrors = append(result.SquaredErrors, bound.SquaredDistance)
				}
			}
		}
		if accepted {
			if e := result.Mesh.Validate(); e != nil {
				return empty, e
			}
			result.TriangleChecks = checks
			return result, nil
		}
	}
	return empty, geom.ErrConvergence
}
