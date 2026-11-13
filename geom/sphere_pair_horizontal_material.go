package geom

import "context"

// SpherePairHorizontalMaterial is an immutable exact horizontal slice of the
// GLOBAL regularized three-dimensional material. It is not regularized again
// in two dimensions: isolated pole/contact points remain when they belong to
// the original material. No unproved planar arc assembly is implied.
type SpherePairHorizontalMaterial struct {
	model  SpherePairMaterial
	height float64
	valid  bool
}

// HorizontalMaterial binds the represented section height without computing or
// rounding either projected circle radius. All model data remain private.
func (s SpherePairMaterial) HorizontalMaterial(height float64) (SpherePairHorizontalMaterial, error) {
	if s.radius2[0] == nil || !finite(height) {
		return SpherePairHorizontalMaterial{}, ErrInvalid
	}
	return SpherePairHorizontalMaterial{model: s, height: height, valid: true}, nil
}

// Height returns the original represented section height.
func (s SpherePairHorizontalMaterial) Height() float64 { return s.height }

// Classify uses exact original sphere constraints at (p.X,p.Y,Height()).
func (s SpherePairHorizontalMaterial) Classify(p Vec2) (BorePointLocation, BoreBoundaryMask, error) {
	if !s.valid {
		return BoreOutside, 0, ErrInvalid
	}
	return s.model.Classify(Vec3{X: p.X, Y: p.Y, Z: s.height})
}

// ClipLine preserves raw planar t and global exposed support masks, including
// isolated section material contacts. Errors return no partial report.
func (s SpherePairHorizontalMaterial) ClipLine(ctx context.Context, origin, direction Vec2, minT, maxT float64, o ToroidLineOptions) (SpherePairMaterialLine, error) {
	if !s.valid {
		return SpherePairMaterialLine{}, ErrInvalid
	}
	return s.model.ClipLine(ctx, Vec3{X: origin.X, Y: origin.Y, Z: s.height}, Vec3{X: direction.X, Y: direction.Y}, minT, maxT, o)
}

// FirstBoundaryHit retains the original 3D outward normal evidence. Its normal
// is not replaced by a projected 2D normal, which could vanish at a pole.
func (s SpherePairHorizontalMaterial) FirstBoundaryHit(ctx context.Context, origin, direction Vec2, maxT float64, o ToroidLineOptions) (SpherePairFirstHit, error) {
	if !s.valid {
		return SpherePairFirstHit{}, ErrInvalid
	}
	return s.model.FirstBoundaryHit(ctx, Vec3{X: origin.X, Y: origin.Y, Z: s.height}, Vec3{X: direction.X, Y: direction.Y}, maxT, o)
}

// Supports returns independent rational support circles and exact radical
// contacts. Those source contacts remain distinct from selected material.
func (s SpherePairHorizontalMaterial) Supports(ctx context.Context, o SurfacePatchEvaluationOptions) (SpherePairSection, error) {
	if !s.valid {
		return SpherePairSection{}, ErrInvalid
	}
	return s.model.HorizontalSection(ctx, s.height, o)
}
