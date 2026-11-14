package step

import (
	"io"
	"math/big"
	"plainsolid/geom"
	"strconv"
	"strings"
)

type mappedRationalBasis struct{ Origin, X, Y, Z [3]*big.Rat }

func mappedRationalDot(a, b [3]*big.Rat) *big.Rat {
	s := new(big.Rat)
	for i := range a {
		s.Add(s, new(big.Rat).Mul(a[i], b[i]))
	}
	return s
}
func mappedRationalCross(a, b [3]*big.Rat) (c [3]*big.Rat) {
	for i := range c {
		j, k := (i+1)%3, (i+2)%3
		c[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	return
}
func mappedRationalUnit(a [3]*big.Rat) ([3]*big.Rat, error) {
	n := mappedRationalDot(a, a)
	if n.Sign() <= 0 {
		return a, mappedPlanarFailure()
	}
	p, q := new(big.Int).Sqrt(n.Num()), new(big.Int).Sqrt(n.Denom())
	if new(big.Int).Mul(p, p).Cmp(n.Num()) != 0 || new(big.Int).Mul(q, q).Cmp(n.Denom()) != 0 {
		return a, mappedPlanarFailure()
	}
	norm := new(big.Rat).SetFrac(p, q)
	for i := range a {
		a[i] = new(big.Rat).Quo(a[i], norm)
	}
	return a, nil
}
func mappedRationalCoordinate(f *File, id int64, kind string, dim int) (v [3]*big.Rat, e error) {
	r, e := simple(f, id, kind, 2)
	if e != nil {
		return v, e
	}
	if r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != dim {
		return v, mappedPlanarFailure()
	}
	for i := range v {
		v[i] = new(big.Rat)
	}
	for i, x := range r.Args[1].Items {
		if x.Kind != Number || len(x.Text) > 128 {
			return v, mappedPlanarFailure()
		}
		if at := strings.IndexAny(x.Text, "Ee"); at >= 0 {
			ex, err := strconv.Atoi(x.Text[at+1:])
			if err != nil || ex < -1024 || ex > 1024 {
				return v, mappedPlanarFailure()
			}
		}
		q, ok := new(big.Rat).SetString(x.Text)
		if !ok || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
			return v, mappedPlanarFailure()
		}
		v[i] = q
	}
	return
}
func mappedRationalFrame(f *File, id int64) (b mappedRationalBasis, e error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return b, e
	}
	b.Origin, e = mappedRationalCoordinate(f, r.Args[1].Ref, "CARTESIAN_POINT", 3)
	if e != nil {
		return b, e
	}
	b.X, e = mappedRationalCoordinate(f, r.Args[3].Ref, "DIRECTION", 3)
	if e != nil {
		return b, e
	}
	b.Z, e = mappedRationalCoordinate(f, r.Args[2].Ref, "DIRECTION", 3)
	if e != nil {
		return b, e
	}
	if mappedRationalDot(b.X, b.Z).Sign() != 0 {
		return b, mappedPlanarFailure()
	}
	b.X, e = mappedRationalUnit(b.X)
	if e != nil {
		return b, e
	}
	b.Z, e = mappedRationalUnit(b.Z)
	if e != nil {
		return b, e
	}
	b.Y = mappedRationalCross(b.Z, b.X)
	return
}
func mappedRationalLine(f *File, id int64, dim int, b *mappedRationalBasis, points [2]geom.Vec3) error {
	r, e := simple(f, id, "LINE", 3)
	if e != nil {
		return e
	}
	p, e := mappedRationalCoordinate(f, r.Args[1].Ref, "CARTESIAN_POINT", dim)
	if e != nil {
		return e
	}
	v, e := simple(f, r.Args[2].Ref, "VECTOR", 3)
	if e != nil {
		return e
	}
	m, e := cylinderSupportNumber(v.Args[2])
	if e != nil || m <= 0 {
		return mappedPlanarFailure()
	}
	d, e := mappedRationalCoordinate(f, v.Args[1].Ref, "DIRECTION", dim)
	if e != nil {
		return e
	}
	if mappedRationalDot(d, d).Sign() == 0 {
		return mappedPlanarFailure()
	}
	if b != nil {
		var q, h [3]*big.Rat
		for i := range q {
			q[i] = new(big.Rat).Add(b.Origin[i], new(big.Rat).Add(new(big.Rat).Mul(b.X[i], p[0]), new(big.Rat).Mul(b.Y[i], p[1])))
			h[i] = new(big.Rat).Add(new(big.Rat).Mul(b.X[i], d[0]), new(big.Rat).Mul(b.Y[i], d[1]))
		}
		p, d = q, h
	}
	for _, point := range points {
		q := mappedRatVec(point)
		for i := range q {
			q[i].Sub(q[i], p[i])
		}
		cross := mappedRationalCross(q, d)
		for _, x := range cross {
			if x.Sign() != 0 {
				return mappedPlanarFailure()
			}
		}
	}
	return nil
}

// ReadPythagoreanMappedPlanarMaterials accepts the mapped material topology with
// rationally normalizable orthogonal plane direction ratios. Model vertices
// remain exact binary64 points; plane/owner correspondence uses exact rationals.
// Occurrence operators and mapping origins retain the original cardinal domain.
func ReadPythagoreanMappedPlanarMaterials(r io.Reader, l Limits) (MappedPlanarMaterials, error) {
	return readMappedPlanarMaterials(r, l, true)
}
