package document

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
)

func evaluateOrthogonalPrismFillet(ctx context.Context, f Feature) (trimmed.Solid, error) {
	if len(f.Profile) < 4 || len(f.Profile) > 64 || len(f.Holes) != 0 || !f.Offset.IsFinite() || f.Offset.X != 0 || f.Offset.Y != 0 || f.Offset.Z <= 0 {
		return trimmed.Solid{}, fmt.Errorf("%w: orthogonal corner fillet needs a positive vertical extrusion", geom.ErrInvalid)
	}
	z := f.Profile[0].Z
	outline := make([]geom.Vec2, len(f.Profile))
	for i, p := range f.Profile {
		if !p.IsFinite() || p.Z != z {
			return trimmed.Solid{}, fmt.Errorf("%w: orthogonal corner fillet needs a horizontal profile", geom.ErrInvalid)
		}
		outline[i] = geom.Vec2{X: p.X, Y: p.Y}
	}
	top, exact := new(big.Rat).Add(new(big.Rat).SetFloat64(z), new(big.Rat).SetFloat64(f.Offset.Z)).Float64()
	if !exact {
		return trimmed.Solid{}, fmt.Errorf("%w: the authored upper cap elevation", geom.ErrUnrepresentable)
	}
	radii := f.CornerRadii
	if len(radii) == 0 {
		radii = make([]fillet.CornerRadius, len(outline))
		for i := range radii {
			radii[i] = fillet.CornerRadius{Corner: i, Radius: f.Radius}
		}
	} else if f.Radius != 0 {
		return trimmed.Solid{}, fmt.Errorf("%w: orthogonal corner fillet selects Radius or CornerRadii", geom.ErrInvalid)
	}
	r, err := fillet.PrismOrthogonalCorners(ctx, outline, z, top, radii,
		fillet.OrthogonalPrismOptions{Circular: trimmed.ExactCircularLoopOptions{DisplayTolerance: f.Tolerance}})
	return r.Solid, err
}
