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
// outer wall, lower end, upper end, inner wall, then any retained inner caps
// in lower-to-upper order. CavityShells is set for an enclosed cavity only;
// its first shell contains the inward second shell.
type CertifiedWrappedFrustumOpenBoreMesh struct {
	Mesh                                            mesh.Mesh
	OriginalFace                                    []int
	Walls                                           [2]CertifiedKnotPatchMesh
	RimVertices                                     [2][2][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	CavityShells                                    []collision.ShellInfo
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
	return tessellateParallelFrustumOpenBore(ctx, outer, inner, o, true, 1)
}

// TessellateWrappedEccentricFrustumOpenBoreCertified supports parallel
// noncoaxial walls when the matched angular cap chart remains an injective
// parameterization of the entire eccentric annulus. At each angle its radial
// segment interpolates a family of strictly nested circles from the original
// inner circle to the original outer circle.
func TessellateWrappedEccentricFrustumOpenBoreCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	return tessellateParallelFrustumOpenBore(ctx, outer, inner, o, false, 1)
}

// TessellateWrappedEccentricFrustumThroughBoreCertified meshes both original
// walls and their two complete annular caps. Each end has its own exact
// radial chart and shared rim indices, with one manifold and contact check
// on the assembled display.
func TessellateWrappedEccentricFrustumThroughBoreCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	return tessellateParallelFrustumOpenBore(ctx, outer, inner, o, false, 2)
}

// TessellateWrappedEccentricFrustumCavityCertified meshes both fully retained
// source shells, including the two inward cavity caps. Exact predicates prove
// the represented inward shell is strictly inside the represented outer one.
func TessellateWrappedEccentricFrustumCavityCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	return tessellateParallelFrustumOpenBore(ctx, outer, inner, o, false, 0)
}

func tessellateParallelFrustumOpenBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	o CertifiedWrappedFrustumMeshOptions, coaxial bool, openings int) (out CertifiedWrappedFrustumOpenBoreMesh, err error) {
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
		if openings != 0 {
			s, e := w.FundamentalPatch().ToBSplineSurface()
			if e != nil {
				return out, e
			}
			data[i] = s.Data()
		}
	}
	if !strictWrappedParallelFrustumBore(proof[0], proof[1], coaxial) ||
		(openings != 0 && !slices.Equal(data[0].KnotsU, data[1].KnotsU)) {
		return out, geom.ErrConvergence
	}
	open := [2]bool{proof[0].LowerZ.Cmp(proof[1].LowerZ) == 0, proof[0].UpperZ.Cmp(proof[1].UpperZ) == 0}
	openCount := 0
	var caps [2]*geom.SurfacePatch
	for end, opened := range open {
		if !opened {
			continue
		}
		openCount++
		caps[end], err = exactFrustumAnnulusPatch(data, end, o.Embedding)
		if err != nil {
			return out, err
		}
	}
	if openCount != openings {
		return out, geom.ErrConvergence
	}
	innerIDs := [3]int{3, -1, -1}
	nextID := 4
	for i, opened := range open {
		if !opened {
			innerIDs[i+1] = nextID
			nextID++
		}
	}
	var e error
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
			centers := len(part.Lateral.Mesh.Vertices)
			for i, p := range part.Mesh.Vertices {
				if end := i - centers; end >= 0 && end < 2 && open[end] {
					ids[i] = -1 // omit the fan center of each annular cap
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
				if owner > 0 && open[owner-1] {
					continue
				}
				tri = [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]}
				if wall == 1 {
					tri[1], tri[2] = tri[2], tri[1]
					owner = innerIDs[owner]
				}
				if e := add(tri, owner, part.SquaredErrors[i]); e != nil {
					return out, e
				}
			}
		}
		u := parts[0].Lateral.UParameters
		if openings != 0 && !slices.Equal(u, parts[1].Lateral.UParameters) {
			return out, geom.ErrConvergence
		}
		ok := true
		for end, capPatch := range caps {
			if capPatch == nil {
				continue
			}
			in, rim := candidate.RimVertices[1][end], candidate.RimVertices[0][end]
			if len(in) != len(rim) || len(u)-1 != len(rim) {
				return out, geom.ErrConvergence
			}
			var memo triangleRectangleMemo
			for i := range rim {
				j := (i + 1) % len(rim)
				ids := [4]int{in[i], in[j], rim[j], rim[i]}
				uv := [4]geom.Vec2{{X: u[i]}, {X: u[i+1]}, {X: u[i+1], Y: 1}, {X: u[i], Y: 1}}
				for _, k := range [2][3]int{{0, 1, 2}, {0, 2, 3}} {
					if end == 1 {
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
					if e := add(t, end+1, q.SquaredDistance); e != nil {
						return out, e
					}
				}
			}
		}
		if !ok {
			n = 2 * grid
			continue
		}
		candidate.TriangleChecks = checks
		if openings == 0 {
			pairs := o.Collision.MaxPairs
			if pairs == 0 {
				pairs = collision.DefaultMaxPairs
			}
			locator, e := collision.NewMeshPointLocator(ctx, candidate.Mesh, candidate.OriginalFace, collision.LocationOptions{
				MaxTriangles:      min(o.MaxTriangles, collision.MaxTriangles),
				MaxCollisionPairs: pairs,
			})
			if e != nil {
				return out, e
			}
			candidate.CavityShells, candidate.Collision = locator.Shells(), locator.BoundaryCollision()
			if len(candidate.CavityShells) != 2 || candidate.CavityShells[0].Parent != -1 || candidate.CavityShells[1].Parent != 0 {
				return out, geom.ErrConvergence
			}
			candidate.SignedDisplayVolume = new(big.Rat).Add(candidate.CavityShells[0].SignedVolume, candidate.CavityShells[1].SignedVolume)
			if candidate.SignedDisplayVolume.Sign() <= 0 {
				return out, geom.ErrConvergence
			}
			return candidate, nil
		}
		candidate.SignedDisplayVolume, candidate.Collision, e = checkClosedPeriodicMesh(ctx, candidate.Mesh, o.Collision)
		if e != nil {
			return out, e
		}
		return candidate, nil
	}
	return out, geom.ErrConvergence
}

func exactFrustumAnnulusPatch(data [2]geom.BSplineSurfaceData, end int, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	weight := func(d geom.BSplineSurfaceData, i int) *big.Rat {
		if len(d.Weights) == 0 {
			return big.NewRat(1, 1)
		}
		return ratInset(d.Weights[i])
	}
	var poles []geom.Vec3
	var weights []float64
	for i := range 9 {
		id := 2*i + end
		a, b := weight(data[0], id), weight(data[1], id)
		if new(big.Rat).Mul(a, weight(data[1], end)).Cmp(new(big.Rat).Mul(b, weight(data[0], end))) != 0 {
			return nil, geom.ErrConvergence
		}
		value, _ := a.Float64()
		poles = append(poles, data[1].Poles[id], data[0].Poles[id])
		weights = append(weights, value, value)
	}
	cap, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, data[0].KnotsU, []float64{0, 0, 1, 1}, weights)
	if e != nil {
		return nil, e
	}
	return geom.NewSurfacePatch(cap, o)
}

// TessellateCanonicalWrappedFrustumThroughBoreCertified restores the native
// wall and annulus face IDs on a certified through-bore display.
func (s Solid) TessellateCanonicalWrappedFrustumThroughBoreCertified(ctx context.Context, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	return s.tessellateCanonicalWrappedFrustumBoreCertified(ctx, o, "tube", 2)
}

// TessellateCanonicalWrappedFrustumCavityCertified certifies the two nested
// display shells and preserves the receiver's original wall and cap face IDs.
func (s Solid) TessellateCanonicalWrappedFrustumCavityCertified(ctx context.Context, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	return s.tessellateCanonicalWrappedFrustumBoreCertified(ctx, o, "cavity", 0)
}

func (s Solid) tessellateCanonicalWrappedFrustumBoreCertified(ctx context.Context, o CertifiedWrappedFrustumMeshOptions, kind string, openings int) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	var empty CertifiedWrappedFrustumOpenBoreMesh
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Embedding)
	if e != nil {
		return empty, e
	}
	if p.Kind != kind {
		return empty, geom.ErrConvergence
	}
	m, e := tessellateParallelFrustumOpenBore(ctx, p.Outer, p.Inner, o, false, openings)
	if e != nil {
		return empty, e
	}
	ids := canonicalFrustumBoreLineFaceIDs(p)
	for i, id := range m.OriginalFace {
		m.OriginalFace[i] = ids[id]
	}
	return m, nil
}
