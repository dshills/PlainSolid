package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphereOffAxisCylinderUnionResult identifies the spherical exterior, the
// exposed cylindrical wall and the upper planar cap.
type SphereOffAxisCylinderUnionResult struct {
	Solid                                      Solid
	SphericalFaces, CylindricalFaces, CapFaces []int
}

// SphereOffAxisCylinderUnion fuses a cylinder rising from a fully buried
// lower disk to a fully exposed upper disk. The tool footprint must be
// strictly contained in the sphere's equatorial disk. The cylindrical and
// spherical supports share the native analytic rim.
func SphereOffAxisCylinderUnion(ctx context.Context, frame geom.Frame, R, r, offset, lower, upper, tolerance float64) (SphereOffAxisCylinderUnionResult, error) {
	var empty SphereOffAxisCylinderUnionResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 || !finite(lower) || !finite(upper) || lower >= upper {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	chart, e := newToolBoreChart(ctx, offsetBoreChart{frame: frame, radius: R, bore: r, offset: offset, floor: upper, kind: toolBoreUnionWall}, tolerance)
	if e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sq := func(a *big.Rat) *big.Rat { return new(big.Rat).Mul(a, a) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	if add(sq(add(q(offset), q(r))), sq(add(q(math.Abs(lower)), pad))).Cmp(sq(q(R))) >= 0 {
		return empty, geom.ErrConvergence
	}
	var out SphereOffAxisCylinderUnionResult
	var faces []Face
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		for _, k := range []int{toolBoreLower, toolBoreUpper, toolBoreUnionWall, toolBoreUnionCap} {
			chart.kind = k
			switch k {
			case toolBoreLower, toolBoreUpper:
				chart.floor = 0 // the spherical chart does not own the tool cap
				out.SphericalFaces = append(out.SphericalFaces, len(faces))
			case toolBoreUnionWall:
				chart.floor = upper
				out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
			case toolBoreUnionCap:
				chart.floor = upper
				out.CapFaces = append(out.CapFaces, len(faces))
			}
			faces = append(faces, Face{Surface: chart, U: u, V: Interval{0, 1}, Reversed: k == toolBoreUpper || k == toolBoreUnionCap})
		}
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}
