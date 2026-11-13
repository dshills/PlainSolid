package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedCylinderFilletSplitHistory maps result faces to original face
// indices. The two newly cut planar faces have ancestry -1.
type WrappedCylinderFilletSplitHistory struct {
	LowerSource, UpperSource []int
	LowerCut, UpperCut       int
}

// SplitWrappedCylinderRimFilletAtZ cuts a rounded cylinder in the open
// interior of its retained cylindrical slab. Each result keeps the
// corresponding original outer cap and exact rolling-circle blend.
// A cut through a toroidal band is outside this constructor.
func (s Solid) SplitWrappedCylinderRimFilletAtZ(ctx context.Context, z float64,
	o geom.SurfacePatchOptions) (Solid, Solid, WrappedCylinderFilletSplitHistory, error) {
	bad := WrappedCylinderFilletSplitHistory{LowerCut: -1, UpperCut: -1}
	if ctx == nil || !finite(z) {
		return Solid{}, Solid{}, bad, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return Solid{}, Solid{}, bad, e
	}
	t := ratInset(z)
	lo, hi := new(big.Rat).Add(p.LowerZ, p.Fillets[0]), new(big.Rat).Sub(p.UpperZ, p.Fillets[1])
	if t.Cmp(lo) <= 0 || t.Cmp(hi) >= 0 {
		return Solid{}, Solid{}, bad, geom.ErrDomain
	}
	w := s.faces[p.Lateral].Surface.(*geom.WrappedSurfacePatch)
	var out [2]Solid
	h := WrappedCylinderFilletSplitHistory{LowerCut: -1, UpperCut: -1}
	for i, span := range [2][2]*big.Rat{{p.LowerZ, t}, {t, p.UpperZ}} {
		if e := ctx.Err(); e != nil {
			return Solid{}, Solid{}, bad, e
		}
		band, e := geom.ReheightWrappedCylinder(ctx, w, span[0], span[1], o)
		if e != nil {
			return Solid{}, Solid{}, bad, e
		}
		q, _, e := NewWrappedCylinder(ctx, band, s.tolerance, o)
		if e != nil {
			return Solid{}, Solid{}, bad, e
		}
		j := WrappedCylinderRimFilletHistory{Lateral: 0, Caps: [2]int{1, 2}, Blends: [2]int{-1, -1}}
		if p.Fillets[i].Sign() > 0 {
			var radius [2]float64
			radius[i], e = geom.RepresentedExactScalar(p.Fillets[i])
			if e != nil {
				return Solid{}, Solid{}, bad, e
			}
			q, j, e = FilletWrappedCylinderRims(ctx, q, radius[0], radius[1], o)
			if e != nil {
				return Solid{}, Solid{}, bad, e
			}
		}
		ancestry := make([]int, len(q.faces))
		for k := range ancestry {
			ancestry[k] = -1
		}
		ancestry[j.Lateral], ancestry[j.Caps[i]] = p.Lateral, p.Caps[i]
		if j.Blends[i] >= 0 {
			ancestry[j.Blends[i]] = p.Blends[i]
		}
		if i == 0 {
			h.LowerSource, h.LowerCut = ancestry, j.Caps[1]
		} else {
			h.UpperSource, h.UpperCut = ancestry, j.Caps[0]
		}
		out[i] = q
	}
	return out[0], out[1], h, nil
}
