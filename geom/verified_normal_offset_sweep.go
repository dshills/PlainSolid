package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetSweep is a proved embedded volume parameterization.
// It retains the original reference chart and exact signed limits. It
// does not build a B-rep or assign material between distinct source charts.
type VerifiedNormalOffsetSweep struct {
	source       *SurfacePatch
	lower, upper *big.Rat
	orientation  int
	projection   NormalOffsetSweepProjection
}

// NewVerifiedNormalOffsetSweep constructs a normal sweep only after the
// full three-dimensional injectivity and differential proof succeeds.
func NewVerifiedNormalOffsetSweep(ctx context.Context, source *SurfacePatch, lower, upper *big.Rat,
	projection NormalOffsetSweepProjection, o NormalOffsetCoverageOptions) (*VerifiedNormalOffsetSweep, NormalOffsetCoverage, error) {
	proof, err := source.CertifyNormalOffsetSweep(ctx, lower, upper, projection, o)
	if err != nil {
		return nil, proof, err
	}
	orientation := 1
	for i, a := range projection.Axes {
		if projection.Reversed[i] {
			orientation = -orientation
		}
		for _, b := range projection.Axes[:i] {
			if b > a {
				orientation = -orientation
			}
		}
	}
	return &VerifiedNormalOffsetSweep{
		source: source, lower: new(big.Rat).Set(lower), upper: new(big.Rat).Set(upper),
		orientation: orientation, projection: projection,
	}, proof, nil
}

// OriginalSource returns the immutable original reference chart and the
// signed coordinate projection used to prove the sweep.
func (s *VerifiedNormalOffsetSweep) OriginalSource() (*SurfacePatch, NormalOffsetSweepProjection) {
	if s == nil {
		return nil, NormalOffsetSweepProjection{}
	}
	return s.source, s.projection
}

// Domain gives the original UV rectangle and an owned copy of the exact
// displacement interval.
func (s *VerifiedNormalOffsetSweep) Domain() (ParameterRectangle, ExactParameterInterval) {
	if s == nil || s.source == nil {
		return ParameterRectangle{}, ExactParameterInterval{}
	}
	return s.source.Domain(), ExactParameterInterval{Lower: new(big.Rat).Set(s.lower), Upper: new(big.Rat).Set(s.upper)}
}

// Orientation reports the sign of the physical three-dimensional Jacobian.
func (s *VerifiedNormalOffsetSweep) Orientation() int {
	if s == nil {
		return 0
	}
	return s.orientation
}

// EvaluateRational encloses a point of the full normal sweep at exact
// original parameters. Neither the parameter nor the normal is rounded
// to select a location.
func (s *VerifiedNormalOffsetSweep) EvaluateRational(ctx context.Context, u, v, displacement *big.Rat,
	source SurfacePatchEvaluationOptions, offset SurfaceNormalOffsetEvaluationOptions) (RationalSurfaceOffsetPoint, error) {
	if s == nil || s.source == nil || displacement == nil {
		return RationalSurfaceOffsetPoint{}, ErrInvalid
	}
	if displacement.Cmp(s.lower) < 0 || displacement.Cmp(s.upper) > 0 {
		return RationalSurfaceOffsetPoint{}, ErrDomain
	}
	jet, err := s.source.EvaluateRational(ctx, u, v, source)
	if err != nil {
		return RationalSurfaceOffsetPoint{}, err
	}
	return jet.NormalOffsetRational(ctx, displacement, offset)
}

// BoundsRational encloses the entire sweep over a positive-area source
// rectangle and a closed displacement interval inside the verified sweep.
// Lower and upper may be equal to enclose a single layer. A side-wall cell
// can use a narrow source rectangle incident to its original edge.
func (s *VerifiedNormalOffsetSweep) BoundsRational(ctx context.Context, rectangle ParameterRectangle,
	lower, upper *big.Rat, o SurfacePatchOptions) (r RationalSurfaceOffsetPoint, err error) {
	if s == nil || s.source == nil || lower == nil || upper == nil || lower.Cmp(upper) > 0 {
		return r, ErrInvalid
	}
	if lower.Cmp(s.lower) < 0 || upper.Cmp(s.upper) > 0 {
		return r, ErrDomain
	}
	query := VerifiedNormalOffsetPatch{source: s.source, distance: lower}
	cell, budget, err := query.offsetBoundsCell(ctx, rectangle, o)
	if budget != nil {
		defer func() { r.ExactOperations = budget.operations }()
	}
	if err != nil {
		return r, err
	}
	if err := budget.check(upper); err != nil {
		return r, err
	}
	jet, err := offsetCartesianRanges(ctx, cell, budget)
	if err != nil {
		return r, err
	}
	a := offsetIntervalArithmetic{b: budget}
	n := a.cross(jet[1][0], jet[0][1])
	length, err := scaledOffsetNormalLength(&n, &a)
	if err != nil {
		return r, err
	}
	d := exactInterval{lower, upper}
	var lo, hi [3]*big.Rat
	for i := range n {
		x := a.add(jet[0][0][i], a.div(a.mul(d, n[i]), length))
		lo[i], hi[i] = x.lo, x.hi
	}
	if a.err != nil {
		return r, a.err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Lower, r.Upper = lo, hi
	r.Bounds = Box{Min: rationalSurfaceVectorBounds(lo).Min, Max: rationalSurfaceVectorBounds(hi).Max}
	return r, nil
}

// NormalOffsetSweepSide identifies one of the six original boundary
// parameter rectangles of the verified full sweep.
type NormalOffsetSweepSide uint8

const (
	NormalOffsetSweepUMin NormalOffsetSweepSide = iota
	NormalOffsetSweepUMax
	NormalOffsetSweepVMin
	NormalOffsetSweepVMax
	NormalOffsetSweepLower
	NormalOffsetSweepUpper
)

// VerifiedNormalOffsetSweepFace is an immutable analytic boundary support.
// A U wall uses (v,t), a V wall uses (u,t), and a layer uses (u,v).
// ReverseWinding specifies the outward side of the given parameterization.
type VerifiedNormalOffsetSweepFace struct {
	sweep *VerifiedNormalOffsetSweep
	side  NormalOffsetSweepSide
}

func (s *VerifiedNormalOffsetSweep) Boundary(side NormalOffsetSweepSide) (VerifiedNormalOffsetSweepFace, error) {
	if s == nil || s.source == nil || side > NormalOffsetSweepUpper {
		return VerifiedNormalOffsetSweepFace{}, ErrInvalid
	}
	return VerifiedNormalOffsetSweepFace{s, side}, nil
}

// ParameterDomain returns exact physical boundary parameter intervals.
// Side walls retain the exact signed displacement as their second axis.
func (f VerifiedNormalOffsetSweepFace) ParameterDomain() [2]ExactParameterInterval {
	if f.sweep == nil {
		return [2]ExactParameterInterval{}
	}
	d, t := f.sweep.Domain()
	axes := [2]ExactParameterInterval{
		{Lower: new(big.Rat).SetFloat64(d.UMin), Upper: new(big.Rat).SetFloat64(d.UMax)},
		{Lower: new(big.Rat).SetFloat64(d.VMin), Upper: new(big.Rat).SetFloat64(d.VMax)},
	}
	if f.side <= NormalOffsetSweepUMax {
		return [2]ExactParameterInterval{axes[1], t}
	}
	if f.side <= NormalOffsetSweepVMax {
		return [2]ExactParameterInterval{axes[0], t}
	}
	return axes
}

func (f VerifiedNormalOffsetSweepFace) ReverseWinding() bool {
	if f.sweep == nil {
		return false
	}
	reverse := f.side == NormalOffsetSweepUMin || f.side == NormalOffsetSweepVMax || f.side == NormalOffsetSweepLower
	return reverse != (f.sweep.orientation < 0)
}

func (f VerifiedNormalOffsetSweepFace) original(a, b *big.Rat) (u, v, distance *big.Rat, err error) {
	if f.sweep == nil || a == nil || b == nil {
		return nil, nil, nil, ErrInvalid
	}
	d := f.sweep.source.Domain()
	switch f.side {
	case NormalOffsetSweepUMin:
		return new(big.Rat).SetFloat64(d.UMin), a, b, nil
	case NormalOffsetSweepUMax:
		return new(big.Rat).SetFloat64(d.UMax), a, b, nil
	case NormalOffsetSweepVMin:
		return a, new(big.Rat).SetFloat64(d.VMin), b, nil
	case NormalOffsetSweepVMax:
		return a, new(big.Rat).SetFloat64(d.VMax), b, nil
	case NormalOffsetSweepLower:
		return a, b, f.sweep.lower, nil
	default:
		return a, b, f.sweep.upper, nil
	}
}

func (f VerifiedNormalOffsetSweepFace) EvaluateRational(ctx context.Context, a, b *big.Rat,
	source SurfacePatchEvaluationOptions, offset SurfaceNormalOffsetEvaluationOptions) (RationalSurfaceOffsetPoint, error) {
	u, v, distance, err := f.original(a, b)
	if err != nil {
		return RationalSurfaceOffsetPoint{}, err
	}
	return f.sweep.EvaluateRational(ctx, u, v, distance, source, offset)
}

// PartialsRational encloses the two analytic boundary partials. On
// a side wall the second derivative vector is the source unit normal.
func (f VerifiedNormalOffsetSweepFace) PartialsRational(ctx context.Context, a, b *big.Rat,
	source SurfacePatchEvaluationOptions, offset SurfaceNormalOffsetEvaluationOptions) (da, db RationalSurfaceOffsetPoint, err error) {
	u, v, distance, err := f.original(a, b)
	if err != nil {
		return da, db, err
	}
	if distance.Cmp(f.sweep.lower) < 0 || distance.Cmp(f.sweep.upper) > 0 {
		return da, db, ErrDomain
	}
	j, err := f.sweep.source.EvaluateSecondJetRational(ctx, u, v, source)
	if err != nil {
		return da, db, err
	}
	du, dv, err := j.offsetPartialsRational(ctx, distance, offset)
	if err != nil {
		return da, db, err
	}
	if f.side >= NormalOffsetSweepLower {
		return du, dv, nil
	}
	base := j.RationalSurfacePatchJet
	z := new(big.Rat)
	base.Point = [3]*big.Rat{z, z, z}
	unit, err := base.NormalOffsetRational(ctx, big.NewRat(1, 1), offset)
	if err != nil {
		return da, db, err
	}
	if f.side <= NormalOffsetSweepUMax {
		return dv, unit, nil
	}
	return du, unit, nil
}

func (f VerifiedNormalOffsetSweepFace) Eval(a, b float64) (Vec3, error) {
	if !finite(a) || !finite(b) {
		return Vec3{}, ErrDomain
	}
	p, err := f.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		return Vec3{}, err
	}
	return rationalOffsetVectorRepresentative(p.Lower, p.Upper)
}

func (f VerifiedNormalOffsetSweepFace) Partials(a, b float64) (Vec3, Vec3, error) {
	if !finite(a) || !finite(b) {
		return Vec3{}, Vec3{}, ErrDomain
	}
	da, db, err := f.PartialsRational(context.Background(), new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	x, err := rationalOffsetVectorRepresentative(da.Lower, da.Upper)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	y, err := rationalOffsetVectorRepresentative(db.Lower, db.Upper)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	return x, y, nil
}

var _ Surface = VerifiedNormalOffsetSweepFace{}
