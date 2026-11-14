package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

type lensUVPolygon [][2]*big.Rat

// Make the convex intersection of both original UV trims in source units.
// The target half planes are pulled back through the exact chart map; their
// orientation in target units works even when that map reverses orientation.
func commonLensMassPolygon(faces [2]Face, sourceToTarget [2][3]*big.Rat, check func(*big.Rat) error) (lensUVPolygon, error) {
	for _, f := range faces {
		if len(f.Outer) > 128 || !convexStraightTubeLensFace(f) {
			return nil, geom.ErrConvergence
		}
	}
	polygon := make(lensUVPolygon, len(faces[0].Outer))
	for i, u := range faces[0].Outer {
		polygon[i] = [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
	}
	for _, u := range faces[1].Outer {
		x, y := ratInset(u.UVStart.X), ratInset(u.UVStart.Y)
		dx := new(big.Rat).Sub(ratInset(u.UVEnd.X), x)
		dy := new(big.Rat).Sub(ratInset(u.UVEnd.Y), y)
		var row [3]*big.Rat
		for j := range row {
			row[j] = new(big.Rat).Sub(new(big.Rat).Mul(dx, sourceToTarget[1][j]), new(big.Rat).Mul(dy, sourceToTarget[0][j]))
		}
		row[2].Add(row[2], new(big.Rat).Sub(new(big.Rat).Mul(dy, x), new(big.Rat).Mul(dx, y)))
		for _, v := range row {
			if e := check(v); e != nil {
				return nil, e
			}
		}
		var e error
		polygon, e = polygon.clip(row, check)
		if e != nil || len(polygon) < 3 {
			return polygon, e
		}
	}
	return polygon, nil
}

// clip keeps row[0]*u + row[1]*v + row[2] >= 0. Exact edge crossings
// keep a cell's integral valid even when its clipped corners are nonbinary.
func (polygon lensUVPolygon) clip(row [3]*big.Rat, check func(*big.Rat) error) (lensUVPolygon, error) {
	if len(polygon) == 0 {
		return polygon, nil
	}
	value := func(p [2]*big.Rat) *big.Rat {
		return new(big.Rat).Add(row[2], new(big.Rat).Add(new(big.Rat).Mul(row[0], p[0]), new(big.Rat).Mul(row[1], p[1])))
	}
	var out lensUVPolygon
	a := polygon[len(polygon)-1]
	va := value(a)
	if e := check(va); e != nil {
		return nil, e
	}
	for _, b := range polygon {
		vb := value(b)
		if e := check(vb); e != nil {
			return nil, e
		}
		if va.Sign()*vb.Sign() < 0 {
			t := new(big.Rat).Quo(va, new(big.Rat).Sub(va, vb))
			var p [2]*big.Rat
			for j := range p {
				p[j] = new(big.Rat).Add(a[j], new(big.Rat).Mul(t, new(big.Rat).Sub(b[j], a[j])))
				if e := check(p[j]); e != nil {
					return nil, e
				}
			}
			out = append(out, p)
		}
		if vb.Sign() >= 0 {
			out = append(out, b)
		}
		a, va = b, vb
	}
	return out, nil
}

func (polygon lensUVPolygon) area(check func(*big.Rat) error) (*big.Rat, error) {
	sum := new(big.Rat)
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		sum.Add(sum, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
		if e := check(sum); e != nil {
			return nil, e
		}
	}
	if sum.Sign() < 0 {
		return nil, geom.ErrConvergence
	}
	return sum.Quo(sum, big.NewRat(2, 1)), nil
}
