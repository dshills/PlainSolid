package brep

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ConvexChamferHistory identifies the new chamfer face and the original
// supports. Original faces retain their indices in the output solid.
type ConvexChamferHistory struct {
	ChamferFace int
	SourceFaces [2]int
	PlaneTests  int
}

// ChamferConvexEdge adds a planar chamfer along one edge of a convex planar
// solid. The setbacks are measured on the two original incident faces,
// perpendicular to the edge and towards each face's interior. The first
// setback applies to referenceFace. Both setbacks must be positive.
//
// The two incident unit normals and their dihedral sine must be rational.
// The cut must lie strictly clear of the original vertices beyond the two
// edge ends, and resulting vertices must be exactly representable.
func (s Solid) ChamferConvexEdge(ctx context.Context, edge, referenceFace int, first, second float64, o ConvexOffsetOptions) (Solid, ConvexChamferHistory, error) {
	if edge < 0 || edge >= len(s.edges) || referenceFace < 0 || referenceFace >= len(s.faces) ||
		first <= 0 || second <= 0 || math.IsNaN(first) || math.IsNaN(second) || math.IsInf(first, 0) || math.IsInf(second, 0) {
		return Solid{}, ConvexChamferHistory{}, geom.ErrInvalid
	}
	work, e := s.prepareConvexOffset(ctx, o)
	if e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	planes, e := s.convexOffsetPlanes(ctx, new(big.Rat), nil, &work)
	if e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	incident := [2]int{-1, -1}
	for i, face := range s.faces {
		for _, u := range face.Boundary {
			if u.Edge == edge {
				if i == referenceFace {
					incident[0] = i
				} else {
					incident[1] = i
				}
			}
		}
	}
	if incident[0] < 0 || incident[1] < 0 {
		return Solid{}, ConvexChamferHistory{}, fmt.Errorf("brep: reference face must use the selected edge: %w", geom.ErrInvalid)
	}
	chamfer, e := convexChamferPlane(planes[incident[0]], planes[incident[1]], new(big.Rat).SetFloat64(first), new(big.Rat).SetFloat64(second))
	if e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	target := s.edges[edge]
	for i, v := range s.vertices {
		if i == target.Start || i == target.End {
			continue
		}
		if e := work.test(); e != nil {
			return Solid{}, ConvexChamferHistory{}, e
		}
		if chamfer.n.dot(exactOffsetVector(v)).Cmp(chamfer.d) >= 0 {
			return Solid{}, ConvexChamferHistory{}, fmt.Errorf("brep: chamfer reaches original vertex %d outside the selected edge: %w", i, geom.ErrDomain)
		}
	}
	planes = append(planes, chamfer)
	points, owners, e := intersectOffsetHalfspaces(ctx, planes, &work)
	if e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	polygons := make([][]int, len(planes))
	for i, plane := range planes {
		polygons[i] = offsetFacetHull(points, owners[i], plane.n)
		if len(polygons[i]) < 3 {
			return Solid{}, ConvexChamferHistory{}, geom.ErrConvergence
		}
	}
	vertices, e := representedOffsetVertices(points)
	if e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	if e = ctx.Err(); e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	out, e := fromPolygons(vertices, polygons)
	if e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	if e = ctx.Err(); e != nil {
		return Solid{}, ConvexChamferHistory{}, e
	}
	return out, ConvexChamferHistory{len(s.faces), incident, work.tests}, nil
}

func convexChamferPlane(a, b offsetPlane, onA, onB *big.Rat) (offsetPlane, error) {
	la, oka := exactOffsetSqrt(a.n.dot(a.n))
	lb, okb := exactOffsetSqrt(b.n.dot(b.n))
	cross := a.n.cross(b.n)
	length, okc := exactOffsetSqrt(cross.dot(cross))
	if !oka || !okb || !okc {
		return offsetPlane{}, geom.ErrUnrepresentable
	}
	if length.Sign() == 0 {
		return offsetPlane{}, geom.ErrDegenerate
	}
	// On A its source plane gap is zero, and distance onA opens a
	// gap of onA*sin(angle) to B. On B the roles are reversed. The
	// combined unit-normal inequality has exactly those intercepts.
	sin := new(big.Rat).Quo(length, new(big.Rat).Mul(la, lb))
	wa, wb := new(big.Rat).Quo(onA, la), new(big.Rat).Quo(onB, lb)
	var p offsetPlane
	for i := range p.n {
		p.n[i] = new(big.Rat).Add(new(big.Rat).Mul(wa, a.n[i]), new(big.Rat).Mul(wb, b.n[i]))
	}
	p.d = new(big.Rat).Add(new(big.Rat).Mul(wa, a.d), new(big.Rat).Mul(wb, b.d))
	p.d.Sub(p.d, new(big.Rat).Mul(sin, new(big.Rat).Mul(onA, onB)))
	return p, nil
}
