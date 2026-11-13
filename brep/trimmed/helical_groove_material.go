package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HelicalGrooveRecipe is the bounded coaxial stock and helical cutter used by
// BuildHelicalGrooveMaterial. A zero BoreRadius selects external cutting;
// a positive bore radius selects an internal groove on a tube.
type HelicalGrooveRecipe struct {
	Frame                           geom.Frame
	OuterRadius, BoreRadius, Height float64
	Profile                         []geom.Vec2
	Lead, Start, End                float64
	Hand                            geom.HelixHandedness
	Starts                          int
}

// HelicalGrooveMaterial couples a validated original curved boundary with
// its immutable meridian proof for independently evaluating material probes.
type HelicalGrooveMaterial struct {
	solid  Solid
	recipe HelicalGrooveRecipe
	starts geom.SeparatedHelicalStarts
}

func BuildHelicalGrooveMaterial(ctx context.Context, r HelicalGrooveRecipe) (HelicalGrooveMaterial, error) {
	var s Solid
	var e error
	if r.BoreRadius == 0 {
		s, e = GrooveCylinderWithHelicalStarts(ctx, r.Frame, r.OuterRadius, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts)
	} else {
		s, e = GrooveTubeWithHelicalStarts(ctx, r.Frame, r.OuterRadius, r.BoreRadius, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts)
	}
	if e != nil {
		return HelicalGrooveMaterial{}, e
	}
	p, e := geom.NewHelicalPolygon(r.Frame, r.Profile, r.Lead, r.Hand, r.Start, r.End)
	if e != nil {
		return HelicalGrooveMaterial{}, e
	}
	starts, e := geom.NewSeparatedHelicalStarts(p, r.Starts)
	if e != nil {
		return HelicalGrooveMaterial{}, e
	}
	r.Profile = append([]geom.Vec2(nil), r.Profile...)
	return HelicalGrooveMaterial{s, r, starts}, nil
}

func (g HelicalGrooveMaterial) Solid() Solid { return g.solid }

// ClassifyAxialProbe classifies an exact point expressed by a positive radial
// distance, rational unit ray (cos,sin) in the original frame, and exact axial
// coordinate. For probes strictly inside stock, cutter membership uses all
// starts and both ends of the finite sweep. A query whose noncardinal turn
// enclosure meets an endpoint returns unresolved, with no tolerance guess.
// The returned line report preserves the responsible start and meridian.
func (g HelicalGrooveMaterial) ClassifyAxialProbe(ctx context.Context, radius, z *big.Rat, ray [2]*big.Rat,
	o geom.HelicalStartsLineOptions) (parametric.Location, geom.HelicalStartsLineResult, error) {
	var report geom.HelicalStartsLineResult
	if ctx == nil || len(g.solid.faces) == 0 || radius == nil || radius.Sign() <= 0 || z == nil || ray[0] == nil || ray[1] == nil ||
		o.MaxCoefficientBits != 0 && o.MaxCoefficientBits < 64 {
		return parametric.Outside, report, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, report, e
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	for _, q := range []*big.Rat{radius, z, ray[0], ray[1]} {
		if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
			return parametric.Outside, report, geom.ErrConvergence
		}
	}
	unit := new(big.Rat).Add(new(big.Rat).Mul(ray[0], ray[0]), new(big.Rat).Mul(ray[1], ray[1]))
	if unit.Cmp(big.NewRat(1, 1)) != 0 {
		return parametric.Outside, report, geom.ErrInvalid
	}
	r := g.recipe
	outer, inner := radius.Cmp(ratInset(r.OuterRadius)), radius.Cmp(ratInset(r.BoreRadius))
	upper := z.Cmp(ratInset(r.Height))
	if outer > 0 || inner < 0 || z.Sign() < 0 || upper > 0 {
		report.Complete = true
		return parametric.Outside, report, nil
	}
	if z.Sign() == 0 || upper == 0 {
		report.Complete = true
		return parametric.Boundary, report, nil
	}
	var line [3]*big.Rat
	x, y, c := r.Frame.XAxis(), r.Frame.YAxis(), r.Frame.Origin()
	for i, a := range [3]float64{c.X, c.Y, c.Z} {
		u := new(big.Rat).Mul(ray[0], ratInset([3]float64{x.X, x.Y, x.Z}[i]))
		v := new(big.Rat).Mul(ray[1], ratInset([3]float64{y.X, y.Y, y.Z}[i]))
		line[i] = new(big.Rat).Add(ratInset(a), new(big.Rat).Mul(radius, new(big.Rat).Add(u, v)))
	}
	report, e := g.starts.IntersectAxisLine(ctx, ray[0], ray[1], line, big.NewRat(1, 1), o)
	if e != nil || !report.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return parametric.Outside, report, e
	}
	// A radial extremum can be a whole cutter side, not merely an
	// isolated axial endpoint. Contact on the original stock wall inside
	// the opening is removed; an internal radial extremum is boundary.
	profileMin, profileMax := r.Profile[0].X, r.Profile[0].X
	for _, p := range r.Profile[1:] {
		profileMin, profileMax = min(profileMin, p.X), max(profileMax, p.X)
	}
	for _, interval := range report.Intervals {
		a, b := interval.Bounds[0], interval.Bounds[1]
		if z.Cmp(a[0]) < 0 || z.Cmp(b[1]) > 0 {
			continue
		}
		if a[0].Cmp(a[1]) == 0 && z.Cmp(a[0]) == 0 || b[0].Cmp(b[1]) == 0 && z.Cmp(b[0]) == 0 {
			return parametric.Boundary, report, nil
		}
		if z.Cmp(a[1]) <= 0 || z.Cmp(b[0]) >= 0 {
			report.Complete = false
			return parametric.Outside, report, geom.ErrConvergence
		}
		for _, source := range interval.Source.Meridians {
			u := source.Turn
			if u[0].Cmp(u[1]) == 0 && (u[0].Cmp(ratInset(r.Start)) == 0 || u[0].Cmp(ratInset(r.End)) == 0) {
				return parametric.Boundary, report, nil
			}
		}
		if outer == 0 || inner == 0 {
			return parametric.Outside, report, nil
		}
		if radius.Cmp(ratInset(profileMin)) == 0 || radius.Cmp(ratInset(profileMax)) == 0 {
			return parametric.Boundary, report, nil
		}
		return parametric.Outside, report, nil
	}
	if outer == 0 || inner == 0 {
		return parametric.Boundary, report, nil
	}
	return parametric.Inside, report, nil
}
