package geom

import (
	"context"
	"math"
)

// SurfaceSample associates a model point with known surface parameters.
// Weight is its nonnegative least-squares weight. If all sample weights
// are zero, every sample receives unit weight. Otherwise zero omits a row.
type SurfaceSample struct {
	UV     Vec2
	Point  Vec3
	Weight float64
}

// ScatteredSurfaceOptions specifies a bounded fixed-basis surface fit.
// Expanded clamped knot vectors are required. Weights optionally supplies
// positive fixed rational weights in u*CountV+v order. Default limits are
// 4096 samples, 256 poles, 50M QR work units, and condition estimate 1e12.
// MaxResidual, when positive, bounds all evaluated sample errors, including
// zero-weight samples. Zero does not impose a residual cap.
type ScatteredSurfaceOptions struct {
	DegreeU, DegreeV, CountU, CountV int
	KnotsU, KnotsV, Weights          []float64
	MaxSamples, MaxPoles             int
	MaxWork                          int64
	ConditionLimit, MaxResidual      float64
}

// ScatteredSurfaceDiagnostics describes the represented fit. Condition is
// the numerical infinity-norm condition estimate for the QR R factor.
// The unweighted residuals include every supplied sample.
type ScatteredSurfaceDiagnostics struct {
	Samples, WeightedSamples, Poles, Rank int
	Condition, PivotRatio                 float64
	MaxResidual, RMSResidual, WeightedRMS float64
}

// ApproximateScatteredBSplineSurface fits a tensor-product surface to samples
// that need not form a grid. All control points are solved simultaneously by
// column-pivoted QR on the weighted rational basis; the fit does not infer UVs,
// knots or rational weights. Numerical conditioning and residual gates are
// explicit. Cancellation is checked before each bounded construction stage.
func ApproximateScatteredBSplineSurface(ctx context.Context, samples []SurfaceSample,
	o ScatteredSurfaceOptions) (*BSplineSurface, ScatteredSurfaceDiagnostics, error) {
	var d ScatteredSurfaceDiagnostics
	if ctx == nil {
		return nil, d, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, d, e
	}
	if o.MaxSamples == 0 {
		o.MaxSamples = 4096
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 50000000
	}
	if o.ConditionLimit == 0 {
		o.ConditionLimit = 1e12
	}
	if o.MaxSamples < 4 || o.MaxSamples > 1<<16 || o.MaxPoles < 4 || o.MaxPoles > 4096 || o.MaxWork < 1 ||
		!finite(o.ConditionLimit) || o.ConditionLimit < 1 || !finite(o.MaxResidual) || o.MaxResidual < 0 ||
		o.DegreeU < 1 || o.DegreeU > 64 || o.DegreeV < 1 || o.DegreeV > 64 || o.CountU <= o.DegreeU || o.CountV <= o.DegreeV {
		return nil, d, ErrInvalid
	}
	if len(samples) > o.MaxSamples || o.CountU > o.MaxPoles/o.CountV {
		return nil, d, ErrConvergence
	}
	n := o.CountU * o.CountV
	d.Samples, d.Poles = len(samples), n
	if len(samples) < n {
		return nil, d, ErrIllConditioned
	}
	if e := validateKnots(o.DegreeU, o.CountU, o.KnotsU); e != nil {
		return nil, d, e
	}
	if e := validateKnots(o.DegreeV, o.CountV, o.KnotsV); e != nil {
		return nil, d, e
	}
	if !projectionKnotsSupported(o.DegreeU, o.CountU, o.KnotsU) || !projectionKnotsSupported(o.DegreeV, o.CountV, o.KnotsV) {
		return nil, d, ErrInvalid
	}
	weights, e := copyWeights(n, o.Weights)
	if e != nil {
		return nil, d, e
	}
	maxWeight := 0.0
	u0, u1 := o.KnotsU[o.DegreeU], o.KnotsU[o.CountU]
	v0, v1 := o.KnotsV[o.DegreeV], o.KnotsV[o.CountV]
	for _, s := range samples {
		if !s.Point.IsFinite() || !finite(s.UV.X) || !finite(s.UV.Y) || s.UV.X < u0 || s.UV.X > u1 || s.UV.Y < v0 || s.UV.Y > v1 || !finite(s.Weight) || s.Weight < 0 {
			return nil, d, ErrInvalid
		}
		maxWeight = max(maxWeight, s.Weight)
		if s.Weight > 0 {
			d.WeightedSamples++
		}
	}
	if maxWeight == 0 {
		d.WeightedSamples = len(samples)
	}
	if d.WeightedSamples < n {
		return nil, d, ErrIllConditioned
	}
	// The factorization is O(rows*poles²), including pivot selection and
	// condition estimation. Charge before creating the dense design matrix.
	work := int64(d.WeightedSamples) * int64(n) * int64(n)
	if work > o.MaxWork || int64(d.WeightedSamples)*int64(n) > 1<<23 {
		return nil, d, ErrConvergence
	}
	origin := samples[0].Point
	var matrix [][]float64
	var rhs [3][]float64
	for _, s := range samples {
		if e := ctx.Err(); e != nil {
			return nil, d, e
		}
		if maxWeight > 0 && s.Weight == 0 {
			continue
		}
		scale := 1.
		if maxWeight > 0 {
			scale = math.Sqrt(s.Weight) / math.Sqrt(maxWeight)
			if scale == 0 {
				return nil, d, ErrIllConditioned
			}
		}
		u, e := interpolationBasis(o.DegreeU, o.CountU, o.KnotsU, s.UV.X)
		if e != nil {
			return nil, d, e
		}
		v, e := interpolationBasis(o.DegreeV, o.CountV, o.KnotsV, s.UV.Y)
		if e != nil {
			return nil, d, e
		}
		row := make([]float64, n)
		den := 0.
		for i, a := range u {
			for j, b := range v {
				k := i*o.CountV + j
				row[k] = a * b * weights[k]
				den += row[k]
			}
		}
		if den <= 0 || !finite(den) {
			return nil, d, ErrIllConditioned
		}
		for i, value := range row {
			row[i] = scale * (value / den)
		}
		matrix = append(matrix, row)
		p := s.Point.Sub(origin)
		if !p.IsFinite() {
			return nil, d, ErrInvalid
		}
		for i, x := range [3]float64{p.X, p.Y, p.Z} {
			rhs[i] = append(rhs[i], x*scale)
		}
	}
	qr, e := factorApproximationQR(matrix)
	d.Rank, d.PivotRatio = qr.rank, qr.pivotRatio
	if e != nil {
		return nil, d, e
	}
	if e := ctx.Err(); e != nil {
		return nil, d, e
	}
	d.Condition, e = qr.conditionEstimate()
	if e != nil || d.Condition > o.ConditionLimit {
		return nil, d, ErrIllConditioned
	}
	var x [3][]float64
	for i := range x {
		x[i], e = qr.solve(rhs[i])
		if e != nil {
			return nil, d, e
		}
	}
	poles := make([]Vec3, n)
	for i := range poles {
		poles[i] = origin.Add(Vec3{X: x[0][i], Y: x[1][i], Z: x[2][i]})
	}
	surface, e := NewBSplineSurface(o.DegreeU, o.DegreeV, o.CountU, o.CountV, poles, o.KnotsU, o.KnotsV, weights)
	if e != nil {
		return nil, d, e
	}
	sum, weighted, weightNorm := 0., 0., 0.
	for _, sample := range samples {
		if e := ctx.Err(); e != nil {
			return nil, d, e
		}
		p, e := surface.Eval(sample.UV.X, sample.UV.Y)
		if e != nil {
			return nil, d, e
		}
		residual := p.Distance(sample.Point)
		if !finite(residual) {
			return nil, d, ErrInvalid
		}
		d.MaxResidual = max(d.MaxResidual, residual)
		sum = math.Hypot(sum, residual)
		w := 1.
		if maxWeight > 0 {
			w = math.Sqrt(sample.Weight) / math.Sqrt(maxWeight)
		}
		weighted = math.Hypot(weighted, residual*w)
		weightNorm = math.Hypot(weightNorm, w)
	}
	d.RMSResidual = sum / math.Sqrt(float64(len(samples)))
	d.WeightedRMS = weighted / weightNorm
	if !finite(d.RMSResidual) || !finite(d.WeightedRMS) {
		return nil, d, ErrInvalid
	}
	if o.MaxResidual > 0 && d.MaxResidual > o.MaxResidual {
		return nil, d, ErrConvergence
	}
	return surface, d, nil
}
