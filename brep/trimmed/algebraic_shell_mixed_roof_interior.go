package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MixedRoofInteriorOptions requests original-graph interpolation accuracy.
// Resource limits are independent: preparation exact operations, fixed-degree
// triangle checks, represented triangles, predicates and contact pairs. The
// refinement evaluator has its own per-evaluation rational arithmetic bounds.
// Interior.Boundary does not resample the already certified input network.
// A nonzero Proof.MaxTriangles additionally caps the interior triangle budget.
type MixedRoofInteriorOptions struct {
	Interior        CurvedQuadraticTrimMeshOptions
	Proof           MixedAffineFragmentFillOptions
	RequireComplete bool
}

// CertifiedMixedRoofInterior is an immutable, possibly INCOMPLETE roof fill.
// Every retained triangle has an original source-graph counterpart lying
// strictly inside both footprints, strictly above both floors, and below the
// opposing roof. Exact Bernstein inequalities prove these facts over the whole
// triangle, not at samples. It is not a closed shell or a full face certificate.
// Omitted candidate triangles are never silently treated as covered material.
// Contact checks cover this displayed patch and every shared network segment,
// not the interiors of unfilled faces.
type CertifiedMixedRoofInterior struct {
	network                            *CertifiedMixedAlgebraicShellBoundary
	geometry                           mesh.Mesh
	face                               int
	uv                                 [][2]*big.Rat
	errors                             []*big.Rat
	clearance                          []*big.Rat
	omitted, checks, pairs, operations int
}

func (r *CertifiedMixedRoofInterior) Mesh() mesh.Mesh {
	if r == nil {
		return mesh.Mesh{}
	}
	return mesh.Mesh{Vertices: append([]geom.Vec3(nil), r.geometry.Vertices...), Triangles: append([][3]int(nil), r.geometry.Triangles...)}
}
func (r *CertifiedMixedRoofInterior) OriginalFace() (native, operand, original int) {
	if r == nil {
		return -1, -1, -1
	}
	f := r.network.source.boundary.faces[r.face]
	return r.face, f.Operand, f.Original
}
func (r *CertifiedMixedRoofInterior) OmittedTriangles() int {
	if r == nil {
		return 0
	}
	return r.omitted
}
func (r *CertifiedMixedRoofInterior) Complete() bool { return r != nil && r.omitted == 0 }

// ExactUV gives original source parameters for referenced roof vertices. Nil
// entries belong to unrelated, unreferenced shared-network vertices.
func (r *CertifiedMixedRoofInterior) ExactUV() [][2]*big.Rat {
	if r == nil {
		return nil
	}
	out := make([][2]*big.Rat, len(r.uv))
	for i, p := range r.uv {
		if p[0] != nil {
			out[i] = [2]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1])}
		}
	}
	return out
}

// TriangleBounds returns squared model correspondence error and a strictly
// positive vertical clearance below the opposing roof and above both floors.
// Footprint containment is proved separately, without reporting a false metric.
func (r *CertifiedMixedRoofInterior) TriangleBounds() (errors, clearance []*big.Rat) {
	if r == nil {
		return
	}
	for _, x := range r.errors {
		errors = append(errors, new(big.Rat).Set(x))
	}
	for _, x := range r.clearance {
		clearance = append(clearance, new(big.Rat).Set(x))
	}
	return
}
func (r *CertifiedMixedRoofInterior) Work() (operations, triangleChecks, contactPairs int) {
	if r == nil {
		return
	}
	return r.operations, r.checks, r.pairs
}
func (r *CertifiedMixedRoofInterior) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

// TessellateUniqueRoofInteriorCertified is restricted to an intersection of two
// certified six-face polynomial graph boxes, with one native retained fragment
// on the chosen original roof. OppositeRoof names an ORIGINAL face of the other
// operand. Original graph identity, whole source box boundaries and positive
// roof height are reproved; no supplied support claim is trusted. Complete mode
// rejects any unproved boundary collar atomically. Shared boundary vertices are
// never moved; newly created interior vertices evaluate the original graph.
func (n *CertifiedMixedAlgebraicShellBoundary) TessellateUniqueRoofInteriorCertified(ctx context.Context, face, oppositeRoof int, o MixedRoofInteriorOptions) (*CertifiedMixedRoofInterior, error) {
	if ctx == nil || n == nil || n.source == nil || !n.embedded || n.source.boundary.op != parametric.Intersection || face < 0 || face >= len(n.source.boundary.faces) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f := n.source.boundary.faces[face]
	if f.Reversed || len(f.Holes) > 0 {
		return nil, geom.ErrUnrepresentable
	}
	count := 0
	for _, q := range n.source.boundary.faces {
		if q.Operand == f.Operand && q.Original == f.Original {
			count++
		}
	}
	if count != 1 {
		return nil, geom.ErrUnrepresentable
	}
	if o.Proof.MaxOperations == 0 {
		o.Proof.MaxOperations = 8000000
	}
	if o.Proof.MaxPredicates == 0 {
		o.Proof.MaxPredicates = 1000000
	}
	if o.Proof.MaxVertices == 0 {
		o.Proof.MaxVertices = 1024
	}
	if o.Proof.MaxPairs == 0 {
		o.Proof.MaxPairs = 2000000
	}
	if o.Interior.MaxTriangles == 0 {
		o.Interior.MaxTriangles = 8192
	}
	if o.Proof.MaxTriangles != 0 {
		if o.Proof.MaxTriangles < 1 || o.Proof.MaxTriangles > 32768 {
			return nil, geom.ErrInvalid
		}
		if o.Proof.MaxTriangles < o.Interior.MaxTriangles {
			o.Interior.MaxTriangles = o.Proof.MaxTriangles
		}
	}
	if o.Interior.MaxTriangleChecks == 0 {
		o.Interior.MaxTriangleChecks = 32768
	}
	if o.Interior.MaxSubdivisions == 0 {
		o.Interior.MaxSubdivisions = 16
	}
	if !finite(o.Interior.Deflection) || o.Interior.Deflection <= 0 || o.Proof.MaxOperations < 1 || o.Proof.MaxOperations > 128000000 || o.Proof.MaxPredicates < 1 || o.Proof.MaxPredicates > 100000000 || o.Proof.MaxVertices < 3 || o.Proof.MaxVertices > 4096 || o.Proof.MaxPairs < 1 || o.Proof.MaxPairs > 16000000 || o.Interior.MaxTriangles < 1 || o.Interior.MaxTriangles > 32768 || o.Interior.MaxTriangleChecks < 1 || o.Interior.MaxTriangleChecks > 1000000 || o.Interior.MaxSubdivisions < 1 || o.Interior.MaxSubdivisions > 128 || o.Interior.MaxSubdivisions&(o.Interior.MaxSubdivisions-1) != 0 {
		return nil, geom.ErrInvalid
	}
	sources := [2]Solid{n.source.boundary.source.a, n.source.boundary.source.b}
	host, e := proveMixedRoofBox(ctx, sources[f.Operand], f.Original, o.Proof.MaxOperations)
	if e != nil {
		return nil, e
	}
	other, e := proveMixedRoofBox(ctx, sources[1-f.Operand], oppositeRoof, o.Proof.MaxOperations-host.operations)
	if e != nil {
		return nil, e
	}
	r := &CertifiedMixedRoofInterior{network: n, face: face, operations: host.operations + other.operations, geometry: mesh.Mesh{Vertices: n.Vertices()}}
	ring := []int{}
	for _, u := range f.Outer {
		path := n.network.Edges[u.Edge]
		for j := range path {
			k := j
			if u.Reversed {
				k = len(path) - 1 - j
			}
			v := path[k]
			if j == 0 && len(ring) > 0 {
				if ring[len(ring)-1] != v {
					return nil, geom.ErrInvalid
				}
				continue
			}
			ring = append(ring, v)
			if len(ring) > o.Proof.MaxVertices+1 {
				return nil, geom.ErrConvergence
			}
		}
	}
	if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
		return nil, geom.ErrInvalid
	}
	ring = ring[:len(ring)-1]
	seed := CertifiedAffineQuadraticTrimMesh{}
	local := make([]int, len(ring))
	display := map[int]geom.Vec3{}
	for i, id := range ring {
		v := n.network.Vertices[id]
		local[i] = i
		seed.ExactUV = append(seed.ExactUV, [2]*big.Rat{ratInset(v.X), ratInset(v.Y)})
		seed.Mesh.Vertices = append(seed.Mesh.Vertices, geom.Vec3{X: v.X, Y: v.Y})
		display[i] = v
	}
	seed.BoundaryLoops = [][]int{local}
	seed.Region.FillSquaredDistance = new(big.Rat).Mul(ratInset(n.deflection), ratInset(n.deflection))
	proof := partitionFillProof{ctx: ctx, points: seed.ExactUV, limit: o.Proof.MaxPredicates}
	if e := proof.simple(local); e != nil {
		return nil, e
	}
	if proof.area(local).Sign() <= 0 {
		return nil, geom.ErrInvalid
	}
	seed.Mesh.Triangles, e = proof.triangulate(local)
	if e != nil {
		return nil, e
	}
	chain := map[[2]int]int{}
	area := new(big.Rat)
	for _, tri := range seed.Mesh.Triangles {
		a := proof.area(tri[:])
		if a.Sign() <= 0 {
			return nil, geom.ErrInvalid
		}
		area.Add(area, a)
		for j := 0; j < 3; j++ {
			partitionChainAdd(chain, tri[j], tri[(j+1)%3], 1)
		}
	}
	for j, v := range local {
		partitionChainAdd(chain, v, local[(j+1)%len(local)], -1)
	}
	if len(chain) != 0 || area.Cmp(proof.area(local)) != 0 {
		return nil, geom.ErrConvergence
	}
	l2, _, ok := quadraticPatchLipschitz(host.roof)
	if !ok {
		return nil, geom.ErrConvergence
	}
	refined, e := tessellateCertifiedTrimBoundaryDisplayMode(ctx, host.roof, seed, o.Interior, 2, nil, l2, true, display)
	if e != nil {
		return nil, e
	}
	if len(refined.Mesh.Triangles) > o.Interior.MaxTriangles {
		return nil, geom.ErrConvergence
	}
	r.checks = refined.TriangleChecks
	remap := make([]int, len(refined.Mesh.Vertices))
	r.uv = make([][2]*big.Rat, len(r.geometry.Vertices))
	for i, v := range refined.Mesh.Vertices {
		if i < len(ring) {
			remap[i] = ring[i]
			if v != r.geometry.Vertices[ring[i]] {
				return nil, geom.ErrInvalid
			}
		} else {
			remap[i] = len(r.geometry.Vertices)
			r.geometry.Vertices = append(r.geometry.Vertices, v)
			r.uv = append(r.uv, [2]*big.Rat{})
		}
		r.uv[remap[i]] = refined.ExactUV[i]
	}
	for index, t := range refined.Mesh.Triangles {
		r.checks++
		if r.checks > o.Interior.MaxTriangleChecks {
			return nil, geom.ErrConvergence
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		xy := [3][2]*big.Rat{refined.ExactUV[t[0]], refined.ExactUV[t[1]], refined.ExactUV[t[2]]}
		inside := true
		for _, box := range []*mixedRoofBox{host, other} {
			d := box.domain
			for _, p := range xy {
				inside = inside && p[0].Cmp(ratInset(d.UMin)) > 0 && p[0].Cmp(ratInset(d.UMax)) < 0 && p[1].Cmp(ratInset(d.VMin)) > 0 && p[1].Cmp(ratInset(d.VMax)) < 0
			}
		}
		if !inside {
			r.omitted++
			continue
		}
		a, e := mixedRoofTrianglePower(host.roof, xy)
		if e != nil {
			return nil, e
		}
		b, e := mixedRoofTrianglePower(other.roof, xy)
		if e != nil {
			return nil, e
		}
		height, _ := mixedRoofPolynomialRange(a)
		height.Sub(height, host.floor)
		otherFloor, _ := mixedRoofPolynomialRange(a)
		otherFloor.Sub(otherFloor, other.floor)
		for i := 0; i <= 5; i++ {
			for j := 0; j <= 5-i; j++ {
				b[i][j].Sub(b[i][j], a[i][j])
			}
		}
		gap, _ := mixedRoofPolynomialRange(b)
		if height.Sign() <= 0 || otherFloor.Sign() <= 0 || gap.Sign() <= 0 {
			r.omitted++
			continue
		}
		margin := new(big.Rat).Set(gap)
		for _, x := range []*big.Rat{height, otherFloor} {
			if x.Cmp(margin) < 0 {
				margin.Set(x)
			}
		}
		r.geometry.Triangles = append(r.geometry.Triangles, [3]int{remap[t[0]], remap[t[1]], remap[t[2]]})
		r.errors = append(r.errors, new(big.Rat).Set(refined.SquaredErrors[index]))
		r.clearance = append(r.clearance, margin)
	}
	if len(r.geometry.Triangles) == 0 || o.RequireComplete && r.omitted > 0 {
		return nil, geom.ErrConvergence
	}
	// The represented mesh is checked independently of source residual proofs.
	contact := partitionFillProof{ctx: ctx, limit: o.Proof.MaxPredicates, work: proof.work, points: make([][2]*big.Rat, len(r.geometry.Vertices))}
	z := make([]*big.Rat, len(contact.points))
	for i, v := range r.geometry.Vertices {
		contact.points[i] = [2]*big.Rat{ratInset(v.X), ratInset(v.Y)}
		z[i] = ratInset(v.Z)
	}
	for _, t := range r.geometry.Triangles {
		sign, e := contact.orient(t[0], t[1], t[2])
		if e != nil {
			return nil, e
		}
		if sign <= 0 {
			return nil, geom.ErrConvergence
		}
		for _, path := range n.network.Edges {
			for j := 1; j < len(path); j++ {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				r.pairs++
				if r.pairs >= o.Proof.MaxPairs {
					return nil, geom.ErrConvergence
				}
				if e := mixedAffineSegmentTriangle(&contact, z, t, path[j-1], path[j]); e != nil {
					return nil, e
				}
			}
		}
	}
	c, e := collision.SelfIntersections(r.geometry, collision.Options{MaxPairs: o.Proof.MaxPairs - r.pairs, MaxContacts: 1})
	r.pairs += c.BroadPhasePairs
	if e != nil {
		return nil, e
	}
	if !c.Complete || len(c.Contacts) > 0 {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}
