package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AffinePartitionFillOptions caps exact predicate calls (default 1M) and
// represented ring vertices (default 256, maximum 1024).
type AffinePartitionFillOptions struct {
	Boundary                   AlgebraicBoundaryMeshOptions
	MaxVertices, MaxPredicates int
}

// CertifiedAffinePartitionFill proves exact projected coverage of the two
// represented fragment polygons and their common represented original polygon.
// Triangles are graphs over ProjectionAxes with shared display coordinates.
// This does not assert fill-distance to the original curved source regions.
type CertifiedAffinePartitionFill struct {
	Boundary           CertifiedAffinePartitionBoundary
	Mesh               mesh.Mesh
	ProjectionAxes     [2]int
	ProjectionReversed bool // Display orientation follows the original physical face.
	NetworkVertices    []int
	OriginalFragment   []int
	ProjectedArea      *big.Rat
	FragmentAreas      [2]*big.Rat
	Predicates         int
}

type partitionFillProof struct {
	ctx         context.Context
	points      [][2]*big.Rat
	work, limit int
}

func (p *partitionFillProof) orient(a, b, c int) (int, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	p.work++
	if p.work > p.limit {
		return 0, geom.ErrConvergence
	}
	x, y, z := p.points[a], p.points[b], p.points[c]
	left := new(big.Rat).Mul(new(big.Rat).Sub(y[0], x[0]), new(big.Rat).Sub(z[1], x[1]))
	right := new(big.Rat).Mul(new(big.Rat).Sub(y[1], x[1]), new(big.Rat).Sub(z[0], x[0]))
	return left.Sub(left, right).Sign(), nil
}
func (p *partitionFillProof) area(ring []int) *big.Rat {
	sum := new(big.Rat)
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		x, y := p.points[a], p.points[b]
		sum.Add(sum, new(big.Rat).Sub(new(big.Rat).Mul(x[0], y[1]), new(big.Rat).Mul(x[1], y[0])))
	}
	return sum.Quo(sum, big.NewRat(2, 1))
}
func betweenRat(x, a, b *big.Rat) bool {
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	return x.Cmp(a) >= 0 && x.Cmp(b) <= 0
}
func (p *partitionFillProof) on(a, b, c int) bool {
	return betweenRat(p.points[c][0], p.points[a][0], p.points[b][0]) && betweenRat(p.points[c][1], p.points[a][1], p.points[b][1])
}
func (p *partitionFillProof) simple(ring []int) error {
	if len(ring) < 3 {
		return geom.ErrDegenerate
	}
	seen := map[string]bool{}
	for _, id := range ring {
		key := p.points[id][0].RatString() + "," + p.points[id][1].RatString()
		if seen[key] {
			return geom.ErrDegenerate
		}
		seen[key] = true
	}
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		for j := i + 1; j < len(ring); j++ {
			c, d := ring[j], ring[(j+1)%len(ring)]
			if a == c || a == d || b == c || b == d {
				continue
			}
			x, e := p.orient(a, b, c)
			if e != nil {
				return e
			}
			y, e := p.orient(a, b, d)
			if e != nil {
				return e
			}
			z, e := p.orient(c, d, a)
			if e != nil {
				return e
			}
			w, e := p.orient(c, d, b)
			if e != nil {
				return e
			}
			if x*y < 0 && z*w < 0 || x == 0 && p.on(a, b, c) || y == 0 && p.on(a, b, d) || z == 0 && p.on(c, d, a) || w == 0 && p.on(c, d, b) {
				return geom.ErrInvalid
			}
		}
	}
	return nil
}
func (p *partitionFillProof) triangulate(ring []int) ([][3]int, error) {
	remaining := append([]int(nil), ring...)
	var tris [][3]int
	for len(remaining) > 3 {
		found := false
		for i, b := range remaining {
			a, c := remaining[(i+len(remaining)-1)%len(remaining)], remaining[(i+1)%len(remaining)]
			turn, e := p.orient(a, b, c)
			if e != nil {
				return nil, e
			}
			if turn <= 0 {
				continue
			}
			blocked := false
			for _, v := range remaining {
				if v == a || v == b || v == c {
					continue
				}
				x, e := p.orient(a, b, v)
				if e != nil {
					return nil, e
				}
				y, e := p.orient(b, c, v)
				if e != nil {
					return nil, e
				}
				z, e := p.orient(c, a, v)
				if e != nil {
					return nil, e
				}
				if x >= 0 && y >= 0 && z >= 0 {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			tris = append(tris, [3]int{a, b, c})
			remaining = append(remaining[:i], remaining[i+1:]...)
			found = true
			break
		}
		if !found {
			return nil, geom.ErrConvergence
		}
	}
	if len(remaining) == 3 {
		turn, e := p.orient(remaining[0], remaining[1], remaining[2])
		if e != nil {
			return nil, e
		}
		if turn <= 0 {
			return nil, geom.ErrDegenerate
		}
		tris = append(tris, [3]int{remaining[0], remaining[1], remaining[2]})
	}
	return tris, nil
}
func partitionChainAdd(chain map[[2]int]int, a, b, amount int) {
	if a > b {
		a, b = b, a
		amount = -amount
	}
	key := [2]int{a, b}
	chain[key] += amount
	if chain[key] == 0 {
		delete(chain, key)
	}
}

// TessellateAffinePartitionFillCertified certifies represented-polygon fill.
// Exact simple-polygon predicates establish winding indicators. Positive
// projected triangles, exact boundary-chain cancellation and equal rational
// area prove each triangle union equals its fragment; the same chain identity
// proves the two fragments partition the represented original polygon.
// Shared vertices make the lifted triangles a continuous graph over this
// projection. Display orientation is finally adjusted to the physical face.
// No original curved-region fill-distance estimate is inferred from these facts.
func (s AlgebraicSubdivision) TessellateAffinePartitionFillCertified(ctx context.Context, face int, line [3]*big.Rat, o AffinePartitionFillOptions) (CertifiedAffinePartitionFill, error) {
	var empty CertifiedAffinePartitionFill
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.MaxVertices < 3 || o.MaxVertices > 1024 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return empty, geom.ErrInvalid
	}
	boundary, e := s.TessellateAffinePartitionBoundaryCertified(ctx, face, line, o.Boundary)
	if e != nil {
		return empty, e
	}
	patch, e := exactFaceSupportPatch(s.source.faces[face], o.Boundary.Proof.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Boundary.Proof.Composition.MaxOperations, MaxCoefficientBits: o.Boundary.Proof.Composition.MaxCoefficientBits})
	if e != nil {
		return empty, e
	}
	axes := chart.ProjectionAxes()
	proof := partitionFillProof{ctx: ctx, limit: o.MaxPredicates, points: make([][2]*big.Rat, len(boundary.Boundary.Vertices))}
	for i, v := range boundary.Boundary.Vertices {
		xyz := []float64{v.X, v.Y, v.Z}
		proof.points[i] = [2]*big.Rat{ratInset(xyz[axes[0]]), ratInset(xyz[axes[1]])}
	}
	var original []int
	for _, use := range s.faces[face].Outer {
		path := append([]int(nil), boundary.Boundary.Edges[use.Edge]...)
		if use.Reversed {
			for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
				path[a], path[b] = path[b], path[a]
			}
		}
		if len(original) > 0 {
			if original[len(original)-1] != path[0] {
				return empty, geom.ErrInvalid
			}
			path = path[1:]
		}
		original = append(original, path...)
	}
	if len(original) < 4 || original[0] != original[len(original)-1] {
		return empty, geom.ErrInvalid
	}
	original = original[:len(original)-1]
	rings := [][]int{original, boundary.Fragments[0], boundary.Fragments[1]}
	reverseDisplay := (proof.area(original).Sign() < 0) != s.faces[face].Reversed
	areas := make([]*big.Rat, 3)
	for i, ring := range rings {
		if len(ring) > o.MaxVertices {
			return empty, geom.ErrConvergence
		}
		if e := proof.simple(ring); e != nil {
			return empty, e
		}
		areas[i] = proof.area(ring)
		if areas[i].Sign() == 0 {
			return empty, geom.ErrDegenerate
		}
		rings[i] = append([]int(nil), ring...)
		if areas[i].Sign() < 0 {
			areas[i].Neg(areas[i])
			for a, b := 0, len(ring)-1; a < b; a, b = a+1, b-1 {
				rings[i][a], rings[i][b] = rings[i][b], rings[i][a]
			}
		}
	}
	if new(big.Rat).Add(areas[1], areas[2]).Cmp(areas[0]) != 0 {
		return empty, geom.ErrConvergence
	}
	chain := map[[2]int]int{}
	for i, ring := range rings {
		sign := 1
		if i == 0 {
			sign = -1
		}
		for j, a := range ring {
			partitionChainAdd(chain, a, ring[(j+1)%len(ring)], sign)
		}
	}
	if len(chain) != 0 {
		return empty, geom.ErrConvergence
	}
	result := CertifiedAffinePartitionFill{Boundary: boundary, ProjectionAxes: axes, ProjectionReversed: reverseDisplay, ProjectedArea: areas[0], FragmentAreas: [2]*big.Rat{areas[1], areas[2]}}
	remap := map[int]int{}
	for fi, ring := range rings[1:] {
		tris, e := proof.triangulate(ring)
		if e != nil {
			return empty, e
		}
		triangleArea := new(big.Rat)
		boundaryChain := map[[2]int]int{}
		for j, a := range ring {
			partitionChainAdd(boundaryChain, a, ring[(j+1)%len(ring)], -1)
		}
		for _, tri := range tris {
			triangleArea.Add(triangleArea, proof.area(tri[:]))
			out := [3]int{}
			for j, id := range tri {
				partitionChainAdd(boundaryChain, id, tri[(j+1)%3], 1)
				mapped, ok := remap[id]
				if !ok {
					mapped = len(result.Mesh.Vertices)
					remap[id] = mapped
					result.Mesh.Vertices = append(result.Mesh.Vertices, boundary.Boundary.Vertices[id])
					result.NetworkVertices = append(result.NetworkVertices, id)
				}
				out[j] = mapped
			}
			if reverseDisplay {
				out[1], out[2] = out[2], out[1]
			}
			result.Mesh.Triangles = append(result.Mesh.Triangles, out)
			result.OriginalFragment = append(result.OriginalFragment, fi)
		}
		if triangleArea.Cmp(areas[fi+1]) != 0 || len(boundaryChain) != 0 {
			return empty, geom.ErrConvergence
		}
	}
	result.Predicates = proof.work
	return result, nil
}
