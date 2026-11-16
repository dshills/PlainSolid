package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedFilletedFrustumMesh retains the re-proved native stock and maps all
// triangles to its face IDs. Rims[end][cap-or-cone] are the single shared model
// samples. Per-triangle bounds include rounding and prove both directions;
// closed display incidence/positive volume/no contacts are independently checked.
// No certified angular or material-volume error is asserted.
type CertifiedFilletedFrustumMesh struct {
	Mesh                                     mesh.Mesh
	Source                                   Solid
	History                                  WrappedFrustumDoubleFilletHistory
	OriginalFace                             []int
	Rims                                     [2][2][]int
	SquaredErrors                            []*big.Rat
	SquaredDeflection, SignedDisplayVolume   *big.Rat
	Collision                                collision.Report
	TriangleChecks, Subdivisions, Predicates int
}

// TessellateFilletedFrustumClosedCertified synchronizes all three periodic
// charts before any shared-index assignment. Full original rim correspondence
// was proved by FilletWrappedFrustumRims; identical original U parameters and
// bit-identical rounded evaluations are required at every join. The caps use
// radial source/chord interpolation about exactly represented centers; the
// boundary residual contracts by the radial fraction. Exact simple star-fan
// validation proves full represented cap coverage. All retries share the
// supplied triangle-check budget; failure returns no partial result.
func TessellateFilletedFrustumClosedCertified(ctx context.Context, source Solid, lower, upper float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedFilletedFrustumMesh, error) {
	return tessellateFilletedFrustumClosed(ctx, source, lower, upper, o, nil)
}

func tessellateFilletedFrustumClosed(ctx context.Context, source Solid, lower, upper float64, o CertifiedWrappedFrustumMeshOptions, angles *filletedFrustumFacetAngles) (out CertifiedFilletedFrustumMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedFilletedFrustumMesh{}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if o.Patch.MaxTriangleChecks == 0 {
		o.Patch.MaxTriangleChecks = 100000
	}
	if o.Patch.MaxSubdivisions == 0 {
		o.Patch.MaxSubdivisions = 32
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.Patch.MaxTriangleChecks < 1 || o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return out, geom.ErrInvalid
	}
	stock, h, e := FilletWrappedFrustumRims(ctx, source, lower, upper, o.Embedding)
	if e != nil {
		return out, e
	}
	owners := [3]int{h.Blends[0], h.Lateral, h.Blends[1]}
	var normals [3]filletedFrustumGridNormals
	if angles != nil {
		core, ok := stock.faces[h.Lateral].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return out, geom.ErrConvergence
		}
		normals[1], e = frustumGridNormalSource(ctx, core, false, o.Embedding)
		if e != nil {
			return out, e
		}
		for end, id := range h.Blends {
			blend, ok := stock.faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok {
				return out, geom.ErrConvergence
			}
			normals[2*end] = filletGridNormalSource(blend, normals[1].center, &h.Circles[end], end == 1)
		}
	}
	var grids [3]CertifiedKnotPatchMesh
	minimum := 1
	checks := 0
	for {
		next := minimum
		for i, id := range owners {
			w, ok := stock.faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok {
				return out, geom.ErrConvergence
			}
			options := o.Patch
			options.MaxTriangleChecks -= checks
			if options.MaxTriangleChecks < 1 {
				return out, geom.ErrConvergence
			}
			grids[i], e = tessellateCertifiedKnotGridFrom(ctx, w.FundamentalPatch(), [2]bool{true, false}, options, minimum)
			if e != nil {
				return out, e
			}
			checks += grids[i].TriangleChecks
			if !grids[i].BothDirections {
				return out, geom.ErrConvergence
			}
			if grids[i].Subdivisions > next {
				next = grids[i].Subdivisions
			}
		}
		if angles != nil && next == minimum {
			maximum := new(big.Rat)
			for i, id := range owners {
				q, err := normals[i].certify(ctx, grids[i], stock.faces[id].Reversed, angles.limit, o.Patch.Evaluation)
				if err != nil {
					return out, err
				}
				if q == nil {
					next = 2 * minimum
					break
				}
				if q.Cmp(maximum) > 0 {
					maximum.Set(q)
				}
			}
			if next == minimum {
				angles.maximum.Set(maximum)
			}
		}
		if next == minimum {
			break
		}
		minimum = next
		if minimum > o.Patch.MaxSubdivisions {
			return out, geom.ErrConvergence
		}
	}
	nu := len(grids[0].UParameters) - 1
	total := 2 * nu
	for _, g := range grids {
		total += len(g.Mesh.Triangles)
		if len(g.UParameters) != nu+1 {
			return out, geom.ErrConvergence
		}
		for i, u := range g.UParameters {
			if u != grids[0].UParameters[i] {
				return out, geom.ErrConvergence
			}
		}
	}
	if total > o.MaxTriangles {
		return out, geom.ErrConvergence
	}
	out.Source, out.History = stock, h
	out.Subdivisions, out.TriangleChecks = minimum, checks
	out.SquaredDeflection = new(big.Rat)
	// Cone first: its lower/upper rows become the immutable join identities.
	maps := [3][]int{}
	for _, i := range []int{1, 0, 2} {
		g := grids[i]
		nv := len(g.VParameters) - 1
		if nv < 1 || len(g.Mesh.Vertices) != nu*(nv+1) {
			return out, geom.ErrConvergence
		}
		maps[i] = make([]int, len(g.Mesh.Vertices))
		for k, p := range g.Mesh.Vertices {
			row, col := k/(nv+1), k%(nv+1)
			shared := -1
			if i == 0 && col == nv {
				coreNV := len(grids[1].VParameters) - 1
				shared = maps[1][row*(coreNV+1)]
			}
			if i == 2 && col == 0 {
				coreNV := len(grids[1].VParameters) - 1
				shared = maps[1][row*(coreNV+1)+coreNV]
			}
			if shared >= 0 {
				if out.Mesh.Vertices[shared] != p {
					return out, geom.ErrConvergence
				}
				maps[i][k] = shared
			} else {
				maps[i][k] = len(out.Mesh.Vertices)
				out.Mesh.Vertices = append(out.Mesh.Vertices, p)
			}
		}
		for k, tri := range g.Mesh.Triangles {
			t := [3]int{maps[i][tri[0]], maps[i][tri[1]], maps[i][tri[2]]}
			if stock.faces[owners[i]].Reversed {
				t[1], t[2] = t[2], t[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, t)
			out.OriginalFace = append(out.OriginalFace, owners[i])
			bound := g.SquaredErrors[k]
			out.SquaredErrors = append(out.SquaredErrors, new(big.Rat).Set(bound))
			if bound.Cmp(out.SquaredDeflection) > 0 {
				out.SquaredDeflection.Set(bound)
			}
		}
	}
	for end := 0; end < 2; end++ {
		i := 2 * end
		g := grids[i]
		nv := len(g.VParameters) - 1
		for role := 0; role < 2; role++ {
			j := role * nv
			if end == 1 {
				j = nv - j
			}
			for row := 0; row < nu; row++ {
				out.Rims[end][role] = append(out.Rims[end][role], maps[i][row*(nv+1)+j])
			}
		}
		plane, ok := stock.faces[h.Caps[end]].Surface.(geom.PlaneSurface)
		if !ok {
			return out, geom.ErrConvergence
		}
		center := plane.Frame().Origin()
		points := make([][2]*big.Rat, nu+1)
		ring := make([]int, nu)
		for row, id := range out.Rims[end][0] {
			p := out.Mesh.Vertices[id]
			if p.Z != center.Z {
				return out, geom.ErrConvergence
			}
			points[row] = [2]*big.Rat{ratInset(p.X), ratInset(p.Y)}
			ring[row] = row
		}
		points[nu] = [2]*big.Rat{ratInset(center.X), ratInset(center.Y)}
		budget := partitionFillProof{ctx: ctx, limit: o.MaxPredicates - out.Predicates, points: points}
		if budget.limit < 1 {
			return out, geom.ErrConvergence
		}
		if e = budget.simple(ring); e != nil {
			return out, e
		}
		for row, a := range points[:nu] {
			b := points[(row+1)%nu]
			ax, ay := new(big.Rat).Sub(a[0], points[nu][0]), new(big.Rat).Sub(a[1], points[nu][1])
			bx, by := new(big.Rat).Sub(b[0], points[nu][0]), new(big.Rat).Sub(b[1], points[nu][1])
			if new(big.Rat).Sub(new(big.Rat).Mul(ax, by), new(big.Rat).Mul(ay, bx)).Sign() <= 0 {
				return out, geom.ErrConvergence
			}
		}
		out.Predicates += budget.work + nu
		if out.Predicates > o.MaxPredicates {
			return out, geom.ErrConvergence
		}
		centerID := len(out.Mesh.Vertices)
		out.Mesh.Vertices = append(out.Mesh.Vertices, center)
		for row, a := range out.Rims[end][0] {
			b := out.Rims[end][0][(row+1)%nu]
			tri := [3]int{centerID, a, b}
			if end == 0 {
				tri[1], tri[2] = tri[2], tri[1]
			}
			if angles != nil && !frustumCapFacetNormalMatches(out.Mesh.Vertices, tri, end == 1) {
				return out, geom.ErrConvergence
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, tri)
			out.OriginalFace = append(out.OriginalFace, h.Caps[end])
			out.SquaredErrors = append(out.SquaredErrors, new(big.Rat).Set(out.SquaredDeflection))
		}
	}
	out.SignedDisplayVolume, out.Collision, e = checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	return out, nil
}
