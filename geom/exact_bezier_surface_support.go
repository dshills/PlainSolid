package geom

import "context"

// ExactBezierSurfaceComparison retains the full bounded coefficient proof.
// ChartOrientation is the sign of the parameter map from A to B, and is zero
// unless equality was proved. At regular points it is also the relative normal
// orientation. It does not assert regularity, injectivity, or shell orientation.
type ExactBezierSurfaceComparison struct {
	ExactSurfaceComparison
	ChartOrientation int
}

// Compare proves equality of rational maps after the explicitly requested chart
// map. It permits distinct degrees and nonproportional homogeneous weight nets;
// equality of serialized data or pointers is neither required nor sufficient.
// An error returns no completed proof, even if some cells were processed.
func (s *ExactBezierSurface) Compare(ctx context.Context, other *ExactBezierSurface, o ExactSurfaceComparisonOptions) (ExactBezierSurfaceComparison, error) {
	if s == nil || other == nil || s.patch == nil || other.patch == nil {
		return ExactBezierSurfaceComparison{}, ErrInvalid
	}
	r, err := CompareExactSurfaces(ctx, s.patch, other.patch, o)
	if err != nil {
		return ExactBezierSurfaceComparison{}, err
	}
	out := ExactBezierSurfaceComparison{ExactSurfaceComparison: r}
	if r.Complete && r.Equal {
		out.ChartOrientation = 1
		for _, reverse := range []bool{o.SwapB, o.ReverseBU, o.ReverseBV} {
			if reverse {
				out.ChartOrientation = -out.ChartOrientation
			}
		}
	}
	return out, nil
}

// Bounds returns an outward rounded model-space control hull of the immutable
// positive-weight source. This encloses the whole closed parameter rectangle;
// it is not a tight surface range or a bound for an independently trimmed face.
func (s *ExactBezierSurface) Bounds() (Box, error) {
	if s == nil || s.patch == nil {
		return Box{}, ErrInvalid
	}
	return s.patch.Bounds()
}
