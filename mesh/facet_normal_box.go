package mesh

import (
	"math"
	"math/big"

	"plainsolid/geom"
)

// CertifyFacetNormalBox bounds the squared tangent of the angle between
// every represented oriented facet and every vector in a directed normal
// box. It requires a strictly positive dot product throughout the box.
// The caller supplies an enclosure of the continuous normals. Radians must
// be strictly between zero and pi/2.
func CertifyFacetNormalBox(box geom.Box, facets [][3]geom.Vec3, radians float64) (*big.Rat, error) {
	if !box.Valid() || len(facets) == 0 || !finite(radians) || radians <= 0 || radians >= math.Pi/2 {
		return nil, geom.ErrInvalid
	}
	cone := newNormalOffsetFacetBox(box)
	limit := normalOffsetFacetTangentLimit(radians)
	upper := new(big.Rat)
	for _, triangle := range facets {
		var p [3]normalOffsetFacetVector
		for i, vertex := range triangle {
			if !vertex.IsFinite() {
				return nil, geom.ErrInvalid
			}
			p[i] = normalOffsetFacetVector{
				new(big.Rat).SetFloat64(vertex.X),
				new(big.Rat).SetFloat64(vertex.Y),
				new(big.Rat).SetFloat64(vertex.Z),
			}
		}
		bound := cone.certify(normalOffsetTriangleNormal(p[0], p[1], p[2]), limit)
		if bound == nil {
			return nil, geom.ErrConvergence
		}
		if bound.Cmp(upper) > 0 {
			upper = bound
		}
	}
	return upper, nil
}
