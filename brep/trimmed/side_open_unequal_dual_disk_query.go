package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Locate classifies a represented world point using the original unequal
// disk equations and signed coordinate frame, without proximity tolerances.
func (c UnequalSideOpenDualDiskCut) Locate(ctx context.Context, point geom.Vec3) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	return c.LocateRational(ctx, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)})
}

// LocateRational preserves arbitrary exact rational input coordinates.
func (c UnequalSideOpenDualDiskCut) LocateRational(ctx context.Context, point [3]*big.Rat) (parametric.Location, error) {
	return locateSideOpenDualDisk(ctx, c.source, point)
}

// Measures integrates the two exposed circular envelope portions separately,
// including asymmetric first moments. Results are rounded numerical analytic
// measures, not certified enclosures. It uses no display mesh or quadrature.
func (c UnequalSideOpenDualDiskCut) Measures() (SideOpenDualDiskMeasures, error) {
	var zero SideOpenDualDiskMeasures
	s := c.source
	if s == nil {
		return zero, geom.ErrInvalid
	}
	left, right := s.discs[0], s.discs[1]
	if left.Center.X > right.Center.X {
		left, right = right, left
	}
	d := new(big.Rat).Sub(ratInset(right.Center.X), ratInset(left.Center.X))
	r := ratInset(left.Radius)
	R := ratInset(right.Radius)
	a := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(d, d), new(big.Rat).Sub(new(big.Rat).Mul(r, r), new(big.Rat).Mul(R, R))), new(big.Rat).Mul(big.NewRat(2, 1), d))
	b := new(big.Rat).Sub(d, a)
	alpha, _ := new(big.Rat).Quo(a, r).Float64()
	beta, _ := new(big.Rat).Quo(b, R).Float64()
	aa, _ := a.Float64()
	bb, _ := b.Float64()
	h2 := new(big.Rat).Sub(new(big.Rat).Mul(r, r), new(big.Rat).Mul(a, a))
	height2, _ := h2.Float64()
	h := math.Sqrt(height2)
	thetaL, thetaR := math.Pi/2+math.Asin(alpha), math.Pi/2+math.Asin(beta)
	rl, rr := left.Radius, right.Radius
	areaL, areaR := (aa*h+rl*rl*thetaL)/2, (bb*h+rr*rr*thetaR)/2
	removedArea := areaL + areaR
	// Relative x moments +/-h³/3 cancel exactly at the radical-axis split.
	removedX := left.Center.X*areaL + right.Center.X*areaR
	removedY := s.min.Y*removedArea + (rl*rl*aa-aa*aa*aa/3+2*rl*rl*rl/3+rr*rr*bb-bb*bb*bb/3+2*rr*rr*rr/3)/2
	width, length := s.max.X-s.min.X, s.max.Y-s.min.Y
	stockVolume := width * length * s.height
	volume := stockVolume - s.depth*removedArea
	opening := right.Center.X + rr - (left.Center.X - rl)
	area := 2*(width*length+(width+length)*s.height) + (rl*thetaL+rr*thetaR-opening)*s.depth
	if s.depth == s.height {
		area -= 2 * removedArea
	}
	centroid := s.frame.ToWorld(geom.Vec3{X: (stockVolume*(s.min.X+width/2) - s.depth*removedX) / volume, Y: (stockVolume*(s.min.Y+length/2) - s.depth*removedY) / volume, Z: (stockVolume*s.height/2 - s.depth*s.depth*removedArea/2) / volume})
	if volume <= 0 || area <= 0 || !finite(volume) || !finite(area) || !centroid.IsFinite() {
		return zero, geom.ErrUnrepresentable
	}
	return SideOpenDualDiskMeasures{Volume: volume, Area: area, Centroid: centroid}, nil
}
