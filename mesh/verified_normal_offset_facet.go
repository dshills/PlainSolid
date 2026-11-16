package mesh

import (
	"math/big"

	"plainsolid/geom"
)

// tan(x) >= x on [0,pi/2). Integrating tan'(x) = 1 + tan(x)^2
// then gives tan(x) >= x + x^3/3, using only exact rational arithmetic.
func normalOffsetFacetTangentLimit(angle float64) *big.Rat {
	r := new(big.Rat).SetFloat64(angle)
	q := new(big.Rat).Mul(r, r)
	q.Mul(q, r).Quo(q, big.NewRat(3, 1)).Add(q, r)
	return q.Mul(q, q)
}

type normalOffsetFacetVector [3]*big.Rat

type normalOffsetFacetRay struct {
	direction   normalOffsetFacetVector
	normSquared *big.Rat
}

type normalOffsetFacetBox []normalOffsetFacetRay

func newNormalOffsetFacetBox(box geom.Box) *normalOffsetFacetBox {
	// NormalBoundsRational rounds Bounds outward from its exact directed
	// rationals. Treat those represented endpoints as exact dyadics.
	lower := [3]float64{box.Min.X, box.Min.Y, box.Min.Z}
	upper := [3]float64{box.Max.X, box.Max.Y, box.Max.Z}
	var coordinates [3][]*big.Rat
	for i, value := range lower {
		coordinates[i] = []*big.Rat{new(big.Rat).SetFloat64(value)}
		if upper[i] != value {
			coordinates[i] = append(coordinates[i], new(big.Rat).SetFloat64(upper[i]))
		}
	}
	var out normalOffsetFacetBox
	for _, x := range coordinates[0] {
		for _, y := range coordinates[1] {
			for _, z := range coordinates[2] {
				n := normalOffsetFacetVector{x, y, z}
				out = append(out, normalOffsetFacetRay{direction: n, normSquared: normalOffsetFacetDot(n, n)})
			}
		}
	}
	return &out
}

func (b *normalOffsetFacetBox) certifyCell(corners [4]normalOffsetGridPoint, triangles [2][3]int, limit *big.Rat) *big.Rat {
	upper := new(big.Rat)
	for _, tri := range triangles {
		a, next, c := corners[tri[0]].exact, corners[tri[1]].exact, corners[tri[2]].exact
		facet := normalOffsetTriangleNormal(a, next, c)
		bound := b.certify(facet, limit)
		if bound == nil {
			return nil
		}
		if bound.Cmp(upper) > 0 {
			upper = bound
		}
	}
	return upper
}

func (b *normalOffsetFacetBox) certify(facet normalOffsetFacetVector, limit *big.Rat) *big.Rat {
	// An acute cone about the represented facet is convex. It suffices
	// to test its entire normal box at the corners, with strict positive
	// dot, then observe that all the true normals lie in that same cone.
	//
	// Compare |F×N|² with limit·(F·N)². Cauchy's identity avoids roots.
	facetNorm2 := normalOffsetFacetDot(facet, facet)
	numerator, denominator := new(big.Rat), big.NewRat(1, 1)
	for _, ray := range *b {
		dot := normalOffsetFacetDot(facet, ray.direction)
		if dot.Sign() <= 0 {
			return nil
		}
		dot2 := dot.Mul(dot, dot)
		cross2 := new(big.Rat).Mul(facetNorm2, ray.normSquared)
		cross2.Sub(cross2, dot2)
		if cross2.Cmp(new(big.Rat).Mul(limit, dot2)) > 0 {
			return nil
		}
		if new(big.Rat).Mul(cross2, denominator).Cmp(new(big.Rat).Mul(numerator, dot2)) > 0 {
			numerator, denominator = cross2, dot2
		}
	}
	return numerator.Quo(numerator, denominator)
}

func normalOffsetTriangleNormal(a, b, c normalOffsetFacetVector) (normal normalOffsetFacetVector) {
	var u, v normalOffsetFacetVector
	for i := range a {
		u[i] = new(big.Rat).Sub(b[i], a[i])
		v[i] = new(big.Rat).Sub(c[i], a[i])
	}
	for i := range normal {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(u[j], v[k]), new(big.Rat).Mul(u[k], v[j]))
	}
	return normal
}

func normalOffsetFacetDot(a, b normalOffsetFacetVector) *big.Rat {
	q := new(big.Rat)
	for i := range a {
		q.Add(q, new(big.Rat).Mul(a[i], b[i]))
	}
	return q
}
