//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRoundedCylinderSection describes the exact circular section at a
// represented height. Its radius is Major+sqrt(Radical), both terms exact
// rationals. Curve is provided when that radius is rational and its native
// construction is lossless. Faces identify the incident original owners.
type WrappedRoundedCylinderSection struct {
	Center         [2]*big.Rat
	Height         *big.Rat
	Major, Radical *big.Rat
	Curve          *geom.ExactSplineCurve
	Faces          []int
}

// SectionWrappedCylinderRimFillet extracts the horizontal circle without
// fitting the section of a toroidal face. An irrational section radius is
// retained algebraically and does not cause the section to fail.
func (s Solid) SectionWrappedCylinderRimFillet(ctx context.Context, height float64,
	o geom.SurfacePatchOptions) (WrappedRoundedCylinderSection, error) {
	var empty WrappedRoundedCylinderSection
	if !finite(height) {
		return empty, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return empty, e
	}
	z := ratInset(height)
	if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 {
		return empty, geom.ErrDomain
	}
	lo, hi := new(big.Rat).Add(p.LowerZ, p.Fillets[0]), new(big.Rat).Sub(p.UpperZ, p.Fillets[1])
	out := WrappedRoundedCylinderSection{Center: p.Center, Height: z, Major: new(big.Rat).Set(p.Radius), Radical: new(big.Rat)}
	wall := s.faces[p.Lateral].Surface.(*geom.WrappedSurfacePatch)
	if z.Cmp(lo) >= 0 && z.Cmp(hi) <= 0 {
		q, e := geom.SectionWrappedFrustum(ctx, wall, height, o)
		if e != nil {
			return empty, e
		}
		out.Curve = q.Curve
		out.Faces = []int{p.Lateral}
		for i, limit := range []*big.Rat{lo, hi} {
			if z.Cmp(limit) == 0 {
				f := p.Blends[i]
				if f < 0 {
					f = p.Caps[i]
				}
				out.Faces = appendUniqueFace(out.Faces, f)
			}
		}
		return out, nil
	}
	i, center := 0, lo
	if z.Cmp(hi) > 0 {
		i, center = 1, hi
	}
	r := p.Fillets[i]
	out.Major.Sub(out.Major, r)
	d := new(big.Rat).Sub(z, center)
	out.Radical.Sub(new(big.Rat).Mul(r, r), d.Mul(d, d))
	out.Faces = []int{p.Blends[i]}
	if z.Cmp([2]*big.Rat{p.LowerZ, p.UpperZ}[i]) == 0 {
		out.Faces = append(out.Faces, p.Caps[i])
	}
	root, e := exactRationalSquareRoot(out.Radical)
	if e != nil {
		return out, nil
	}
	radius := new(big.Rat).Add(out.Major, root)
	w, _, e := geom.DraftWrappedFrustumExact(ctx, wall, new(big.Rat).Sub(radius, p.Radius), new(big.Rat).Sub(radius, p.Radius), o)
	if e != nil {
		return empty, e
	}
	// The stock's exact circumferential basis is already proved; reheight
	// its constant-radius support to include the requested section.
	h := new(big.Rat).Add(z, big.NewRat(1, 1))
	w, e = geom.ReheightWrappedCylinder(ctx, w, z, h, o)
	if e != nil {
		return empty, e
	}
	q, e := geom.SectionWrappedFrustum(ctx, w, height, o)
	if e != nil {
		return empty, e
	}
	out.Curve = q.Curve
	return out, nil
}
