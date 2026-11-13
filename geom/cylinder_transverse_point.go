package geom

import "math/big"

// CylinderSetPoint separates exposed regularized boundary from source incidence.
// Incidence may be nonzero at an interior union point or an excluded difference
// point when a source cap is canceled. Masks retain original operand Min/Max.
type CylinderSetPoint struct {
	Location            BorePointLocation
	Boundary, Incidence BoreBoundaryMask
}

// TransverseCylinderSetQuery is an immutable exact regularized set query for
// parallel cylinders with strictly transverse radial disk overlap. It does not
// certify a native B-rep; use a modeling constructor for that separate proof.
type TransverseCylinderSetQuery struct {
	pair       CylinderPairCommon
	difference bool
}

// NewTransverseCylinderSetQuery selects union, or regularized A minus B when
// difference is true. Radial tangencies and containment are not in this domain.
func NewTransverseCylinderSetQuery(a, b RawCylinder, difference bool) (TransverseCylinderSetQuery, error) {
	pair, e := NewCylinderPairCommon(a, b)
	if e != nil {
		return TransverseCylinderSetQuery{}, e
	}
	x, y := pair.s[0], pair.s[1]
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		if new(big.Rat).Mul(x.axis[j], y.axis[k]).Cmp(new(big.Rat).Mul(x.axis[k], y.axis[j])) != 0 {
			return TransverseCylinderSetQuery{}, ErrDomain
		}
	}
	dot, sq := new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		d := new(big.Rat).Sub(y.origin[i], x.origin[i])
		dot.Add(dot, new(big.Rat).Mul(d, x.axis[i]))
		sq.Add(sq, new(big.Rat).Mul(d, d))
	}
	d2 := new(big.Rat).Sub(sq, new(big.Rat).Quo(new(big.Rat).Mul(dot, dot), x.norm2))
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	sum := new(big.Rat).Add(q(a.Radius), q(b.Radius))
	diff := new(big.Rat).Sub(q(a.Radius), q(b.Radius))
	if d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0 || d2.Cmp(new(big.Rat).Mul(diff, diff)) <= 0 {
		return TransverseCylinderSetQuery{}, ErrDomain
	}
	return TransverseCylinderSetQuery{pair: pair, difference: difference}, nil
}

// Classify uses exact represented coordinates. It cancels opposed cap interiors
// for union and same-oriented cutter-covered cap points for regularized
// difference. Strict transverse side intersections remain boundary events.
func (s TransverseCylinderSetQuery) Classify(p Vec3) (CylinderSetPoint, error) {
	out := CylinderSetPoint{Location: BoreOutside}
	if s.pair.s[0].norm2 == nil || !p.IsFinite() {
		return out, ErrInvalid
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	v := [3]*big.Rat{q(p.X), q(p.Y), q(p.Z)}
	var signs [2][3]int
	for i, c := range s.pair.s {
		dot, sq := new(big.Rat), new(big.Rat)
		for j := 0; j < 3; j++ {
			d := new(big.Rat).Sub(v[j], c.origin[j])
			dot.Add(dot, new(big.Rat).Mul(d, c.axis[j]))
			sq.Add(sq, new(big.Rat).Mul(d, d))
		}
		rad := new(big.Rat).Sub(new(big.Rat).Mul(c.norm2, new(big.Rat).Sub(sq, c.radius2)), new(big.Rat).Mul(dot, dot))
		signs[i] = [3]int{rad.Sign(), new(big.Rat).Mul(c.min, c.norm2).Cmp(dot), dot.Cmp(new(big.Rat).Mul(c.max, c.norm2))}
	}
	return s.classifySigns(signs), nil
}

func (s TransverseCylinderSetQuery) classifySigns(signs [2][3]int) CylinderSetPoint {
	out := CylinderSetPoint{Location: BoreOutside}
	var masks [2]BoreBoundaryMask
	var state [2]BorePointLocation
	for i := 0; i < 2; i++ {
		state[i] = BoreInside
		for j, z := range signs[i] {
			if z > 0 {
				state[i] = BoreOutside
			}
			if z == 0 {
				masks[i] |= 1 << uint(3*i+j)
			}
		}
		if state[i] != BoreOutside && masks[i] != 0 {
			state[i] = BoreBoundary
			out.Incidence |= masks[i]
		}
	}
	// Active cap normals are exactly parallel; their dot sign determines which
	// axial halfspaces meet. All radial claims below are strict or transversal.
	capRelation := 0
	cap := [2]int{}
	for i := 0; i < 2; i++ {
		if signs[i][1] == 0 {
			cap[i] = -1
		} else if signs[i][2] == 0 {
			cap[i] = 1
		}
	}
	if cap[0] != 0 && cap[1] != 0 {
		dot := new(big.Rat)
		for j := 0; j < 3; j++ {
			dot.Add(dot, new(big.Rat).Mul(s.pair.s[0].axis[j], s.pair.s[1].axis[j]))
		}
		capRelation = dot.Sign() * cap[0] * cap[1]
	}
	if s.difference {
		if state[0] == BoreOutside || state[1] == BoreInside {
			return out
		}
		if state[0] == BoreInside && state[1] == BoreOutside {
			out.Location = BoreInside
			return out
		}
		if state[0] == BoreBoundary && state[1] == BoreBoundary && signs[1][0] < 0 && capRelation > 0 {
			return out
		}
		out.Location = BoreBoundary
		if state[0] == BoreBoundary {
			out.Boundary |= masks[0]
		}
		if state[1] == BoreBoundary && capRelation >= 0 {
			mask := masks[1]
			if capRelation > 0 {
				mask &^= RawCylinderSecondMin | RawCylinderSecondMax
			}
			out.Boundary |= mask
		}
		return out
	}
	if state[0] == BoreInside || state[1] == BoreInside || (state[0] == BoreBoundary && state[1] == BoreBoundary && signs[0][0] < 0 && signs[1][0] < 0 && capRelation < 0) {
		out.Location = BoreInside
		return out
	}
	if state[0] == BoreOutside && state[1] == BoreOutside {
		return out
	}
	out.Location = BoreBoundary
	out.Boundary = out.Incidence
	if capRelation > 0 {
		if signs[1][0] < 0 {
			out.Boundary &^= RawCylinderFirstSide
		}
		if signs[0][0] < 0 {
			out.Boundary &^= RawCylinderSecondSide
		}
	}
	if capRelation < 0 {
		if signs[1][0] < 0 {
			out.Boundary &^= RawCylinderFirstMin | RawCylinderFirstMax
		}
		if signs[0][0] < 0 {
			out.Boundary &^= RawCylinderSecondMin | RawCylinderSecondMax
		}
	}
	return out
}
