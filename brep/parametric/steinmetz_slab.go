package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SteinmetzSlab is the cylinder common restricted to lower≤X-center.X≤upper.
// Roots retain exact cap crossing provenance, lower then upper with zero caps
// omitted. IdealVolumeBounds include all
// algebraic ordinate and represented-angle enclosure errors. Native charts
// remain analytically cylindrical/planar with explicit sewing tolerance.
type SteinmetzSlab struct {
	Solid                                                Solid
	XAxisFaces, ZAxisFaces, LowerCapFaces, UpperCapFaces []int
	Roots                                                []*geom.ExactAlgebraicRoot
	IdealVolumeBounds                                    [2]*big.Rat
	SeamSquaredBounds                                    []*big.Rat
	Bands                                                int
}

// PerpendicularCylinderSlab supports two strictly interior X caps and an
// unclipped Z cylinder. At most five common angular bands (twenty faces) are
// assembled. Root options apply independently to each of the two cap proofs;
// native angular discrepancy is retained in SeamSquaredBounds.
func PerpendicularCylinderSlab(ctx context.Context, center geom.Vec3, radius, lower, upper, zHalfLength, tolerance float64, o geom.ExactCurvePlaneEventOptions) (SteinmetzSlab, error) {
	fail := func(e error) (SteinmetzSlab, error) { return SteinmetzSlab{}, e }
	if ctx == nil || !center.IsFinite() || !finite(radius) || !finite(lower) || !finite(upper) || !finite(zHalfLength) || !finite(tolerance) || radius <= 0 || tolerance <= 0 || lower >= upper || lower <= -radius || upper >= radius {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	guard := new(big.Rat).Add(q(radius), pad)
	if q(zHalfLength).Cmp(guard) <= 0 || new(big.Rat).Sub(q(upper), q(lower)).Cmp(pad) <= 0 {
		return fail(geom.ErrDomain)
	}
	// Use a safely representable nonclipping opposite X cap solely while
	// obtaining the independently validated one-cap proof.
	far := radius * 2
	if !finite(far) || q(far).Cmp(guard) <= 0 {
		return fail(geom.ErrDomain)
	}
	full := new(big.Rat).Mul(big.NewRat(16, 3), new(big.Rat).Mul(new(big.Rat).Mul(q(radius), q(radius)), q(radius)))
	cuts := []float64{-math.Pi / 2, math.Pi / 2}
	seen := map[float64]bool{}
	var result SteinmetzSlab
	var cumulative [2][2]*big.Rat
	for i, c := range []float64{lower, upper} {
		if c == 0 {
			half := new(big.Rat).Quo(full, big.NewRat(2, 1))
			cumulative[i] = [2]*big.Rat{new(big.Rat).Set(half), new(big.Rat).Set(half)}
			continue
		}
		proof, e := PerpendicularCylinderAlgebraicCap(ctx, center, radius, c, far, zHalfLength, tolerance, o)
		if e != nil {
			return fail(e)
		}
		cumulative[i] = proof.IdealVolumeBounds
		result.Roots = append(result.Roots, proof.OrdinateRoot)
		result.SeamSquaredBounds = append(result.SeamSquaredBounds, new(big.Rat).Set(proof.SeamSquaredBound))
		if !seen[math.Abs(c)] {
			seen[math.Abs(c)] = true
			cuts = append(cuts, -proof.SplitAngle, proof.SplitAngle)
		}
	}
	sort.Float64s(cuts)
	var faces []Face
	for i := 0; i+1 < len(cuts); i++ {
		u := Interval{cuts[i], cuts[i+1]}
		if !u.valid() {
			return fail(geom.ErrConvergence)
		}
		alo, ahi := steinmetzCosBounds(radius, (u.Min+u.Max)/2)
		active := true
		for _, threshold := range []float64{lower, -upper} {
			if threshold <= 0 {
				continue
			}
			if ahi.Cmp(q(threshold)) < 0 {
				active = false
			} else if alo.Cmp(q(threshold)) <= 0 {
				return fail(geom.ErrConvergence)
			}
		}
		if !active {
			continue
		}
		lowFixed, highFixed := lower >= 0, upper <= 0
		if lower < 0 {
			if alo.Cmp(q(-lower)) > 0 {
				lowFixed = true
			} else if ahi.Cmp(q(-lower)) >= 0 {
				return fail(geom.ErrConvergence)
			}
		}
		if upper > 0 {
			if alo.Cmp(q(upper)) > 0 {
				highFixed = true
			} else if ahi.Cmp(q(upper)) >= 0 {
				return fail(geom.ErrConvergence)
			}
		}
		for kind := 0; kind < 4; kind++ {
			id := len(faces)
			switch kind {
			case 0, 1:
				result.XAxisFaces = append(result.XAxisFaces, id)
			case 2:
				if lowFixed {
					result.LowerCapFaces = append(result.LowerCapFaces, id)
				} else {
					result.ZAxisFaces = append(result.ZAxisFaces, id)
				}
			case 3:
				if highFixed {
					result.UpperCapFaces = append(result.UpperCapFaces, id)
				} else {
					result.ZAxisFaces = append(result.ZAxisFaces, id)
				}
			}
			faces = append(faces, Face{Surface: steinmetzSlabChart{center, radius, lower, upper, lowFixed, highFixed, kind}, U: u, V: Interval{0, 1}, Reversed: kind == 0 || kind == 2})
		}
		result.Bands++
	}
	if len(faces) == 0 {
		return fail(geom.ErrConvergence)
	}
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	result.Solid = solid
	result.IdealVolumeBounds = [2]*big.Rat{new(big.Rat).Sub(cumulative[1][0], cumulative[0][1]), new(big.Rat).Sub(cumulative[1][1], cumulative[0][0])}
	if result.IdealVolumeBounds[0].Sign() <= 0 {
		return fail(geom.ErrConvergence)
	}
	return result, nil
}

// Exact degree48 cosine enclosure for |u|<2, scaled by represented radius.
func steinmetzCosBounds(radius, u float64) (*big.Rat, *big.Rat) {
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sum, term := new(big.Rat), big.NewRat(1, 1)
	for k := 0; k <= 48; k++ {
		if k%4 == 0 {
			sum.Add(sum, term)
		} else if k%4 == 2 {
			sum.Sub(sum, term)
		}
		term.Quo(new(big.Rat).Mul(term, q(u)), big.NewRat(int64(k+1), 1))
	}
	mid := new(big.Rat).Mul(q(radius), sum)
	err := new(big.Rat).Mul(q(radius), new(big.Rat).Abs(term))
	return new(big.Rat).Sub(mid, err), new(big.Rat).Add(mid, err)
}

type steinmetzSlabChart struct {
	center               geom.Vec3
	radius, lower, upper float64
	lowFixed, highFixed  bool
	kind                 int
}

func (s steinmetzSlabChart) valid() bool {
	return s.center.IsFinite() && finite(s.radius) && finite(s.lower) && finite(s.upper) && s.radius > 0 && s.lower > s.radius*-1 && s.upper < s.radius && s.lower < s.upper && s.kind >= 0 && s.kind < 4
}
func (s steinmetzSlabChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s steinmetzSlabChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s steinmetzSlabChart) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !s.valid() || !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	sin, cos := math.Sincos(u)
	if u == -math.Pi/2 || u == math.Pi/2 {
		cos = 0
	}
	a, da := s.radius*cos, -s.radius*sin
	lo, hi, dlo, dhi := -a, a, -da, da
	if s.lowFixed {
		lo, dlo = s.lower, 0
	}
	if s.highFixed {
		hi, dhi = s.upper, 0
	}
	p := geom.Vec3{X: lo + (hi-lo)*v, Y: s.radius * sin, Z: a}
	du := geom.Vec3{X: dlo + (dhi-dlo)*v, Y: a, Z: da}
	dv := geom.Vec3{X: hi - lo}
	if s.kind == 1 {
		p.Z, du.Z = -a, -da
	}
	if s.kind >= 2 {
		p.X, du.X = lo, dlo
		if s.kind == 3 {
			p.X, du.X = hi, dhi
		}
		p.Z, du.Z = (2*v-1)*a, (2*v-1)*da
		dv = geom.Vec3{Z: 2 * a}
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return p, du, dv, nil
}
func (s steinmetzSlabChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	b := math.Nextafter(2*s.radius, math.Inf(1))
	if !finite(b) {
		return 0, 0, 0, geom.ErrInvalid
	}
	return b, b, 0, nil
}

type steinmetzSlabData struct {
	Lower, Upper        float64
	LowFixed, HighFixed bool
	Kind                int
}
