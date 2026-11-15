package collision

import (
	"context"
	"fmt"

	"plainsolid/brep"
)

// PlanarRegionCandidates lists original faces participating in positive-area
// coplanar overlap. It is an exact discovery result, not a proof that entire
// faces or only those faces are in contact.
type PlanarRegionCandidates struct {
	Faces      [2][]int
	BroadPairs int
}

// FindPlanarRegionCandidates identifies source faces before a caller
// reconciles different original seam divisions. It rejects transverse
// crossings and missing positive area. Validate the completed seam and
// material separately before using these candidates to join the bodies.
func FindPlanarRegionCandidates(ctx context.Context, a, b brep.Solid, o PlanarFaceContactOptions) (PlanarRegionCandidates, error) {
	var zero PlanarRegionCandidates
	if ctx == nil || o.Cross.MaxContacts < 0 || o.Cross.MaxPairs < 0 || o.MaxPerimeterChecks < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	options, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	am, bm := a.Tessellate(), b.Tessellate()
	if len(am.Triangles) > options.MaxTriangles-len(bm.Triangles) {
		return zero, ErrLimit
	}
	m := [2][]int{a.TriangleFaces(), b.TriangleFaces()}
	seen := [2][]bool{make([]bool, len(a.Faces())), make([]bool, len(b.Faces()))}
	result, err := sweepIntersections(ctx, am, bm, false, o.Cross, func(i, j int, k Kind, _ []point) error {
		switch k {
		case CoplanarOverlap:
			seen[0][m[0][i]], seen[1][m[1][j]] = true, true
		case Touching:
		default:
			return fmt.Errorf("%w: transverse contact outside a coincident planar region", ErrInvalid)
		}
		return nil
	})
	if err != nil {
		return zero, err
	}
	if !result.Complete {
		return zero, ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out := PlanarRegionCandidates{BroadPairs: result.BroadPhasePairs}
	for i, faces := range seen {
		for id, on := range faces {
			if on {
				out.Faces[i] = append(out.Faces[i], id)
			}
		}
		if len(out.Faces[i]) == 0 {
			return zero, fmt.Errorf("%w: no positive common face area", ErrInvalid)
		}
	}
	return out, nil
}
