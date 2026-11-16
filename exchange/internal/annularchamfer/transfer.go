package annularchamfer

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

// Band owns one original partition of an affine circular wall. A nil
// patch in the face-indexed band list identifies a planar annular cap.
type Band struct {
	Patch *geom.WrappedSurfacePatch
	X, Y  *big.Rat
	Z, R  [2]*big.Rat
}

// Graph retains every original model vertex, shared circle quarter, seam
// generator, and plane/wall face use. Bands preserves the source evaluator.
type Graph struct {
	borefillet.Graph
	Bands []Band
}

// Extract checks both original wall chains and both cap wires, including
// global strict nesting. Exactly three consecutive slope laws on each
// axis prove a central tapered stock and two opposite rim chamfers.
func Extract(ctx context.Context, s trimmed.Solid, o geom.SurfacePatchOptions) (Graph, error) {
	if ctx == nil {
		return Graph{}, geom.ErrInvalid
	}
	native := s.Topology()
	if native.Faces < 8 || native.Faces > 12 || native.InnerWires != 2 || len(native.Shells) != 1 || native.EulerCharacteristic != 0 {
		return Graph{}, geom.ErrConvergence
	}
	p, err := s.ValidateWrappedEccentricCircularStationBore(ctx, o)
	if err != nil {
		return Graph{}, err
	}
	if !p.Complete || p.Outer.Center[0].Cmp(p.Inner.Center[0]) == 0 && p.Outer.Center[1].Cmp(p.Inner.Center[1]) == 0 ||
		!chamferSlopes(p.Outer, false) || !chamferSlopes(p.Inner, true) {
		return Graph{}, geom.ErrConvergence
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	g := Graph{Graph: borefillet.Graph{
		Vertices: make([][3]*big.Rat, len(vertices)), Edges: make([]borefillet.Edge, len(edges)), Faces: make([]borefillet.Face, len(faces)),
	}, Bands: make([]Band, len(faces))}
	for i, v := range vertices {
		g.Vertices[i] = borefillet.Vec(v.X, v.Y, v.Z)
	}
	for i, e := range edges {
		g.Edges[i] = borefillet.Edge{Start: e.Start, End: e.End}
	}
	for i, f := range faces {
		g.Faces[i] = borefillet.Face{Reversed: f.Reversed, Outer: f.Outer, Holes: f.Holes}
	}
	for end, index := range [2]int{0, len(p.Outer.Heights) - 1} {
		g.Faces[p.Caps[end]].Origin = [3]*big.Rat{new(big.Rat), new(big.Rat), p.Outer.Heights[index]}
	}
	for side, ids := range [][]int{p.OuterWalls, p.InnerWalls} {
		chain := [2]trimmed.WrappedStationLoftProof{p.Outer, p.Inner}[side]
		for i, id := range ids {
			patch, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok || faces[id].Reversed != (side == 1) {
				return Graph{}, geom.ErrConvergence
			}
			b := Band{Patch: patch, X: chain.Center[0], Y: chain.Center[1],
				Z: [2]*big.Rat{chain.Heights[i], chain.Heights[i+1]}, R: [2]*big.Rat{chain.Radii[i], chain.Radii[i+1]}}
			g.Bands[id] = b
			embedding, err := geom.CertifyWrappedFrustumEmbedding(ctx, patch, o)
			if err != nil {
				return Graph{}, err
			}
			if err := addBandEdges(&g, id, b, embedding.QuarterParameters); err != nil {
				return Graph{}, err
			}
		}
	}
	for _, e := range g.Edges {
		if e.Circle.Radius == nil && !e.Forward {
			return Graph{}, geom.ErrConvergence
		}
	}
	return g, nil
}

func chamferSlopes(w trimmed.WrappedStationLoftProof, inward bool) bool {
	n := len(w.Heights) - 1
	if n < 3 || n > 5 {
		return false
	}
	type run struct {
		start int
		slope *big.Rat
	}
	var runs []run
	for i := range n {
		m := new(big.Rat).Quo(new(big.Rat).Sub(w.Radii[i+1], w.Radii[i]), new(big.Rat).Sub(w.Heights[i+1], w.Heights[i]))
		if len(runs) == 0 || m.Cmp(runs[len(runs)-1].slope) != 0 {
			runs = append(runs, run{i, m})
		}
	}
	if len(runs) != 3 || runs[1].slope.Sign() == 0 {
		return false
	}
	m := runs[1].slope
	if inward {
		if runs[0].slope.Cmp(m) >= 0 || runs[2].slope.Cmp(m) <= 0 {
			return false
		}
	} else if runs[0].slope.Cmp(m) <= 0 || runs[2].slope.Cmp(m) >= 0 {
		return false
	}
	// The central wall must extend to a positive-radius sharp stock at
	// both end planes before the four material-removing bands are cut.
	i := runs[1].start
	for _, j := range []int{0, n} {
		r := new(big.Rat).Add(w.Radii[i], new(big.Rat).Mul(m, new(big.Rat).Sub(w.Heights[j], w.Heights[i])))
		if r.Sign() <= 0 {
			return false
		}
	}
	return true
}

func addBandEdges(g *Graph, id int, b Band, quarter [5]float64) error {
	d := b.Patch.Domain()
	for _, u := range g.Faces[id].Outer {
		e := &g.Edges[u.Edge]
		a, c := g.Vertices[e.Start], g.Vertices[e.End]
		if u.UVStart.X == u.UVEnd.X {
			if !seamEndpoint(a, b, c) && !seamEndpoint(c, b, a) {
				return geom.ErrConvergence
			}
			e.Forward = true
			continue
		}
		lo, hi := min(u.UVStart.X, u.UVEnd.X), max(u.UVStart.X, u.UVEnd.X)
		ok := false
		for i := range 4 {
			ok = ok || lo == quarter[i] && hi == quarter[i+1]
		}
		if !ok || u.UVStart.Y != u.UVEnd.Y {
			return geom.ErrConvergence
		}
		end := 0
		if u.UVStart.Y == d.VMax {
			end = 1
		} else if u.UVStart.Y != d.VMin {
			return geom.ErrConvergence
		}
		if a[2].Cmp(b.Z[end]) != 0 || c[2].Cmp(b.Z[end]) != 0 {
			return geom.ErrConvergence
		}
		p, q := [2]*big.Rat{new(big.Rat).Sub(a[0], b.X), new(big.Rat).Sub(a[1], b.Y)},
			[2]*big.Rat{new(big.Rat).Sub(c[0], b.X), new(big.Rat).Sub(c[1], b.Y)}
		dot := func(x, y [2]*big.Rat) *big.Rat {
			return new(big.Rat).Add(new(big.Rat).Mul(x[0], y[0]), new(big.Rat).Mul(x[1], y[1]))
		}
		r := b.R[end]
		r2 := new(big.Rat).Mul(r, r)
		if dot(p, q).Sign() != 0 || dot(p, p).Cmp(r2) != 0 || dot(q, q).Cmp(r2) != 0 {
			return geom.ErrConvergence
		}
		if prior := e.Circle; prior.Radius != nil && (prior.Radius.Cmp(r) != 0 ||
			prior.Center[0].Cmp(b.X) != 0 || prior.Center[1].Cmp(b.Y) != 0 || prior.Center[2].Cmp(b.Z[end]) != 0) {
			return geom.ErrConvergence
		}
		e.Circle = borefillet.Circle{Center: [3]*big.Rat{b.X, b.Y, b.Z[end]}, Radius: r}
		e.Forward = new(big.Rat).Mul(p[0], q[1]).Cmp(new(big.Rat).Mul(p[1], q[0])) > 0
	}
	return nil
}

func seamEndpoint(lower [3]*big.Rat, b Band, upper [3]*big.Rat) bool {
	return lower[2].Cmp(b.Z[0]) == 0 && lower[1].Cmp(b.Y) == 0 && lower[0].Cmp(new(big.Rat).Add(b.X, b.R[0])) == 0 &&
		upper[2].Cmp(b.Z[1]) == 0 && upper[1].Cmp(b.Y) == 0 && upper[0].Cmp(new(big.Rat).Add(b.X, b.R[1])) == 0
}
