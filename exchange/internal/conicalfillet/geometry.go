// Package conicalfillet reconstructs the exact geometry admitted by the
// STEP and IGES conical rolling-rim codecs. Each codec verifies its own
// analytic source records and directed incidences.
package conicalfillet

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

var errGeometry = errors.New("exchange: unsupported exact conical rim fillet with eccentric bore")

// Toroid holds the exact analytic support of an original rolling rim.
type Toroid struct {
	Center       geom.Vec3
	Major, Minor float64
}

// Source contains represented original support data already authenticated
// by a neutral-format graph decoder. CapZ and Toroids are in end order.
type Source struct {
	ConeCenter   geom.Vec2
	ConeStations [2]trimmed.WrappedLoftStation
	CapZ         [2]float64
	Toroids      [2]Toroid
	BoreCenter   geom.Vec2
	BoreRadius   float64
}

// Material contains the native solid and exact analytic support graph
// independent of source numbering and file dialect.
type Material struct {
	Solid        trimmed.Solid
	Graph        borefillet.Graph
	Cone, Bore   int
	Caps, Blends [2]int
	Display      DisplayRecipe
}

// Reconstruct proves the tangency and separation of a single exact conical
// rolling-rim body and derives the original shared model support graph.
func Reconstruct(ctx context.Context, f Source, o geom.SurfacePatchOptions) (Material, error) {
	stock, err := buildStock(ctx, f, o)
	if err != nil {
		return Material{}, err
	}
	display := DisplayRecipe{stock: stock, lower: f.Toroids[0].Minor, upper: f.Toroids[1].Minor,
		center: f.BoreCenter, radius: f.BoreRadius}
	result, err := trimmed.FilletAndBoreWrappedFrustumSet(ctx, stock, display.lower, display.upper,
		[]trimmed.FilletedFrustumBoreCut{{Center: display.center, Radius: display.radius}}, o)
	if err != nil {
		return Material{}, err
	}
	graph, err := makeGraph(f, result)
	if err != nil {
		return Material{}, err
	}
	return Material{
		Solid: result.Solid, Graph: graph, Cone: result.Fillets.Lateral, Bore: result.Cuts[0].BoreFace,
		Caps: result.Fillets.Caps, Blends: result.Fillets.Blends, Display: display,
	}, nil
}

func rat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }

func buildStock(ctx context.Context, f Source,
	o geom.SurfacePatchOptions) (trimmed.Solid, error) {
	var out trimmed.Solid
	stock := f.ConeCenter
	if stock.X == f.BoreCenter.X && stock.Y == f.BoreCenter.Y {
		return out, errGeometry
	}
	lower, upper := f.ConeStations[0], f.ConeStations[1]
	z0, z1 := f.CapZ[0], f.CapZ[1]
	if z0 >= lower.Z || lower.Z >= upper.Z || z1 <= upper.Z || lower.Radius >= upper.Radius {
		return out, errGeometry
	}
	diff := func(a, b float64) *big.Rat { return new(big.Rat).Sub(rat(a), rat(b)) }
	slope := new(big.Rat).Quo(diff(upper.Radius, lower.Radius), diff(upper.Z, lower.Z))
	extension := func(z float64) *big.Rat {
		return new(big.Rat).Add(rat(lower.Radius), new(big.Rat).Mul(slope, diff(z, lower.Z)))
	}
	r0, err := geom.RepresentedExactScalar(extension(z0))
	if err != nil || r0 <= 0 {
		return out, errGeometry
	}
	height, err := geom.RepresentedExactScalar(diff(z1, z0))
	if err != nil {
		return out, err
	}
	base, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: stock.X, Y: stock.Y, Z: z0}, r0, height, o)
	if err != nil {
		return out, err
	}
	wall, _, err := geom.DraftWrappedFrustumExact(ctx, base, new(big.Rat), new(big.Rat).Sub(extension(z1), rat(r0)), o)
	if err != nil {
		return out, err
	}
	frustum, _, err := trimmed.NewWrappedFrustum(ctx, wall, 1e-9, o)
	return frustum, err
}

func makeGraph(source Source, result trimmed.WrappedFilletedFrustumBoreSet) (borefillet.Graph, error) {
	fail := func() (borefillet.Graph, error) {
		return borefillet.Graph{}, errGeometry
	}
	s, h := result.Solid, result.Fillets
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(faces) != 6 || len(edges) != 28 || len(vertices) != 24 || len(result.Cuts) != 1 {
		return fail()
	}
	g := borefillet.Graph{Vertices: make([][3]*big.Rat, len(vertices)),
		Edges: make([]borefillet.Edge, len(edges)), Faces: make([]borefillet.Face, len(faces))}
	for i, v := range vertices {
		g.Vertices[i] = borefillet.Vec(v.X, v.Y, v.Z)
	}
	for i, e := range edges {
		g.Edges[i] = borefillet.Edge{Start: e.Start, End: e.End, Forward: true}
	}
	for i, f := range faces {
		g.Faces[i] = borefillet.Face{Reversed: f.Reversed, Outer: f.Outer, Holes: f.Holes}
	}
	stock := borefillet.Vec(source.ConeCenter.X, source.ConeCenter.Y, 0)
	bore := borefillet.Vec(source.BoreCenter.X, source.BoreCenter.Y, 0)
	ring := func(ids []int, center [3]*big.Rat, r *big.Rat) bool {
		if len(ids) != 4 {
			return false
		}
		for _, id := range ids {
			g.Edges[id].Circle = borefillet.Circle{Center: center, Radius: r}
			if !checkEdgeSense(&g, id, true) {
				return false
			}
		}
		return true
	}
	for end, c := range h.Circles {
		t := source.Toroids[end]
		finite := source.ConeStations[end]
		if t.Center.X != source.ConeCenter.X || t.Center.Y != source.ConeCenter.Y ||
			c.Center[0].Cmp(rat(t.Major)) != 0 || c.Center[1].Cmp(rat(t.Center.Z)) != 0 ||
			c.Radius.Cmp(rat(t.Minor)) != 0 || c.Tangency[0].Cmp(rat(finite.Radius)) != 0 ||
			c.Tangency[1].Cmp(rat(finite.Z)) != 0 {
			return fail()
		}
		capZ, tanZ := rat(source.CapZ[end]), c.Tangency[1]
		if !ring(h.Rims[end][0][:], [3]*big.Rat{stock[0], stock[1], capZ}, c.Center[0]) ||
			!ring(h.Rims[end][1][:], [3]*big.Rat{stock[0], stock[1], tanZ}, c.Tangency[0]) ||
			!ring(result.Cuts[0].BoreRims[end], [3]*big.Rat{bore[0], bore[1], capZ}, rat(source.BoreRadius)) {
			return fail()
		}
		// The checked native rim constructor owns exactly one directed
		// meridian edge on both sides of each toroidal seam. Its span
		// is less than a semicircle, even at the obtuse upper fillet.
		seam := -1
		for _, u := range faces[h.Blends[end]].Outer {
			if g.Edges[u.Edge].Circle.Radius != nil {
				continue
			}
			if seam >= 0 && seam != u.Edge {
				return fail()
			}
			seam = u.Edge
		}
		if seam < 0 {
			return fail()
		}
		centerX := new(big.Rat).Add(stock[0], c.Center[0])
		g.Edges[seam].Circle = borefillet.Circle{
			Center: [3]*big.Rat{centerX, stock[1], c.Center[1]}, Radius: c.Radius, Meridian: true,
		}
		if !checkEdgeSense(&g, seam, false) {
			return fail()
		}
	}
	return g, nil
}

func checkEdgeSense(g *borefillet.Graph, edge int, quarter bool) bool {
	e := &g.Edges[edge]
	dim, fixed := 1, 2
	if e.Circle.Meridian {
		dim, fixed = 2, 1
	}
	var xy [2][2]*big.Rat
	for i, p := range [2][3]*big.Rat{g.Vertices[e.Start], g.Vertices[e.End]} {
		if p[fixed].Cmp(e.Circle.Center[fixed]) != 0 {
			return false
		}
		x := new(big.Rat).Sub(p[0], e.Circle.Center[0])
		y := new(big.Rat).Sub(p[dim], e.Circle.Center[dim])
		r2 := new(big.Rat).Mul(e.Circle.Radius, e.Circle.Radius)
		if new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)).Cmp(r2) != 0 ||
			e.Circle.Meridian && x.Sign() < 0 {
			return false
		}
		xy[i] = [2]*big.Rat{x, y}
	}
	a, b := xy[0], xy[1]
	if quarter && new(big.Rat).Add(new(big.Rat).Mul(a[0], b[0]), new(big.Rat).Mul(a[1], b[1])).Sign() != 0 {
		return false
	}
	cross := new(big.Rat).Mul(a[0], b[1]).Cmp(new(big.Rat).Mul(a[1], b[0]))
	e.Forward = cross > 0
	return cross != 0
}
