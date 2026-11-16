package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type parabolaCircleFootprintEdge struct {
	owners     [2]Use
	points     []int
	parameters []*big.Rat
}

func parabolaCircleMeshEdgeKey(a, b int) [2]int { return [2]int{min(a, b), max(a, b)} }

func parabolaTwoCirclesLiftFootprint(ctx context.Context, s Solid, h ParabolaCircleCutComponentHistory, height [2]parabolaCircleMeshHeight,
	foot CertifiedMixedExactAffineMesh, topUses, bottomUses [][]Use, n int, o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaTwoCirclesComponentMesh, error) {
	var empty CertifiedParabolaTwoCirclesComponentMesh
	uv := slices.Clone(foot.ExactUV)
	seam := make([]bool, len(uv))
	boundary := make(map[[2]int]bool)
	var sides []parabolaCircleFootprintEdge
	for _, sample := range foot.Samples {
		top, low := topUses[sample.Loop][sample.Trim], bottomUses[sample.Loop][sample.Trim]
		if top.Reversed || low.Reversed || len(sample.Vertices) < 2 || len(sample.Vertices) != len(sample.Parameters) {
			return empty, geom.ErrConvergence
		}
		if len(sample.Vertices) > o.MaxSubdivisions+1 {
			return empty, errRefine
		}
		part := parabolaCircleFootprintEdge{owners: [2]Use{top, low}, points: slices.Clone(sample.Vertices), parameters: slices.Clone(sample.Parameters)}
		if part.parameters[0].Cmp(part.parameters[len(part.parameters)-1]) > 0 {
			slices.Reverse(part.points)
			slices.Reverse(part.parameters)
		}
		for j, id := range part.points {
			a, b := s.edges[top.Edge], s.edges[low.Edge]
			if top.Edge == low.Edge || j == 0 && a.Start == b.Start || j == len(part.points)-1 && a.End == b.End {
				seam[id] = true
			}
			if j > 0 {
				boundary[parabolaCircleMeshEdgeKey(part.points[j-1], id)] = true
			}
		}
		sides = append(sides, part)
	}
	uv, triangles, err := refineParabolaCircleFootprint(ctx, uv, foot.Mesh.Triangles, boundary, seam, n, o.MaxTriangles/2)
	if err != nil {
		return empty, err
	}
	display := CertifiedParabolaTwoCirclesComponentMesh{Region: h.Region, OriginalVertices: make([]int, len(s.vertices)),
		OriginalEdges: make([]ParabolaTwoCirclesMeshEdge, len(s.edges)), SignedDisplayVolume: new(big.Rat)}
	for i := range display.OriginalVertices {
		display.OriginalVertices[i] = -1
	}
	ids := make([][2]int, len(uv))
	represented := make([][2]*big.Rat, len(uv))
	gaps := make([]*big.Rat, len(uv))
	rounding := new(big.Rat)
	for i, q := range uv {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		high, low := height[0].evaluate(q), height[1].evaluate(q)
		shared := i < len(seam) && seam[i]
		if shared && high.Cmp(low) != 0 || !shared && high.Cmp(low) <= 0 {
			return empty, errRefine
		}
		x, _ := q[0].Float64()
		y, _ := q[1].Float64()
		represented[i] = [2]*big.Rat{ratInset(x), ratInset(y)}
		top, _ := high.Float64()
		bottom, _ := low.Float64()
		gaps[i] = new(big.Rat).Sub(ratInset(top), ratInset(bottom))
		if !shared && gaps[i].Sign() <= 0 {
			return empty, errRefine
		}
		for side, z := range [2]float64{top, bottom} {
			p := geom.Vec3{X: x, Y: y, Z: z}
			if !p.IsFinite() {
				return empty, geom.ErrConvergence
			}
			if side == 1 && shared {
				ids[i][side] = ids[i][0]
				continue
			}
			ids[i][side] = len(display.Mesh.Vertices)
			display.Mesh.Vertices = append(display.Mesh.Vertices, p)
			exact := [3]*big.Rat{q[0], q[1], high}
			if side == 1 {
				exact[2] = low
			}
			errorSum := new(big.Rat)
			for j, r := range exactBoundPoint(p) {
				errorSum.Add(errorSum, new(big.Rat).Abs(new(big.Rat).Sub(exact[j], r)))
			}
			if errorSum.Cmp(rounding) > 0 {
				rounding.Set(errorSum)
			}
		}
	}
	emit := func(face int, tri [3]int) error {
		if len(display.Mesh.Triangles) == o.MaxTriangles {
			return parametric.ErrBudget
		}
		display.Mesh.Triangles = append(display.Mesh.Triangles, tri)
		display.OriginalFace = append(display.OriginalFace, face)
		return nil
	}
	width := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	for _, t := range triangles {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		det := exactUVSignedArea(represented[t[0]], represented[t[1]], represented[t[2]])
		if det.Sign() <= 0 {
			return empty, errRefine
		}
		gap := new(big.Rat)
		for _, i := range t {
			gap.Add(gap, gaps[i])
		}
		if gap.Sign() <= 0 {
			return empty, errRefine
		}
		display.SignedDisplayVolume.Add(display.SignedDisplayVolume, gap.Mul(gap, det))
		for j, i := range t {
			b := t[(j+1)%3]
			for axis := range width {
				q := new(big.Rat).Abs(new(big.Rat).Sub(uv[i][axis], uv[b][axis]))
				if q.Cmp(width[axis]) > 0 {
					width[axis].Set(q)
				}
			}
		}
		if err := emit(h.RoofFaces[0], [3]int{ids[t[0]][0], ids[t[1]][0], ids[t[2]][0]}); err != nil {
			return empty, err
		}
		if err := emit(h.RoofFaces[1], [3]int{ids[t[0]][1], ids[t[2]][1], ids[t[1]][1]}); err != nil {
			return empty, err
		}
	}
	display.SignedDisplayVolume.Quo(display.SignedDisplayVolume, big.NewRat(6, 1))

	for _, side := range sides {
		for owner, use := range side.owners {
			e := s.edges[use.Edge]
			if e.Range != (parametric.Interval{Max: 1}) {
				return empty, geom.ErrConvergence
			}
			edge := ParabolaTwoCirclesMeshEdge{Edge: use.Edge, Parameters: ownParabolaCircleMeshParameters(side.parameters)}
			for _, i := range side.points {
				edge.Vertices = append(edge.Vertices, ids[i][owner])
			}
			if old := display.OriginalEdges[use.Edge]; len(old.Vertices) != 0 && !slices.Equal(old.Vertices, edge.Vertices) {
				return empty, geom.ErrConvergence
			}
			display.OriginalEdges[use.Edge] = edge
			for j, vertex := range [2]int{e.Start, e.End} {
				i := edge.Vertices[j*(len(edge.Vertices)-1)]
				if old := display.OriginalVertices[vertex]; old != -1 && old != i || s.vertices[vertex] != display.Mesh.Vertices[i] {
					return empty, geom.ErrConvergence
				}
				display.OriginalVertices[vertex] = i
			}
		}
		if side.owners[0].Edge == side.owners[1].Edge {
			continue
		}
		face := -1
		for _, id := range h.WallFaces {
			uses := s.faces[id].Outer
			if slices.ContainsFunc(uses, func(u Use) bool { return u.Edge == side.owners[0].Edge }) &&
				slices.ContainsFunc(uses, func(u Use) bool { return u.Edge == side.owners[1].Edge }) {
				wall, ok := s.faces[id].Surface.(geom.PlaneSurface)
				if !ok || face != -1 {
					return empty, geom.ErrConvergence
				}
				origin, normal := wall.Frame().Origin(), wall.Frame().ZAxis()
				a, b := side.owners[0].UVStart, side.owners[0].UVEnd
				xPlane := a.X == b.X && origin.X == a.X && normal.Y == 0 && (normal.X == -1 || normal.X == 1)
				yPlane := a.Y == b.Y && origin.Y == a.Y && normal.X == 0 && (normal.Y == -1 || normal.Y == 1)
				if normal.Z != 0 || origin.Z != 0 || !(xPlane || yPlane) {
					return empty, geom.ErrConvergence
				}
				face = id
			}
		}
		if face < 0 {
			return empty, geom.ErrConvergence
		}
		for i, point := range side.points[:len(side.points)-1] {
			a, b := ids[point], ids[side.points[i+1]]
			if a[0] != a[1] {
				if err := emit(face, [3]int{a[0], a[1], b[1]}); err != nil {
					return empty, err
				}
			}
			if b[0] != b[1] {
				if err := emit(face, [3]int{a[0], b[1], b[0]}); err != nil {
					return empty, err
				}
			}
		}
	}
	for i, samples := range display.OriginalEdges {
		if len(samples.Vertices) != 0 {
			continue
		}
		e := s.edges[i]
		a, b := display.OriginalVertices[e.Start], display.OriginalVertices[e.End]
		start, end := s.vertices[e.Start], s.vertices[e.End]
		if a < 0 || b < 0 || start.X != end.X || start.Y != end.Y {
			return empty, geom.ErrConvergence
		}
		display.OriginalEdges[i] = ParabolaTwoCirclesMeshEdge{Edge: i, Vertices: []int{a, b},
			Parameters: []*big.Rat{ratInset(e.Range.Min), ratInset(e.Range.Max)}}
	}
	_, trimError, err := graphAreaSqrt(foot.Region.FillSquaredDistance, 64, 16384)
	if err != nil {
		return empty, err
	}
	if o.NormalTurn > 0 {
		pad := new(big.Rat).Add(trimError, rounding)
		display.SquaredSourceNormalTurn = parabolaCircleMeshSourceTurnSquared(height, width, pad)
	}
	display.DistanceUpper = new(big.Rat)
	for _, surface := range height {
		// The XY chord homotopy has distance at most trimError. The L1
		// bound below transports it over either original polynomial chart.
		// Each wall is the vertical interval between those roof restrictions
		// on one fixed stock side. Pairing the same vertical fraction gives
		// the same bound there, including collapsed intervals at the seam.
		slope := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Add(surface.slope[0], surface.slope[1]))
		transport := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(slope, trimError))
		total := transport.Add(transport, surface.residual(width[0], width[1]))
		total.Add(total, rounding)
		if total.Cmp(display.DistanceUpper) > 0 {
			display.DistanceUpper.Set(total)
		}
	}
	return display, nil
}

func refineParabolaCircleFootprint(ctx context.Context, uv [][2]*big.Rat, tris [][3]int, boundary map[[2]int]bool, seam []bool,
	n, budget int) ([][2]*big.Rat, [][3]int, error) {
	width := [2]*big.Rat{}
	for axis := range width {
		lo, hi := uv[0][axis], uv[0][axis]
		for _, p := range uv[1:] {
			if p[axis].Cmp(lo) < 0 {
				lo = p[axis]
			}
			if p[axis].Cmp(hi) > 0 {
				hi = p[axis]
			}
		}
		width[axis] = new(big.Rat).Quo(new(big.Rat).Sub(hi, lo), big.NewRat(int64(n), 1))
	}
	isSeam := func(i int) bool { return i < len(seam) && seam[i] }
	for range 32 {
		if len(tris) > budget {
			return nil, nil, parametric.ErrBudget
		}
		mids := make(map[[2]int]int)
		next := make([][3]int, 0, len(tris))
		for _, t := range tris {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			ids := [3]int{-1, -1, -1}
			count := 0
			for j, a := range t {
				b := t[(j+1)%3]
				key := parabolaCircleMeshEdgeKey(a, b)
				if boundary[key] {
					continue
				}
				id, exists := mids[key]
				if !exists {
					split := isSeam(a) && isSeam(b)
					for axis := range width {
						gap := new(big.Rat).Abs(new(big.Rat).Sub(uv[a][axis], uv[b][axis]))
						split = split || gap.Cmp(width[axis]) > 0
					}
					if !split {
						continue
					}
					var point [2]*big.Rat
					for axis := range point {
						point[axis] = new(big.Rat).Quo(new(big.Rat).Add(uv[a][axis], uv[b][axis]), big.NewRat(2, 1))
					}
					id = len(uv)
					uv = append(uv, point)
					mids[key] = id
				}
				ids[j] = id
				count++
			}
			switch count {
			case 0:
				next = append(next, t)
			case 1:
				for j, id := range ids {
					if id >= 0 {
						next = append(next, [3]int{t[j], id, t[(j+2)%3]}, [3]int{id, t[(j+1)%3], t[(j+2)%3]})
						break
					}
				}
			case 2:
				for j, x := range ids {
					y := ids[(j+1)%3]
					if x >= 0 && y >= 0 {
						a, b, c := t[j], t[(j+1)%3], t[(j+2)%3]
						next = append(next, [3]int{b, y, x}, [3]int{a, x, c}, [3]int{x, y, c})
						break
					}
				}
			case 3:
				next = append(next, [3]int{t[0], ids[0], ids[2]}, [3]int{ids[0], t[1], ids[1]},
					[3]int{ids[2], ids[1], t[2]}, [3]int{ids[0], ids[1], ids[2]})
			}
			if len(next) > budget {
				return nil, nil, parametric.ErrBudget
			}
		}
		if len(mids) == 0 {
			return uv, next, nil
		}
		tris = next
	}
	return nil, nil, parametric.ErrBudget
}
