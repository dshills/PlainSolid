package heal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// ErrUnsafeCollapse means the requested edge cannot be contracted under the
// distance, planar support, or topology contract.
var ErrUnsafeCollapse = errors.New("heal: unsafe short edge collapse")

type ShortEdgeOptions struct {
	// MaxDistance is a positive finite limit in model units.
	MaxDistance float64
	// KeepEnd retains the edge's stored end instead of its stored start.
	KeepEnd    bool
	Validation collision.LocationOptions
}

// ShortEdgeResult maps original indices to the new solid. Both endpoint IDs
// map to the retained vertex, and only the contracted edge maps to -1. Face
// indices are unchanged. DistanceSquared is the exact displacement certificate.
type ShortEdgeResult struct {
	Solid                     brep.Solid
	VertexMap, EdgeMap        []int
	KeptVertex, RemovedVertex int // original IDs
	AffectedFaces             []int
	DistanceSquared           *big.Rat
}

// CollapseShortEdge removes one explicitly selected internal edge from a planar
// patch, moving only one endpoint to the other. Every face incident to the
// removed endpoint must be on the same exact plane and survive the contraction.
// Only internal patch edges can move, so the patch boundary and the material
// it encloses are unchanged. Both the source and result must be embedded and
// correctly oriented. No partial result is returned on failure.
func CollapseShortEdge(ctx context.Context, solid brep.Solid, edge int, options ShortEdgeOptions) (ShortEdgeResult, error) {
	var empty ShortEdgeResult
	if ctx == nil || options.MaxDistance <= 0 || math.IsNaN(options.MaxDistance) || math.IsInf(options.MaxDistance, 0) {
		return empty, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	v, e, f := solid.Vertices(), solid.Edges(), solid.Faces()
	if edge < 0 || edge >= len(e) {
		return empty, ErrInvalid
	}
	if len(v) > MaxInputVertices || len(f) > MaxFaces {
		return empty, ErrLimit
	}
	keep, drop := e[edge].Start, e[edge].End
	if options.KeepEnd {
		keep, drop = drop, keep
	}
	distance := exactSquaredDistance(v[keep], v[drop])
	limit := new(big.Rat).SetFloat64(options.MaxDistance)
	if distance.Sign() == 0 || distance.Cmp(limit.Mul(limit, limit)) > 0 {
		return empty, fmt.Errorf("%w: endpoint displacement", ErrUnsafeCollapse)
	}
	var affected []int
	var plane [3]geom.Vec3
	for i, face := range f {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		loops := append([][]brep.EdgeUse{face.Boundary}, face.Holes...)
		touches := false
		for _, loop := range loops {
			for _, use := range loop {
				if e[use.Edge].Start == drop || e[use.Edge].End == drop {
					touches = true
				}
			}
		}
		if !touches {
			continue
		}
		if len(affected) == 0 {
			for j, use := range face.Boundary[:3] {
				p := e[use.Edge].Start
				if use.Reversed {
					p = e[use.Edge].End
				}
				plane[j] = v[p]
			}
			if !exactPlaneTriple(plane) {
				return empty, fmt.Errorf("%w: no exact plane at the first incident face corner", ErrUnsafeCollapse)
			}
		}
		for _, loop := range loops {
			for _, use := range loop {
				p := e[use.Edge].Start
				if use.Reversed {
					p = e[use.Edge].End
				}
				if sign, err := geom.Orient3D(plane[0], plane[1], plane[2], v[p]); err != nil || sign != 0 {
					return empty, fmt.Errorf("%w: incident face %d leaves the exact patch plane", ErrUnsafeCollapse, i)
				}
			}
		}
		affected = append(affected, i)
	}
	if _, err := collision.NewMeshPointLocator(ctx, solid.Tessellate(), solid.TriangleFaces(), options.Validation); err != nil {
		return empty, planarValidationError(err)
	}
	nextV := append(v[:drop:drop], v[drop+1:]...)
	vm := make([]int, len(v))
	for i := range vm {
		vm[i] = i
		if i > drop {
			vm[i]--
		}
	}
	vm[drop] = vm[keep]
	em := make([]int, len(e))
	nextE := make([]brep.Edge, 0, len(e)-1)
	for i, current := range e {
		if i == edge {
			em[i] = -1
			continue
		}
		em[i] = len(nextE)
		nextE = append(nextE, brep.Edge{Start: vm[current.Start], End: vm[current.End]})
	}
	rewrite := func(loop []brep.EdgeUse) []brep.EdgeUse {
		out := loop[:0]
		for _, use := range loop {
			use.Edge = em[use.Edge]
			if use.Edge >= 0 {
				out = append(out, use)
			}
		}
		return out
	}
	for i := range f {
		f[i].Boundary = rewrite(f[i].Boundary)
		for j := range f[i].Holes {
			f[i].Holes[j] = rewrite(f[i].Holes[j])
		}
	}
	candidate, err := brep.NewSolid(nextV, nextE, f)
	if err != nil {
		return empty, fmt.Errorf("%w: %v", ErrUnsafeCollapse, err)
	}
	before, after := solid.Topology(), candidate.Topology()
	before.Vertices--
	before.Edges--
	if before != after {
		return empty, fmt.Errorf("%w: boundary topology changed", ErrUnsafeCollapse)
	}
	if _, err = collision.NewMeshPointLocator(ctx, candidate.Tessellate(), candidate.TriangleFaces(), options.Validation); err != nil {
		if errors.Is(err, collision.ErrInvalid) {
			return empty, fmt.Errorf("%w: %w", ErrUnsafeCollapse, err)
		}
		return empty, planarValidationError(err)
	}
	return ShortEdgeResult{Solid: candidate, VertexMap: vm, EdgeMap: em, KeptVertex: keep, RemovedVertex: drop, AffectedFaces: affected, DistanceSquared: distance}, nil
}

func exactPlaneTriple(p [3]geom.Vec3) bool {
	for _, xy := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		project := func(a geom.Vec3) geom.Vec2 {
			v := [3]float64{a.X, a.Y, a.Z}
			return geom.Vec2{X: v[xy[0]], Y: v[xy[1]]}
		}
		if sign, err := geom.Orient2D(project(p[0]), project(p[1]), project(p[2])); err == nil && sign != 0 {
			return true
		}
	}
	return false
}

func exactSquaredDistance(a, b geom.Vec3) *big.Rat {
	sum := new(big.Rat)
	for _, axis := range [3][2]float64{{a.X, b.X}, {a.Y, b.Y}, {a.Z, b.Z}} {
		d := new(big.Rat).Sub(new(big.Rat).SetFloat64(axis[0]), new(big.Rat).SetFloat64(axis[1]))
		sum.Add(sum, d.Mul(d, d))
	}
	return sum
}
