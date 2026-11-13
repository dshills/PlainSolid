package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// finiteParallelCylinderBlind constructs a strictly contained radial cut with
// one open end. Shared quarter angles identify each analytic rim continuously;
// sewing is only an assembly check, not the radial containment proof.
func finiteParallelCylinderBlind(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (FiniteCylinderPairDifferenceResult, bool, error) {
	var out FiniteCylinderPairDifferenceResult
	axis, scale := coordinateCylinderAxis(stock.Axis)
	other, os := coordinateCylinderAxis(cutter.Axis)
	if scale == 0 || os == 0 || axis.Cross(other) != (geom.Vec3{}) || stock.Radius <= cutter.Radius {
		return out, false, nil
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	span := func(c geom.RawCylinder) [2]*big.Rat {
		a := new(big.Rat).Add(q(c.Origin.Dot(axis)), new(big.Rat).Mul(q(c.Axis.Dot(axis)), q(c.Min)))
		b := new(big.Rat).Add(q(c.Origin.Dot(axis)), new(big.Rat).Mul(q(c.Axis.Dot(axis)), q(c.Max)))
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		return [2]*big.Rat{a, b}
	}
	a, b := span(stock), span(cutter)
	lower := b[0].Cmp(a[0]) <= 0 && b[1].Cmp(a[0]) > 0 && b[1].Cmp(a[1]) < 0
	upper := b[1].Cmp(a[1]) >= 0 && b[0].Cmp(a[0]) > 0 && b[0].Cmp(a[1]) < 0
	if !lower && !upper {
		return out, false, nil
	}
	floor := b[0]
	if lower {
		floor = b[1]
	}
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if new(big.Rat).Sub(floor, a[0]).Cmp(pad) <= 0 || new(big.Rat).Sub(a[1], floor).Cmp(pad) <= 0 || q(cutter.Radius).Cmp(pad) <= 0 {
		return out, true, geom.ErrDomain
	}
	gap := new(big.Rat).Sub(new(big.Rat).Sub(q(stock.Radius), q(cutter.Radius)), pad)
	ds := new(big.Rat)
	sv, cv := [3]float64{stock.Origin.X, stock.Origin.Y, stock.Origin.Z}, [3]float64{cutter.Origin.X, cutter.Origin.Y, cutter.Origin.Z}
	for i, v := range [3]float64{axis.X, axis.Y, axis.Z} {
		if v == 0 {
			d := new(big.Rat).Sub(q(sv[i]), q(cv[i]))
			ds.Add(ds, new(big.Rat).Mul(d, d))
		}
	}
	if gap.Sign() <= 0 || ds.Cmp(new(big.Rat).Mul(gap, gap)) >= 0 {
		return out, true, geom.ErrDomain
	}
	h, err := geom.RepresentedExactScalar(new(big.Rat).Sub(a[1], a[0]))
	if err != nil {
		return out, true, err
	}
	z, err := geom.RepresentedExactScalar(new(big.Rat).Sub(floor, a[0]))
	if err != nil {
		return out, true, err
	}
	var start, inner geom.Vec3
	for i, p := range [3]*float64{&start.X, &start.Y, &start.Z} {
		d := q([3]float64{stock.Axis.X, stock.Axis.Y, stock.Axis.Z}[i])
		*p, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(sv[i]), new(big.Rat).Mul(q(stock.Min), d)))
		if err != nil {
			return out, true, err
		}
		if _, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(sv[i]), new(big.Rat).Mul(q(stock.Max), d))); err != nil {
			return out, true, err
		}
	}
	inner = start
	for i, p := range [3]*float64{&inner.X, &inner.Y, &inner.Z} {
		if [3]float64{axis.X, axis.Y, axis.Z}[i] == 0 {
			*p = cv[i]
		}
	}
	f, err := geom.FrameFromNormal(start, axis)
	if err != nil {
		return out, true, err
	}
	g, err := geom.NewFrame(inner, f.XAxis(), axis)
	if err != nil {
		return out, true, err
	}
	outerS, err := geom.NewCylinderSurface(f, stock.Radius)
	if err != nil {
		return out, true, err
	}
	innerS, err := geom.NewCylinderSurface(g, cutter.Radius)
	if err != nil {
		return out, true, err
	}
	// Prove all translated axial chart endpoints are representable, including
	// the original cutter floor, before any floating-point frame translation.
	at := func(origin geom.Vec3, height float64) (geom.Frame, error) {
		p := origin
		for i, v := range [3]*float64{&p.X, &p.Y, &p.Z} {
			var e error
			*v, e = geom.RepresentedExactScalar(new(big.Rat).Add(q([3]float64{origin.X, origin.Y, origin.Z}[i]), new(big.Rat).Mul(q([3]float64{axis.X, axis.Y, axis.Z}[i]), q(height))))
			if e != nil {
				return geom.Frame{}, e
			}
		}
		return geom.NewFrame(p, f.XAxis(), axis)
	}
	floorF, err := at(inner, z)
	if err != nil {
		return out, true, err
	}
	entry, opposite := h, 0.
	wall := Interval{z, h}
	if lower {
		entry, opposite = 0, h
		wall = Interval{0, z}
	}
	capF, err := at(start, opposite)
	if err != nil {
		return out, true, err
	}
	if _, err = at(inner, entry); err != nil {
		return out, true, err
	}
	var faces []Face
	var low, high []int
	for i := 0; i < 4; i++ {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		base := len(faces)
		faces = append(faces,
			Face{Surface: reparameterizedCylinder{outerS, u}, U: normalized, V: Interval{0, h}},
			Face{Surface: reparameterizedCylinder{innerS, u}, U: normalized, V: wall, Reversed: true},
			Face{Surface: ruledArcs{outerS, innerS, u, u, entry}, U: normalized, V: normalized, Reversed: lower},
			Face{Surface: disk{capF}, U: u, V: Interval{0, stock.Radius}, Reversed: lower},
			Face{Surface: disk{floorF}, U: u, V: Interval{0, cutter.Radius}, Reversed: !lower})
		out.StockWallFaces[0] = append(out.StockWallFaces[0], base)
		out.CutterWallFaces[0] = append(out.CutterWallFaces[0], base+1)
		out.CutterCapFaces[0] = append(out.CutterCapFaces[0], base+4)
		if lower {
			low = append(low, base+2)
			high = append(high, base+3)
		} else {
			low = append(low, base+3)
			high = append(high, base+2)
		}
	}
	out.StockCapFaces[0] = append(low, high...)
	solid, err := joinPrimitivePatches(faces, tolerance)
	if err != nil {
		return FiniteCylinderPairDifferenceResult{}, true, err
	}
	if err = ctx.Err(); err != nil {
		return FiniteCylinderPairDifferenceResult{}, true, err
	}
	out.Solids[0] = solid
	return out, true, nil
}
