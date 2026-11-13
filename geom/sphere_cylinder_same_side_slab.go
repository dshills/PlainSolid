package geom

import (
	"context"
	"math/big"
)

type SphereCylinderWallWindowKind uint8

const (
	SphereCylinderWallAbsent SphereCylinderWallWindowKind = iota
	SphereCylinderWallCapToCap
	SphereCylinderWallSphereLimited
)

// SphereCylinderWallWindow uses the exact angular interval in Arrangement.
// CapToCap retains the full axial window. SphereLimited runs from NearCap to
// the sphere height on Side; Absent has no positive-length retained wall.
type SphereCylinderWallWindow struct {
	Sector int
	Kind   SphereCylinderWallWindowKind
}

// SphereCylinderSlabStation retains simultaneous cap/sphere/cylinder event
// ownership and the two adjacent open wall windows. At a NearCap event the
// wall collapses to a point; at a FarCap event its upper/lower bound switches.
type SphereCylinderSlabStation struct {
	Station, BeforeWindow, AfterWindow int
	NearCapEvent, FarCapEvent          bool
}
type SphereCylinderSameSideSlab struct {
	Complete        bool
	NearCap, FarCap int
	// Side is the sign of axial displacement from sphere center along the
	// signed raw axis. It distinguishes the upper/lower physical side.
	Side            int
	Arrangement     SphereCylinderDoubleCapEvents
	SphereWall      SphereCylinderWallGraph
	Windows         []SphereCylinderWallWindow
	Stations        []SphereCylinderSlabStation
	ExactOperations int
}

// ClassifySameSideSphereCylinderSlab proves the complete boundary incidence
// pattern for the intersection of sphere material and a finite cylinder when
// both caps are strict transverse lenses on the SAME open side of equator.
// It does not construct a B-rep. Circle cap arcs remain in Arrangement.Caps;
// sphere-wall arcs and exact model endpoints remain in SphereWall. Open
// angular windows plus station incidences cover all angular directions.
// Equatorial, opposite-side or non-lens cap inputs return ErrDomain.
func ClassifySameSideSphereCylinderSlab(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, caps [2]*big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderSameSideSlab, err error) {
	if ctx == nil || caps[0] == nil || caps[1] == nil || caps[0].Cmp(caps[1]) >= 0 {
		return out, ErrInvalid
	}
	ops := o.MaxOperations
	if ops == 0 {
		ops = 10000
	}
	if ops < 1 {
		return out, ErrInvalid
	}
	defer func() {
		if err != nil {
			out.Complete = false
			out.Windows = nil
			out.Stations = nil
		}
	}()
	a, e := OrderSphereCylinderDoubleCapEvents(ctx, center, sphereRadius, origin, axis, radius, caps, o)
	out.ExactOperations = a.ExactOperations
	if e != nil {
		return out, e
	}
	out.Arrangement = a
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sign := int64(1)
	if coordinate(axis, ax) < 0 {
		sign = -1
	}
	heights := [2]*big.Rat{}
	for i := 0; i < 2; i++ {
		heights[i] = new(big.Rat).Mul(big.NewRat(sign, 1), new(big.Rat).Add(new(big.Rat).Sub(r(coordinate(origin, ax)), r(coordinate(center, ax))), new(big.Rat).Mul(r(coordinate(axis, ax)), caps[i])))
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: out.ExactOperations}
	for _, h := range heights {
		if e := budget.check(h); e != nil {
			out.ExactOperations = budget.operations
			return out, e
		}
	}
	out.ExactOperations = budget.operations
	if heights[0].Sign() == 0 || heights[1].Sign() == 0 || heights[0].Sign() != heights[1].Sign() {
		return out, ErrDomain
	}
	out.Side = heights[0].Sign()
	out.NearCap, out.FarCap = 0, 1
	if out.Side < 0 {
		out.NearCap, out.FarCap = 1, 0
	}
	if out.ExactOperations >= ops {
		return out, ErrConvergence
	}
	oo := o
	oo.MaxOperations = ops - out.ExactOperations
	graph, e := BuildFiniteSphereCylinderWallGraph(ctx, center, sphereRadius, origin, axis, radius, caps, oo)
	out.ExactOperations += graph.ExactOperations
	if e != nil {
		return out, e
	}
	out.SphereWall = graph
	for i, sector := range a.Sectors {
		near, far := sector.InsideCap[out.NearCap], sector.InsideCap[out.FarCap]
		if far && !near {
			return out, ErrConvergence
		}
		kind := SphereCylinderWallAbsent
		if far {
			kind = SphereCylinderWallCapToCap
		} else if near {
			kind = SphereCylinderWallSphereLimited
		}
		out.Windows = append(out.Windows, SphereCylinderWallWindow{i, kind})
	}
	for i, station := range a.Stations {
		q := SphereCylinderSlabStation{Station: i, BeforeWindow: (i + len(a.Stations) - 1) % len(a.Stations), AfterWindow: i}
		for _, owner := range station.Owners {
			if owner.Cap == out.NearCap {
				q.NearCapEvent = true
			}
			if owner.Cap == out.FarCap {
				q.FarCapEvent = true
			}
		}
		// Strict same-side distinct heights imply distinct thresholds. Each near
		// event separates absent and sphere-limited wall; each far event separates
		// full and sphere-limited wall. Prove these expected incident cell labels.
		before, after := out.Windows[q.BeforeWindow].Kind, out.Windows[q.AfterWindow].Kind
		matches := func(a, b SphereCylinderWallWindowKind) bool {
			return before == a && after == b || before == b && after == a
		}
		if q.NearCapEvent == q.FarCapEvent {
			return out, ErrConvergence
		}
		if q.NearCapEvent && !matches(SphereCylinderWallAbsent, SphereCylinderWallSphereLimited) {
			return out, ErrConvergence
		}
		if q.FarCapEvent && !matches(SphereCylinderWallCapToCap, SphereCylinderWallSphereLimited) {
			return out, ErrConvergence
		}
		out.Stations = append(out.Stations, q)
	}
	out.Complete = true
	return out, nil
}
