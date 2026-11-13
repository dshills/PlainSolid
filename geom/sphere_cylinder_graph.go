package geom

import (
	"context"
	"math/big"
	"strconv"
)

// SphereCylinderWallEvent identifies a model point without rounding its
// algebraic coordinates. ProjectionExtreme ±1 denotes ±sqrt(|Projection|²);
// otherwise ProjectionValue is exact rational. AngularHalf selects a side of
// the projected-center ray; at either extreme the halves coincide and Half=0.
// HeightSign=0 denotes the common zero-height fold/node, otherwise the sheet.
// CapMask bit0/1 identifies the original lower/upper raw-axis cap exactly.
type SphereCylinderWallEvent struct {
	// SurfaceTangent remains true at a clipped node even if only two germs survive.
	SurfaceTangent          bool
	ProjectionValue         *big.Rat
	ProjectionExtreme       int
	AngularHalf, HeightSign int
	CapMask                 uint8
	IncidentArcs            []int
}
type SphereCylinderWallArc struct {
	Branch, AngularHalf int
	Events              [2]int
}

// Singular denotes graph branching (degree greater than two); differential
// surface tangency is recorded separately on each event.
type SphereCylinderWallComponent struct {
	Events, Arcs                         []int
	CycleRank                            int
	IsolatedPoint, HasBoundary, Singular bool
}
type SphereCylinderWallGraph struct {
	Complete        bool
	Domain          FiniteSphereCylinderIntersection
	Events          []SphereCylinderWallEvent
	Arcs            []SphereCylinderWallArc
	Components      []SphereCylinderWallComponent
	ExactOperations int
}

// BuildFiniteSphereCylinderWallGraph certifies complete incidences of finite
// off-axis sphere/cylinder wall intersections, including isolated points,
// four-germ tangent-surface nodes, and zero-width sheet duplicates on a cap.
// Each edge is a strictly monotone projection interval on one angular half
// and one height sheet. Endpoint identities use exact projection equations,
// half-plane side and height sheet; only exact folds/extrema are identified.
// Ordered Events on an arc follow increasing projection, not global traversal.
func BuildFiniteSphereCylinderWallGraph(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, window [2]*big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderWallGraph, err error) {
	d, e := IntersectFiniteOffAxisSphereCylinder(ctx, center, sphereRadius, origin, axis, radius, window, o)
	out.Domain = d
	out.ExactOperations = d.ExactOperations
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: d.ExactOperations}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Events = nil
			out.Arcs = nil
			out.Components = nil
		}
	}()
	H, fold := d.Angular.ProjectionSquared, d.Angular.Threshold
	cmp := func(q *big.Rat, sign int) int {
		if sign > 0 {
			if q.Sign() < 0 {
				return -1
			}
			return new(big.Rat).Mul(q, q).Cmp(H)
		}
		if q.Sign() > 0 {
			return 1
		}
		return -new(big.Rat).Mul(q, q).Cmp(H)
	}
	type level struct {
		q   *big.Rat
		ext int
	}
	key := func(v level) string {
		if v.ext != 0 {
			return "e" + strconv.Itoa(v.ext)
		}
		return "q" + v.q.RatString()
	}
	equalRat := func(v level, q *big.Rat) bool {
		if v.ext != 0 {
			return cmp(q, v.ext) == 0
		}
		return v.q.Cmp(q) == 0
	}
	capLevels := [2]*big.Rat{}
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for i := 0; i < 2; i++ {
		height := new(big.Rat).Add(new(big.Rat).Sub(r(coordinate(origin, ax)), r(coordinate(center, ax))), new(big.Rat).Mul(r(coordinate(axis, ax)), window[i]))
		capLevels[i] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(height, height), d.Angular.A), new(big.Rat).Mul(big.NewRat(2, 1), r(radius)))
		if e = b.check(capLevels[i]); e != nil {
			return out, e
		}
	}
	ids := map[string]int{}
	node := func(v level, half, sign int) (int, error) {
		if v.ext != 0 {
			half = 0
		}
		if equalRat(v, fold) {
			sign = 0
		}
		idkey := key(v) + ":" + strconv.Itoa(half) + ":" + strconv.Itoa(sign)
		if id, ok := ids[idkey]; ok {
			return id, nil
		}
		ev := SphereCylinderWallEvent{SurfaceTangent: sign == 0 && v.ext != 0, ProjectionExtreme: v.ext, AngularHalf: half, HeightSign: sign}
		if v.q != nil {
			ev.ProjectionValue = new(big.Rat).Set(v.q)
			if e := b.check(ev.ProjectionValue); e != nil {
				return 0, e
			}
		}
		for i := 0; i < 2; i++ {
			if equalRat(v, capLevels[i]) && (sign == d.CapHeightSigns[i] || sign == 0 && d.CapHeightSigns[i] == 0) {
				ev.CapMask |= 1 << i
			}
		}
		id := len(out.Events)
		ids[idkey] = id
		out.Events = append(out.Events, ev)
		return id, nil
	}
	for i, branch := range d.Branches {
		if !branch.Present {
			continue
		}
		lo, hi := level{q: branch.ProjectionMin}, level{q: branch.ProjectionMax}
		if cmp(lo.q, -1) <= 0 {
			lo = level{ext: -1}
		} else if cmp(lo.q, 1) == 0 {
			lo = level{ext: 1}
		}
		if cmp(hi.q, 1) >= 0 {
			hi = level{ext: 1}
		} else if cmp(hi.q, -1) == 0 {
			hi = level{ext: -1}
		}
		for half := 0; half < 2; half++ {
			u, e := node(lo, half, branch.HeightSign)
			if e != nil {
				return out, e
			}
			v, e := node(hi, half, branch.HeightSign)
			if e != nil {
				return out, e
			}
			if key(lo) == key(hi) {
				continue
			}
			id := len(out.Arcs)
			out.Arcs = append(out.Arcs, SphereCylinderWallArc{i, half, [2]int{u, v}})
			out.Events[u].IncidentArcs = append(out.Events[u].IncidentArcs, id)
			out.Events[v].IncidentArcs = append(out.Events[v].IncidentArcs, id)
		}
	}
	seen := make([]bool, len(out.Events))
	for first := range out.Events {
		if seen[first] {
			continue
		}
		comp := SphereCylinderWallComponent{}
		queue := []int{first}
		seen[first] = true
		edges := map[int]bool{}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			comp.Events = append(comp.Events, v)
			degree := len(out.Events[v].IncidentArcs)
			if degree == 1 {
				comp.HasBoundary = true
			}
			if degree > 2 {
				comp.Singular = true
			}
			for _, edge := range out.Events[v].IncidentArcs {
				if !edges[edge] {
					edges[edge] = true
					comp.Arcs = append(comp.Arcs, edge)
				}
				ends := out.Arcs[edge].Events
				for _, w := range ends {
					if !seen[w] {
						seen[w] = true
						queue = append(queue, w)
					}
				}
			}
		}
		comp.CycleRank = len(comp.Arcs) - len(comp.Events) + 1
		comp.IsolatedPoint = len(comp.Arcs) == 0
		out.Components = append(out.Components, comp)
	}
	out.Complete = true
	return out, nil
}
