package geom

import "context"

// WrappedSurfaceRecipe is lossless native reconstruction data. Surface is an
// immutable conventional spline and Options retains wrapped axes and required
// seam order. Reconstruct using NewSurfacePatch followed by
// NewWrappedSurfacePatch; serialized diagnostics are never proof inputs.
type WrappedSurfaceRecipe struct {
	Surface *BSplineSurface
	Options WrappedSurfaceOptions
}

// ExportRecipe refuses any fundamental coefficient that would round during
// conventional spline export. Returned spline data do not alias the source.
func (w *WrappedSurfacePatch) ExportRecipe() (WrappedSurfaceRecipe, error) {
	if w == nil || w.patch == nil {
		return WrappedSurfaceRecipe{}, ErrInvalid
	}
	surface, e := w.patch.ToBSplineSurface()
	if e != nil {
		return WrappedSurfaceRecipe{}, e
	}
	return WrappedSurfaceRecipe{Surface: surface, Options: w.options}, nil
}

// Transformed applies an exact represented affine map to the fundamental
// homogeneous support and revalidates its seam order. Singular placements are
// allowed: continuity is preserved, while regularity is not claimed. The
// transform's existing patch budget and this independent seam budget both apply.
func (w *WrappedSurfacePatch) Transformed(ctx context.Context, placement Affine, o WrappedSurfaceOptions) (*WrappedSurfacePatch, SurfaceSeamCertificate, error) {
	if ctx == nil || w == nil || w.patch == nil {
		return nil, SurfaceSeamCertificate{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, SurfaceSeamCertificate{}, e
	}
	p, e := w.patch.Transformed(placement)
	if e != nil {
		return nil, SurfaceSeamCertificate{}, e
	}
	// Placement preserves the wrapper's semantics; caller options supply only
	// arithmetic limits, never permission to weaken its certified seam order.
	o.WrapU, o.WrapV, o.ContinuityOrder = w.options.WrapU, w.options.WrapV, w.options.ContinuityOrder
	return NewWrappedSurfacePatch(ctx, p, o)
}
