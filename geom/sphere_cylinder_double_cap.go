package geom

import (
	"context"
	"math/big"
	"sort"
)

// SphereCylinderCapEventOwner binds an angular station to an exact model
// point in Caps[Cap].Events.Points[Point]. Different caps may share a radial
// direction while their model points remain axially distinct.
type SphereCylinderCapEventOwner struct{ Cap, Point int }

// SphereCylinderAngularStation identifies a unit-cylinder radial ray by
// Projection·ray=ProjectionValue and side RootSign of axis×Projection. This
// exact pair uniquely selects a ray, independent of the two cap chord hubs.
type SphereCylinderAngularStation struct {
	ProjectionValue *big.Rat
	RootSign        int
	Owners          []SphereCylinderCapEventOwner
}

// SphereCylinderAngularSector runs from station Start to End positively around
// signed axis. InsideCap describes the OPEN angular sector on the cylinder
// circle, excluding its endpoint event ownership. End may wrap to zero.
type SphereCylinderAngularSector struct {
	Start, End int
	InsideCap  [2]bool
}
type SphereCylinderDoubleCapEvents struct {
	Complete        bool
	Caps            [2]SphereCylinderCapMaterial
	Stations        []SphereCylinderAngularStation
	Sectors         []SphereCylinderAngularSector
	CapPointStation [2][2]int
	ExactOperations int
}

// OrderSphereCylinderDoubleCapEvents accepts two distinct raw-axis cap planes
// whose material boundaries are strict transverse lenses. It proves complete
// cyclic event ordering on the COMMON cylinder circle, starting after the
// projected-center ray. Equal absolute cap heights yield shared angular
// stations, never welded model endpoints. Different chord hubs require no
// affine identification. This is angular arrangement evidence, not a solid
// shell or a claim that cap-plane arcs have identical model curves.
func OrderSphereCylinderDoubleCapEvents(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, caps [2]*big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderDoubleCapEvents, err error) {
	if ctx == nil || caps[0] == nil || caps[1] == nil || caps[0].Cmp(caps[1]) >= 0 {
		return out, ErrInvalid
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Stations = nil
			out.Sectors = nil
		}
	}()
	thresholds := [2]*big.Rat{}
	for i := 0; i < 2; i++ {
		if b.operations >= ops {
			return out, ErrConvergence
		}
		oo := o
		oo.MaxOperations = ops - b.operations
		q, e := ClassifySphereCylinderCapMaterial(ctx, center, sphereRadius, origin, axis, radius, caps[i], oo)
		b.operations += q.ExactOperations
		if e != nil {
			return out, e
		}
		out.Caps[i] = q
		if q.Kind != SphereCylinderCapLens || len(q.Events.Points) != 2 {
			return out, ErrDomain
		}
		thresholds[i] = new(big.Rat).Quo(q.RadicalAxisValue, new(big.Rat).SetFloat64(radius))
		if e = b.check(thresholds[i]); e != nil {
			return out, e
		}
	}
	// Upper angular half has decreasing projection; lower half increasing.
	for _, sign := range []int{1, -1} {
		order := []int{0, 1}
		sort.Slice(order, func(i, j int) bool {
			c := thresholds[order[i]].Cmp(thresholds[order[j]])
			if sign > 0 {
				return c > 0
			}
			return c < 0
		})
		for _, cap := range order {
			point := 0
			if sign > 0 {
				point = 1
			}
			last := len(out.Stations) - 1
			if last >= 0 && out.Stations[last].RootSign == sign && out.Stations[last].ProjectionValue.Cmp(thresholds[cap]) == 0 {
				out.Stations[last].Owners = append(out.Stations[last].Owners, SphereCylinderCapEventOwner{cap, point})
				out.CapPointStation[cap][point] = last
			} else {
				out.CapPointStation[cap][point] = len(out.Stations)
				out.Stations = append(out.Stations, SphereCylinderAngularStation{new(big.Rat).Set(thresholds[cap]), sign, []SphereCylinderCapEventOwner{{cap, point}}})
			}
		}
	}
	for i, a := range out.Stations {
		j := (i + 1) % len(out.Stations)
		c := out.Stations[j]
		sector := SphereCylinderAngularSector{Start: i, End: j}
		if a.RootSign == c.RootSign {
			mid := new(big.Rat).Mul(new(big.Rat).Add(a.ProjectionValue, c.ProjectionValue), big.NewRat(1, 2))
			if e := b.check(mid); e != nil {
				return out, e
			}
			for cap := 0; cap < 2; cap++ {
				sector.InsideCap[cap] = mid.Cmp(thresholds[cap]) > 0
			}
		} else if a.RootSign < 0 {
			sector.InsideCap = [2]bool{true, true}
		}
		// Positive-to-negative crossing passes through minimum projection and is
		// outside both strict-lens caps. Negative-to-positive passes maximum.
		out.Sectors = append(out.Sectors, sector)
	}
	out.Complete = true
	return out, nil
}
