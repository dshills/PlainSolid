package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type WrappedStationMassInterval struct{ Lower, Upper *big.Rat }

// WrappedRationalStationMassOptions separates native authentication from the
// aggregate rational integration ledger. VolumeTolerance is a positive exact
// requested enclosure width. Defaults:1024 segments,1M operations,16384 bits;
// maximum16384 segments,8M operations,65536 bits. Failure returns no partial mass.
type WrappedRationalStationMassOptions struct {
	Source                                         geom.SurfacePatchOptions
	VolumeTolerance                                *big.Rat
	MaxSegments, MaxOperations, MaxCoefficientBits int
}

// WrappedRationalStationMassBounds are unit-density measures of the exact
// original native material. Source uses original IDs; no display mesh is used.
// ProfileArea is dimensionless. Axial factors are exact integrals of scale²,
// scale³ and z*scale². The axial centroid is exact. Horizontal first moments
// retain signed uncertainty unless exact central symmetry proves them zero.
type WrappedRationalStationMassBounds struct {
	Source                                                 WrappedRationalStationProof
	ProfileArea, Volume                                    WrappedStationMassInterval
	FirstMoments, Centroid                                 [3]WrappedStationMassInterval
	AxialVolumeFactor, AxialCubicFactor, AxialFirstZFactor *big.Rat
	Segments, ExactOperations                              int
}

// WrappedRationalStationMeasure contains the certified mathematical bounds
// shared by filled and hollow homothetic profiles. The axial factors for a
// bore are integrals of outer minus inner powers of the original scale.
type WrappedRationalStationMeasure struct {
	ProfileArea, Volume                                    WrappedStationMassInterval
	FirstMoments, Centroid                                 [3]WrappedStationMassInterval
	AxialVolumeFactor, AxialCubicFactor, AxialFirstZFactor *big.Rat
	Segments, ExactOperations                              int
}

type stationMassBudget struct {
	ctx               context.Context
	limit, bits, used int
	err               error
}

func (b *stationMassBudget) check(q *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return new(big.Rat)
	}
	b.used++
	if b.used > b.limit || q == nil || q.Num().BitLen() > b.bits || q.Denom().BitLen() > b.bits {
		b.err = geom.ErrConvergence
		return new(big.Rat)
	}
	return q
}
func (b *stationMassBudget) op(op byte, x, y *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	b.check(x)
	b.check(y)
	if b.err != nil {
		return new(big.Rat)
	}
	q := new(big.Rat)
	switch op {
	case '+':
		q.Add(x, y)
	case '-':
		q.Sub(x, y)
	case '*':
		q.Mul(x, y)
	case '/':
		if y.Sign() == 0 {
			b.err = geom.ErrDomain
			return q
		}
		q.Quo(x, y)
	}
	return b.check(q)
}
func (b *stationMassBudget) add(x, y *big.Rat) *big.Rat { return b.op('+', x, y) }
func (b *stationMassBudget) sub(x, y *big.Rat) *big.Rat { return b.op('-', x, y) }
func (b *stationMassBudget) mul(x, y *big.Rat) *big.Rat { return b.op('*', x, y) }
func (b *stationMassBudget) div(x, y *big.Rat) *big.Rat { return b.op('/', x, y) }
func (b *stationMassBudget) cross(x, y [2]*big.Rat) *big.Rat {
	return b.sub(b.mul(x[0], y[1]), b.mul(x[1], y[0]))
}
func (b *stationMassBudget) scaled(q WrappedStationMassInterval, s *big.Rat) WrappedStationMassInterval {
	lo, hi := b.mul(q.Lower, s), b.mul(q.Upper, s)
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	return WrappedStationMassInterval{lo, hi}
}
func (b *stationMassBudget) plus(a, c WrappedStationMassInterval) WrappedStationMassInterval {
	return WrappedStationMassInterval{b.add(a.Lower, c.Lower), b.add(a.Upper, c.Upper)}
}
func (b *stationMassBudget) quotient(a, c WrappedStationMassInterval) WrappedStationMassInterval {
	if b.err != nil {
		return WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
	}
	if c.Lower.Sign() <= 0 {
		b.err = geom.ErrDomain
		return WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
	}
	var lo, hi *big.Rat
	for _, x := range []*big.Rat{a.Lower, a.Upper} {
		for _, y := range []*big.Rat{c.Lower, c.Upper} {
			v := b.div(x, y)
			if lo == nil || v.Cmp(lo) < 0 {
				lo = v
			}
			if hi == nil || v.Cmp(hi) > 0 {
				hi = v
			}
		}
	}
	return WrappedStationMassInterval{lo, hi}
}

type stationMassSegment [3][3]*big.Rat // exact normalized homogeneous XYW

// BoundWrappedRationalStationMass encloses the original profile by exact inner
// chord polygons and outer rational-control polygons. Per-sector convexity and
// polar-wedge signs are checked exactly; positive homogeneous subdivision keeps
// the curve in each control triangle. Uncertain area/first moments are charged
// to that triangle. Analytic station integrals then transfer these bounds to
// volume and centroid. Central symmetry is used only after exact weight-ratio
// equality. Unsupported exact source exports reject rather than rounding poles.
func (s Solid) BoundWrappedRationalStationMass(ctx context.Context, o WrappedRationalStationMassOptions) (WrappedRationalStationMassBounds, error) {
	empty := WrappedRationalStationMassBounds{}
	o, err := normalizeWrappedStationMassOptions(ctx, o)
	if err != nil {
		return empty, err
	}
	proof, bands, err := s.validateWrappedProfileStationLoft(ctx, o.Source, false)
	if err != nil {
		return empty, err
	}
	source := WrappedRationalStationProof{proof.Center, proof.Heights, proof.Radii, proof.History, proof.Complete}
	result, err := boundWrappedStationProfileMass(ctx, proof, nil, bands[0], o)
	if err != nil {
		return empty, err
	}
	return WrappedRationalStationMassBounds{
		Source: source, ProfileArea: result.ProfileArea, Volume: result.Volume,
		FirstMoments: result.FirstMoments, Centroid: result.Centroid,
		AxialVolumeFactor: result.AxialVolumeFactor, AxialCubicFactor: result.AxialCubicFactor,
		AxialFirstZFactor: result.AxialFirstZFactor, Segments: result.Segments, ExactOperations: result.ExactOperations,
	}, nil
}

// WrappedRationalStationBoreMassBounds binds the measure to both authenticated
// original wall families and the two annular caps.
type WrappedRationalStationBoreMassBounds struct {
	WrappedRationalStationMeasure
	Source WrappedStationBoreProof
}

// BoundWrappedRationalStationBoreMass uses the common profile once. Analytic
// axial factors subtract the strictly contained inner band integrals before
// the profile interval is multiplied, retaining their exact correlation.
func (s Solid) BoundWrappedRationalStationBoreMass(ctx context.Context, o WrappedRationalStationMassOptions) (WrappedRationalStationBoreMassBounds, error) {
	var empty WrappedRationalStationBoreMassBounds
	o, err := normalizeWrappedStationMassOptions(ctx, o)
	if err != nil {
		return empty, err
	}
	proof, err := s.ValidateWrappedRationalStationBore(ctx, o.Source)
	if err != nil {
		return empty, err
	}
	band := s.faces[proof.OuterWalls[0]].Surface.(*geom.WrappedSurfacePatch)
	result, err := boundWrappedStationProfileMass(ctx, proof.Outer, &proof.Inner, band, o)
	if err != nil {
		return empty, err
	}
	return WrappedRationalStationBoreMassBounds{WrappedRationalStationMeasure: result, Source: proof}, nil
}

func normalizeWrappedStationMassOptions(ctx context.Context, o WrappedRationalStationMassOptions) (WrappedRationalStationMassOptions, error) {
	if o.MaxSegments == 0 {
		o.MaxSegments = 1024
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if ctx == nil || o.VolumeTolerance == nil || o.VolumeTolerance.Sign() <= 0 || o.MaxSegments < 4 || o.MaxSegments > 16384 || o.MaxOperations < 1 || o.MaxOperations > 8000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return o, geom.ErrInvalid
	}
	return o, ctx.Err()
}

func boundWrappedStationProfileMass(ctx context.Context, proof WrappedStationLoftProof, inner *WrappedStationLoftProof,
	band *geom.WrappedSurfacePatch, o WrappedRationalStationMassOptions) (WrappedRationalStationMeasure, error) {
	empty := WrappedRationalStationMeasure{}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	tolerance := b.check(o.VolumeTolerance)
	if b.err != nil {
		return empty, b.err
	}
	tolerance = new(big.Rat).Set(tolerance)
	segments, weights, err := stationProfileSegments(band, b)
	if err != nil {
		return empty, err
	}
	symmetric := stationProfileCentrallySymmetric(weights, b)
	vFactor, cFactor, zFactor := wrappedStationAxialMassFactors(proof, b)
	if inner != nil {
		v, c, z := wrappedStationAxialMassFactors(*inner, b)
		vFactor, cFactor, zFactor = b.sub(vFactor, v), b.sub(cFactor, c), b.sub(zFactor, z)
	}
	if b.err != nil {
		return empty, b.err
	}
	if vFactor.Sign() <= 0 || cFactor.Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	for {
		area, moments, err := stationProfileEnclosure(segments, true, b)
		if err != nil {
			return empty, err
		}
		volume := b.scaled(area, vFactor)
		width := b.sub(volume.Upper, volume.Lower)
		if b.err != nil {
			return empty, b.err
		}
		if width.Cmp(tolerance) <= 0 {
			out := WrappedRationalStationMeasure{ProfileArea: area, Volume: volume, AxialVolumeFactor: vFactor, AxialCubicFactor: cFactor, AxialFirstZFactor: zFactor, Segments: len(segments)}
			for axis := 0; axis < 2; axis++ {
				if symmetric {
					moments[axis] = WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
				}
				out.FirstMoments[axis] = b.plus(b.scaled(volume, proof.Center[axis]), b.scaled(moments[axis], cFactor))
				shift := b.scaled(b.quotient(moments[axis], area), b.div(cFactor, vFactor))
				out.Centroid[axis] = b.plus(shift, WrappedStationMassInterval{proof.Center[axis], proof.Center[axis]})
			}
			out.FirstMoments[2] = b.scaled(area, zFactor)
			centroidZ := b.div(zFactor, vFactor)
			out.Centroid[2] = WrappedStationMassInterval{new(big.Rat).Set(centroidZ), new(big.Rat).Set(centroidZ)}
			if b.err != nil {
				return empty, b.err
			}
			out.ExactOperations = b.used
			return out, nil
		}
		if len(segments) > o.MaxSegments/2 {
			return empty, geom.ErrConvergence
		}
		segments = splitStationProfileSegments(segments, b)
		if b.err != nil {
			return empty, b.err
		}
	}
}

func stationProfileSegments(band *geom.WrappedSurfacePatch, b *stationMassBudget) ([]stationMassSegment, [9]*big.Rat, error) {
	bs, e := band.FundamentalPatch().ToBSplineSurface()
	if e != nil {
		return nil, [9]*big.Rat{}, e
	}
	data := bs.Data()
	var weights [9]*big.Rat
	for i := range weights {
		weights[i] = big.NewRat(1, 1)
		if len(data.Weights) > 0 {
			weights[i] = ratInset(data.Weights[2*i])
		}
		b.check(weights[i])
		if weights[i].Sign() <= 0 {
			return nil, [9]*big.Rat{}, geom.ErrDomain
		}
	}
	coordinates := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	segments := make([]stationMassSegment, 4)
	for quarter := range segments {
		for j := 0; j < 3; j++ {
			i := 2*quarter + j
			segments[quarter][j] = [3]*big.Rat{b.mul(big.NewRat(coordinates[i][0], 1), weights[i]), b.mul(big.NewRat(coordinates[i][1], 1), weights[i]), new(big.Rat).Set(weights[i])}
		}
	}
	return segments, weights, b.err
}

func stationProfileCentrallySymmetric(weights [9]*big.Rat, b *stationMassBudget) bool {
	for i := 0; i < 5; i++ {
		if b.mul(weights[i], weights[4]).Cmp(b.mul(weights[i+4], weights[0])) != 0 {
			return false
		}
	}
	return true
}

func stationProfileEnclosure(segments []stationMassSegment, includeMoments bool, b *stationMassBudget) (WrappedStationMassInterval, [2]WrappedStationMassInterval, error) {
	area := WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
	moments := [2]WrappedStationMassInterval{{new(big.Rat), new(big.Rat)}, {new(big.Rat), new(big.Rat)}}
	for _, segment := range segments {
		p, lower, gap, err := stationProfileTriangle(segment, b)
		if err != nil {
			return area, moments, err
		}
		area.Lower = b.add(area.Lower, lower)
		area.Upper = b.add(area.Upper, b.add(lower, gap))
		if !includeMoments {
			continue
		}
		for axis := 0; axis < 2; axis++ {
			inner := b.div(b.mul(lower, b.add(p[0][axis], p[2][axis])), big.NewRat(3, 1))
			lo, hi := new(big.Rat), new(big.Rat)
			for _, v := range p {
				if v[axis].Cmp(lo) < 0 {
					lo = v[axis]
				}
				if v[axis].Cmp(hi) > 0 {
					hi = v[axis]
				}
			}
			moments[axis].Lower = b.add(moments[axis].Lower, b.add(inner, b.mul(lo, gap)))
			moments[axis].Upper = b.add(moments[axis].Upper, b.add(inner, b.mul(hi, gap)))
		}
		if b.err != nil {
			return area, moments, b.err
		}
	}
	return area, moments, b.err
}

func stationProfileTriangle(segment stationMassSegment, b *stationMassBudget) (p [3][2]*big.Rat, lower, gap *big.Rat, err error) {
	for i := 0; i < 3; i++ {
		p[i] = [2]*big.Rat{b.div(segment[i][0], segment[i][2]), b.div(segment[i][1], segment[i][2])}
	}
	x := [2]*big.Rat{b.sub(p[1][0], p[0][0]), b.sub(p[1][1], p[0][1])}
	y := [2]*big.Rat{b.sub(p[2][0], p[0][0]), b.sub(p[2][1], p[0][1])}
	chord, turn := b.cross(p[0], p[2]), b.cross(x, y)
	first, last := b.cross(p[0], p[1]), b.cross(p[1], p[2])
	if b.err != nil {
		return p, nil, nil, b.err
	}
	if chord.Sign() <= 0 || turn.Sign() <= 0 || first.Sign() <= 0 || last.Sign() <= 0 {
		return p, nil, nil, geom.ErrConvergence
	}
	lower, gap = b.div(chord, big.NewRat(2, 1)), b.div(turn, big.NewRat(2, 1))
	return p, lower, gap, b.err
}

func splitStationProfileSegments(segments []stationMassSegment, b *stationMassBudget) []stationMassSegment {
	next := make([]stationMassSegment, 0, len(segments)*2)
	for _, q := range segments {
		var a, c, m [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			a[axis] = b.div(b.add(q[0][axis], q[1][axis]), big.NewRat(2, 1))
			c[axis] = b.div(b.add(q[1][axis], q[2][axis]), big.NewRat(2, 1))
			m[axis] = b.div(b.add(a[axis], c[axis]), big.NewRat(2, 1))
		}
		next = append(next, stationMassSegment{q[0], a, m}, stationMassSegment{m, c, q[2]})
		if b.err != nil {
			return nil
		}
	}
	return next
}

func wrappedStationAxialMassFactors(proof WrappedStationLoftProof, b *stationMassBudget) (vFactor, cFactor, zFactor *big.Rat) {
	vFactor, cFactor, zFactor = new(big.Rat), new(big.Rat), new(big.Rat)
	for i := 0; i+1 < len(proof.Heights); i++ {
		z0, z1, r0, r1 := proof.Heights[i], proof.Heights[i+1], proof.Radii[i], proof.Radii[i+1]
		h := b.sub(z1, z0)
		a, c, d := b.mul(r0, r0), b.mul(r0, r1), b.mul(r1, r1)
		local := b.div(b.add(b.add(a, c), d), big.NewRat(3, 1))
		vFactor = b.add(vFactor, b.mul(h, local))
		cubic := b.add(b.add(b.mul(a, r0), b.mul(a, r1)), b.add(b.mul(d, r0), b.mul(d, r1)))
		cFactor = b.add(cFactor, b.div(b.mul(h, cubic), big.NewRat(4, 1)))
		delta := b.sub(r1, r0)
		tMoment := b.add(b.add(b.div(a, big.NewRat(2, 1)), b.div(b.mul(big.NewRat(2, 1), b.mul(r0, delta)), big.NewRat(3, 1))), b.div(b.mul(delta, delta), big.NewRat(4, 1)))
		zFactor = b.add(zFactor, b.mul(h, b.add(b.mul(z0, local), b.mul(h, tMoment))))
	}
	return vFactor, cFactor, zFactor
}
