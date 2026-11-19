//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRationalProfileStation places a homothetic copy of the authored XY
// profile at Height. Scale is relative to its original square control-polygon
// gauge, not a circular radius. Scaling is about the profile's own center.
type WrappedRationalProfileStation struct{ Height, Scale *big.Rat }

// LoftWrappedRationalProfileStations authors a native loft directly from a
// nine-pole positive-weight quadratic XY profile and2..17 ordered stations.
// The exact control polygon must follow the canonical four square quadrants,
// starting at +X, counterclockwise, with Z=0. Arbitrary strictly ordered original
// quadrant knot intervals and the stored profile weights are retained verbatim.
// Each band's V parameter runs0..1; station slopes may form a reentrant waist.
// No circle or rounded profile is substituted. Exact station coordinates must
// fit native binary64 poles/vertices; nonrepresentable products fail atomically.
// Nonbinary input scales can succeed when their products are representable.
// One cumulative coordinate ledger uses o.MaxOperations/default1M (maximum8M)
// and o.MaxCoefficientBits/default16384 (maximum65536). Each at-most16 band
// construction and the existing full native sewing proof keep separate source
// limits. Existing circular and already-authored band APIs are unchanged.
func LoftWrappedRationalProfileStations(ctx context.Context, profile *geom.BSpline, stations []WrappedRationalProfileStation, tolerance float64, o geom.SurfacePatchOptions) (Solid, WrappedLoftHistory, error) {
	fail := func(e error) (Solid, WrappedLoftHistory, error) { return Solid{}, WrappedLoftHistory{}, e }
	if ctx == nil || profile == nil || profile.Degree() != 2 || len(stations) < 2 || len(stations) > 17 || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	poles, weights, knots := profile.Poles(), profile.Weights(), profile.Knots()
	if len(poles) != 9 || len(weights) != 9 || len(knots) != 12 {
		return fail(geom.ErrDomain)
	}
	operations, bits := o.MaxOperations, o.MaxCoefficientBits
	if operations == 0 {
		operations = 1000000
	}
	if bits == 0 {
		bits = 16384
	}
	if operations < 1 || operations > 8000000 || bits < 64 || bits > 65536 || o.MaxPoles != 0 && o.MaxPoles < 4 {
		return fail(geom.ErrInvalid)
	}
	b := stationMassBudget{ctx: ctx, limit: operations, bits: bits}
	center := [2]*big.Rat{b.div(b.add(ratInset(poles[0].X), ratInset(poles[4].X)), big.NewRat(2, 1)), ratInset(poles[0].Y)}
	radius := b.sub(ratInset(poles[0].X), center[0])
	if b.err != nil {
		return fail(b.err)
	}
	if radius.Sign() <= 0 {
		return fail(geom.ErrDomain)
	}
	coordinates := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	for i, p := range poles {
		if p.Z != 0 || !finite(weights[i]) || weights[i] <= 0 {
			return fail(geom.ErrDomain)
		}
		for axis, value := range []float64{p.X, p.Y} {
			want := b.add(center[axis], b.mul(radius, big.NewRat(coordinates[i][axis], 1)))
			if b.err != nil {
				return fail(b.err)
			}
			if want.Cmp(ratInset(value)) != 0 {
				return fail(geom.ErrDomain)
			}
		}
	}
	cuts := [5]float64{knots[2], knots[3], knots[5], knots[7], knots[9]}
	expected := []float64{cuts[0], cuts[0], cuts[0], cuts[1], cuts[1], cuts[2], cuts[2], cuts[3], cuts[3], cuts[4], cuts[4], cuts[4]}
	for i, k := range knots {
		if k != expected[i] {
			return fail(geom.ErrDomain)
		}
	}
	for i := 1; i < len(cuts); i++ {
		if cuts[i] <= cuts[i-1] {
			return fail(geom.ErrDomain)
		}
	}
	// Snapshot the exact station values before constructing any native output.
	saved := make([]WrappedRationalProfileStation, len(stations))
	for i, s := range stations {
		if s.Height == nil || s.Scale == nil {
			return fail(geom.ErrInvalid)
		}
		b.check(s.Height)
		b.check(s.Scale)
		if b.err != nil {
			return fail(b.err)
		}
		if s.Scale.Sign() <= 0 || i > 0 && s.Height.Cmp(saved[i-1].Height) <= 0 {
			return fail(geom.ErrDomain)
		}
		saved[i] = WrappedRationalProfileStation{new(big.Rat).Set(s.Height), new(big.Rat).Set(s.Scale)}
	}
	exactFloat := func(q *big.Rat) (float64, error) {
		b.check(q)
		if b.err != nil {
			return 0, b.err
		}
		f, exact := q.Float64()
		if !exact || !finite(f) {
			return 0, geom.ErrUnrepresentable
		}
		return f, nil
	}
	bands := make([]*geom.WrappedSurfacePatch, 0, len(saved)-1)
	for band := 0; band < len(saved)-1; band++ {
		var net []geom.Vec3
		var ws []float64
		for i, p := range poles {
			for _, station := range saved[band : band+2] {
				x := b.add(center[0], b.mul(station.Scale, b.sub(ratInset(p.X), center[0])))
				y := b.add(center[1], b.mul(station.Scale, b.sub(ratInset(p.Y), center[1])))
				xf, e := exactFloat(x)
				if e != nil {
					return fail(e)
				}
				yf, e := exactFloat(y)
				if e != nil {
					return fail(e)
				}
				zf, e := exactFloat(station.Height)
				if e != nil {
					return fail(e)
				}
				net = append(net, geom.Vec3{X: xf, Y: yf, Z: zf})
				ws = append(ws, weights[i])
			}
		}
		surface, e := geom.NewBSplineSurface(2, 1, 9, 2, net, knots, []float64{0, 0, 1, 1}, ws)
		if e != nil {
			return fail(e)
		}
		// BSpline weights already have maximum1. Enforce that the new tensor
		// constructor has not changed even one represented source coefficient.
		data := surface.Data()
		for i, w := range data.Weights {
			if w != ws[i] {
				return fail(geom.ErrUnrepresentable)
			}
		}
		patch, e := geom.NewSurfacePatch(surface, o)
		if e != nil {
			return fail(e)
		}
		wrapped, _, e := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true, MaxPoles: o.MaxPoles, MaxOperations: operations, MaxCoefficientBits: bits})
		if e != nil {
			return fail(e)
		}
		bands = append(bands, wrapped)
	}
	if b.err != nil {
		return fail(b.err)
	}
	return NewWrappedRationalStationLoft(ctx, bands, tolerance, o)
}
