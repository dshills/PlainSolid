package geom

import "context"

// Reparameterize maps the fundamental rectangle exactly, optionally reflecting
// either axis, and revalidates the certified seam order. Wrapped-axis choices
// are preserved. Derivatives use the new parameter units (and reflection sign).
// Unrepresentable mapped knots return ErrUnrepresentable; no rounded editing is
// accepted. Limits cover both mapping and seam verification cumulatively.
func (w *WrappedSurfacePatch) Reparameterize(ctx context.Context, target ParameterRectangle, reverseU, reverseV bool, o SurfacePatchOptions) (result *WrappedSurfacePatch, seam SurfaceSeamCertificate, err error) {
	if ctx == nil || w == nil || w.patch == nil {
		return nil, seam, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, seam, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return nil, seam, e
	}
	mapped, e := w.patch.Reparameterize(target, reverseU, reverseV, options)
	if e != nil {
		return nil, seam, e
	}
	remaining := options.MaxOperations - mapped.operations
	if remaining < 1 {
		return nil, seam, ErrConvergence
	}
	config := w.options
	config.MaxPoles = options.MaxPoles
	config.MaxOperations = remaining
	config.MaxCoefficientBits = options.MaxCoefficientBits
	result, seam, err = NewWrappedSurfacePatch(ctx, mapped, config)
	seam.ExactOperations += mapped.operations
	if result != nil {
		result.seam = seam
	}
	return
}

// SeamIso returns the immutable exact lower or upper edge descriptor on a
// wrapped axis. Its curve parameter is the other original chart coordinate.
// Opposite descriptors have the constructor's exact position identity; they
// retain distinct one-sided normal-partial semantics in the source patch.
func (w *WrappedSurfacePatch) SeamIso(axis int, upper bool, o SurfacePatchOptions) (*SurfaceIsoCurve, error) {
	if w == nil || w.patch == nil || axis < 0 || axis > 1 {
		return nil, ErrInvalid
	}
	if w.period[axis] == nil {
		return nil, ErrDomain
	}
	d := w.patch.Domain()
	lo, hi := d.UMin, d.UMax
	if axis == 1 {
		lo, hi = d.VMin, d.VMax
	}
	if upper {
		lo = hi
	}
	return w.patch.IsoCurve(axis, lo, o)
}
