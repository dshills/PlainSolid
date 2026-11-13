package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TessellateAdaptivePatchCertified refines only U/V intervals participating in
// failed certificates. Intervals split globally across the tensor grid, avoiding
// hanging nodes. MaxSubdivisions bounds the maximum dyadic factor of any initial
// knot interval. Subdivisions in the result is an upper bound on that factor;
// actual counts and locations are given by UParameters/VParameters.
func TessellateAdaptivePatchCertified(ctx context.Context, p *geom.SurfacePatch, o CertifiedPatchMeshOptions) (CertifiedKnotPatchMesh, error) {
	return tessellateCertifiedAdaptiveGrid(ctx, p, [2]bool{}, o)
}

// TessellateAdaptiveWrappedPatchCertified additionally shares exact seam vertex
// indices. Opposite seams use the same refined axis arrays by construction.
func TessellateAdaptiveWrappedPatchCertified(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedPatchMeshOptions) (CertifiedKnotPatchMesh, error) {
	if w == nil {
		return CertifiedKnotPatchMesh{}, geom.ErrInvalid
	}
	periods := w.Periods()
	return tessellateCertifiedAdaptiveGrid(ctx, w.FundamentalPatch(), [2]bool{periods[0] != nil, periods[1] != nil}, o)
}
func splitCertifiedGridIntervals(cuts []float64, split []bool) ([]float64, error) {
	result := make([]float64, 0, len(cuts)+len(split))
	for i, refine := range split {
		result = append(result, cuts[i])
		if refine {
			mid := new(big.Rat).Quo(new(big.Rat).Add(ratInset(cuts[i]), ratInset(cuts[i+1])), big.NewRat(2, 1))
			f, _ := mid.Float64()
			if !finite(f) || ratInset(f).Cmp(mid) != 0 || f <= cuts[i] || f >= cuts[i+1] {
				return nil, geom.ErrConvergence
			}
			result = append(result, f)
		}
	}
	return append(result, cuts[len(cuts)-1]), nil
}
func tessellateCertifiedAdaptiveGrid(ctx context.Context, p *geom.SurfacePatch, wrap [2]bool, o CertifiedPatchMeshOptions) (CertifiedKnotPatchMesh, error) {
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
	us, vs := p.KnotBreaks()
	checks := 0
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		nu, nv := len(us)-1, len(vs)-1
		if nu < 1 || nv < 1 || nu > 4096 || nv > 4096 || nu*nv > 1<<20 || 2*nu*nv > o.MaxTriangleChecks-checks {
			return empty, geom.ErrConvergence
		}
		refineU, refineV := make([]bool, nu), make([]bool, nv)
		if wrap[0] && nu < 4 || wrap[1] && nv < 4 {
			for i := range refineU {
				refineU[i] = wrap[0] && nu < 4
			}
			for i := range refineV {
				refineV[i] = wrap[1] && nv < 4
			}
			var e error
			us, e = splitCertifiedGridIntervals(us, refineU)
			if e != nil {
				return empty, e
			}
			vs, e = splitCertifiedGridIntervals(vs, refineV)
			if e != nil {
				return empty, e
			}
			continue
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
					bound, e := BoundSplineTriangleDeviation(ctx, p, uv, model)
					checks++
					result.BothDirections = result.BothDirections && bound.BothDirections
					if e != nil {
						return empty, e
					}
					if bound.SquaredDistance.Cmp(tolerance) > 0 {
						accepted = false
						refineU[i], refineV[j] = true, true
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
		var e error
		us, e = splitCertifiedGridIntervals(us, refineU)
		if e != nil {
			return empty, e
		}
		vs, e = splitCertifiedGridIntervals(vs, refineV)
		if e != nil {
			return empty, e
		}

	}
	return empty, geom.ErrConvergence
}
