package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// UnequalPerpendicularDifference is one connected X-cylinder stock with a
// transverse narrower Z-cylinder through-bore. Cutter faces point into the
// removed material. Volume is π*StockVolumePiCoefficient−intersection volume.
type UnequalPerpendicularDifference struct {
	Solid                                                         Solid
	StockWallFaces, CutterWallFaces, LowerCapFaces, UpperCapFaces []int
	StockVolumePiCoefficient                                      *big.Rat
}

// UnequalPerpendicularCylinderDifference retains the centered wider X stock
// after subtracting a narrower Z cutter. Both symmetric cap pairs must clear
// the opposite radius by 8*tolerance. The represented Pythagorean shoulder gate
// matches UnequalPerpendicularCylinderUnion; unsupported radii fail explicitly.
// The result has one connected genus-one boundary, not two separated end solids.
func UnequalPerpendicularCylinderDifference(ctx context.Context, center geom.Vec3, stockRadius, cutterRadius, stockHalf, cutterHalf, tolerance float64) (UnequalPerpendicularDifference, error) {
	return UnequalPerpendicularCylinderDifferenceWindows(ctx, center, stockRadius, cutterRadius, Interval{-stockHalf, stockHalf}, Interval{-cutterHalf, cutterHalf}, tolerance)
}

// UnequalPerpendicularCylinderDifferenceWindows retains stock between independently
// placed clear X caps; both Z cutter caps must also strictly clear the stock.
func UnequalPerpendicularCylinderDifferenceWindows(ctx context.Context, center geom.Vec3, stockRadius, cutterRadius float64, stockWindow, cutterWindow Interval, tolerance float64) (UnequalPerpendicularDifference, error) {
	empty := UnequalPerpendicularDifference{}
	donor, e := UnequalPerpendicularCylinderUnionWindows(ctx, center, stockRadius, cutterRadius, stockWindow, cutterWindow, tolerance)
	if e != nil {
		return empty, e
	}
	var out UnequalPerpendicularDifference
	var faces []Face
	for _, f := range donor.Solid.Faces() {
		chart, ok := f.Surface.(unequalUnionChart)
		if !ok {
			return empty, geom.ErrInvalid
		}
		if chart.kind >= 4 {
			continue
		}
		id := len(faces)
		faces = append(faces, Face{Surface: f.Surface, U: f.U, V: f.V, Reversed: f.Reversed})
		if chart.kind < 2 {
			out.StockWallFaces = append(out.StockWallFaces, id)
		} else if chart.side < 0 {
			out.LowerCapFaces = append(out.LowerCapFaces, id)
		} else {
			out.UpperCapFaces = append(out.UpperCapFaces, id)
		}
	}
	for _, sign := range []int{-1, 1} {
		out.CutterWallFaces = append(out.CutterWallFaces, len(faces))
		faces = append(faces, Face{Surface: unequalCylinderCommonChart{center: center, small: cutterRadius, large: stockRadius, k: 0, sign: sign}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: sign > 0})
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	r, l := new(big.Rat).SetFloat64(stockRadius), new(big.Rat).Sub(new(big.Rat).SetFloat64(stockWindow.Max), new(big.Rat).SetFloat64(stockWindow.Min))
	out.StockVolumePiCoefficient = new(big.Rat).Mul(new(big.Rat).Mul(r, r), l)
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}
