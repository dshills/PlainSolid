package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

func mappedChainKnotTrimCuts(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, prescribed [][]*big.Rat, o geom.SurfaceCurveAgreementOptions) ([][]*big.Rat, error) {
	out := make([][]*big.Rat, len(outer))
	for i, trim := range outer {
		cells, e := mixedExactCells(ctx, p, trim, o)
		if e != nil {
			return nil, e
		}
		if !cells.Complete || len(cells.Cells) == 0 {
			return nil, geom.ErrConvergence
		}
		if len(cells.Cells) == 1 {
			out[i] = prescribed[i]
			continue
		}
		cuts := slices.Clone(prescribed[i])
		cuts = append(cuts, new(big.Rat), big.NewRat(1, 1))
		for _, cell := range cells.Cells[:len(cells.Cells)-1] {
			end := cell.EndpointIntervals()[1]
			station := end.Lower
			if station.Cmp(end.Upper) != 0 {
				root, e := cell.EndpointRoot(1)
				if e != nil {
					return nil, e
				}
				station, e = root.RecoverRational(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
				if e != nil {
					return nil, e
				}
				if station == nil {
					return nil, geom.ErrUnrepresentable
				}
			}
			cuts = append(cuts, station)
		}
		slices.SortFunc(cuts, (*big.Rat).Cmp)
		out[i] = slices.CompactFunc(cuts, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
	}
	return out, nil
}

// Partition every seed triangle in exact chart coordinates. The previous
// chord polygon stays fixed: a knot crossing its boundary must already be an
// authored boundary sample or there would be an unwelded closing edge.
func alignMappedChainUVSeed(ctx context.Context, p *geom.SurfacePatch, seed CertifiedAffineQuadraticTrimMesh, maxTriangles int) (CertifiedAffineQuadraticTrimMesh, error) {
	var zero CertifiedAffineQuadraticTrimMesh
	us, vs := p.KnotBreaks()
	if len(us)+len(vs) > 14 {
		return zero, geom.ErrConvergence
	}
	knots := [2][]*big.Rat{}
	for axis, raw := range [2][]float64{us, vs} {
		for _, k := range raw {
			knots[axis] = append(knots[axis], ratInset(k))
		}
	}
	for _, loop := range seed.BoundaryLoops {
		for i, a := range loop {
			b := loop[(i+1)%len(loop)]
			for axis, breaks := range knots {
				for _, knot := range breaks[1 : len(breaks)-1] {
					if seed.ExactUV[a][axis].Cmp(knot)*seed.ExactUV[b][axis].Cmp(knot) < 0 {
						return zero, fmt.Errorf("knot crosses a display boundary without an authored sample: %w", geom.ErrConvergence)
					}
				}
			}
		}
	}

	uvKey := func(q [2]*big.Rat) [2]string { return [2]string{q[0].RatString(), q[1].RatString()} }
	ids := make(map[[2]string]int, len(seed.ExactUV))
	for i, q := range seed.ExactUV {
		id := uvKey(q)
		if _, exists := ids[id]; exists {
			return zero, geom.ErrConvergence
		}
		ids[id] = i
	}
	result := seed
	result.Mesh.Triangles, result.OriginalFace = nil, nil
	result.DisplaySquaredDistance = new(big.Rat).Set(seed.DisplaySquaredDistance)
	var triangles [][3]int
	index := func(q [2]*big.Rat) int {
		key := uvKey(q)
		id, exists := ids[key]
		if !exists {
			id = len(result.ExactUV)
			ids[key] = id
			result.ExactUV = append(result.ExactUV, q)
			result.ExactPoints = append(result.ExactPoints, [3]*big.Rat{q[0], q[1], new(big.Rat)})
			u, _ := q[0].Float64()
			v, _ := q[1].Float64()
			result.Mesh.Vertices = append(result.Mesh.Vertices, geom.Vec3{X: u, Y: v})
			du, dv := new(big.Rat).Sub(q[0], ratInset(u)), new(big.Rat).Sub(q[1], ratInset(v))
			errorSquared := new(big.Rat).Add(du.Mul(du, du), dv.Mul(dv, dv))
			if errorSquared.Cmp(result.DisplaySquaredDistance) > 0 {
				result.DisplaySquaredDistance.Set(errorSquared)
			}
		}
		return id
	}
	for _, t := range seed.Mesh.Triangles {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		tr := [][2]*big.Rat{seed.ExactUV[t[0]], seed.ExactUV[t[1]], seed.ExactUV[t[2]]}
		area, covered := exactUVSignedArea(tr[0], tr[1], tr[2]), new(big.Rat)
		if area.Sign() <= 0 {
			return zero, geom.ErrConvergence
		}
		for i := 0; i+1 < len(us); i++ {
			u := clipExactUVPolygon(tr, 0, knots[0][i], true)
			u = clipExactUVPolygon(u, 0, knots[0][i+1], false)
			for j := 0; j+1 < len(vs) && len(u) >= 3; j++ {
				q := clipExactUVPolygon(u, 1, knots[1][j], true)
				q = clipExactUVPolygon(q, 1, knots[1][j+1], false)
				for k := 1; k+1 < len(q); k++ {
					part := exactUVSignedArea(q[0], q[k], q[k+1])
					if part.Sign() == 0 {
						continue
					}
					if part.Sign() < 0 || len(triangles) >= maxTriangles {
						return zero, geom.ErrConvergence
					}
					triangles = append(triangles, [3]int{index(q[0]), index(q[k]), index(q[k+1])})
					covered.Add(covered, part)
				}
			}
		}
		if covered.Cmp(area) != 0 {
			return zero, geom.ErrConvergence
		}
	}
	return certifyAffineTrimTriangles(ctx, result, triangles, maxTriangles)
}

// Interior UV refinement preserves the original one-sided cells of the
// aligned seed. Independently check the retained final triangles before
// assigning the span indexes used for one-sided differential evaluation.
func mappedChainTriangleSupportCells(p *geom.SurfacePatch, face CertifiedMixedExactCurvedMesh) ([][2]int, error) {
	u, v := p.KnotBreaks()
	breaks := [2][]*big.Rat{}
	for axis, raw := range [2][]float64{u, v} {
		for _, knot := range raw {
			breaks[axis] = append(breaks[axis], ratInset(knot))
		}
	}
	cells := make([][2]int, 0, len(face.Mesh.Triangles))
	for _, t := range face.Mesh.Triangles {
		var cell [2]int
		for axis, knots := range breaks {
			a, b, c := face.ExactUV[t[0]][axis], face.ExactUV[t[1]][axis], face.ExactUV[t[2]][axis]
			i := 0
			for ; i+1 < len(knots); i++ {
				lo, hi := knots[i], knots[i+1]
				if a.Cmp(lo) >= 0 && b.Cmp(lo) >= 0 && c.Cmp(lo) >= 0 &&
					a.Cmp(hi) <= 0 && b.Cmp(hi) <= 0 && c.Cmp(hi) <= 0 {
					break
				}
			}
			if i+1 == len(knots) {
				return nil, geom.ErrConvergence
			}
			cell[axis] = i
		}
		cells = append(cells, cell)
	}
	return cells, nil
}
