package offset

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type convexFootprintSource struct {
	points                       []pt
	originalVertex, originalEdge []int
	radiusSquared                *big.Rat
	primitives                   []ConvexOffsetPrimitive
	placement                    geom.Affine
	z                            [2]*big.Rat
}

// ConvexFootprintPoint contains exact material membership in the ideal
// dilated section. SquaredDistanceToProfile is measured to the original
// closed polygon. Edges and Corners on Boundary identify original supporting
// sides or vertices. A tangent line/arc junction retains both source owners.
type ConvexFootprintPoint struct {
	Location                 parametric.Location
	SquaredDistanceToProfile *big.Rat
	Edges, Corners           []int
}

// LocateRational classifies a point in local section coordinates. It uses the
// Euclidean closest point on the original polygon and checks the distance
// against the exact squared offset. The native angular arc approximation plays
// no role in classification. Input coefficients are limited to 16384 bits.
func (q ConvexFootprintDilation) LocateRational(ctx context.Context, point [2]*big.Rat) (ConvexFootprintPoint, error) {
	var empty ConvexFootprintPoint
	if ctx == nil || q.source == nil || point[0] == nil || point[1] == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	for _, v := range point {
		if v.Num().BitLen() > 16384 || v.Denom().BitLen() > 16384 {
			return empty, ErrLimit
		}
	}
	original := q.source
	p := pt{point[0], point[1]}
	inside, onOriginal := true, false
	for i, a := range original.points {
		b := original.points[(i+1)%len(original.points)]
		side := footprintCross(footprintDifference(b, a), footprintDifference(p, a)).Sign()
		inside, onOriginal = inside && side >= 0, onOriginal || side == 0
	}
	if inside && (!onOriginal || original.radiusSquared.Sign() > 0) {
		return ConvexFootprintPoint{Location: parametric.Inside, SquaredDistanceToProfile: new(big.Rat)}, nil
	}
	type nearest struct{ edge, corner int }
	var closest []nearest
	var distance *big.Rat
	for i, a := range original.points {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		j := (i + 1) % len(original.points)
		d := footprintDifference(original.points[j], a)
		relative := footprintDifference(p, a)
		along := footprintDot(relative, d)
		lengthSquared := footprintDot(d, d)
		edge, corner := -1, -1
		projected := a
		if along.Cmp(lengthSquared) >= 0 {
			projected = original.points[j]
			corner = original.originalVertex[j]
		} else if along.Sign() <= 0 {
			corner = original.originalVertex[i]
		} else {
			t := new(big.Rat).Quo(along, lengthSquared)
			projected = pt{add(a.x, mul(t, d.x)), add(a.y, mul(t, d.y))}
		}
		if along.Sign() >= 0 && along.Cmp(lengthSquared) <= 0 && footprintCross(d, relative).Sign() <= 0 {
			edge = original.originalEdge[i]
		}
		v := footprintDifference(p, projected)
		squared := footprintDot(v, v)
		if distance == nil || squared.Cmp(distance) < 0 {
			distance = squared
			closest = closest[:0]
		}
		if squared.Cmp(distance) == 0 {
			closest = append(closest, nearest{edge, corner})
		}
	}
	result := ConvexFootprintPoint{SquaredDistanceToProfile: distance}
	switch distance.Cmp(original.radiusSquared) {
	case -1:
		result.Location = parametric.Inside
	case 0:
		result.Location = parametric.Boundary
		for _, n := range closest {
			if n.edge >= 0 {
				result.Edges = append(result.Edges, n.edge)
			}
			if n.corner >= 0 {
				result.Corners = append(result.Corners, n.corner)
			}
		}
		slices.Sort(result.Edges)
		slices.Sort(result.Corners)
		result.Edges, result.Corners = slices.Compact(result.Edges), slices.Compact(result.Corners)
	default:
		result.Location = parametric.Outside
	}
	return result, nil
}

func footprintDot(a, b pt) *big.Rat { return add(mul(a.x, b.x), mul(a.y, b.y)) }
