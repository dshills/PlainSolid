package collision

import (
	"context"
	"fmt"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlanarFaceDiscoveryOptions additionally bounds the total number of original
// edge uses and candidate vertices examined during face discovery.
type PlanarFaceDiscoveryOptions struct {
	Contact           PlanarFaceContactOptions
	MaxBoundaryChecks int // Zero selects one million.
}

type faceBoundaryKey struct {
	anchor, prev, next geom.Vec3
	edges, holes       int
}

type faceBoundaryCandidate struct {
	face, anchor int
	boundary     []geom.Vec3
}

// DiscoverPlanarFaceContact identifies and proves a unique complete common
// original face without trusting supplied face indices or triangulation IDs.
func DiscoverPlanarFaceContact(ctx context.Context, a, b brep.Solid, o PlanarFaceDiscoveryOptions) (PlanarFaceContact, error) {
	pairs, err := discoverPlanarFacePairs(ctx, a, b, o)
	if err != nil {
		return PlanarFaceContact{}, err
	}
	if len(pairs) != 1 {
		return PlanarFaceContact{}, fmt.Errorf("%w: more than one matching original face perimeter", ErrInvalid)
	}
	return ValidatePlanarFaceContact(ctx, a, b, pairs[0][0], pairs[0][1], o.Contact)
}

// DiscoverPlanarFaceContactSet finds all complete original matches and then
// proves that they cover all contact between the two bodies.
func DiscoverPlanarFaceContactSet(ctx context.Context, a, b brep.Solid, o PlanarFaceDiscoveryOptions) (PlanarFaceContactSet, error) {
	pairs, err := discoverPlanarFacePairs(ctx, a, b, o)
	if err != nil {
		return PlanarFaceContactSet{}, err
	}
	return ValidatePlanarFaceContactSet(ctx, a, b, pairs, o.Contact)
}

func discoverPlanarFacePairs(ctx context.Context, a, b brep.Solid, o PlanarFaceDiscoveryOptions) ([][2]int, error) {
	if ctx == nil || o.MaxBoundaryChecks < 0 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxWork := o.MaxBoundaryChecks
	if maxWork == 0 {
		maxWork = DefaultMaxPairs
	}
	budget := locationBudget{ctx: ctx, limit: maxWork}
	index := make(map[faceBoundaryKey][]faceBoundaryCandidate)
	var candidates [][2]int

	for body, solid := range [2]brep.Solid{b, a} {
		vs, es := solid.Vertices(), solid.Edges()
		for i, face := range solid.Faces() {
			n := len(face.Boundary)
			if err := budget.tick(n); err != nil {
				return nil, err
			}
			c := faceBoundaryCandidate{face: i, boundary: make([]geom.Vec3, n)}
			for j, use := range face.Boundary {
				e := es[use.Edge]
				k := e.Start
				if use.Reversed {
					k = e.End
				}
				p := vs[k]
				c.boundary[j] = p
				if lexVecLess(p, c.boundary[c.anchor]) {
					c.anchor = j
				}
			}
			k := faceBoundaryKey{
				anchor: c.boundary[c.anchor], prev: c.boundary[(c.anchor+n-1)%n],
				next: c.boundary[(c.anchor+1)%n], edges: n, holes: len(face.Holes),
			}
			if body == 0 {
				k.prev, k.next = k.next, k.prev
				index[k] = append(index[k], c)
				continue
			}
			for _, q := range index[k] {
				if err := budget.tick(n); err != nil {
					return nil, err
				}
				same := true
				for j := 0; j < n; j++ {
					if c.boundary[(c.anchor+j)%n] != q.boundary[(q.anchor+n-j)%n] {
						same = false
						break
					}
				}
				if same {
					candidates = append(candidates, [2]int{i, q.face})
				}
			}
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no complete matching original face perimeter", ErrInvalid)
	}
	return candidates, nil
}

func lexVecLess(a, b geom.Vec3) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.Z < b.Z
}
