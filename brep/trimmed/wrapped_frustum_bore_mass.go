package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedFrustumBoreAreaTerm represents one original boundary face's area as
// π × Coefficient × sqrt(Radicand). Disk and annular cap radicands equal one.
// Face uses native receiver indices or the canonical original construction.
type WrappedFrustumBoreAreaTerm struct {
	Face                  int
	Coefficient, Radicand *big.Rat
}

// WrappedFrustumBoreMass gives exact unit-density mass properties. Volume is
// π × VolumePi. FirstMomentPi and SecondMomentPi are the integrals of x_i
// and x_i*x_j divided by π in original model coordinates. Centroid is exact;
// InertiaAtCentroidPi is the symmetric centroidal inertia tensor divided by π.
// Area consists of one nonnegative exact radical term for each original face.
type WrappedFrustumBoreMass struct {
	VolumePi                            *big.Rat
	Centroid, FirstMomentPi             [3]*big.Rat
	SecondMomentPi, InertiaAtCentroidPi [3][3]*big.Rat
	Area                                []WrappedFrustumBoreAreaTerm
}

// MeasureWrappedEccentricFrustumBore evaluates exact section integrals of two
// independently proved strictly nested parallel circular frustums. Supports
// through, blind and fully enclosed bores. No boundary mesh is used.
func MeasureWrappedEccentricFrustumBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	o geom.SurfacePatchOptions) (WrappedFrustumBoreMass, error) {
	var empty WrappedFrustumBoreMass
	a, e := geom.CertifyWrappedFrustumEmbedding(ctx, outer, o)
	if e != nil {
		return empty, e
	}
	b, e := geom.CertifyWrappedFrustumEmbedding(ctx, inner, o)
	if e != nil {
		return empty, e
	}
	if !strictWrappedParallelFrustumBore(a, b, false) {
		return empty, geom.ErrConvergence
	}
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	quo := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	integer := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	convolution := func(a, b []*big.Rat) []*big.Rat {
		p := make([]*big.Rat, len(a)+len(b)-1)
		for i := range p {
			p[i] = new(big.Rat)
		}
		for i, x := range a {
			for j, y := range b {
				p[i+j].Add(p[i+j], mul(x, y))
			}
		}
		return p
	}
	type moments struct {
		volume *big.Rat
		first  [3]*big.Rat
		second [3][3]*big.Rat
	}
	measure := func(p geom.WrappedFrustumEmbedding) moments {
		h := sub(p.UpperZ, p.LowerZ)
		r := []*big.Rat{p.LowerRadius, sub(p.UpperRadius, p.LowerRadius)}
		z := []*big.Rat{p.LowerZ, h}
		r2 := convolution(r, r)
		integral := func(q []*big.Rat) *big.Rat {
			x := new(big.Rat)
			for i, c := range q {
				x.Add(x, quo(c, integer(int64(i+1))))
			}
			return x.Mul(x, h)
		}
		v, mZ, mZZ := integral(r2), integral(convolution(r2, z)), integral(convolution(r2, convolution(z, z)))
		r4 := quo(integral(convolution(r2, r2)), integer(4))
		x, y := p.CenterX, p.CenterY
		xx, yy := add(mul(v, sq(x)), r4), add(mul(v, sq(y)), r4)
		xy, xz, yz := mul(v, mul(x, y)), mul(mZ, x), mul(mZ, y)
		return moments{v, [3]*big.Rat{mul(v, x), mul(v, y), mZ}, [3][3]*big.Rat{{xx, xy, xz}, {xy, yy, yz}, {xz, yz, mZZ}}}
	}
	aM, bM := measure(a), measure(b)
	r := WrappedFrustumBoreMass{VolumePi: sub(aM.volume, bM.volume)}
	if r.VolumePi.Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	for i := range 3 {
		r.FirstMomentPi[i] = sub(aM.first[i], bM.first[i])
		r.Centroid[i] = quo(r.FirstMomentPi[i], r.VolumePi)
		for j := range 3 {
			r.SecondMomentPi[i][j] = sub(aM.second[i][j], bM.second[i][j])
		}
	}
	central := func(i, j int) *big.Rat {
		return sub(r.SecondMomentPi[i][j], mul(r.FirstMomentPi[i], r.Centroid[j]))
	}
	for i := range 3 {
		for j := range 3 {
			if i == j {
				r.InertiaAtCentroidPi[i][j] = add(central((i+1)%3, (i+1)%3), central((i+2)%3, (i+2)%3))
			} else {
				x := central(i, j)
				r.InertiaAtCentroidPi[i][j] = x.Neg(x)
			}
		}
	}
	wallArea := func(i int, p geom.WrappedFrustumEmbedding) WrappedFrustumBoreAreaTerm {
		return WrappedFrustumBoreAreaTerm{i, add(p.LowerRadius, p.UpperRadius), add(sq(sub(p.UpperZ, p.LowerZ)), sq(sub(p.UpperRadius, p.LowerRadius)))}
	}
	r.Area = []WrappedFrustumBoreAreaTerm{wallArea(0, a), {Face: 1, Coefficient: sq(a.LowerRadius), Radicand: integer(1)},
		{Face: 2, Coefficient: sq(a.UpperRadius), Radicand: integer(1)}, wallArea(3, b)}
	ra := [2]*big.Rat{b.LowerRadius, b.UpperRadius}
	open := [2]bool{a.LowerZ.Cmp(b.LowerZ) == 0, a.UpperZ.Cmp(b.UpperZ) == 0}
	for i, rB := range ra {
		if open[i] {
			r.Area[i+1].Coefficient.Sub(r.Area[i+1].Coefficient, sq(rB))
		} else {
			r.Area = append(r.Area, WrappedFrustumBoreAreaTerm{len(r.Area), sq(rB), integer(1)})
		}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return r, nil
}

// MeasureCanonicalWrappedFrustumBore rechecks native material and maps each
// exact area expression to its receiver face, including reindexed solids.
func (s Solid) MeasureCanonicalWrappedFrustumBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedFrustumBoreMass, error) {
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return WrappedFrustumBoreMass{}, e
	}
	m, e := MeasureWrappedEccentricFrustumBore(ctx, p.Outer, p.Inner, o)
	if e != nil {
		return WrappedFrustumBoreMass{}, e
	}
	ids := canonicalFrustumBoreLineFaceIDs(p)
	for i := range m.Area {
		m.Area[i].Face = ids[m.Area[i].Face]
	}
	return m, nil
}
