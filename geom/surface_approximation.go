package geom

import "math"

// SurfaceApproximationOptions specifies separable tensor least squares. U and V
// require explicit parameter arrays and pole counts; their degree/knot/fixed
// rational-weight/sample-weight options follow ApproximationOptions. Rational
// surface weights are the outer product of the two fixed pole-weight vectors;
// arbitrary nonseparable weights are not fitted. Axis MaxResidual fields must
// be zero; MaxResidual here caps the final grid residual. Zero global limits
// select 65536 samples, 4096 poles and 100M estimated QR work units.
type SurfaceApproximationOptions struct {
	U, V                 ApproximationOptions
	MaxResidual          float64
	MaxSamples, MaxPoles int
	MaxWork              int64
}

// SurfaceApproximationDiagnostics reports the two axis solves and final physical
// grid residuals. Axis diagnostics describe the worst condition estimate across
// row solves; their residual fields are intermediate, not surface-fit residuals.
// RMSResidual is unweighted; WeightedRMSResidual uses the outer product of axis
// sample weights. Condition estimates are numerical, not certified bounds.
type SurfaceApproximationDiagnostics struct {
	U, V                                          ApproximationDiagnostics
	MaxResidual, RMSResidual, WeightedRMSResidual float64
	Samples, Poles                                int
}

// ApproximateBSplineSurface fits points indexed u*countV+v at a tensor grid.
// Sequential column-pivoted QR solves compute the separable weighted least
// squares solution without normal equations. Matching sample/pole counts with
// full-rank design matrices gives numerical interpolation; overdetermined fits
// do not impose endpoint constraints. Inputs are copied and failures return no
// surface. Rank deficiency, conditioning, work and residual failures are explicit.
func ApproximateBSplineSurface(points []Vec3, countU, countV int, options SurfaceApproximationOptions) (surface *BSplineSurface, d SurfaceApproximationDiagnostics, err error) {
	o := options
	if o.MaxSamples == 0 {
		o.MaxSamples = 65536
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100000000
	}
	mu, mv := o.U.PoleCount, o.V.PoleCount
	if countU < 2 || countV < 2 || countU > len(points)/countV || countU*countV != len(points) || mu < 2 || mv < 2 || mu > countU || mv > countV || len(o.U.Parameters) != countU || len(o.V.Parameters) != countV || o.U.MaxResidual != 0 || o.V.MaxResidual != 0 || !finite(o.MaxResidual) || o.MaxResidual < 0 || o.MaxSamples < 4 || o.MaxPoles < 4 || o.MaxWork < 1 {
		return nil, d, ErrInvalid
	}
	if len(points) > o.MaxSamples || mu > o.MaxPoles/mv {
		return nil, d, ErrConvergence
	}
	remaining := o.MaxWork
	for _, factors := range [][4]int{{countU, mu, mu, countV}, {countV, mv, mv, mu}} {
		work := int64(1)
		for _, f := range factors {
			if work > remaining/int64(f) {
				return nil, d, ErrConvergence
			}
			work *= int64(f)
		}
		remaining -= work
	}
	for _, p := range points {
		if !p.IsFinite() {
			return nil, d, ErrInvalid
		}
	}
	intermediate := make([]Vec3, mu*countV)
	var uk, uw, vk, vw []float64
	du, dv := 0, 0
	for v := 0; v < countV; v++ {
		column := make([]Vec3, countU)
		for u := range column {
			column[u] = points[u*countV+v]
		}
		curve, diag, e := ApproximateBSpline(column, o.U)
		if e != nil {
			return nil, d, e
		}
		if v == 0 || diag.RConditionEstimate > d.U.RConditionEstimate {
			d.U = diag
		}
		uk, uw, du = curve.knots, curve.weights, curve.degree
		for u, p := range curve.poles {
			intermediate[u*countV+v] = p
		}
	}
	poles := make([]Vec3, mu*mv)
	for u := 0; u < mu; u++ {
		curve, diag, e := ApproximateBSpline(intermediate[u*countV:(u+1)*countV], o.V)
		if e != nil {
			return nil, d, e
		}
		if u == 0 || diag.RConditionEstimate > d.V.RConditionEstimate {
			d.V = diag
		}
		vk, vw, dv = curve.knots, curve.weights, curve.degree
		copy(poles[u*mv:(u+1)*mv], curve.poles)
	}
	weights := make([]float64, len(poles))
	for u := 0; u < mu; u++ {
		for v := 0; v < mv; v++ {
			weights[u*mv+v] = uw[u] * vw[v]
		}
	}
	surface, err = NewBSplineSurface(du, dv, mu, mv, poles, uk, vk, weights)
	if err != nil {
		return nil, d, err
	}
	d.Samples, d.Poles = len(points), len(poles)
	// Scaled sum-of-squares via Hypot avoids squaring residuals prematurely.
	sum, weighted, weightNorm := 0.0, 0.0, 0.0
	maxU, maxV := 1.0, 1.0
	if o.U.SampleWeights != nil {
		maxU = 0
		for _, w := range o.U.SampleWeights {
			maxU = max(maxU, w)
		}
	}
	if o.V.SampleWeights != nil {
		maxV = 0
		for _, w := range o.V.SampleWeights {
			maxV = max(maxV, w)
		}
	}
	for u := 0; u < countU; u++ {
		for v := 0; v < countV; v++ {
			p, e := surface.Eval(o.U.Parameters[u], o.V.Parameters[v])
			if e != nil {
				return nil, d, e
			}
			residual := p.Distance(points[u*countV+v])
			if !finite(residual) {
				return nil, d, ErrInvalid
			}
			d.MaxResidual = max(d.MaxResidual, residual)
			sum = math.Hypot(sum, residual)
			wu, wv := 1.0, 1.0
			if o.U.SampleWeights != nil {
				wu = o.U.SampleWeights[u]
			}
			if o.V.SampleWeights != nil {
				wv = o.V.SampleWeights[v]
			}
			w := math.Sqrt(wu/maxU) * math.Sqrt(wv/maxV)
			weighted = math.Hypot(weighted, residual*w)
			weightNorm = math.Hypot(weightNorm, w)
		}
	}
	d.RMSResidual = sum / math.Sqrt(float64(len(points)))
	d.WeightedRMSResidual = weighted / weightNorm
	if !finite(d.RMSResidual) || !finite(d.WeightedRMSResidual) {
		return nil, d, ErrInvalid
	}
	if o.MaxResidual > 0 && d.MaxResidual > o.MaxResidual {
		return nil, d, ErrConvergence
	}
	return surface, d, nil
}
