package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const (
	envelopeTriangleVertex   = -3
	envelopeTriangleOutside  = -2
	envelopeTriangleInterior = -1
)

// Insert a uniform grid in the proved chord fill before flipping its
// diagonals. Bisecting the skinny triangles of an initial ear
// triangulation can leave a short transverse altitude along a stock
// side. Such triangles can fail the normal proof even at small widths.
// Grid points belong only to the interior. All original shared coedge
// samples, edge directions, and the boundary polygon stay fixed.
func insertCrossingEnvelopeFacetGrid(ctx context.Context, uv [][2]*big.Rat, triangles [][3]int,
	n, budget int) ([][2]*big.Rat, [][3]int, error) {
	const maximumTests = 12_000_000
	work := maximumTests
	stations := make([]*big.Rat, n)
	for i := range stations {
		stations[i] = big.NewRat(int64(2*i+1), int64(2*n))
	}
	for _, x := range stations {
		for _, y := range stations {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			q := [2]*big.Rat{x, y}
			uses := [2]int{-1, -1}
			edges := [2]int{-1, -1}
			for id, t := range triangles {
				if work == 0 {
					return nil, nil, parametric.ErrBudget
				}
				work--
				side := crossingEnvelopeTrianglePoint(uv, t, q)
				switch side {
				case envelopeTriangleVertex:
					uses[0] = -1
				case envelopeTriangleOutside:
					continue
				case envelopeTriangleInterior:
					uses[0] = id
				default: // A point on one edge.
					if uses[0] < 0 {
						uses[0], edges[0] = id, side
						continue
					}
					a := triangles[uses[0]]
					if a[edges[0]] != t[(side+1)%3] || a[(edges[0]+1)%3] != t[side] {
						return nil, nil, geom.ErrConvergence
					}
					uses[1], edges[1] = id, side
				}
				break
			}
			if uses[0] < 0 || edges[0] >= 0 && uses[1] < 0 {
				continue
			}
			if len(triangles)+2 > budget {
				return nil, nil, parametric.ErrBudget
			}
			point := len(uv)
			uv = append(uv, q)
			if edges[0] < 0 {
				id := uses[0]
				a, b, c := triangles[id][0], triangles[id][1], triangles[id][2]
				triangles[id] = [3]int{a, b, point}
				triangles = append(triangles, [3]int{b, c, point}, [3]int{c, a, point})
				continue
			}
			for i, id := range uses {
				t, j := triangles[id], edges[i]
				a, b, c := t[j], t[(j+1)%3], t[(j+2)%3]
				triangles[id] = [3]int{a, point, c}
				triangles = append(triangles, [3]int{point, b, c})
			}
		}
	}
	return uv, triangles, nil
}

func crossingEnvelopeTrianglePoint(uv [][2]*big.Rat, tri [3]int, q [2]*big.Rat) int {
	for axis := range q {
		a, b, c := q[axis].Cmp(uv[tri[0]][axis]), q[axis].Cmp(uv[tri[1]][axis]), q[axis].Cmp(uv[tri[2]][axis])
		if a < 0 && b < 0 && c < 0 || a > 0 && b > 0 && c > 0 {
			return envelopeTriangleOutside
		}
	}
	edge := envelopeTriangleInterior
	for j, a := range tri {
		b := tri[(j+1)%3]
		switch exactUVSignedArea(uv[a], uv[b], q).Sign() {
		case -1:
			return envelopeTriangleOutside
		case 0:
			if edge >= 0 {
				return envelopeTriangleVertex
			}
			edge = j
		}
	}
	return edge
}
