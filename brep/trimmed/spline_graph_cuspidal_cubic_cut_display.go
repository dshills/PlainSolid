package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Tessellate builds one conforming, embedded display from the original
// polynomial roofs. It bounds sampled deflection and optional normal angle,
// and requires the exact mesh volume to agree with the source integral to 2%.
// MaxSubdivisions limits the segments on each original rail.
func (r *CuspidalCubicCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	return r.TessellateWithVolumeTolerance(ctx, o, .02)
}

// TessellateWithVolumeTolerance uses the supplied positive finite relative
// error for the exact mesh volume. A truncated proof returns no mesh.
func (r *CuspidalCubicCut) TessellateWithVolumeTolerance(ctx context.Context, o parametric.TessellationOptions,
	relativeTolerance float64) (mesh.Mesh, error) {
	if r == nil || r.section == nil || !finite(relativeTolerance) || relativeTolerance <= 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	o, err := normalizeTrimmedTessellationOptions(ctx, o)
	if err != nil {
		return mesh.Mesh{}, err
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	p, err := prepareCuspidalCutDisplay(ctx, r)
	if err != nil {
		return mesh.Mesh{}, err
	}
	expected := new(big.Rat).Mul(p.volume, big.NewRat(6, 1))
	tolerance := new(big.Rat).Mul(expected, ratInset(relativeTolerance))
	o.MaxTriangles = min(o.MaxTriangles, collision.MaxTriangles)
	maximum := o.MaxSubdivisions
	if r.history.Region == 0 {
		// The center station divides each long stock rail into two strips.
		maximum /= 2
	}
	for n := 2; n <= maximum; n *= 2 {
		required := 16*n*n + 16*n - 2
		if r.history.Region == 1 {
			required = 4*n*n - 2
		}
		if required > o.MaxTriangles {
			break
		}
		candidate, err := p.candidate(ctx, n, o, required)
		if errors.Is(err, errRefine) {
			continue
		}
		if err != nil {
			return mesh.Mesh{}, err
		}
		if !clippedCutDisplayMatchesVolume(ctx, candidate, expected, tolerance) {
			if err := ctx.Err(); err != nil {
				return mesh.Mesh{}, err
			}
			continue
		}
		locator, err := collision.NewMeshPointLocator(ctx, candidate, nil, collision.LocationOptions{MaxTriangles: o.MaxTriangles})
		if errors.Is(err, collision.ErrLimit) {
			return mesh.Mesh{}, fmt.Errorf("trimmed: cuspidal Cut display collision work: %w", parametric.ErrBudget)
		}
		if err != nil {
			return mesh.Mesh{}, err
		}
		if len(locator.Shells()) != 1 {
			return mesh.Mesh{}, geom.ErrConvergence
		}
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		return candidate, nil
	}
	return mesh.Mesh{}, parametric.ErrBudget
}

type cuspidalCutDisplay struct {
	cut      *CuspidalCubicCut
	domain   geom.ParameterRectangle
	maps     [2][2][3]*big.Rat
	reversed [2]bool
	volume   *big.Rat
}

func prepareCuspidalCutDisplay(ctx context.Context, cut *CuspidalCubicCut) (cuspidalCutDisplay, error) {
	p := cuspidalCutDisplay{cut: cut}
	stock, err := geom.NewSurfacePatch(cut.sources[0], geom.SurfacePatchOptions{})
	if err != nil {
		return p, err
	}
	reference, err := geom.NewAffineXYRectangleReference(ctx, stock, geom.SurfacePatchOptions{})
	if err != nil {
		return p, err
	}
	p.domain = reference.Domain()
	var patches [2]*geom.SurfacePatch
	for owner, roof := range cut.sources[1:] {
		weights := roof.Data().Weights
		for _, w := range weights {
			if w != weights[0] {
				return p, parametric.ErrDeflectionUnsupported
			}
		}
		patches[owner], err = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
		if err != nil {
			return p, err
		}
		alignment, err := geom.AlignAffineXYCharts(ctx, reference, patches[owner], geom.SurfacePatchOptions{})
		if err != nil {
			return p, err
		}
		m, err := alignment.MapCoefficients()
		if err != nil {
			return p, err
		}
		p.maps[owner] = m
		p.reversed[owner] = m[0][0].Sign()*m[1][1].Sign()-m[0][1].Sign()*m[1][0].Sign() < 0
	}
	p.volume, err = cuspidalCutPolynomialVolume(ctx, cut, patches, p.domain)
	return p, err
}

type cuspidalCutDisplayPoint struct {
	xy  geom.Vec2
	uv  [2]geom.Vec2
	ids [2]int
	q   float64 // outward cusp parameter, or -1 off the section
}

type cuspidalCutDisplayGrid struct {
	source *cuspidalCutDisplay
	mesh   mesh.Mesh
	native map[geom.Vec2][2]int
	points []cuspidalCutDisplayPoint
	faces  [][3]int
}

func (p *cuspidalCutDisplay) candidate(ctx context.Context, n int, o parametric.TessellationOptions, required int) (mesh.Mesh, error) {
	s, h := p.cut.solid, p.cut.history
	g := cuspidalCutDisplayGrid{source: p, mesh: mesh.Mesh{Vertices: s.Vertices(), Triangles: make([][3]int, 0, required)},
		native: make(map[geom.Vec2][2]int, len(s.vertices))}
	for owner, face := range h.RoofFaces {
		for _, use := range s.faces[face].Outer {
			id := s.edges[use.Edge].Start
			if use.Reversed {
				id = s.edges[use.Edge].End
			}
			v := s.vertices[id]
			xy := geom.Vec2{X: v.X, Y: v.Y}
			ids, ok := g.native[xy]
			if !ok {
				ids = [2]int{-1, -1}
			}
			if ids[owner] >= 0 {
				return mesh.Mesh{}, geom.ErrConvergence
			}
			ids[owner] = id
			g.native[xy] = ids
		}
	}
	if err := g.cover(ctx, n); err != nil {
		return mesh.Mesh{}, err
	}
	if len(g.native) != 0 {
		return mesh.Mesh{}, geom.ErrConvergence
	}
	if err := g.emit(ctx, o); err != nil {
		return mesh.Mesh{}, err
	}
	if len(g.mesh.Triangles) != required {
		return mesh.Mesh{}, geom.ErrConvergence
	}
	return g.mesh, nil
}

// The left half of the exterior spans the full height. Its last row is
// shared by the two right strips, which follow the two original cusp germs
// with the same outward station q. The pointed region uses only the strip
// between the germs and collapses its first row to the original cusp.
func (g *cuspidalCutDisplayGrid) cover(ctx context.Context, n int) error {
	p, d := g.source, g.source.domain
	s, history := p.cut.solid, p.cut.history
	cusp := s.vertices[history.Vertices[0]]
	exits := [2]geom.Vec3{s.vertices[history.Vertices[1]], s.vertices[history.Vertices[2]]}
	pt := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	row := func(x, low, high float64, germ int, q float64) ([]int, error) {
		ids := make([]int, n+1)
		for j := range ids {
			station := -1.
			if (j == 0 && germ <= 0 || j == n && germ >= 0) && q >= 0 {
				station = q
			}
			id, err := g.point(pt(x, cuspidalCutDisplayLerp(low, high, float64(j)/float64(n))), station)
			if err != nil {
				return nil, err
			}
			ids[j] = id
		}
		return ids, nil
	}
	var prior [][]int
	if history.Region == 0 {
		var left []int
		for i := 0; i <= n; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			x := cuspidalCutDisplayLerp(d.UMin, cusp.X, float64(i)/float64(n))
			station := -1.
			if i == n {
				station = 0
			}
			lower, err := row(x, d.VMin, cusp.Y, 1, station)
			if err != nil {
				return err
			}
			full := lower
			for j := 1; j <= n; j++ {
				id, err := g.point(pt(x, cuspidalCutDisplayLerp(cusp.Y, d.VMax, float64(j)/float64(n))), -1)
				if err != nil {
					return err
				}
				full = append(full, id)
			}
			g.addStrip(left, full)
			left = full
		}
		prior = [][]int{left[:n+1], left[n:]}
	} else {
		id, err := g.point(pt(cusp.X, cusp.Y), 0)
		if err != nil {
			return err
		}
		collapsed := make([]int, n+1)
		for j := range collapsed {
			collapsed[j] = id
		}
		prior = [][]int{collapsed}
	}
	for i := 1; i <= n; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		q := float64(i) / float64(n)
		x := cuspidalCutDisplayLerp(cusp.X, d.UMax, q*q)
		lower := cuspidalCutDisplayLerp(cusp.Y, exits[0].Y, q*q*q)
		upper := cuspidalCutDisplayLerp(cusp.Y, exits[1].Y, q*q*q)
		bands := [][2]float64{{d.VMin, lower}, {upper, d.VMax}}
		germs := []int{1, -1}
		if history.Region == 1 {
			bands, germs = [][2]float64{{lower, upper}}, []int{0}
		}
		for j, band := range bands {
			next, err := row(x, band[0], band[1], germs[j], q)
			if err != nil {
				return err
			}
			g.addStrip(prior[j], next)
			prior[j] = next
		}
	}
	return nil
}

func cuspidalCutDisplayLerp(a, b, t float64) float64 {
	return mix(geom.Vec2{X: a}, geom.Vec2{X: b}, t).X
}

func (g *cuspidalCutDisplayGrid) point(xy geom.Vec2, q float64) (int, error) {
	if !uvFinite(xy) {
		return 0, geom.ErrConvergence
	}
	p := g.source
	point := cuspidalCutDisplayPoint{xy: xy, q: q}
	x, y := ratInset(xy.X), ratInset(xy.Y)
	for owner, m := range p.maps {
		var v [2]float64
		for i, row := range m {
			value := new(big.Rat).Mul(row[0], x)
			value.Add(value, new(big.Rat).Mul(row[1], y)).Add(value, row[2])
			v[i], _ = value.Float64()
		}
		point.uv[owner] = geom.Vec2{X: v[0], Y: v[1]}
	}
	if native, ok := g.native[xy]; ok {
		if native[0] < 0 || native[1] < 0 || (native[0] == native[1]) != (q >= 0) {
			return 0, geom.ErrConvergence
		}
		point.ids = native
		delete(g.native, xy)
	} else if q >= 0 {
		exact, err := p.germPoint(p.germ(xy), q)
		if err != nil {
			return 0, err
		}
		if !closePoint(exact, geom.Vec3{X: xy.X, Y: xy.Y, Z: exact.Z}, p.cut.tolerance) {
			return 0, geom.ErrConvergence
		}
		id := len(g.mesh.Vertices)
		point.ids = [2]int{id, id}
		g.mesh.Vertices = append(g.mesh.Vertices, geom.Vec3{X: xy.X, Y: xy.Y, Z: exact.Z})
	} else {
		for owner, uv := range point.uv {
			value, err := p.cut.sources[owner+1].Eval(uv.X, uv.Y)
			if err != nil {
				return 0, err
			}
			v := geom.Vec3{X: xy.X, Y: xy.Y, Z: value.Z}
			if !closePoint(value, v, p.cut.tolerance) {
				return 0, geom.ErrConvergence
			}
			point.ids[owner] = len(g.mesh.Vertices)
			g.mesh.Vertices = append(g.mesh.Vertices, v)
		}
	}
	if q < 0 && !(g.mesh.Vertices[point.ids[0]].Z > g.mesh.Vertices[point.ids[1]].Z) {
		return 0, geom.ErrConvergence
	}
	g.points = append(g.points, point)
	return len(g.points) - 1, nil
}

func (p *cuspidalCutDisplay) germ(xy geom.Vec2) int {
	if xy.Y < p.cut.solid.vertices[p.cut.history.Vertices[0]].Y {
		return 0
	}
	return 1
}

func (p *cuspidalCutDisplay) germPoint(germ int, q float64) (geom.Vec3, error) {
	edge := p.cut.solid.edges[p.cut.history.Germs[germ]]
	if edge.Start != p.cut.history.Vertices[0] {
		q = 1 - q
	}
	return edge.Curve.Eval(cuspidalCutDisplayLerp(edge.Range.Min, edge.Range.Max, q))
}

func (g *cuspidalCutDisplayGrid) addStrip(previous, next []int) {
	if len(previous) == 0 {
		return
	}
	for j := 1; j < len(next); j++ {
		a, b, c, d := previous[j-1], next[j-1], next[j], previous[j]
		g.faces = append(g.faces, [3]int{a, b, c})
		if a != d {
			g.faces = append(g.faces, [3]int{a, c, d})
		}
	}
}

func (g *cuspidalCutDisplayGrid) emit(ctx context.Context, o parametric.TessellationOptions) error {
	boundary := make(map[[2]int]bool)
	for _, face := range g.faces {
		if err := ctx.Err(); err != nil {
			return err
		}
		var points [3]cuspidalCutDisplayPoint
		for i, id := range face {
			points[i] = g.points[id]
			other := face[(i+1)%3]
			if boundary[[2]int{other, id}] {
				delete(boundary, [2]int{other, id})
			} else if boundary[[2]int{id, other}] {
				return geom.ErrConvergence
			} else {
				boundary[[2]int{id, other}] = true
			}
		}
		turn, err := geom.Orient2D(points[0].xy, points[1].xy, points[2].xy)
		if err != nil || turn <= 0 || (points[0].q >= 0 && points[1].q >= 0 && points[2].q >= 0) {
			return geom.ErrConvergence
		}
		for owner, roof := range g.source.cut.sources[1:] {
			var s samples
			var p [3]geom.Vec3
			for i, point := range points {
				s.uv[i], s.ids[i], s.edge[i] = point.uv[owner], point.ids[owner], internalSide()
				p[i] = g.mesh.Vertices[s.ids[i]]
			}
			if g.source.reversed[owner] {
				s.uv[1], s.uv[2] = s.uv[2], s.uv[1]
				p[1], p[2] = p[2], p[1]
			}
			if err := check(roof, s, p, o.Deflection, o.Angle); err != nil {
				return err
			}
			if owner == 1 {
				s.ids[1], s.ids[2] = s.ids[2], s.ids[1]
			}
			g.mesh.Triangles = append(g.mesh.Triangles, s.ids)
		}
	}
	// Visit edges in projected face order so wall triangles have canonical
	// indices. Both caps share this incidence ledger and the original seam.
	for _, face := range g.faces {
		for i, id := range face {
			next := face[(i+1)%3]
			if !boundary[[2]int{id, next}] {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			a, b := g.points[id], g.points[next]
			if err := g.closeEdge(a, b, o.Deflection); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *cuspidalCutDisplayGrid) closeEdge(a, b cuspidalCutDisplayPoint, deflection float64) error {
	if a.q >= 0 && b.q >= 0 {
		germ := g.source.germ(a.xy)
		if a.q == 0 {
			germ = g.source.germ(b.xy)
		} else if b.q > 0 && g.source.germ(b.xy) != germ {
			return geom.ErrConvergence
		}
		middle, err := g.source.germPoint(germ, (a.q+b.q)/2)
		if err != nil {
			return err
		}
		chord := g.mesh.Vertices[a.ids[0]].Scale(.5).Add(g.mesh.Vertices[b.ids[0]].Scale(.5))
		if !closePoint(middle, chord, deflection) {
			return errRefine
		}
		return nil
	}
	d := g.source.domain
	onLongitudinalWall := a.xy.X == b.xy.X && (a.xy.X == d.UMin || a.xy.X == d.UMax)
	onTransverseWall := a.xy.Y == b.xy.Y && (a.xy.Y == d.VMin || a.xy.Y == d.VMax)
	if !onLongitudinalWall && !onTransverseWall {
		return geom.ErrConvergence
	}
	if b.ids[0] != b.ids[1] {
		g.mesh.Triangles = append(g.mesh.Triangles, [3]int{a.ids[1], b.ids[1], b.ids[0]})
	}
	if a.ids[0] != a.ids[1] {
		g.mesh.Triangles = append(g.mesh.Triangles, [3]int{a.ids[1], b.ids[0], a.ids[0]})
	}
	return nil
}
