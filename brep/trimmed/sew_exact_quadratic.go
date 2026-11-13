package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
)

// compareSewExactQuadratics only consumes native normalized exact quadratic
// trims. Equality is continuous exact surface-curve composition, not endpoint
// coincidence. Other source representations and subranges stay unsupported.
func compareSewExactQuadratics(ctx context.Context, a, b parametric.Edge, reversed bool, o edgecheck.Options) (bool, error) {
	x, ok := a.Curve.(surfaceTrimCurve)
	if !ok {
		return false, nil
	}
	y, ok := b.Curve.(surfaceTrimCurve)
	if !ok {
		return false, nil
	}
	if _, ok = x.curve.(ExactQuadraticUV); !ok {
		return false, nil
	}
	if _, ok = y.curve.(ExactQuadraticUV); !ok {
		return false, nil
	}
	if a.Range != (parametric.Interval{Max: 1}) || b.Range != (parametric.Interval{Max: 1}) {
		return false, nil
	}
	return compareSplineRationalTrimUse(ctx, a, y.surface, Use{UVStart: y.a, UVEnd: y.b, Curve: y.curve, Reversed: reversed}, o)
}
