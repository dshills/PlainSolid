package geom

import (
	"context"
	"math/big"
)

type CylinderCapSectionKind uint8

const (
	CylinderCapEllipse CylinderCapSectionKind = iota
	CylinderCapStrip
	CylinderCapTangentLine
	CylinderCapEmpty
)

// CylinderCapRegion describes the exact planar common of one selected cap disk
// and the other finite cylinder. Coordinates (x,y) map to Origin+x*U+y*V;
// the rational basis is not normalized. Quadratics are ordered x²,xy,y²,x,y,1.
// All four constraints must be <=0: disk, opposite radial, opposite min/max.
// Kind describes the opposite infinite cylinder's section, not the clipped
// set's connectivity. All returned rationals belong to the caller.
type CylinderCapRegion struct {
	Complete             bool
	Kind                 CylinderCapSectionKind
	Origin, U, V         [3]*big.Rat
	Disk, OppositeRadial [6]*big.Rat
	OppositeCaps         [2][3]*big.Rat // x,y,constant
	EllipseCenter        [2]*big.Rat    // present only for ellipse
	ExactOperations      int
}

// DescribeCylinderCapRegion selects A's Min or Max cap. It produces a complete
// exact constraint representation, including cap restrictions of B, without
// claiming to enumerate the circle/conic arrangement or its connected regions.
func DescribeCylinderCapRegion(ctx context.Context, a, b RawCylinder, maxCap bool, o SurfacePatchEvaluationOptions) (out CylinderCapRegion, err error) {
	s, e := NewCylinderPairCommon(a, b)
	if e != nil {
		return out, e
	}
	if ctx == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil {
			out.Complete = false
		}
	}()
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	dot := func(a, b [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := 0; i < 3; i++ {
			q.Add(q, mul(a[i], b[i]))
		}
		return q
	}
	cross := func(a, b [3]*big.Rat) (q [3]*big.Rat) {
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			q[i] = sub(mul(a[j], b[k]), mul(a[k], b[j]))
		}
		return
	}
	A, B := s.s[0], s.s[1]
	cap := A.min
	if maxCap {
		cap = A.max
	}
	for i := 0; i < 3; i++ {
		out.Origin[i] = add(A.origin[i], mul(cap, A.axis[i]))
	}
	seed := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	k := 0
	if A.axis[1].Sign() == 0 && A.axis[2].Sign() == 0 {
		k = 1
	}
	seed[k].SetInt64(1)
	out.U = cross(A.axis, seed)
	out.V = cross(A.axis, out.U)
	for i := 0; i < 6; i++ {
		out.Disk[i] = new(big.Rat)
	}
	out.Disk[0] = dot(out.U, out.U)
	out.Disk[1] = mul(big.NewRat(2, 1), dot(out.U, out.V))
	out.Disk[2] = dot(out.V, out.V)
	out.Disk[5].Neg(A.radius2)
	delta := [3]*big.Rat{}
	for i := 0; i < 3; i++ {
		delta[i] = sub(out.Origin[i], B.origin[i])
	}
	q := [3]*big.Rat{dot(out.U, B.axis), dot(out.V, B.axis), dot(delta, B.axis)}
	out.OppositeRadial = [6]*big.Rat{sub(mul(B.norm2, dot(out.U, out.U)), mul(q[0], q[0])), mul(big.NewRat(2, 1), sub(mul(B.norm2, dot(out.U, out.V)), mul(q[0], q[1]))), sub(mul(B.norm2, dot(out.V, out.V)), mul(q[1], q[1])), mul(big.NewRat(2, 1), sub(mul(B.norm2, dot(delta, out.U)), mul(q[2], q[0]))), mul(big.NewRat(2, 1), sub(mul(B.norm2, dot(delta, out.V)), mul(q[2], q[1]))), sub(mul(B.norm2, sub(dot(delta, delta), B.radius2)), mul(q[2], q[2]))}
	out.OppositeCaps = [2][3]*big.Rat{{new(big.Rat).Neg(q[0]), new(big.Rat).Neg(q[1]), sub(mul(B.min, B.norm2), q[2])}, {new(big.Rat).Set(q[0]), new(big.Rat).Set(q[1]), sub(q[2], mul(B.max, B.norm2))}}
	f := out.OppositeRadial
	det := sub(mul(big.NewRat(4, 1), mul(f[0], f[2])), mul(f[1], f[1]))
	if det.Sign() > 0 {
		out.Kind = CylinderCapEllipse
		out.EllipseCenter = [2]*big.Rat{new(big.Rat).Quo(sub(mul(f[1], f[4]), mul(big.NewRat(2, 1), mul(f[2], f[3]))), det), new(big.Rat).Quo(sub(mul(f[1], f[3]), mul(big.NewRat(2, 1), mul(f[0], f[4]))), det)}
	} else {
		idx, lin := 0, 3
		if f[0].Sign() == 0 {
			idx, lin = 2, 4
		}
		minimum := sub(f[5], new(big.Rat).Quo(mul(f[lin], f[lin]), mul(big.NewRat(4, 1), f[idx])))
		switch minimum.Sign() {
		case -1:
			out.Kind = CylinderCapStrip
		case 0:
			out.Kind = CylinderCapTangentLine
		case 1:
			out.Kind = CylinderCapEmpty
		}
	}
	for _, vec := range [][3]*big.Rat{out.Origin, out.U, out.V, out.OppositeCaps[0], out.OppositeCaps[1]} {
		for _, r := range vec {
			if e := budget.check(r); e != nil {
				return out, e
			}
		}
	}
	for _, poly := range [][6]*big.Rat{out.Disk, out.OppositeRadial} {
		for _, r := range poly {
			if e := budget.check(r); e != nil {
				return out, e
			}
		}
	}
	for _, r := range out.EllipseCenter {
		if r != nil {
			if e := budget.check(r); e != nil {
				return out, e
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	return out, nil
}
