package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

// finiteBoreEnvelope is a conservative product of an axial interval and disk.
// It is recovered from validated native cylinder support, never caller metadata.
type finiteBoreEnvelope struct {
	axis            int
	center          geom.Vec3
	radius, padding float64
	box             geom.Box
	valid           bool
}

func finiteBoreSupport(surface geom.Surface, box geom.Box, padding float64) finiteBoreEnvelope {
	s, ok := surface.(geom.CylinderSurface)
	if !ok {
		return finiteBoreEnvelope{}
	}
	f, r := s.Parameters()
	axis := -1
	for i, v := range []float64{f.ZAxis().X, f.ZAxis().Y, f.ZAxis().Z} {
		if v != 0 {
			if axis >= 0 || (v != 1 && v != -1) {
				return finiteBoreEnvelope{}
			}
			axis = i
		}
	}
	if axis < 0 || !finite(r) || r <= 0 || !finite(padding) || padding < 0 || !box.Valid() {
		return finiteBoreEnvelope{}
	}
	return finiteBoreEnvelope{axis, f.Origin(), r, padding, box, true}
}

// finiteBoresSeparated proves strict separation without square roots. For
// perpendicular cylinders the surviving transverse intervals have halfwidths
// sqrt(rA²-dz²),sqrt(rB²-dx²). Their sum is compared by exact represented-rational
// arithmetic. Expanded axial intervals and radii make this a clearance proof.
func finiteBoresSeparated(a, b finiteBoreEnvelope) bool {
	if !a.valid || !b.valid {
		return false
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	add := func(x, y *big.Rat) *big.Rat { return new(big.Rat).Add(x, y) }
	sub := func(x, y *big.Rat) *big.Rat { return new(big.Rat).Sub(x, y) }
	mul := func(x, y *big.Rat) *big.Rat { return new(big.Rat).Mul(x, y) }
	sq := func(x *big.Rat) *big.Rat { return mul(x, x) }
	xyz := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	ac, bc := xyz(a.center), xyz(b.center)
	al, ah, bl, bh := xyz(a.box.Min), xyz(a.box.Max), xyz(b.box.Min), xyz(b.box.Max)
	ra, rb := add(q(a.radius), q(a.padding)), add(q(b.radius), q(b.padding))
	if a.axis == b.axis {
		if ah[a.axis] < bl[b.axis] || bh[b.axis] < al[a.axis] {
			return true
		}
		d2 := new(big.Rat)
		for i := range 3 {
			if i != a.axis {
				d2.Add(d2, sq(sub(q(ac[i]), q(bc[i]))))
			}
		}
		return d2.Cmp(sq(add(ra, rb))) > 0
	}
	distance := func(p, lo, hi float64) *big.Rat {
		if p < lo {
			return sub(q(lo), q(p))
		}
		if p > hi {
			return sub(q(p), q(hi))
		}
		return new(big.Rat)
	}
	da := distance(ac[b.axis], bl[b.axis], bh[b.axis])
	db := distance(bc[a.axis], al[a.axis], ah[a.axis])
	u, v := sub(sq(ra), sq(da)), sub(sq(rb), sq(db))
	if u.Sign() < 0 || v.Sign() < 0 {
		return true
	}
	shared := 3 - a.axis - b.axis
	d2 := sq(sub(q(ac[shared]), q(bc[shared])))
	residual := sub(d2, add(u, v))
	return residual.Sign() > 0 && sq(residual).Cmp(mul(big.NewRat(4, 1), mul(u, v))) > 0
}
