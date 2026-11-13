package scene

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// SplineGraphPickItem identifies a proved shared-base graph envelope.
// Translation places the unit chart in scene coordinates.
type SplineGraphPickItem struct {
	ID          string
	Graph       trimmed.SharedBaseSplineGraphMaterial
	Operation   parametric.BooleanOp
	Translation geom.Vec3
}

// SplineGraphPickResult retains the exact first contact enclosure and
// original base, top, or wall ownership. Distance is in the camera ray
// parameter measured from the near plane.
type SplineGraphPickResult struct {
	ID         string
	Parameter  geom.ToroidLineRoot
	Base       bool
	TopFaces   []int
	Sides      []int
	Coincident bool
}

// PickSplineGraphEnvelopes picks among exact graph envelopes, including
// envelopes that cannot be assembled as native topology. It supports
// isolated general line hits and coincident vertical or side-wall hits.
// Unresolved geometry or depth order fails the entire selection.
func PickSplineGraphEnvelopes(ctx context.Context, camera Camera, screen geom.Vec2, items []SplineGraphPickItem,
	o geom.ToroidLineOptions) (SplineGraphPickResult, bool, error) {
	var empty SplineGraphPickResult
	if ctx == nil || len(items) > 1024 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 ||
		o.ParameterTolerance < 0 || math.IsNaN(o.ParameterTolerance) || math.IsInf(o.ParameterTolerance, 0) {
		return empty, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, false, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	ray, e := camera.Ray(screen)
	if e != nil {
		return empty, false, e
	}
	var hits []SplineGraphPickResult
	ids := map[string]bool{}
	for _, x := range items {
		if x.ID == "" || ids[x.ID] {
			return empty, false, geom.ErrInvalid
		}
		ids[x.ID] = true
		a, e := exactWrappedRayOrigin(ray.Origin, x.Translation)
		if e != nil {
			return empty, false, fmt.Errorf("scene: graph item %q: %w", x.ID, e)
		}
		d := ray.Direction
		if d.X == 0 && d.Y == 0 {
			q, e := x.Graph.IntersectEnvelopeVerticalLine(ctx, a, d.Z, 0, ray.Length, x.Operation,
				geom.SurfacePatchEvaluationOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits})
			if e != nil {
				return empty, false, fmt.Errorf("scene: graph item %q: %w", x.ID, e)
			}
			for _, h := range q.Hits {
				hits = append(hits, SplineGraphPickResult{ID: x.ID,
					Parameter: geom.ToroidLineRoot{Min: h.Parameter, Max: new(big.Rat).Set(h.Parameter), Multiplicity: 1},
					Base:      h.Base, TopFaces: h.TopFaces, Sides: graphPickSides(a), Coincident: h.SideWall})
			}
			if q.Coincident != nil {
				t := q.Coincident[0]
				hits = append(hits, SplineGraphPickResult{ID: x.ID,
					Parameter: geom.ToroidLineRoot{Min: t, Max: new(big.Rat).Set(t), Multiplicity: 1},
					Sides:     graphPickSides(a), Coincident: true})
			}
			continue
		}
		var local []trimmed.SharedBaseGraphEnvelopeHit
		var spans [][2]geom.ToroidLineRoot
		onSide := d.X == 0 && (a.X == 0 || a.X == 1) || d.Y == 0 && (a.Y == 0 || a.Y == 1)
		if onSide {
			q, e := x.Graph.IntersectEnvelopeSideLine(ctx, a, d, 0, ray.Length, x.Operation, o)
			if e != nil {
				return empty, false, fmt.Errorf("scene: graph item %q: %w", x.ID, e)
			}
			local, spans = q.Hits, q.Spans
		} else {
			local, e = x.Graph.IntersectEnvelopeLine(ctx, a, d, 0, ray.Length, x.Operation, o)
			if e != nil {
				return empty, false, fmt.Errorf("scene: graph item %q: %w", x.ID, e)
			}
		}
		start := len(hits)
		for _, h := range local {
			hits = append(hits, SplineGraphPickResult{ID: x.ID, Parameter: h.Parameter, Base: h.Base, TopFaces: h.TopFaces, Sides: h.Sides})
		}
		for _, s := range spans {
			var existing bool
			for i := start; i < len(hits); i++ {
				q := &hits[i]
				if sameGraphPickBracket(q.Parameter, s[0]) {
					q.Coincident, existing = true, true
					break
				}
			}
			if !existing {
				hits = append(hits, SplineGraphPickResult{ID: x.ID, Parameter: s[0], Sides: graphPickSides(a), Coincident: true})
			}
		}
	}
	if len(hits) == 0 {
		return empty, false, nil
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if c := hits[i].Parameter.Min.Cmp(hits[j].Parameter.Min); c != 0 {
			return c < 0
		}
		return hits[i].ID < hits[j].ID
	})
	best := hits[0]
	for _, q := range hits[1:] {
		if q.Parameter.Min.Cmp(best.Parameter.Max) > 0 {
			break
		}
		if !sameGraphPickBracket(q.Parameter, best.Parameter) || best.Parameter.Min.Cmp(best.Parameter.Max) != 0 {
			return empty, false, geom.ErrConvergence
		}
		if q.ID == best.ID {
			best.Base, best.Coincident = best.Base || q.Base, best.Coincident || q.Coincident
			best.TopFaces = mergeWrappedPickFaces(best.TopFaces, q.TopFaces)
			best.Sides = mergeWrappedPickFaces(best.Sides, q.Sides)
		}
	}
	return best, true, nil
}

func sameGraphPickBracket(a, b geom.ToroidLineRoot) bool {
	return a.Min.Cmp(b.Min) == 0 && a.Max.Cmp(b.Max) == 0
}

func graphPickSides(a geom.Vec3) []int {
	var r []int
	for i, v := range [4]bool{a.Y == 0, a.X == 1, a.Y == 1, a.X == 0} {
		if v {
			r = append(r, i)
		}
	}
	return r
}
