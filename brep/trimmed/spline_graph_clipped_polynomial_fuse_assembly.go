package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func (b *clippedStockShellBuilder) prepareSpans(ctx context.Context, atoms []geom.ClippedPolynomialStockBoundary,
	selectAtom func(int, geom.ClippedPolynomialStockBoundary) (uint8, [2]geom.ClippedPolynomialWall)) error {
	for atom, source := range atoms {
		if e := ctx.Err(); e != nil {
			return e
		}
		owners, walls := selectAtom(atom, source)
		if owners == 0 {
			continue
		}
		span := clippedStockSpan{atom: atom, source: source, walls: walls}
		var e error
		for i, v := range source.Points {
			span.points[i], e = exactGraphPartitionUV(v)
			if e != nil {
				return e
			}
			for owner, uv := range source.OwnerUV {
				if owners&(1<<owner) == 0 {
					continue
				}
				if uv[i][0] == nil {
					return geom.ErrConvergence
				}
				span.traces[owner][i], e = b.lift(owner, uv[i], span.points[i])
				if e != nil {
					return fmt.Errorf("atom %d source %d endpoint %d: %w", atom, owner, i, e)
				}
			}
		}
		a, z := span.traces[1], span.traces[2]
		const bothRoofs = (1 << 1) | (1 << 2)
		if owners&bothRoofs != bothRoofs || a[0].point != z[0].point || a[1].point != z[1].point {
			b.spans = append(b.spans, span)
			continue
		}
		// Two roof curves with identical endpoints need distinct native
		// edge pairs. In a Cut the taller trace can be an internal cap
		// seam while the lower trace bounds the exposed wall. Split every
		// incident trace at the same exact original chart station.
		middle, e := exactGraphPartitionUV(clippedStockMidpoint(source.Points))
		if e != nil {
			return e
		}
		left, right := span, span
		left.points[1], right.points[0] = middle, middle
		for owner, uv := range source.OwnerUV {
			if owners&(1<<owner) == 0 {
				continue
			}
			at, e := b.lift(owner, clippedStockMidpoint(uv), middle)
			if e != nil {
				return fmt.Errorf("atom %d source %d riser midpoint: %w", atom, owner, e)
			}
			left.traces[owner][1], right.traces[owner][0] = at, at
		}
		b.spans = append(b.spans, left, right)
	}
	return nil
}

// Only original cap traces that are exposed in the union require native
// vertex representation. The two adjacent caps also give the limits of
// any interior riser. A covered source has no native vertex on this span.
func clippedFuseSpanOwners(material geom.ClippedSplineGraphPolynomialMaterial, atom geom.ClippedPolynomialStockBoundary) uint8 {
	var owners uint8
	for _, tile := range atom.Tiles {
		switch {
		case tile < 0:
			owners |= 1
		case tile == 0:
			owner := clippedFuseRoofOwner(material.Overlap.Projection.Arrangement.Regions[atom.OverlapRegion].HeightSide)
			owners |= 1 << owner
		default:
			owners |= material.Tiles[tile].RoofMask << 1
		}
	}
	return owners
}

func clippedFuseRoofOwner(sign int) int {
	if sign < 0 {
		return 2
	}
	return 1
}

func clippedStockMidpoint(points [2][2]*big.Rat) (uv [2]*big.Rat) {
	for i := range uv {
		uv[i] = new(big.Rat).Quo(new(big.Rat).Add(points[0][i], points[1][i]), big.NewRat(2, 1))
	}
	return uv
}

func (b *clippedStockShellBuilder) lift(owner int, uv [2]*big.Rat, xy geom.Vec2) (clippedFuseEndpoint, error) {
	u, e := exactGraphPartitionUV(uv)
	if e != nil {
		return clippedFuseEndpoint{}, e
	}
	p, e := b.evaluate(b.sources[owner], openSectionPiece{a: u})
	if e != nil {
		return clippedFuseEndpoint{}, e
	}
	if p.X != xy.X || p.Y != xy.Y {
		return clippedFuseEndpoint{}, geom.ErrConvergence
	}
	return clippedFuseEndpoint{u, p}, nil
}

func (b *clippedStockShellBuilder) vertex(p geom.Vec3) int {
	if id, ok := b.vertexIDs[p]; ok {
		return id
	}
	id := len(b.vertices)
	b.vertices = append(b.vertices, p)
	b.vertexIDs[p] = id
	return id
}

func (b *clippedStockShellBuilder) edge(p, q geom.Vec3, curve geom.Curve, length float64) (id int, reversed, added bool) {
	i, j := b.vertex(p), b.vertex(q)
	key := [2]int{min(i, j), max(i, j)}
	if id, ok := b.edgeIDs[key]; ok {
		return id, b.edges[id].Start != i, false
	}
	id = len(b.edges)
	b.edges = append(b.edges, parametric.Edge{Start: i, End: j, Curve: curve, Range: parametric.Interval{Max: length}})
	b.edgeIDs[key] = id
	return id, false, true
}

func (b *clippedStockShellBuilder) graphUse(owner int, a, z clippedFuseEndpoint, trim PCurve) (Use, bool) {
	curve := surfaceTrimCurve{b.sources[owner], a.uv, z.uv, trim}
	id, rev, added := b.edge(a.point, z.point, curve, 1)
	return Use{Edge: id, Reversed: rev, UVStart: a.uv, UVEnd: z.uv, Curve: trim}, added
}

func (b *clippedStockShellBuilder) trace(owner int, ref clippedStockDirectedSpan) Use {
	start, end := ref.ends()
	points := b.spans[ref.id].traces[owner]
	use, _ := b.graphUse(owner, points[start], points[end], nil)
	return use
}

// perimeter returns the proved directed span sequence for a tile, an open
// portion of the overlap perimeter, or the union's outside boundary.
func (b *clippedStockShellBuilder) perimeter(tile, region int, start, end geom.Vec2) ([]clippedStockDirectedSpan, error) {
	byStart := make(map[geom.Vec2]clippedStockDirectedSpan)
	for i, span := range b.spans {
		left, right := span.source.Tiles[0], span.source.Tiles[1]
		if region >= 0 && span.source.OverlapRegion != region {
			continue
		}
		var reverse bool
		switch {
		case tile < 0 && left == -1, tile >= 0 && right == tile:
			reverse = true
		case tile < 0 && right == -1, tile >= 0 && left == tile:
		default:
			continue
		}
		ref := clippedStockDirectedSpan{id: i, reverse: reverse}
		a, _ := ref.ends()
		p := span.points[a]
		if _, ok := byStart[p]; ok {
			return nil, geom.ErrConvergence
		}
		byStart[p] = ref
	}
	if tile < 0 {
		// For offset footprints the bounding-box corner may be outside
		// both inputs. Choose the lexicographically first actual vertex.
		first := true
		for p := range byStart {
			if first || p.X < start.X || p.X == start.X && p.Y < start.Y {
				start = p
				first = false
			}
		}
		end = start
	}
	out := make([]clippedStockDirectedSpan, 0, len(byStart))
	next := start
	for len(byStart) > 0 {
		ref, ok := byStart[next]
		if !ok {
			return nil, geom.ErrConvergence
		}
		delete(byStart, next)
		out = append(out, ref)
		_, z := ref.ends()
		next = b.spans[ref.id].points[z]
	}
	if next != end || len(out) == 0 {
		return nil, geom.ErrConvergence
	}
	return out, nil
}

func (b *clippedStockShellBuilder) cap(owner int, loop []Use, chart graphEnvelopeChart, bottom bool) int {
	if chart.reversed {
		reverseGraphUses(loop)
	}
	id := len(b.faces)
	b.faces = append(b.faces, Face{Surface: b.sources[owner], Outer: loop, Reversed: chart.reversed != bottom})
	return id
}

func (b *clippedStockShellBuilder) graphContourUses(owner int, selected []geom.SplineGraphPolynomialUse, pieces [][]openSectionPiece) ([]Use, []int, error) {
	var loop []Use
	var section []int
	for _, use := range selected {
		chain := pieces[use.Edge]
		for j, piece := range chain {
			if use.Reversed {
				piece = chain[len(chain)-1-j]
				piece.a, piece.b, piece.c = piece.b, piece.a, reversePCurve(piece.c)
			}
			a, e := b.evaluate(b.sources[owner], openSectionPiece{a: piece.a})
			if e != nil {
				return nil, nil, e
			}
			z, e := b.evaluate(b.sources[owner], openSectionPiece{a: piece.b})
			if e != nil {
				return nil, nil, e
			}
			u, added := b.graphUse(owner, clippedFuseEndpoint{piece.a, a}, clippedFuseEndpoint{piece.b, z}, piece.c)
			if added {
				section = append(section, u.Edge)
			}
			loop = append(loop, u)
		}
	}
	return loop, section, nil
}

func (b *clippedStockShellBuilder) addCaps(material geom.ClippedSplineGraphPolynomialMaterial, uv []geom.Vec2, pieces [2][][]openSectionPiece,
	charts [3]graphEnvelopeChart) (faces, owners, section []int, base int, err error) {
	for i, region := range material.Overlap.Projection.Arrangement.Regions {
		owner := clippedFuseRoofOwner(region.HeightSide)
		loop, newEdges, e := b.graphContourUses(owner, region.Edges, pieces[owner-1])
		if e != nil {
			return nil, nil, nil, -1, e
		}
		section = append(section, newEdges...)
		if arc := region.Perimeter; arc != nil {
			if arc.ParameterDirection != 1 {
				return nil, nil, nil, -1, geom.ErrConvergence
			}
			spans, e := b.perimeter(0, i, uv[arc.Start], uv[arc.End])
			if e != nil {
				return nil, nil, nil, -1, e
			}
			for _, span := range spans {
				loop = append(loop, b.trace(owner, span))
			}
		}
		faces = append(faces, b.cap(owner, loop, charts[owner], false))
		owners = append(owners, owner-1)
	}
	for i, tile := range material.Tiles[1:] {
		owner := 1
		if tile.RoofMask == 2 {
			owner = 2
		}
		corner := geom.Vec2{X: tile.Physical.UMin, Y: tile.Physical.VMin}
		spans, e := b.perimeter(i+1, -1, corner, corner)
		if e != nil {
			return nil, nil, nil, -1, e
		}
		loop := make([]Use, 0, len(spans))
		for _, span := range spans {
			loop = append(loop, b.trace(owner, span))
		}
		faces = append(faces, b.cap(owner, loop, charts[owner], false))
		owners = append(owners, owner-1)
	}
	spans, e := b.perimeter(-1, -1, geom.Vec2{}, geom.Vec2{})
	if e != nil {
		return nil, nil, nil, -1, e
	}
	if e := b.certifyBaseOutline(spans, material.Tiles); e != nil {
		return nil, nil, nil, -1, e
	}
	loop := make([]Use, 0, len(spans))
	for _, span := range spans {
		loop = append(loop, b.trace(0, span))
	}
	return faces, owners, section, b.cap(0, loop, charts[0], true), nil
}

// The exterior atlas provides one directed cycle around the union. Check
// that its axial trim segments form a simple ring and that its positive
// signed area equals the sum of all disjoint physical stock tiles. The affine
// original base map carries this exact domain to its unmodified UV chart.
func (b *clippedStockShellBuilder) certifyBaseOutline(spans []clippedStockDirectedSpan, tiles []geom.ClippedPolynomialStockTile) error {
	if len(spans) < 4 {
		return geom.ErrConvergence
	}
	vertices := make([]geom.Vec2, len(spans))
	for i, ref := range spans {
		start, _ := ref.ends()
		vertices[i] = b.spans[ref.id].points[start]
	}
	twiceArea := new(big.Rat)
	for i, p := range vertices {
		q := vertices[(i+1)%len(vertices)]
		if (p.X == q.X) == (p.Y == q.Y) {
			return geom.ErrConvergence
		}
		for j := i + 2; j < len(vertices); j++ {
			if i == 0 && j == len(vertices)-1 {
				continue
			}
			r, s := vertices[j], vertices[(j+1)%len(vertices)]
			// Binary endpoints are exact. For axis-aligned edges their
			// closed bounding rectangles intersect iff the edges do.
			if max(min(p.X, q.X), min(r.X, s.X)) <= min(max(p.X, q.X), max(r.X, s.X)) &&
				max(min(p.Y, q.Y), min(r.Y, s.Y)) <= min(max(p.Y, q.Y), max(r.Y, s.Y)) {
				return geom.ErrConvergence
			}
		}
		twiceArea.Add(twiceArea, new(big.Rat).Mul(ratInset(p.X), ratInset(q.Y)))
		twiceArea.Sub(twiceArea, new(big.Rat).Mul(ratInset(q.X), ratInset(p.Y)))
	}
	want := new(big.Rat)
	for _, tile := range tiles {
		d := tile.Physical
		width := new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin))
		height := new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))
		want.Add(want, width.Mul(width, height))
	}
	want.Mul(want, big.NewRat(2, 1))
	if twiceArea.Sign() <= 0 || twiceArea.Cmp(want) != 0 {
		return geom.ErrConvergence
	}
	return nil
}

// addWalls joins every exposed interval to its two original model traces.
// Every participating station uses the common ordered set of exact heights;
// a tapered endpoint emits no zero-length edge.
func (b *clippedStockShellBuilder) addWalls(ctx context.Context) ([]ClippedPolynomialWallBinding, error) {
	stations := make(map[geom.Vec2][]float64)
	for _, span := range b.spans {
		for _, wall := range span.walls {
			if wall.Lower == geom.ClippedPolynomialNoHeight {
				continue
			}
			for i, xy := range span.points {
				lo, hi := span.traces[wall.Lower-1][i].point.Z, span.traces[wall.Upper-1][i].point.Z
				if lo > hi || lo == hi && wall.Lower == geom.ClippedPolynomialBaseHeight {
					return nil, geom.ErrConvergence
				}
				stations[xy] = append(stations[xy], lo, hi)
			}
		}
	}
	for p, heights := range stations {
		slices.Sort(heights)
		stations[p] = slices.Compact(heights)
	}
	var out []ClippedPolynomialWallBinding
	for i, span := range b.spans {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for side, wall := range span.walls {
			if wall.Lower == geom.ClippedPolynomialNoHeight {
				continue
			}
			ref := clippedStockDirectedSpan{id: i, reverse: side == 1}
			r, e := b.wall(ref, wall, stations)
			if e != nil {
				return nil, e
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func (b *clippedStockShellBuilder) wall(ref clippedStockDirectedSpan, interval geom.ClippedPolynomialWall,
	stations map[geom.Vec2][]float64) (ClippedPolynomialWallBinding, error) {
	span := b.spans[ref.id]
	start, end := ref.ends()
	p, q := span.points[start], span.points[end]
	surface, e := newSplineGraphAffineWall(geom.Vec3{X: p.X, Y: p.Y}, geom.Vec3{X: q.X, Y: q.Y})
	if e != nil {
		return ClippedPolynomialWallBinding{}, e
	}
	out := ClippedPolynomialWallBinding{Face: len(b.faces), Atom: span.atom, EndingRoofs: span.source.EndingRoofs,
		Low: interval.Lower, High: interval.Upper, Points: [2]geom.Vec2{p, q}}
	var trace [2]Use
	var levels [2][2]float64
	for i, owner := range [2]int{int(interval.Lower - 1), int(interval.Upper - 1)} {
		a, z := span.traces[owner][start], span.traces[owner][end]
		levels[i] = [2]float64{a.point.Z, z.point.Z}
		model := b.trace(owner, ref)
		out.TraceEdges[i], out.OwnerUV[i] = model.Edge, [2]geom.Vec2{a.uv, z.uv}
		trace[i] = Use{Edge: model.Edge, Reversed: model.Reversed, UVStart: geom.Vec2{Y: a.point.Z}, UVEnd: geom.Vec2{X: 1, Y: z.point.Z},
			Curve: affineGraphWallTrim{wallTrim{surface: b.sources[owner], a: a.uv, b: z.uv, length: 1, start: a.point.Z, end: z.point.Z}}}
	}
	if trace[0].Edge == trace[1].Edge {
		return ClippedPolynomialWallBinding{}, geom.ErrConvergence
	}
	vertical := [2][]Use{}
	for i, xy := range out.Points {
		chain, e := b.verticalStation(float64(i), xy, stations[xy], levels[0][i], levels[1][i])
		if e != nil {
			return ClippedPolynomialWallBinding{}, e
		}
		vertical[i] = chain
		for _, use := range chain {
			out.Stations[i] = append(out.Stations[i], use.Edge)
		}
	}
	loop := []Use{trace[0]}
	loop = append(loop, vertical[1]...)
	high := []Use{trace[1]}
	reverseGraphUses(high)
	loop = append(loop, high...)
	reverseGraphUses(vertical[0])
	loop = append(loop, vertical[0]...)
	b.faces = append(b.faces, Face{Surface: surface, Outer: loop})
	return out, nil
}

func (b *clippedStockShellBuilder) verticalStation(u float64, xy geom.Vec2, heights []float64, lo, hi float64) ([]Use, error) {
	if lo == hi {
		return nil, nil
	}
	i, hasLo := slices.BinarySearch(heights, lo)
	j, hasHi := slices.BinarySearch(heights, hi)
	if !hasLo || !hasHi || i >= j {
		return nil, geom.ErrConvergence
	}
	var uses []Use
	for k := i; k < j; k++ {
		a, z := heights[k], heights[k+1]
		p, q := geom.Vec3{X: xy.X, Y: xy.Y, Z: a}, geom.Vec3{X: xy.X, Y: xy.Y, Z: z}
		length, e := geom.RepresentedExactScalar(new(big.Rat).Sub(ratInset(z), ratInset(a)))
		if e != nil {
			return nil, e
		}
		line, e := geom.NewLine(p, geom.Vec3{Z: 1})
		if e != nil {
			return nil, e
		}
		id, reverse, _ := b.edge(p, q, line, length)
		uses = append(uses, Use{Edge: id, Reversed: reverse, UVStart: geom.Vec2{X: u, Y: a}, UVEnd: geom.Vec2{X: u, Y: z}})
	}
	return uses, nil
}
