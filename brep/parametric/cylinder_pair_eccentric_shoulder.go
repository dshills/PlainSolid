package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// finiteParallelEccentricShoulderUnion retains a circular boss that extends
// from one or both ends of a wider cylinder. Exact disk containment and interval
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
	if n1.Cmp(w0) < 0 || n0.Cmp(w1) > 0 {
		return zero, false, nil
	}
	lower, upper := n0.Cmp(w0) < 0, n1.Cmp(w1) > 0
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
	if lower && new(big.Rat).Sub(w0, n0).Cmp(pad) <= 0 || upper && new(big.Rat).Sub(n1, w1).Cmp(pad) <= 0 || new(big.Rat).Sub(w1, w0).Cmp(pad) <= 0 {
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
	for i, p := range []*big.Rat{w0, w1, n0, n1} {
		if i == 2 && !lower || i == 3 && !upper {
			continue
		}
		if _, err = at(wide.Origin, p); err != nil {
			return zero, true, err
		}
		z[i], err = local(p)
		if err != nil {
			return zero, true, err
		}
	}
	var caps [2]geom.Frame
	for i, extended := range [2]bool{lower, upper} {
		origin, station := wide.Origin, [2]*big.Rat{w0, w1}[i]
		if extended {
			origin, station = narrow.Origin, [2]*big.Rat{n0, n1}[i]
		}
		capOrigin, err := at(origin, station)
		if err != nil {
			return zero, true, err
		}
		caps[i], err = geom.NewFrame(capOrigin, f.XAxis(), axis)
		if err != nil {
			return zero, true, err
		}
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
		out.WallFaces[outer] = append(out.WallFaces[outer], len(faces))
		faces = append(faces, Face{Surface: reparameterizedCylinder{outerSurface, u}, U: normalized, V: Interval{z[0], z[1]}})
		if lower != upper {
			// Keep the one-shoulder face ordering used by native recipes.
			j := 0
			if upper {
				j = 1
			}
			interval := [2]Interval{{z[2], z[0]}, {z[1], z[3]}}[j]
			out.WallFaces[1-outer] = append(out.WallFaces[1-outer], len(faces))
			faces = append(faces, Face{Surface: reparameterizedCylinder{innerSurface, u}, U: normalized, V: interval})
			addCap(outer, upper, len(faces))
			faces = append(faces, Face{Surface: ruledArcs{outerSurface, innerSurface, u, u, z[j]}, U: normalized, V: normalized, Reversed: lower})
			addCap(outer, lower, len(faces))
			faces = append(faces, Face{Surface: disk{caps[1-j]}, U: u, V: Interval{0, wide.Radius}, Reversed: lower})
			addCap(1-outer, upper == forward, len(faces))
			faces = append(faces, Face{Surface: disk{caps[j]}, U: u, V: Interval{0, narrow.Radius}, Reversed: upper})
			continue
		}
		for j, extended := range [2]bool{lower, upper} {
			if extended {
				interval := Interval{z[2], z[0]}
				if j == 1 {
					interval = Interval{z[1], z[3]}
				}
				out.WallFaces[1-outer] = append(out.WallFaces[1-outer], len(faces))
				faces = append(faces, Face{Surface: reparameterizedCylinder{innerSurface, u}, U: normalized, V: interval})
				addCap(outer, j == 1, len(faces))
				faces = append(faces, Face{Surface: ruledArcs{outerSurface, innerSurface, u, u, z[j]}, U: normalized, V: normalized, Reversed: j == 0})
				addCap(1-outer, (j == 1) == forward, len(faces))
				faces = append(faces, Face{Surface: disk{caps[j]}, U: u, V: Interval{0, narrow.Radius}, Reversed: j == 1})
			} else {
				addCap(outer, j == 1, len(faces))
				faces = append(faces, Face{Surface: disk{caps[j]}, U: u, V: Interval{0, wide.Radius}, Reversed: j == 1})
			}
		}
	}
	out.Solid, err = joinPrimitivePatches(faces, tolerance)
	if err != nil {
		return zero, true, err
	}
	commonLo, commonHi := w0, w1
	if n0.Cmp(commonLo) > 0 {
		commonLo = n0
	}
	if n1.Cmp(commonHi) < 0 {
		commonHi = n1
	}
	common := new(big.Rat).Sub(commonHi, commonLo)
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
