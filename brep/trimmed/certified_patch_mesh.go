package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedPatchMeshOptions bounds uniform refinement. MaxSubdivisions must
// be a power of two (default32, maximum128). MaxTriangleChecks cumulatively
// bounds triangle certificates over all attempted grids (default100000).
type CertifiedPatchMeshOptions struct {
	Deflection                         float64
	MaxSubdivisions, MaxTriangleChecks int
	Evaluation                         geom.SurfacePatchEvaluationOptions
}

// CertifiedPatchMesh contains a conforming indexed mesh and one exact
// one-sided squared deviation certificate per triangle. UVTriangles retain
// fundamental-domain coordinates even where seam vertices share an index.
// BothDirections distinguishes a proved reverse Hausdorff bound from the
// default one-sided certificate. Neither proves orientation or embedding.
type CertifiedPatchMesh struct {
	Mesh                         mesh.Mesh
	UVTriangles                  [][3]geom.Vec2
	SquaredErrors                []*big.Rat
	Subdivisions, TriangleChecks int
	// BothDirections is true when every triangle also proves reverse deviation.
	BothDirections bool
}

// TessellatePatchCertified uniformly meshes a full exact spline rectangle.
// Grid parameters must be losslessly representable binary64 values; model
// vertices are nearest rounded exact evaluations, included in the bound.
// No partial mesh is returned if any proof, resource or validity check fails.
func TessellatePatchCertified(ctx context.Context, p *geom.SurfacePatch, o CertifiedPatchMeshOptions) (CertifiedPatchMesh, error) {
	return tessellateCertifiedGrid(ctx, p, [2]bool{}, o)
}

// TessellateWrappedPatchCertified synchronizes seam vertices only using the
// wrapper's exact seam certificates. A doubly wrapped embedded toroidal patch
// can yield a closed mesh; this function does not prove its embedding.
func TessellateWrappedPatchCertified(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedPatchMeshOptions) (CertifiedPatchMesh, error) {
	if w == nil {
		return CertifiedPatchMesh{}, geom.ErrInvalid
	}
	periods := w.Periods()
	return tessellateCertifiedGrid(ctx, w.FundamentalPatch(), [2]bool{periods[0] != nil, periods[1] != nil}, o)
}

func tessellateCertifiedGrid(ctx context.Context, p *geom.SurfacePatch, wrap [2]bool, o CertifiedPatchMeshOptions) (CertifiedPatchMesh, error) {
	empty := CertifiedPatchMesh{}
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
	d := p.Domain()
	checks := 0
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		// Fewer than four segments cannot represent a general closed seam.
		if (wrap[0] || wrap[1]) && n < 4 {
			continue
		}
		if 2*n*n > o.MaxTriangleChecks-checks {
			return empty, geom.ErrConvergence
		}
		coordinates := func(lo, hi float64) ([]float64, error) {
			r := make([]float64, n+1)
			a, b := ratInset(lo), ratInset(hi)
			span := new(big.Rat).Sub(b, a)
			for i := range r {
				x := new(big.Rat).Add(a, new(big.Rat).Mul(span, big.NewRat(int64(i), int64(n))))
				f, _ := x.Float64()
				if !finite(f) || ratInset(f).Cmp(x) != 0 {
					return nil, geom.ErrConvergence
				}
				r[i] = f
			}
			return r, nil
		}
		us, e := coordinates(d.UMin, d.UMax)
		if e != nil {
			return empty, e
		}
		vs, e := coordinates(d.VMin, d.VMax)
		if e != nil {
			return empty, e
		}
		result := CertifiedPatchMesh{Subdivisions: n, BothDirections: true}
		ids := make([]int, (n+1)*(n+1))
		canonical := map[[2]int]int{}
		for i := 0; i <= n; i++ {
			for j := 0; j <= n; j++ {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				ci, cj := i, j
				if wrap[0] && i == n {
					ci = 0
				}
				if wrap[1] && j == n {
					cj = 0
				}
				key := [2]int{ci, cj}
				if id, ok := canonical[key]; ok {
					ids[i*(n+1)+j] = id
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
				ids[i*(n+1)+j] = id
			}
		}
		accepted := true
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				corners := [4][2]int{{i, j}, {i + 1, j}, {i + 1, j + 1}, {i, j + 1}}
				for _, tri := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					var uv [3]geom.Vec2
					var model [3]geom.Vec3
					var index [3]int
					for k, c := range tri {
						a, b := corners[c][0], corners[c][1]
						uv[k] = geom.Vec2{X: us[a], Y: vs[b]}
						index[k] = ids[a*(n+1)+b]
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
