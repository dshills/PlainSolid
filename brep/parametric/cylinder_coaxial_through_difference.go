package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// CoaxialCylinderThroughDifference retains the stock cap locations and both
// cylindrical radii. Lower/upper refer to the original stock parameter order.
type CoaxialCylinderThroughDifference struct {
	Solid                                                         Solid
	StockWallFaces, CutterWallFaces, LowerCapFaces, UpperCapFaces []int
	VolumePiCoefficient                                           *big.Rat
}

// FiniteCoaxialCylinderThroughDifference subtracts a smaller coaxial cutter
// covering the entire stock axial interval. Axes must be signed coordinate
// directions; their magnitudes and parameter ranges are preserved exactly.
// Equal cap locations are supported. Unsupported or unrepresentable geometry
// returns no partial result. The retained boundary is one genus-one shell.
func FiniteCoaxialCylinderThroughDifference(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (CoaxialCylinderThroughDifference, error) {
	var zero CoaxialCylinderThroughDifference
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if _, err := geom.NewCylinderPairCommon(stock, cutter); err != nil {
		return zero, err
	}
	unit, scale := coordinateCylinderAxis(stock.Axis)
	other, otherScale := coordinateCylinderAxis(cutter.Axis)
	if scale == 0 || otherScale == 0 || unit.Cross(other) != (geom.Vec3{}) || cutter.Radius >= stock.Radius {
		return zero, geom.ErrDomain
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sv, cv := [3]float64{stock.Origin.X, stock.Origin.Y, stock.Origin.Z}, [3]float64{cutter.Origin.X, cutter.Origin.Y, cutter.Origin.Z}
	for i, v := range [3]float64{unit.X, unit.Y, unit.Z} {
		if v == 0 && sv[i] != cv[i] {
			return zero, geom.ErrDomain
		}
	}
	span := func(c geom.RawCylinder) [2]*big.Rat {
		base, delta := q(c.Origin.Dot(unit)), q(c.Axis.Dot(unit))
		a := new(big.Rat).Add(base, new(big.Rat).Mul(q(c.Min), delta))
		b := new(big.Rat).Add(base, new(big.Rat).Mul(q(c.Max), delta))
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		return [2]*big.Rat{a, b}
	}
	a, b := span(stock), span(cutter)
	if b[0].Cmp(a[0]) > 0 || b[1].Cmp(a[1]) < 0 {
		return zero, geom.ErrDomain
	}
	var start geom.Vec3
	for i, p := range [3]*float64{&start.X, &start.Y, &start.Z} {
		d := q([3]float64{stock.Axis.X, stock.Axis.Y, stock.Axis.Z}[i])
		var err error
		*p, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(sv[i]), new(big.Rat).Mul(q(stock.Min), d)))
		if err != nil {
			return zero, err
		}
		if _, err = geom.RepresentedExactScalar(new(big.Rat).Add(q(sv[i]), new(big.Rat).Mul(q(stock.Max), d))); err != nil {
			return zero, err
		}
	}
	heightRat := new(big.Rat).Sub(a[1], a[0])
	height, err := geom.RepresentedExactScalar(heightRat)
	if err != nil {
		return zero, err
	}
	gap := new(big.Rat).Sub(q(stock.Radius), q(cutter.Radius))
	clearance := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if gap.Cmp(clearance) <= 0 || q(cutter.Radius).Cmp(clearance) <= 0 || heightRat.Cmp(clearance) <= 0 {
		return zero, geom.ErrDomain
	}
	frame, err := geom.FrameFromNormal(start, unit)
	if err != nil {
		return zero, err
	}
	s, err := cylinderAnnulus(frame, DiskProfile{Radius: stock.Radius}, DiskProfile{Radius: cutter.Radius}, height, tolerance)
	if err != nil {
		return zero, err
	}
	out := CoaxialCylinderThroughDifference{Solid: s}
	for i := 0; i < 4; i++ {
		out.StockWallFaces = append(out.StockWallFaces, 4*i)
		out.CutterWallFaces = append(out.CutterWallFaces, 4*i+1)
		out.LowerCapFaces = append(out.LowerCapFaces, 4*i+2)
		out.UpperCapFaces = append(out.UpperCapFaces, 4*i+3)
	}
	out.VolumePiCoefficient = new(big.Rat).Mul(heightRat, new(big.Rat).Sub(new(big.Rat).Mul(q(stock.Radius), q(stock.Radius)), new(big.Rat).Mul(q(cutter.Radius), q(cutter.Radius))))
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
