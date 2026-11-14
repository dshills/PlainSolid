package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

type FilletedFrustumRimParameterOptions struct {
	Support FilletedFrustumDistanceOptions
	// Aggregate across original curve extraction, restriction and inversion.
	// Zero defaults:2M operations,16384 coefficient bits,8192 subdivisions.
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
}

// FilletedFrustumRimParameter is one exact ORIGINAL model-edge parameter.
// Vertex=-1 means strict edge interior; at a quarter vertex both adjacent
// edges are returned independently, including the periodic0/4 endpoints.
type FilletedFrustumRimParameter struct {
	Edge, Vertex int
	Parameter    *big.Rat
}

type FilletedFrustumNearestRimParameters struct {
	Baseline                      FilletedFrustumDistance
	Located                       bool
	Faces                         []int
	Contacts                      []FilletedFrustumRimParameter
	ExactOperations, Subdivisions int
}

// LocateNearestFilletRimRational localizes an already-proved exact rational
// nearest point on an original smooth cap/blend or cone/blend rim. Exact
// homogeneous coordinate GCD inversion is performed on every original edge
// range. No azimuth-derived or rounded parameter is substituted. Algebraic
// unrecovered, multiple-per-edge or constant preimages fail atomically.
// An unsupported/nonexact nearest witness retains only the baseline.
func (q FilletedFrustumMaterial) LocateNearestFilletRimRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumRimParameterOptions) (out FilletedFrustumNearestRimParameters, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumNearestRimParameters{}
		}
	}()
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return out, geom.ErrInvalid
	}
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o.Support)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	if !base.Exact || len(base.WitnessFaces) != 2 {
		return out, nil
	}
	var edges []int
	for end := 0; end < 2; end++ {
		for role, other := range []int{q.fillets.Caps[end], q.fillets.Lateral} {
			a, b := base.WitnessFaces[0], base.WitnessFaces[1]
			blend := q.fillets.Blends[end]
			if (a == blend && b == other) || (b == blend && a == other) {
				edges = append(edges, q.fillets.Rims[end][role][:]...)
			}
		}
	}
	if len(edges) != 4 {
		return out, nil
	}
	sort.Ints(edges)
	remaining, subs := o.MaxOperations, o.MaxSubdivisions
	charge := func(n, s int) error {
		if n < 0 || s < 0 || n > remaining || s > subs {
			return geom.ErrConvergence
		}
		remaining -= n
		subs -= s
		out.ExactOperations += n
		out.Subdivisions += s
		return nil
	}
	for _, id := range edges {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if remaining < 1 || subs < 1 {
			return out, geom.ErrConvergence
		}
		edge := q.solid.edges[id]
		curve, ok := edge.Curve.(*geom.BSpline)
		if !ok {
			return out, geom.ErrConvergence
		}
		exact, e := geom.NewExactSplineCurve(curve, geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return out, e
		}
		if e = charge(exact.ExactOperations(), 0); e != nil {
			return out, e
		}
		if remaining < 1 {
			return out, geom.ErrConvergence
		}
		exact, e = exact.Restrict(edge.Range.Min, edge.Range.Max, geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return out, e
		}
		if e = charge(exact.ExactOperations(), 0); e != nil {
			return out, e
		}
		if remaining < 1 {
			return out, geom.ErrConvergence
		}
		found, e := geom.FindExactCurvePointParameters(ctx, exact, base.Witness, geom.ExactCurveSegmentOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: subs})
		if e != nil {
			return out, e
		}
		if e = charge(found.Operations, found.Subdivisions); e != nil {
			return out, e
		}
		if !found.Complete || len(found.ConstantSpans) != 0 || len(found.Roots) > 1 {
			return out, geom.ErrConvergence
		}
		if len(found.Roots) == 0 {
			continue
		}
		root := found.Roots[0]
		if root.Parameter == nil {
			return out, geom.ErrConvergence
		}
		t := root.Parameter
		lo, hi := new(big.Rat).SetFloat64(edge.Range.Min), new(big.Rat).SetFloat64(edge.Range.Max)
		if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 {
			return out, geom.ErrConvergence
		}
		vertex := -1
		if t.Cmp(lo) == 0 {
			vertex = edge.Start
		}
		if t.Cmp(hi) == 0 {
			vertex = edge.End
		}
		out.Contacts = append(out.Contacts, FilletedFrustumRimParameter{Edge: id, Vertex: vertex, Parameter: new(big.Rat).Set(t)})
	}
	if len(out.Contacts) < 1 || len(out.Contacts) > 2 {
		return out, geom.ErrConvergence
	}
	if len(out.Contacts) == 2 && (out.Contacts[0].Vertex < 0 || out.Contacts[0].Vertex != out.Contacts[1].Vertex) {
		return out, geom.ErrConvergence
	}
	out.Located = true
	out.Faces = append([]int(nil), base.WitnessFaces...)
	return out, nil
}

type FilletedFrustumOccurrenceRimParameters struct {
	Original     FilletedFrustumNearestRimParameters
	WorldWitness [3]*big.Rat
}

// LocateNearestFilletRimRational preserves exact original edge parameters and
// shared vertex identities under proper rigid placement; only the model point
// changes frame. Pose arithmetic is bounded to4096 bits and errors are atomic.
func (q FilletedFrustumOccurrence) LocateNearestFilletRimRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumRimParameterOptions) (out FilletedFrustumOccurrenceRimParameters, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceRimParameters{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.LocateNearestFilletRimRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	if original.Located {
		out.WorldWitness, e = q.pose.TransformPoint(ctx, original.Baseline.Witness, false, 4096)
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
