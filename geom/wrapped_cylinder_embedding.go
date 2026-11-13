package geom

import (
	"context"
	"math/big"
)

// WrappedCylinderEmbedding proves that a canonical wrapped tensor patch
// is the outward lateral boundary of an extruded star-shaped rational disk.
// QuarterParameters are in the original circumferential parameter units.
type WrappedCylinderEmbedding struct {
	CenterX, CenterY, Radius, LowerZ, UpperZ *big.Rat
	QuarterParameters                        [5]float64
	Circular, Complete                       bool
	ExactOperations                          int
}

// CertifyWrappedCylinderEmbedding requires U to be wrapped and V unwrapped,
// with four canonical rational quadratic U quadrants and one affine V span.
// The Z-axis cylinder has XY controls centered and scaled from the ordered
// square quadrant polygon. The same positive U weights occur at both ends
// up to a single common scale; V weights must in fact be equal for this
// affine-height certificate. Increasing polar angle in each quadrant gives
// a simple star-shaped profile and a nonzero circumferential derivative.
// Strictly increasing Z gives a regular outward lateral chart. The two
// planar disks of this profile cap the chart without any other contacts.
func CertifyWrappedCylinderEmbedding(ctx context.Context, w *WrappedSurfacePatch, o SurfacePatchOptions) (r WrappedCylinderEmbedding, err error) {
	p, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	r.ExactOperations = p.ExactOperations
	if e != nil {
		return r, e
	}
	if p.LowerRadius.Cmp(p.UpperRadius) != 0 {
		return r, ErrConvergence
	}
	return WrappedCylinderEmbedding{
		CenterX: p.CenterX, CenterY: p.CenterY, Radius: p.LowerRadius, LowerZ: p.LowerZ, UpperZ: p.UpperZ,
		QuarterParameters: p.QuarterParameters, Circular: p.Circular, Complete: true, ExactOperations: p.ExactOperations,
	}, nil
}

// WrappedFrustumEmbedding proves a capped canonical rational draft profile.
// Radius changes affinely with height and stays strictly positive.
type WrappedFrustumEmbedding struct {
	CenterX, CenterY, LowerZ, UpperZ, LowerRadius, UpperRadius *big.Rat
	QuarterParameters                                          [5]float64
	Circular, Complete                                         bool
	ExactOperations                                            int
}

// CertifyWrappedFrustumEmbedding proves the lateral chart of a homothetic
// rational profile with two positive endpoint scales. Every model height
// has a simple star-shaped cross section centered on the same axis, and
// its height coordinate distinguishes different cross sections.
func CertifyWrappedFrustumEmbedding(ctx context.Context, w *WrappedSurfacePatch, o SurfacePatchOptions) (r WrappedFrustumEmbedding, err error) {
	o, e := patchOptions(o)
	if ctx == nil || e != nil || w == nil || w.patch == nil {
		return r, ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return r, e
	}
	p := w.patch
	if w.seam.PositionMatched != ([2]bool{true, false}) || p.countU != 9 || p.countV != 2 ||
		p.degreeU != 2 || p.degreeV != 1 || len(p.net) > o.MaxPoles {
		return r, ErrConvergence
	}
	k := p.knotsU
	if len(k) != 12 || k[0] != k[1] || k[1] != k[2] || k[3] != k[4] ||
		k[5] != k[6] || k[7] != k[8] || k[9] != k[10] || k[10] != k[11] {
		return r, ErrConvergence
	}
	cuts := [5]float64{k[2], k[3], k[5], k[7], k[9]}
	for i := range 4 {
		if cuts[i] >= cuts[i+1] {
			return r, ErrConvergence
		}
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	at := func(i, j, axis int) *big.Rat {
		h := p.net[2*i+j]
		return new(big.Rat).Quo(h[axis], h[3])
	}
	cx := new(big.Rat).Quo(new(big.Rat).Add(at(0, 0, 0), at(4, 0, 0)), big.NewRat(2, 1))
	cy := at(0, 0, 1)
	radii := [2]*big.Rat{new(big.Rat).Sub(at(0, 0, 0), cx), new(big.Rat).Sub(at(0, 1, 0), cx)}
	z := [2]*big.Rat{at(0, 0, 2), at(0, 1, 2)}
	for _, v := range []*big.Rat{cx, cy, radii[0], radii[1], z[0], z[1]} {
		if e = b.check(v); e != nil {
			return r, e
		}
	}
	if radii[0].Sign() <= 0 || radii[1].Sign() <= 0 || z[0].Cmp(z[1]) >= 0 {
		return r, ErrConvergence
	}
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	circular := true
	for i, xy := range q {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		if p.net[2*i][3].Cmp(p.net[2*i+1][3]) != 0 {
			return r, ErrConvergence
		}
		for j := range 2 {
			x := new(big.Rat).Add(cx, new(big.Rat).Mul(radii[j], big.NewRat(xy[0], 1)))
			y := new(big.Rat).Add(cy, new(big.Rat).Mul(radii[j], big.NewRat(xy[1], 1)))
			h := p.net[2*i+j]
			for axis, c := range [3]*big.Rat{x, y, z[j]} {
				v := new(big.Rat).Mul(c, h[3])
				if e = b.check(v); e != nil {
					return r, e
				}
				if v.Cmp(h[axis]) != 0 {
					return r, ErrConvergence
				}
			}
		}
		if i%2 == 0 && i < 8 {
			a, c, d := p.net[2*i][3], p.net[2*i+2][3], p.net[2*i+4][3]
			left := new(big.Rat).Mul(a, d)
			right := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(c, c))
			for _, w := range []*big.Rat{left, right} {
				if e = b.check(w); e != nil {
					return r, e
				}
			}
			circular = circular && left.Cmp(right) == 0
		}
	}
	r.CenterX, r.CenterY, r.LowerZ, r.UpperZ = cx, cy, z[0], z[1]
	r.LowerRadius, r.UpperRadius = radii[0], radii[1]
	r.QuarterParameters, r.Circular, r.Complete = cuts, circular, true
	return r, nil
}
