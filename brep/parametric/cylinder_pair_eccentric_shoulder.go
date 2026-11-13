package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// finiteParallelEccentricShoulderUnion retains a circular boss that extends
// from one end of a wider cylinder. Exact disk containment and interval
// ordering prove that the overlapping boss wall is wholly internal. The
// exposed shoulder uses an annulus on the wider original cap plane.
func finiteParallelEccentricShoulderUnion(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairUnionResult, bool, error) {
	var zero FiniteCylinderPairUnionResult
	if a.Radius == b.Radius {
		return zero, false, nil
	}
	wide, narrow := a, b
	outer := 0
	if a.Radius < b.Radius {
		wide, narrow, outer = b, a, 1
	}
	axis, speed := coordinateCylinderAxis(wide.Axis)
	_, innerSpeed := coordinateCylinderAxis(narrow.Axis)
	if speed == 0 || innerSpeed == 0 {
		return zero, false, nil
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	span := func(c geom.RawCylinder) (lo, hi *big.Rat, forward bool) {
		base, velocity := q(c.Origin.Dot(axis)), q(c.Axis.Dot(axis))
		lo = new(big.Rat).Add(base, new(big.Rat).Mul(velocity, q(c.Min)))
		hi = new(big.Rat).Add(base, new(big.Rat).Mul(velocity, q(c.Max)))
		if lo.Cmp(hi) > 0 {
			return hi, lo, false
		}
		return lo, hi, true
	}
	w0, w1, _ := span(wide)
	n0, n1, forward := span(narrow)
	lower := n0.Cmp(w0) < 0 && n1.Cmp(w0) >= 0 && n1.Cmp(w1) <= 0
	upper := n1.Cmp(w1) > 0 && n0.Cmp(w1) <= 0 && n0.Cmp(w0) >= 0
	if !lower && !upper {
		return zero, false, nil
	}
	var d2 big.Rat
	for i, v := range [3]float64{axis.X, axis.Y, axis.Z} {
		if v == 0 {
			p := new(big.Rat).Sub(q([3]float64{wide.Origin.X, wide.Origin.Y, wide.Origin.Z}[i]), q([3]float64{narrow.Origin.X, narrow.Origin.Y, narrow.Origin.Z}[i]))
			d2.Add(&d2, p.Mul(p, p))
		}
	}
	clearance := new(big.Rat).Sub(q(wide.Radius), q(narrow.Radius))
	if d2.Sign() == 0 || d2.Cmp(new(big.Rat).Mul(clearance, clearance)) >= 0 {
		return zero, false, nil
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	clearance.Sub(clearance, pad)
	if clearance.Sign() <= 0 || d2.Cmp(new(big.Rat).Mul(clearance, clearance)) >= 0 || q(narrow.Radius).Cmp(pad) <= 0 {
		return zero, true, geom.ErrDomain
	}
	low, high := w1, n1
	shoulder, wideEnd, narrowEnd := w1, w0, n1
	if lower {
		low, high = n0, w0
		shoulder, wideEnd, narrowEnd = w0, w1, n0
	}
	if new(big.Rat).Sub(high, low).Cmp(pad) <= 0 || new(big.Rat).Sub(w1, w0).Cmp(pad) <= 0 {
		return zero, true, geom.ErrDomain
	}
	// Every used axial value and every difference from the shared frame origin
	// must be representable; no source cap moves to fit an approximate chart.
	base := w0
	if lower {
		base = n0
	}
	at := func(origin geom.Vec3, z *big.Rat) (geom.Vec3, error) {
		for i, v := range [3]float64{axis.X, axis.Y, axis.Z} {
			if v != 0 {
				x, err := geom.RepresentedExactScalar(new(big.Rat).Mul(z, q(v)))
				if err != nil {
					return geom.Vec3{}, err
				}
				*[3]*float64{&origin.X, &origin.Y, &origin.Z}[i] = x
			}
		}
		return origin, nil
	}
	local := func(z *big.Rat) (float64, error) { return geom.RepresentedExactScalar(new(big.Rat).Sub(z, base)) }
	ws, err := at(wide.Origin, base)
	if err != nil {
		return zero, true, err
	}
	ns, err := at(narrow.Origin, base)
	if err != nil {
		return zero, true, err
	}
	f, err := geom.FrameFromNormal(ws, axis)
	if err != nil {
		return zero, true, err
	}
	g, err := geom.NewFrame(ns, f.XAxis(), axis)
	if err != nil {
		return zero, true, err
	}
	outerSurface, err := geom.NewCylinderSurface(f, wide.Radius)
	if err != nil {
		return zero, true, err
	}
	innerSurface, err := geom.NewCylinderSurface(g, narrow.Radius)
	if err != nil {
		return zero, true, err
	}
	var z [4]float64
	for i, p := range []*big.Rat{w0, w1, low, high} {
		if _, err = at(wide.Origin, p); err != nil {
			return zero, true, err
		}
		z[i], err = local(p)
		if err != nil {
			return zero, true, err
		}
	}
	wideCapOrigin, err := at(wide.Origin, wideEnd)
	if err != nil {
		return zero, true, err
	}
	narrowCapOrigin, err := at(narrow.Origin, narrowEnd)
	if err != nil {
		return zero, true, err
	}
	wideCap, err := geom.NewFrame(wideCapOrigin, f.XAxis(), axis)
	if err != nil {
		return zero, true, err
	}
	narrowCap, err := geom.NewFrame(narrowCapOrigin, f.XAxis(), axis)
	if err != nil {
		return zero, true, err
	}
	shoulderZ, err := local(shoulder)
	if err != nil {
		return zero, true, err
	}
	var out FiniteCylinderPairUnionResult
	var faces []Face
	addCap := func(owner int, rawMax bool, face int) {
		if rawMax {
			out.UpperCapFaces[owner] = append(out.UpperCapFaces[owner], face)
		} else {
			out.LowerCapFaces[owner] = append(out.LowerCapFaces[owner], face)
		}
	}
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		id := len(faces)
		faces = append(faces,
			Face{Surface: reparameterizedCylinder{outerSurface, u}, U: normalized, V: Interval{z[0], z[1]}},
			Face{Surface: reparameterizedCylinder{innerSurface, u}, U: normalized, V: Interval{z[2], z[3]}},
			Face{Surface: ruledArcs{outerSurface, innerSurface, u, u, shoulderZ}, U: normalized, V: normalized, Reversed: lower},
			Face{Surface: disk{wideCap}, U: u, V: Interval{0, wide.Radius}, Reversed: lower},
			Face{Surface: disk{narrowCap}, U: u, V: Interval{0, narrow.Radius}, Reversed: upper})
		out.WallFaces[outer] = append(out.WallFaces[outer], id)
		out.WallFaces[1-outer] = append(out.WallFaces[1-outer], id+1)
		addCap(outer, upper, id+2)
		addCap(outer, lower, id+3)
		addCap(1-outer, upper == forward, id+4)
	}
	out.Solid, err = joinPrimitivePatches(faces, tolerance)
	if err != nil {
		return zero, true, err
	}
	common := new(big.Rat)
	if lower {
		common.Sub(n1, w0)
	} else {
		common.Sub(w1, n0)
	}
	if common.Sign() == 0 {
		out.CommonVolumeSquared = common
	} else {
		r := q(narrow.Radius)
		out.CommonVolumePiCoefficient = common.Mul(common, r.Mul(r, r))
	}
	if err := ctx.Err(); err != nil {
		return zero, true, err
	}
	return out, true, nil
}
