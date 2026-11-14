package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedWrappedFrustumOpenBoreMesh retains the two original wall grids
// and shared rim vertices. OriginalFace indexes the canonical native bore:
// outer wall, lower end, upper end, inner wall, and inward floor.
type CertifiedWrappedFrustumOpenBoreMesh struct {
	Mesh                                            mesh.Mesh
	OriginalFace                                    []int
	Walls                                           [2]CertifiedKnotPatchMesh
	RimVertices                                     [2][2][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Subdivisions, TriangleChecks                    int
}

// TessellateWrappedFrustumOpenBoreCertified keeps the finite material of a
// circular coaxial frustum with one open cap and a retained bore floor.
// Both walls use the same angular knot grid. At the opening they share the
// exact rational radial cap chart, with no tolerance welding. All display
// facets have two-sided correspondence bounds and the complete display has
// a separate orientation, manifold and contact certificate.
func TessellateWrappedFrustumOpenBoreCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	o CertifiedWrappedFrustumMeshOptions) (out CertifiedWrappedFrustumOpenBoreMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedWrappedFrustumOpenBoreMesh{}
		}
	}()
	if ctx == nil || outer == nil || inner == nil || !finite(o.Patch.Deflection) || o.Patch.Deflection <= 0 {
		return out, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if o.Patch.MaxTriangleChecks == 0 {
		o.Patch.MaxTriangleChecks = 100000
	}
	if o.Patch.MaxSubdivisions == 0 {
		o.Patch.MaxSubdivisions = 32
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 8000000 || o.Patch.MaxTriangleChecks < 1 || o.Patch.MaxSubdivisions < 1 ||
		o.Patch.MaxSubdivisions > 128 || o.Patch.MaxSubdivisions&(o.Patch.MaxSubdivisions-1) != 0 {
		return out, geom.ErrInvalid
	}
	walls := [2]*geom.WrappedSurfacePatch{outer, inner}
	var proof [2]geom.WrappedFrustumEmbedding
	var data [2]geom.BSplineSurfaceData
	for i, w := range walls {
		proof[i], err = geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Embedding)
		if err != nil {
			return out, err
		}
		s, e := w.FundamentalPatch().ToBSplineSurface()
		if e != nil {
			return out, e
		}
		data[i] = s.Data()
	}
	if !strictWrappedCircularFrustumBore(proof[0], proof[1]) || !slices.Equal(data[0].KnotsU, data[1].KnotsU) {
		return out, geom.ErrConvergence
	}
	lowOpen := proof[0].LowerZ.Cmp(proof[1].LowerZ) == 0
	highOpen := proof[0].UpperZ.Cmp(proof[1].UpperZ) == 0
	if lowOpen == highOpen {
		return out, geom.ErrConvergence
	}
	open := 0
	if highOpen {
		open = 1
	}
	weight := func(d geom.BSplineSurfaceData, i int) *big.Rat {
		if len(d.Weights) == 0 {
			return big.NewRat(1, 1)
		}
		return ratInset(d.Weights[i])
	}
	var poles []geom.Vec3
	var weights []float64
	for i := range 9 {
		id := 2*i + open
		a, b := weight(data[0], id), weight(data[1], id)
		if new(big.Rat).Mul(a, weight(data[1], open)).Cmp(new(big.Rat).Mul(b, weight(data[0], open))) != 0 {
			return out, geom.ErrConvergence
		}
		value, _ := a.Float64()
		poles = append(poles, data[1].Poles[id], data[0].Poles[id])
		weights = append(weights, value, value)
	}
	cap, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, data[0].KnotsU, []float64{0, 0, 1, 1}, weights)
	if e != nil {
		return out, e
	}
	capPatch, e := geom.NewSurfacePatch(cap, o.Embedding)
	if e != nil {
		return out, e
	}
	limit := ratInset(o.Patch.Deflection)
	limit.Mul(limit, limit)
	checks := 0
	for n := 1; n <= o.Patch.MaxSubdivisions; {
		var parts [2]CertifiedWrappedFrustumMesh
		grid := n
		for i, w := range walls {
			child := o
			child.Patch.MaxTriangleChecks -= checks
			if child.Patch.MaxTriangleChecks < 1 {
				return out, geom.ErrConvergence
			}
			parts[i], e = tessellateWrappedFrustumFrom(ctx, w, child, grid)
			if e != nil {
				return out, e
			}
			checks += parts[i].Lateral.TriangleChecks
			grid = parts[i].Lateral.Subdivisions
		}
		if parts[0].Lateral.Subdivisions != grid {
			n = grid
			continue
		}
		var candidate CertifiedWrappedFrustumOpenBoreMesh
		for i, p := range parts {
			candidate.Walls[i] = p.Lateral
		}
		candidate.Subdivisions = grid
		candidate.SourceImageSquaredDistance = new(big.Rat)
		add := func(tri [3]int, owner int, bound *big.Rat) error {
			if len(candidate.Mesh.Triangles) >= o.MaxTriangles {
				return geom.ErrConvergence
			}
			candidate.Mesh.Triangles = append(candidate.Mesh.Triangles, tri)
			candidate.OriginalFace = append(candidate.OriginalFace, owner)
			candidate.SquaredErrors = append(candidate.SquaredErrors, new(big.Rat).Set(bound))
			if bound.Cmp(candidate.SourceImageSquaredDistance) > 0 {
				candidate.SourceImageSquaredDistance.Set(bound)
			}
			return nil
		}
		for wall, part := range parts {
			ids := make([]int, len(part.Mesh.Vertices))
			unused := len(part.Lateral.Mesh.Vertices) + open
			for i, p := range part.Mesh.Vertices {
				if i == unused {
					ids[i] = -1 // omit the center of the two discarded cap fans
					continue
				}
				ids[i] = len(candidate.Mesh.Vertices)
				candidate.Mesh.Vertices = append(candidate.Mesh.Vertices, p)
			}
			for end, row := range part.RimVertices {
				for _, i := range row {
					candidate.RimVertices[wall][end] = append(candidate.RimVertices[wall][end], ids[i])
				}
			}
			for i, tri := range part.Mesh.Triangles {
				owner := part.OriginalFace[i]
				if owner == open+1 {
					continue
				}
				tri = [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]}
				if wall == 1 {
					tri[1], tri[2] = tri[2], tri[1]
					if owner == 0 {
						owner = 3
					} else {
						owner = 4
					}
				}
				if e := add(tri, owner, part.SquaredErrors[i]); e != nil {
					return out, e
				}
			}
		}
		u := parts[0].Lateral.UParameters
		if !slices.Equal(u, parts[1].Lateral.UParameters) || len(u)-1 != len(candidate.RimVertices[0][open]) {
			return out, geom.ErrConvergence
		}
		in, rim := candidate.RimVertices[1][open], candidate.RimVertices[0][open]
		ok := true
		var memo triangleRectangleMemo
		for i := range rim {
			j := (i + 1) % len(rim)
			ids := [4]int{in[i], in[j], rim[j], rim[i]}
			uv := [4]geom.Vec2{{X: u[i]}, {X: u[i+1]}, {X: u[i+1], Y: 1}, {X: u[i], Y: 1}}
			for _, k := range [2][3]int{{0, 1, 2}, {0, 2, 3}} {
				if open == 1 {
					k[1], k[2] = k[2], k[1]
				}
				t := [3]int{ids[k[0]], ids[k[1]], ids[k[2]]}
				par := [3]geom.Vec2{uv[k[0]], uv[k[1]], uv[k[2]]}
				xyz := [3]geom.Vec3{candidate.Mesh.Vertices[t[0]], candidate.Mesh.Vertices[t[1]], candidate.Mesh.Vertices[t[2]]}
				if checks >= o.Patch.MaxTriangleChecks {
					return out, geom.ErrConvergence
				}
				q, e := boundSplineTriangleDeviationPrepared(ctx, capPatch, par, xyz, &memo)
				checks++
				if e != nil || !q.BothDirections {
					if e == nil {
						e = geom.ErrConvergence
					}
					return out, e
				}
				ok = ok && q.SquaredDistance.Cmp(limit) <= 0
				if e := add(t, open+1, q.SquaredDistance); e != nil {
					return out, e
				}
			}
		}
		if !ok {
			n = 2 * grid
			continue
		}
		candidate.TriangleChecks = checks
		candidate.SignedDisplayVolume, candidate.Collision, e = checkClosedPeriodicMesh(ctx, candidate.Mesh, o.Collision)
		if e != nil {
			return out, e
		}
		return candidate, nil
	}
	return out, geom.ErrConvergence
}
