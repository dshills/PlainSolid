package intersect

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// IsoPlaneTubeCoverageOptions bounds a full original-domain reduction. Zero
// values select 16M operations, 16384 bits, 4096 tubes and 256 knot/clip cells.
type IsoPlaneTubeCoverageOptions struct {
	// AllowTrapezoidCharts permits exact rational inversion of proved planar trapezoid supports.
	AllowTrapezoidCharts                                  bool
	MaxOperations, MaxCoefficientBits, MaxTubes, MaxCells int
}

// IsoPlaneTubeCoverage proves every original pair contact lies in a freshly
// verified tube. The raw plane equation fixes exactly one original source UV
// coordinate; all retained intervals follow complete exact chart clipping.
// Nonlinear trapezoid charts and boundary anchors require the explicit option.
// This is coverage only, not a branch graph or shell continuation certificate.
type IsoPlaneTubeCoverage struct {
	Covered                  bool
	SourceOperand, FixedAxis int
	FixedParameter           *big.Rat
	Spans                    [][2]*big.Rat
	AcceptedTubes            []int
	ExactOperations          int
}
type isoCoverWork struct{ left, used, bits int }

func (w *isoCoverWork) charge(n int) error {
	if n < 0 || n > w.left {
		return geom.ErrConvergence
	}
	w.left -= n
	w.used += n
	return nil
}
func (w *isoCoverWork) patch() (geom.SurfacePatchOptions, error) {
	if w.left < 1 {
		return geom.SurfacePatchOptions{}, geom.ErrConvergence
	}
	return geom.SurfacePatchOptions{MaxOperations: w.left, MaxCoefficientBits: w.bits}, nil
}

// CertifyIsoPlaneSplineTubeCoverage accepts an exact affine scalar plane
// residual depending on only U or V, and rational recovered clip endpoints.
// Tube free axes may differ: each covered source interval is independently
// clipped against that tube's original affine target rectangle. No projected
// interval alone is accepted, and no public ProvenBranch flag is trusted.
// Missing intervals, tangent isolated global contacts and coincidence fail closed.
func CertifyIsoPlaneSplineTubeCoverage(ctx context.Context, a, b Patch, seeds SplineSeedReport, o IsoPlaneTubeCoverageOptions) (IsoPlaneTubeCoverage, error) {
	empty := IsoPlaneTubeCoverage{}
	if ctx == nil || !a.valid() || !b.valid() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	var validation error
	o, validation = normalizedIsoPlaneCoverageOptions(o)
	if validation != nil {
		return empty, validation
	}
	if len(seeds.Attempts) > o.MaxTubes {
		return empty, geom.ErrConvergence
	}
	w := &isoCoverWork{left: o.MaxOperations, bits: o.MaxCoefficientBits}
	var patches [2]*geom.SurfacePatch
	for i, p := range [2]Patch{a, b} {
		opts, e := w.patch()
		if e != nil {
			return empty, e
		}
		q, e := exactSplinePatch(p, opts)
		if e != nil {
			return empty, e
		}
		if e = w.charge(q.ExactOperations()); e != nil {
			return empty, e
		}
		patches[i] = q
	}
	for source := 0; source < 2; source++ {
		p, target := patches[source], patches[source^1]
		opts, e := w.patch()
		if e != nil {
			return empty, e
		}
		chart, e := newIsoCoverageChart(ctx, target, o, w, true)
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) || errors.Is(e, geom.ErrUnrepresentable) {
				continue
			}
			return empty, e
		}
		opts, e = w.patch()
		if e != nil {
			return empty, e
		}
		plane, e := chart.RawSupportPlane(opts)
		if e != nil {
			return empty, e
		}
		if e = w.charge(plane.ExactOperations()); e != nil {
			return empty, e
		}
		equation := plane.Coefficients()
		normal := [3]*big.Rat{equation[0], equation[1], equation[2]}
		d := p.Domain()
		lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
		hi := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}
		var values [3]*big.Rat
		for i, uv := range [3][2]*big.Rat{lo, {hi[0], lo[1]}, {lo[0], hi[1]}} {
			opts, e = w.patch()
			if e != nil {
				return empty, e
			}
			jet, e := p.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: w.bits})
			if e != nil {
				return empty, e
			}
			if e = w.charge(jet.ExactOperations); e != nil {
				return empty, e
			}
			values[i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				values[i].Add(values[i], new(big.Rat).Mul(normal[j], jet.Point[j]))
			}
		}
		uv := [2]*big.Rat{}
		for i := range uv {
			uv[i] = new(big.Rat).Quo(new(big.Rat).Sub(values[i+1], values[0]), new(big.Rat).Sub(hi[i], lo[i]))
		}
		if (uv[0].Sign() == 0) == (uv[1].Sign() == 0) {
			continue
		}
		offset := new(big.Rat).Set(values[0])
		for i := range uv {
			offset.Sub(offset, new(big.Rat).Mul(uv[i], lo[i]))
		}
		opts, e = w.patch()
		if e != nil {
			return empty, e
		}
		identity, e := geom.CertifySurfaceAffineForm(ctx, p, normal, uv, offset, geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: opts, MaxCells: o.MaxCells})
		if e != nil {
			return empty, e
		}
		if e = w.charge(identity.ExactOperations); e != nil {
			return empty, e
		}
		if !identity.Proven {
			continue
		}
		axis := 0
		if uv[0].Sign() == 0 {
			axis = 1
		}
		fixed := new(big.Rat).Quo(new(big.Rat).Neg(new(big.Rat).Add(offset, equation[3])), uv[axis])
		result := IsoPlaneTubeCoverage{SourceOperand: source, FixedAxis: axis, FixedParameter: new(big.Rat).Set(fixed)}
		if fixed.Cmp(lo[axis]) < 0 || fixed.Cmp(hi[axis]) > 0 {
			result.Covered = true
			result.ExactOperations = w.used
			return result, nil
		}
		f, e := geom.RepresentedExactScalar(fixed)
		if e != nil {
			return empty, ErrUnresolved
		}
		opts, e = w.patch()
		if e != nil {
			return empty, e
		}
		iso, e := p.IsoCurve(axis, f, opts)
		if e != nil {
			return empty, e
		}
		if e = w.charge(iso.ExactOperations()); e != nil {
			return empty, e
		}
		full, e := clipIsoCover(ctx, iso, chart, w)
		if e != nil {
			return empty, e
		}
		if len(full.Spans)+len(full.Contacts)+len(full.CoincidentBoundary) > o.MaxCells {
			return empty, geom.ErrConvergence
		}
		if !full.Complete || len(full.Contacts) > 0 || len(full.CoincidentBoundary) > 0 {
			return empty, ErrUnresolved
		}
		var intervals [][2]*big.Rat
		for index, seed := range seeds.Attempts {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			if seed.Tube == nil || seed.BoundaryAnchor != nil && !o.AllowTrapezoidCharts {
				continue
			}
			boxes := [2]geom.ParameterRectangle{seed.Cell.DomainA, seed.Cell.DomainB}
			var restricted [2]*geom.SurfacePatch
			for i := range restricted {
				opts, e = w.patch()
				if e != nil {
					return empty, e
				}
				q, n, e := patches[i].TrimWithWorkLimit(boxes[i], opts.MaxOperations)
				if e != nil {
					return empty, e
				}
				if e = w.charge(n); e != nil {
					return empty, e
				}
				restricted[i] = q
			}
			opts, e = w.patch()
			if e != nil {
				return empty, e
			}
			proof, e := geom.CertifySurfacePairTube(restricted[0], restricted[1], geom.SurfacePairTubeOptions{FreeAxis: seed.Tube.FreeAxis, MaxOperations: opts.MaxOperations, MaxCoefficientBits: w.bits})
			if seed.BoundaryAnchor != nil {
				if ce := w.charge(proof.Test.ExactOperations); ce != nil {
					return empty, ce
				}
				opts, e = w.patch()
				if e != nil {
					return empty, e
				}
				proof, e = certifySplineTubeCell(ctx, restricted[0], restricted[1], geom.SurfacePairTubeOptions{FreeAxis: seed.Tube.FreeAxis, MaxOperations: opts.MaxOperations, MaxCoefficientBits: w.bits}, seed.BoundaryAnchor)
			}
			if e != nil {
				return empty, e
			}
			if e = w.charge(proof.Test.ExactOperations); e != nil {
				return empty, e
			}
			if !proof.ProvenBranch {
				return empty, ErrUnresolved
			}
			box := boxes[source]
			rangeFixed := [2]float64{box.UMin, box.UMax}
			if axis == 1 {
				rangeFixed = [2]float64{box.VMin, box.VMax}
			}
			if fixed.Cmp(new(big.Rat).SetFloat64(rangeFixed[0])) < 0 || fixed.Cmp(new(big.Rat).SetFloat64(rangeFixed[1])) > 0 {
				continue
			}
			opts, e = w.patch()
			if e != nil {
				return empty, e
			}
			targetChart, e := newIsoCoverageChart(ctx, restricted[source^1], o, w, false)
			if e != nil {
				return empty, e
			}
			opts, e = w.patch()
			if e != nil {
				return empty, e
			}
			curve, e := restricted[source].IsoCurve(axis, f, opts)
			if e != nil {
				return empty, e
			}
			if e = w.charge(curve.ExactOperations()); e != nil {
				return empty, e
			}
			clipped, e := clipIsoCover(ctx, curve, targetChart, w)
			if e != nil {
				return empty, e
			}
			if !clipped.Complete || len(clipped.CoincidentBoundary) > 0 {
				return empty, ErrUnresolved
			}
			if len(clipped.Spans)+len(clipped.Contacts) > 0 {
				result.AcceptedTubes = append(result.AcceptedTubes, index)
			}
			for _, span := range clipped.Spans {
				intervals = append(intervals, [2]*big.Rat{span.Start.Parameter, span.End.Parameter})
			}
			for _, point := range clipped.Contacts {
				intervals = append(intervals, [2]*big.Rat{point.Parameter, point.Parameter})
			}
			if len(intervals) > o.MaxCells {
				return empty, geom.ErrConvergence
			}
		}
		sort.Slice(intervals, func(i, j int) bool { return intervals[i][0].Cmp(intervals[j][0]) < 0 })
		for _, span := range full.Spans {
			cursor := new(big.Rat).Set(span.Start.Parameter)
			for _, rangeU := range intervals {
				if rangeU[0].Cmp(cursor) > 0 {
					break
				}
				if rangeU[1].Cmp(cursor) > 0 {
					cursor.Set(rangeU[1])
				}
				if cursor.Cmp(span.End.Parameter) >= 0 {
					break
				}
			}
			if cursor.Cmp(span.End.Parameter) < 0 {
				return empty, ErrUnresolved
			}
			result.Spans = append(result.Spans, [2]*big.Rat{new(big.Rat).Set(span.Start.Parameter), new(big.Rat).Set(span.End.Parameter)})
		}
		result.Covered = true
		result.ExactOperations = w.used
		return result, nil
	}
	return empty, ErrUnresolved
}
func clipIsoCover(ctx context.Context, curve *geom.SurfaceIsoCurve, chart isoCoverageChart, w *isoCoverWork) (geom.AffineChartCurveClip, error) {
	opts, e := w.patch()
	if e != nil {
		return geom.AffineChartCurveClip{}, e
	}
	var r geom.AffineChartCurveClip
	switch c := chart.(type) {
	case *geom.AffineSurfaceChart:
		r, e = geom.ClipIsoCurveToAffineChart(ctx, curve, c, geom.SplinePlaneOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: w.bits})
	case *geom.TrapezoidSurfaceChart:
		r, e = geom.ClipIsoCurveToTrapezoidChart(ctx, curve, c, geom.SplinePlaneOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: w.bits})
	default:
		return r, geom.ErrInvalid
	}
	if e != nil {
		return r, e
	}
	if e = w.charge(r.Operations); e != nil {
		return geom.AffineChartCurveClip{}, e
	}
	return r, nil
}

func normalizedIsoPlaneCoverageOptions(o IsoPlaneTubeCoverageOptions) (IsoPlaneTubeCoverageOptions, error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxTubes == 0 {
		o.MaxTubes = 4096
	}
	if o.MaxCells == 0 {
		o.MaxCells = 256
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxTubes < 1 || o.MaxTubes > 4096 || o.MaxCells < 1 || o.MaxCells > 4096 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// Failed recognition consumes its full reserved cap because these constructors
// deliberately publish no partial certificate or partial arithmetic report.
type isoCoverageChart interface {
	ExactOperations() int
	RawSupportPlane(geom.SurfacePatchOptions) (geom.RawSupportPlane, error)
}

func newIsoCoverageChart(ctx context.Context, p *geom.SurfacePatch, o IsoPlaneTubeCoverageOptions, w *isoCoverWork, initial bool) (isoCoverageChart, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 && !o.AllowTrapezoidCharts {
			break
		}
		opts, e := w.patch()
		if e != nil {
			return nil, e
		}
		cap := opts.MaxOperations
		if initial || o.AllowTrapezoidCharts {
			cap = min(cap, 1000000)
		}
		co := geom.AffineSurfaceChartOptions{MaxCells: o.MaxCells, MaxOperations: cap, MaxCoefficientBits: w.bits}
		var chart isoCoverageChart
		if attempt == 0 {
			chart, e = geom.NewAffineSurfaceChart(ctx, p, co)
		} else {
			chart, e = geom.NewTrapezoidSurfaceChart(ctx, p, co)
		}
		if e == nil {
			if e = w.charge(chart.ExactOperations()); e != nil {
				return nil, e
			}
			return chart, nil
		}
		if ce := w.charge(cap); ce != nil {
			return nil, ce
		}
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) && !errors.Is(e, geom.ErrUnrepresentable) {
			return nil, e
		}
	}
	return nil, geom.ErrConvergence
}
