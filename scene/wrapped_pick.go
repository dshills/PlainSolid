package scene

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedPickItem identifies a certified canonical capped frustum,
// circular-profile toroid, circular station loft, nested through-bore,
// or filleted circular cylinder
// in scene coordinates.
type WrappedPickItem struct {
	ID          string
	Solid       trimmed.Solid
	Translation geom.Vec3 // optional scene placement of the local solid
}

// WrappedPickResult identifies the nearest proved scene contact. Its
// distance is measured on the represented camera ray from the near plane.
// Part reports capped body boundary ancestry; it is zero for a toroid.
// Faces identifies original face indices when supplied by the shape query.
type WrappedPickResult struct {
	ID         string
	Distance   geom.ToroidLineRoot
	Part       geom.CylinderBoundaryPart
	Faces      []int
	Coincident bool
}

// PickWrappedMaterials queries the exact wrapped supports instead of a
// display mesh. Overlapping first-hit enclosures are unresolved unless
// they are the same exact parameter, in which case ID order breaks the
// tie. An unsupported body cannot be silently skipped.
func PickWrappedMaterials(ctx context.Context, camera Camera, screen geom.Vec2, items []WrappedPickItem,
	o geom.ToroidLineOptions) (WrappedPickResult, bool, error) {
	var zero WrappedPickResult
	if ctx == nil || len(items) > 1024 {
		return zero, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	ray, e := camera.Ray(screen)
	if e != nil {
		return zero, false, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	var hits []WrappedPickResult
	ids := map[string]bool{}
	for _, x := range items {
		if e := ctx.Err(); e != nil {
			return zero, false, e
		}
		if x.ID == "" || ids[x.ID] {
			return zero, false, fmt.Errorf("scene: invalid wrapped item ID %q", x.ID)
		}
		ids[x.ID] = true
		origin, e := exactWrappedRayOrigin(ray.Origin, x.Translation)
		if e != nil {
			return zero, false, fmt.Errorf("scene: wrapped item %q: %w", x.ID, e)
		}
		switch x.Solid.Topology().Faces {
		case 1:
			roots, e := x.Solid.IntersectCanonicalWrappedToroidLine(ctx, origin, ray.Direction, 0, ray.Length, o)
			if e != nil {
				return zero, false, fmt.Errorf("scene: wrapped item %q: %w", x.ID, e)
			}
			for _, r := range roots {
				hits = append(hits, WrappedPickResult{ID: x.ID, Distance: r})
			}
		case 3:
			r, e := x.Solid.IntersectCanonicalWrappedFrustumLine(ctx, origin, ray.Direction, 0, ray.Length, o)
			if e != nil {
				return zero, false, fmt.Errorf("scene: wrapped item %q: %w", x.ID, e)
			}
			first := len(hits)
			for _, h := range r.Hits {
				hits = append(hits, WrappedPickResult{ID: x.ID, Distance: h.Parameter, Part: h.Parts})
			}
			for _, h := range r.Overlaps {
				hits = appendWrappedPickOverlap(hits, first, WrappedPickResult{ID: x.ID, Distance: h.Start, Part: h.Part, Coincident: true})
			}
		default:
			r, e := x.Solid.IntersectWrappedCircularStationLoftLine(ctx, origin, ray.Direction, 0, ray.Length, o)
			if errors.Is(e, geom.ErrConvergence) && x.Solid.Topology().Faces <= 5 {
				r, e = x.Solid.IntersectWrappedCylinderRimFilletLine(ctx, origin, ray.Direction, 0, ray.Length, o)
			}
			if errors.Is(e, geom.ErrConvergence) && x.Solid.Topology().InnerWires == 2 {
				r, e = x.Solid.IntersectWrappedCircularStationBoreLine(ctx, origin, ray.Direction, 0, ray.Length, o)
			}
			if e != nil {
				return zero, false, fmt.Errorf("scene: wrapped item %q: %w", x.ID, e)
			}
			first := len(hits)
			for _, h := range r.Hits {
				hits = append(hits, WrappedPickResult{ID: x.ID, Distance: h.Parameter, Part: h.Parts, Faces: h.Faces})
			}
			for _, h := range r.Overlaps {
				hits = appendWrappedPickOverlap(hits, first, WrappedPickResult{ID: x.ID, Distance: h.Start, Part: h.Part, Faces: []int{h.Face}, Coincident: true})
			}
		}
	}
	if len(hits) == 0 {
		return zero, false, nil
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if c := hits[i].Distance.Min.Cmp(hits[j].Distance.Min); c != 0 {
			return c < 0
		}
		return hits[i].ID < hits[j].ID
	})
	best := hits[0]
	equalPoint := func(a, b geom.ToroidLineRoot) bool {
		return a.Min.Cmp(a.Max) == 0 && b.Min.Cmp(b.Max) == 0 && a.Min.Cmp(b.Min) == 0
	}
	for _, h := range hits[1:] {
		if h.Distance.Min.Cmp(best.Distance.Max) > 0 {
			break
		}
		if !equalPoint(best.Distance, h.Distance) {
			return zero, false, fmt.Errorf("scene: overlapping nearest wrapped contact enclosures: %w", geom.ErrConvergence)
		}
		if h.ID == best.ID {
			best.Part |= h.Part
			best.Coincident = best.Coincident || h.Coincident
			best.Faces = mergeWrappedPickFaces(best.Faces, h.Faces)
		}
	}
	best.Distance.Min, best.Distance.Max = new(big.Rat).Set(best.Distance.Min), new(big.Rat).Set(best.Distance.Max)
	return best, true, nil
}

func exactWrappedRayOrigin(origin, translation geom.Vec3) (geom.Vec3, error) {
	if !translation.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	if translation == (geom.Vec3{}) {
		return origin, nil
	}
	x, t := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{translation.X, translation.Y, translation.Z}
	var q [3]*big.Rat
	for i := range q {
		q[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(x[i]), new(big.Rat).SetFloat64(t[i]))
	}
	return geom.RepresentedExactPoint(q)
}

func appendWrappedPickOverlap(hits []WrappedPickResult, first int, overlap WrappedPickResult) []WrappedPickResult {
	// A cap overlap may start at the same algebraic rim root returned as
	// a point hit. The shape query derives both brackets from that root.
	for i := first; i < len(hits); i++ {
		h := &hits[i]
		if h.Part&overlap.Part != 0 && h.Distance.Min.Cmp(overlap.Distance.Min) == 0 && h.Distance.Max.Cmp(overlap.Distance.Max) == 0 {
			h.Coincident = true
			h.Faces = mergeWrappedPickFaces(h.Faces, overlap.Faces)
			return hits
		}
	}
	return append(hits, overlap)
}

func mergeWrappedPickFaces(a, b []int) []int {
	for _, f := range b {
		i := sort.SearchInts(a, f)
		if i == len(a) || a[i] != f {
			a = append(a, f)
			copy(a[i+1:], a[i:])
			a[i] = f
		}
	}
	return a
}
