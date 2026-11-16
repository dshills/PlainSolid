package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// mappedRoofQuality improves the triangulation in the physical XY projection.
// The same diagonal must be valid in the original UV chart. A positive exact
// incircle predicate licenses a flip. Trim and support-knot edges stay fixed.
type mappedRoofQuality struct {
	orientation int
	knots       [2][]*big.Rat
	remaining   int
}

func newMappedRoofQuality(ctx context.Context, p *geom.SurfacePatch, maxFlips int) (*mappedRoofQuality, error) {
	graph, err := geom.NewAffineXYGraph(ctx, p, geom.SurfacePatchOptions{})
	if err != nil {
		return nil, err
	}
	r := &mappedRoofQuality{orientation: graph.Orientation(), remaining: maxFlips}
	u, v := p.KnotBreaks()
	for axis, knots := range [2][]float64{u, v} {
		for _, knot := range knots[1 : len(knots)-1] {
			r.knots[axis] = append(r.knots[axis], ratInset(knot))
		}
	}
	return r, nil
}

type mappedRoofEdge [2]int

func roofEdge(a, b int) mappedRoofEdge { return mappedRoofEdge{min(a, b), max(a, b)} }

type roofEdgeOwners struct {
	tris [2]int
	uses int
}

func (r *mappedRoofQuality) improve(ctx context.Context, face *CertifiedCurvedQuadraticTrimMesh) error {
	triangles := face.Mesh.Triangles
	uv := face.ExactUV
	xy := func(i int) [2]*big.Rat { p := face.ExactPoints[i]; return [2]*big.Rat{p[0], p[1]} }

	edges := make(map[mappedRoofEdge]roofEdgeOwners)
	enqueued := make(map[mappedRoofEdge]bool)
	queue := make([]mappedRoofEdge, 0, len(triangles)*2)
	add := func(id int, tri [3]int) error {
		for j, a := range tri {
			k := roofEdge(a, tri[(j+1)%3])
			x := edges[k]
			if x.uses == 2 {
				return geom.ErrConvergence
			}
			x.tris[x.uses] = id
			x.uses++
			edges[k] = x
			if x.uses == 2 && !enqueued[k] {
				queue = append(queue, k)
				enqueued[k] = true
			}
		}
		return nil
	}
	remove := func(id int, tri [3]int) {
		for j, a := range tri {
			k := roofEdge(a, tri[(j+1)%3])
			x := edges[k]
			if x.uses == 1 {
				delete(edges, k)
				continue
			}
			if x.tris[0] == id {
				x.tris[0] = x.tris[1]
			}
			x.uses--
			edges[k] = x
		}
	}
	onKnot := func(a, b int) bool {
		for axis, knots := range r.knots {
			for _, knot := range knots {
				if uv[a][axis].Cmp(knot) == 0 && uv[b][axis].Cmp(knot) == 0 {
					return true
				}
			}
		}
		return false
	}
	for i, t := range triangles {
		if err := add(i, t); err != nil {
			return err
		}
	}
	for k := 0; k < len(queue); k++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		e := queue[k]
		delete(enqueued, e)
		slot := edges[e]
		if slot.uses != 2 || onKnot(e[0], e[1]) {
			continue
		}
		i, j := slot.tris[0], slot.tris[1]
		left, right := triangles[i], triangles[j]
		var a, b, c, d int
		for k := range left {
			a, b, c = left[k], left[(k+1)%3], left[(k+2)%3]
			if roofEdge(a, b) == e {
				break
			}
		}
		for k := range right {
			if right[k] == b && right[(k+1)%3] == a {
				d = right[(k+2)%3]
				break
			}
			if k == 2 {
				return geom.ErrConvergence
			}
		}
		if c == d || edges[roofEdge(c, d)].uses != 0 ||
			exactUVSignedArea(uv[c], uv[d], uv[b]).Sign() <= 0 ||
			exactUVSignedArea(uv[d], uv[c], uv[a]).Sign() <= 0 {
			continue
		}
		x, y, z, q := xy(a), xy(b), xy(c), xy(d)
		if exactUVSignedArea(x, y, z).Sign() != r.orientation ||
			exactUVSignedArea(y, x, q).Sign() != r.orientation ||
			exactUVSignedArea(z, q, y).Sign() != r.orientation ||
			exactUVSignedArea(q, z, x).Sign() != r.orientation ||
			roofIncircle(x, y, z, q)*r.orientation <= 0 {
			continue
		}
		if r.remaining == 0 {
			return fmt.Errorf("mapped roof diagonal flip budget: %w", geom.ErrConvergence)
		}
		r.remaining--
		remove(i, left)
		remove(j, right)
		triangles[i], triangles[j] = [3]int{c, d, b}, [3]int{d, c, a}
		if err := add(i, triangles[i]); err != nil {
			return err
		}
		if err := add(j, triangles[j]); err != nil {
			return err
		}
	}
	return nil
}

// roofIncircle is the translated determinant of rows (x,y,x²+y²).
// Its orientation-dependent sign is strict. Cocircular points retain their
// authored triangulation independently of queue order.
func roofIncircle(a, b, c, d [2]*big.Rat) int {
	x, y, lift := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	for i, p := range [3][2]*big.Rat{a, b, c} {
		x[i] = new(big.Rat).Sub(p[0], d[0])
		y[i] = new(big.Rat).Sub(p[1], d[1])
		lift[i] = new(big.Rat).Add(new(big.Rat).Mul(x[i], x[i]), new(big.Rat).Mul(y[i], y[i]))
	}
	det := new(big.Rat)
	for i, q := range lift {
		j, k := (i+1)%3, (i+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(x[j], y[k]), new(big.Rat).Mul(x[k], y[j]))
		det.Add(det, cross.Mul(cross, q))
	}
	return det.Sign()
}
