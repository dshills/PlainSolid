package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// UnitGraphSelection selects where a unit-square graph pair contains material.
type UnitGraphSelection int

const (
	// UnitGraphFull requires strict ordering everywhere on the chart.
	UnitGraphFull UnitGraphSelection = iota
	// UnitGraphAbove selects the part where the first graph is above the second.
	UnitGraphAbove
	// UnitGraphBelow selects the part where the first graph is below the second.
	UnitGraphBelow
)

// UnitSplineGraphQuery classifies a body between two spline graphs
// using their exact identity XY chart. It can classify points on the vertical
// walls and corners without relying on numerical projection.
type UnitSplineGraphQuery struct {
	a, b, delta *geom.SurfacePatch
	order       int
	proof       geom.SurfacePointSignOptions
}

// NewUnitSplineGraphQuery proves the whole-chart or regular open-cut domain
// supported by FillSplineGraphPair and SealOpenSplineGraphPair.
func NewUnitSplineGraphQuery(ctx context.Context, a, b *geom.BSplineSurface, selection UnitGraphSelection, o intersect.SplinePlaneOptions) (*UnitSplineGraphQuery, error) {
	if ctx == nil || a == nil || b == nil || o.Domain != (geom.ParameterRectangle{}) || selection < UnitGraphFull || selection > UnitGraphBelow {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if e := requireIdentitySplineChart(ctx, a, o.Patch); e != nil {
		return nil, e
	}
	d, pa, pb, e := exactGraphPatchesContext(ctx, a, b, o.Patch, o.Domain)
	if e != nil {
		return nil, e
	}
	pl, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	trace, e := geom.TraceSurfacePlaneContext(ctx, d, pl, o.Trace)
	if e != nil {
		return nil, e
	}
	if !trace.Complete || len(trace.Isolation.Coplanar) != 0 || len(trace.Isolation.Tangencies) != 0 {
		return nil, geom.ErrConvergence
	}
	proof := geom.SurfacePointSignOptions{MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits}
	order := 0
	if selection == UnitGraphFull {
		if len(trace.Contours) != 0 {
			return nil, fmt.Errorf("trimmed: whole-chart query requires strictly ordered graphs")
		}
		h, e := geom.ClassifySurfacePointPlane(d, pl, .5, .5, proof)
		if e != nil {
			return nil, e
		}
		order = h.Sign
	} else {
		if len(trace.Contours) != 1 || trace.Contours[0].Closed {
			return nil, fmt.Errorf("trimmed: side query requires one regular open section")
		}
		c := trace.Contours[0]
		// Prove that the two endpoints meet distinct sides away from the
		// corners. Every chart-boundary zero then borders a retained wall.
		first, last := c.Arcs[0], c.Arcs[len(c.Arcs)-1]
		i, j := 0, 1
		if first.Reversed {
			i = 1
		}
		if last.Reversed {
			j = 0
		}
		a, b := trace.Isolation.Arcs[first.Index].Endpoints[i].Parameters, trace.Isolation.Arcs[last.Index].Endpoints[j].Parameters
		side := func(x geom.UVBox) int {
			switch {
			case x.Min.Y == 0 && x.Max.Y == 0 && x.Min.X > 0 && x.Max.X < 1:
				return 0
			case x.Min.X == 1 && x.Max.X == 1 && x.Min.Y > 0 && x.Max.Y < 1:
				return 1
			case x.Min.Y == 1 && x.Max.Y == 1 && x.Min.X > 0 && x.Max.X < 1:
				return 2
			case x.Min.X == 0 && x.Max.X == 0 && x.Min.Y > 0 && x.Max.Y < 1:
				return 3
			}
			return -1
		}
		if i, j := side(a), side(b); i < 0 || j < 0 || i == j {
			return nil, fmt.Errorf("trimmed: section endpoints need distinct open chart sides")
		}
		order = 1
		if selection == UnitGraphBelow {
			order = -1
		}
	}
	if order == 0 {
		return nil, geom.ErrConvergence
	}
	return &UnitSplineGraphQuery{pa, pb, d, order, proof}, nil
}

// Classify determines material location with exact signs of rational
// surface evaluations at the represented query coordinates.
func (q *UnitSplineGraphQuery) Classify(ctx context.Context, p geom.Vec3) (parametric.Location, error) {
	if ctx == nil || q == nil || q.a == nil || !p.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, e
	}
	if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
		return parametric.Outside, nil
	}
	zero, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	d, e := geom.ClassifySurfacePointPlane(q.delta, zero, p.X, p.Y, q.proof)
	if e != nil {
		return parametric.Outside, e
	}
	if d.Sign != 0 && d.Sign != q.order {
		return parametric.Outside, nil
	}
	z, _ := geom.NewPlane(geom.Vec3{Z: p.Z}, geom.Vec3{Z: 1})
	a, e := geom.ClassifySurfacePointPlane(q.a, z, p.X, p.Y, q.proof)
	if e != nil {
		return parametric.Outside, e
	}
	b, e := geom.ClassifySurfacePointPlane(q.b, z, p.X, p.Y, q.proof)
	if e != nil {
		return parametric.Outside, e
	}
	if a.Sign == b.Sign && a.Sign != 0 {
		return parametric.Outside, nil
	}
	if a.Sign == 0 || b.Sign == 0 || p.X == 0 || p.X == 1 || p.Y == 0 || p.Y == 1 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}
