package geom

// SurfaceHeightPatchOptions bounds exact common-XY comparison, subtraction and
// outward hull construction. Zero fields select 1M operations and 16384 bits.
type SurfaceHeightPatchOptions struct{ MaxOperations, MaxCoefficientBits int }

// NewSurfaceHeightDifferencePatch constructs (X,Y,ZA-ZB) exactly from immutable
// homogeneous control nets. Both patches must have identical degrees, full knot
// vectors and XY parameter maps, with globally proportional positive weights.
// All comparisons/subtractions are rational: no float64 pole difference is
// substituted for the exact difference, and no sampled equality is accepted.
// Unsupported basis/XY/weight mismatches return ErrInvalid. The resulting
// patch inherits A's subdivision limits; ExactOperations reports this build's
// comparison/construction work. Its exact net can be traced directly with the
// SurfacePatch plane/arc APIs even when no equivalent float64 pole net exists.
func NewSurfaceHeightDifferencePatch(a, b *SurfacePatch, o SurfaceHeightPatchOptions) (*SurfacePatch, error) {
	if a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return nil, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	p, ok, e := commonXYDifference(a, b, budget)
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, ErrInvalid
	}
	p.operations = budget.operations
	return p, nil
}
