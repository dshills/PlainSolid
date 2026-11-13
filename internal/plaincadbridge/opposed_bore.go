package plaincadbridge

import (
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// sourceCylinders retains original tools when an exact union replaces opposing
// blind tools by a through tool. Other algorithms use canonical cylinders.
func (b *boreBody) sourceCylinders() []*cylinderBody {
	if b.inputCylinders != nil {
		return b.inputCylinders
	}
	return b.cylinders
}

// mergeOpposedBlindBores proves a deliberately narrow regular union in the
// world coordinate chart. Equality-only floor contact is not accepted.
func mergeOpposedBlindBores(base brep.Solid, tools []*cylinderBody) ([]*cylinderBody, bool) {
	if len(tools) < 2 {
		return nil, false
	}
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	bounds, ok := orthogonalPrism(base, f)
	if !ok {
		return nil, false
	}
	lo := [3]float64{bounds.Min.X, bounds.Min.Y, bounds.Min.Z}
	hi := [3]float64{bounds.Max.X, bounds.Max.Y, bounds.Max.Z}
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	for i, a := range tools {
		for j := i + 1; j < len(tools); j++ {
			b := tools[j]
			if a.radius != b.radius || a.frame.ZAxis() != b.frame.ZAxis().Scale(-1) {
				continue
			}
			axis, sign := -1, 0
			for k, n := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
				if a.frame.ZAxis() == n {
					axis, sign = k, 1
				}
				if a.frame.ZAxis() == n.Scale(-1) {
					axis, sign = k, -1
				}
			}
			if axis < 0 {
				continue
			}
			ap, bp := a.frame.Origin(), b.frame.Origin()
			aa := [3]float64{ap.X, ap.Y, ap.Z}
			bb := [3]float64{bp.X, bp.Y, bp.Z}
			same := true
			for k := 0; k < 3; k++ {
				if k != axis && aa[k] != bb[k] {
					same = false
				}
			}
			if !same {
				continue
			}
			ca, e := geom.ClassifyAxialBlindBoreContact(bounds, ap, a.radius, a.height, axis, sign)
			if e != nil || ca.Kind != geom.BlindBoreStrictInterior {
				continue
			}
			cb, e := geom.ClassifyAxialBlindBoreContact(bounds, bp, b.radius, b.height, axis, -sign)
			if e != nil || cb.Kind != geom.BlindBoreStrictInterior {
				continue
			}
			// Native constructors store normalized frames. Demand exact represented
			// coordinate radial axes, not merely numerically close orthonormal vectors.
			cardinal := func(v geom.Vec3) bool {
				return v == (geom.Vec3{X: 1}) || v == (geom.Vec3{X: -1}) || v == (geom.Vec3{Y: 1}) || v == (geom.Vec3{Y: -1}) || v == (geom.Vec3{Z: 1}) || v == (geom.Vec3{Z: -1})
			}
			if !cardinal(a.frame.XAxis()) || !cardinal(a.frame.YAxis()) || !cardinal(b.frame.XAxis()) || !cardinal(b.frame.YAxis()) {
				continue
			}
			thickness := new(big.Rat).Sub(rat(hi[axis]), rat(lo[axis]))
			if new(big.Rat).Add(rat(a.height), rat(b.height)).Cmp(thickness) <= 0 {
				continue
			}
			h, _ := thickness.Float64()
			if !finite(h) || rat(h).Cmp(thickness) != 0 {
				continue
			}
			c, e := newCylinder(a.frame, a.radius, h)
			if e != nil {
				continue
			}
			if c.frame.Origin().Add(c.frame.ZAxis().Scale(h)) != bp {
				continue
			}
			out := append([]*cylinderBody(nil), tools...)
			out[i] = c
			out = append(out[:j], out[j+1:]...)
			return out, true
		}
	}
	return nil, false
}
