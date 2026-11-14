package geom

import "context"

// ClipIsoCurveToTrapezoidChart isolates every original UV side event after
// exact rational inverse projection. The scope and rational endpoint recovery
// match ClipIsoCurveToAffineChart. Original curve and chart units are retained;
// coincident sides and unrecovered algebraic roots remain unresolved. Errors
// publish no partial clip (Operations still reports consumed work).
func ClipIsoCurveToTrapezoidChart(ctx context.Context, c *SurfaceIsoCurve, chart *TrapezoidSurfaceChart, o SplinePlaneOptions) (r AffineChartCurveClip, err error) {
	if ctx == nil || c == nil || chart == nil {
		return r, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	defer func() {
		if err != nil {
			r = AffineChartCurveClip{Operations: r.Operations}
		}
	}()
	projected, n, e := ProjectIsoCurveToTrapezoidChart(ctx, c, chart, SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	r.Operations = n
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: n}
	return clipProjectedIsoChart(ctx, c, projected, chart.domain, o, budget)
}
