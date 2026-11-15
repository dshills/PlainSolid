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
	var zero PlanarFaceContact
	if ctx == nil || o.MaxBoundaryChecks < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	maxWork := o.MaxBoundaryChecks
	if maxWork == 0 {
		maxWork = DefaultMaxPairs
	}
	budget := locationBudget{ctx: ctx, limit: maxWork}
	index := make(map[faceBoundaryKey][]faceBoundaryCandidate)
	candidate := [2]int{-1, -1}

	for body, solid := range [2]brep.Solid{b, a} {
		vs, es := solid.Vertices(), solid.Edges()
		for i, face := range solid.Faces() {
			n := len(face.Boundary)
			if err := budget.tick(n); err != nil {
				return zero, err
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
					return zero, err
				}
				same := true
				for j := 0; j < n; j++ {
					if c.boundary[(c.anchor+j)%n] != q.boundary[(q.anchor+n-j)%n] {
						same = false
						break
					}
				}
				if same {
					if candidate[0] != -1 {
						return zero, fmt.Errorf("%w: more than one matching original face perimeter", ErrInvalid)
					}
					candidate = [2]int{i, q.face}
				}
			}
		}
	}
	if candidate[0] == -1 {
		return zero, fmt.Errorf("%w: no complete matching original face perimeter", ErrInvalid)
	}
	return ValidatePlanarFaceContact(ctx, a, b, candidate[0], candidate[1], o.Contact)
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
