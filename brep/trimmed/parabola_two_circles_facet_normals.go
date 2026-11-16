package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func improveParabolaCircleFacetGrid(ctx context.Context, uv [][2]*big.Rat, triangles [][3]int, seam []bool) error {
	// Interior edge flips preserve the exact chord polygon on both owners.
	// A new diagonal between two seam vertices would create an extra
	// contact between the two roofs. Such diagonals are excluded.
	isSeam := func(i int) bool { return i < len(seam) && seam[i] }
	allowed := func(a, b int) bool { return !isSeam(a) || !isSeam(b) }
	remaining := 8 * len(triangles)
	return improveRoofDiagonals(ctx, triangles, uv, func(i int) [2]*big.Rat { return uv[i] }, 1, &remaining, nil, allowed)
}

func certifyParabolaCircleFacetNormals(ctx context.Context, s Solid, h ParabolaCircleCutComponentHistory,
	heights [2]parabolaCircleMeshHeight, uv [][2]*big.Rat, footprint [][3]int, display CertifiedParabolaTwoCirclesComponentMesh,
	placement *parabolaCircleDisplayPlacement, pad, limit *big.Rat) (*big.Rat, error) {
	// Use the exact inverse chart of the represented world points. A proper
	// signed-axis isometry preserves normal angles and oriented cofactors.
	points := make([]exactBoundVector, len(display.Mesh.Vertices))
	for i, p := range display.Mesh.Vertices {
		q := exactBoundPoint(p)
		if placement != nil {
			q = parabolaCircleRigidPoint(placement.chart.inverse, q)
		}
		points[i] = q
	}
	facet := func(i int) [3]*big.Rat {
		t := display.Mesh.Triangles[i]
		a, b, c := points[t[0]], points[t[1]], points[t[2]]
		return ddCross(boundSubtract(b, a), boundSubtract(c, a))
	}
	maximum := new(big.Rat)
	add := func(a, b [3]*big.Rat) bool {
		q, ok := ddTanSquared(a, b)
		if !ok || q.Cmp(limit) > 0 {
			return false
		}
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
		return true
	}
	for i, tri := range footprint {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		center, radius := parabolaCircleFacetSourceBox(uv, tri, pad)
		for side, roof := range heights {
			f := 2*i + side
			if display.OriginalFace[f] != h.RoofFaces[side] {
				return nil, geom.ErrConvergence
			}
			n := facet(f)
			g := roof.gradient(center)
			m := roof.normalHessian
			du := new(big.Rat).Add(new(big.Rat).Mul(m[0], radius[0]), new(big.Rat).Mul(m[1], radius[1]))
			dv := new(big.Rat).Add(new(big.Rat).Mul(m[1], radius[0]), new(big.Rat).Mul(m[2], radius[1]))
			rayZ := big.NewRat(1, 1)
			if s.faces[h.RoofFaces[side]].Reversed {
				rayZ.Neg(rayZ)
			}
			// The source gradient lies in this rectangle on the whole
			// padded footprint. Positive-dot circular cones below pi/2
			// are convex, so their four corners certify every source ray.
			for _, u := range [2]*big.Rat{new(big.Rat).Sub(g[0], du), new(big.Rat).Add(g[0], du)} {
				for _, v := range [2]*big.Rat{new(big.Rat).Sub(g[1], dv), new(big.Rat).Add(g[1], dv)} {
					ray := [3]*big.Rat{new(big.Rat).Neg(new(big.Rat).Mul(u, rayZ)),
						new(big.Rat).Neg(new(big.Rat).Mul(v, rayZ)), rayZ}
					if !add(n, ray) {
						return nil, errRefine
					}
				}
			}
		}
	}
	for i, owner := range display.OriginalFace[2*len(footprint):] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f := s.faces[owner]
		p, ok := f.Surface.(geom.PlaneSurface)
		if !ok {
			return nil, geom.ErrConvergence
		}
		normal := p.Frame().ZAxis()
		if f.Reversed {
			normal = normal.Scale(-1)
		}
		if !add(facet(i+2*len(footprint)), exactBoundPoint(normal)) {
			return nil, errRefine
		}
	}
	return maximum, nil
}

func parabolaCircleFacetSourceBox(uv [][2]*big.Rat, t [3]int, pad *big.Rat) (center, radius [2]*big.Rat) {
	for axis := range center {
		lo, hi := uv[t[0]][axis], uv[t[0]][axis]
		for _, id := range t[1:] {
			x := uv[id][axis]
			if x.Cmp(lo) < 0 {
				lo = x
			}
			if x.Cmp(hi) > 0 {
				hi = x
			}
		}
		a, b := new(big.Rat).Sub(lo, pad), new(big.Rat).Add(hi, pad)
		if a.Sign() < 0 {
			a.SetInt64(0)
		}
		if b.Cmp(big.NewRat(1, 1)) > 0 {
			b.SetInt64(1)
		}
		center[axis] = new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
		radius[axis] = new(big.Rat).Sub(b, center[axis])
	}
	return center, radius
}

func (h parabolaCircleMeshHeight) gradient(uv [2]*big.Rat) [2]*big.Rat {
	var height, du, dv, row, rowDerivative big.Rat
	for u := h.degree[0]; u >= 0; u-- {
		row.SetInt64(0)
		rowDerivative.SetInt64(0)
		for v := h.degree[1]; v >= 0; v-- {
			rowDerivative.Mul(&rowDerivative, uv[1])
			rowDerivative.Add(&rowDerivative, &row)
			row.Mul(&row, uv[1])
			if c := h.coefficients[u][v]; c != nil {
				row.Add(&row, c)
			}
		}
		du.Mul(&du, uv[0])
		du.Add(&du, &height)
		dv.Mul(&dv, uv[0])
		dv.Add(&dv, &rowDerivative)
		height.Mul(&height, uv[0])
		height.Add(&height, &row)
	}
	return [2]*big.Rat{&du, &dv}
}
