package borefillet

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type SupportKind uint8

const (
	Plane SupportKind = iota
	Cylinder
	Torus
)

// Graph retains the original shared vertices, model edge incidences, and
// face orientations. Coordinates and support data are in the caller's unit.
type Graph struct {
	Vertices [][3]*big.Rat
	Edges    []Edge
	Faces    []Face
}

type Edge struct {
	Start, End int
	Circle     Circle // radius nil for a straight generator
	Forward    bool   // model direction agrees with the positive circle angle
}

type Circle struct {
	Center   [3]*big.Rat
	Radius   *big.Rat
	Meridian bool // XZ plane, positive angle rotates +X towards +Z
}

type Face struct {
	Kind         SupportKind
	Origin       [3]*big.Rat
	Major, Minor *big.Rat // cylinder radius is Major
	Reversed     bool
	Outer        []trimmed.Use
	Holes        [][]trimmed.Use
}

func Vec(x, y, z float64) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).SetFloat64(x), new(big.Rat).SetFloat64(y), new(big.Rat).SetFloat64(z)}
}

// Extract proves the exact native body and resolves each original trim to
// one complete circular quadrant or straight generator. Both owner charts
// must supply the same model support for every shared rim.
func Extract(ctx context.Context, s trimmed.Solid, o geom.SurfacePatchOptions) (Graph, error) {
	p, err := s.ValidateWrappedCylinderBoreRimFillet(ctx, o)
	if err != nil {
		return Graph{}, err
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	g := Graph{Vertices: make([][3]*big.Rat, len(v)), Edges: make([]Edge, len(edges)), Faces: make([]Face, len(faces))}
	for i, q := range v {
		g.Vertices[i] = Vec(q.X, q.Y, q.Z)
	}
	for i, e := range edges {
		g.Edges[i].Start, g.Edges[i].End = e.Start, e.End
	}
	for i, f := range faces {
		g.Faces[i] = Face{Reversed: f.Reversed, Outer: f.Outer, Holes: f.Holes}
	}
	basis := func(c geom.WrappedCylinderEmbedding, z *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{c.CenterX, c.CenterY, z}
	}
	lines := make([]bool, len(edges))
	wall := func(id int, cylinder geom.WrappedCylinderEmbedding, end int) error {
		domain := faces[id].Surface.(*geom.WrappedSurfacePatch).Domain()
		parameters := cylinder.QuarterParameters
		f := &g.Faces[id]
		f.Kind, f.Origin, f.Major = Cylinder, basis(cylinder, cylinder.LowerZ), cylinder.Radius
		if end >= 0 {
			shoulder := [2]*big.Rat{cylinder.LowerZ, cylinder.UpperZ}[end]
			f.Kind, f.Origin, f.Minor = Torus, basis(cylinder, shoulder), p.Fillets[end]
			f.Major = new(big.Rat).Add(cylinder.Radius, f.Minor)
			parameters = [5]float64{0, 1, 2, 3, 4}
		}
		for _, u := range faces[id].Outer {
			a, b := u.UVStart, u.UVEnd
			edge := &g.Edges[u.Edge]
			circle := Circle{Center: basis(cylinder, g.Vertices[edge.Start][2]), Radius: cylinder.Radius}
			if a.X == b.X {
				if end < 0 {
					lines[u.Edge] = true
					continue
				}
				circle = Circle{
					Center: [3]*big.Rat{new(big.Rat).Add(cylinder.CenterX, f.Major), cylinder.CenterY, f.Origin[2]},
					Radius: f.Minor, Meridian: true,
				}
			} else {
				original := false
				for i := range 4 {
					original = original || min(a.X, b.X) == parameters[i] && max(a.X, b.X) == parameters[i+1]
				}
				if !original {
					return geom.ErrConvergence
				}
				if end >= 0 && (end == 0 && a.Y == domain.VMax || end == 1 && a.Y == domain.VMin) {
					circle.Radius = f.Major
				}
			}
			if old := edge.Circle; old.Radius != nil && !equalCircles(old, circle) {
				return geom.ErrConvergence
			}
			edge.Circle = circle
		}
		return nil
	}
	if err := wall(p.Outer, p.Stock, -1); err != nil {
		return Graph{}, err
	}
	if err := wall(p.Bore, p.Hole, -1); err != nil {
		return Graph{}, err
	}
	for end, z := range [2]*big.Rat{p.Stock.LowerZ, p.Stock.UpperZ} {
		g.Faces[p.Caps[end]].Origin = [3]*big.Rat{new(big.Rat), new(big.Rat), z}
		if id := p.Blends[end]; id >= 0 {
			if err := wall(id, p.Hole, end); err != nil {
				return Graph{}, err
			}
		}
	}
	for i, e := range g.Edges {
		a, b := g.Vertices[e.Start], g.Vertices[e.End]
		if lines[i] {
			if e.Circle.Radius != nil || a[0].Cmp(b[0]) != 0 || a[1].Cmp(b[1]) != 0 || a[2].Cmp(b[2]) == 0 {
				return Graph{}, geom.ErrConvergence
			}
			g.Edges[i].Forward = true
			continue
		}
		if e.Circle.Radius == nil {
			return Graph{}, geom.ErrConvergence
		}
		g.Edges[i].Forward, err = circleSense(e.Circle, a, b)
		if err != nil {
			return Graph{}, err
		}
	}
	return g, nil
}

func equalCircles(a, b Circle) bool {
	return a.Meridian == b.Meridian && a.Radius.Cmp(b.Radius) == 0 && a.Center[0].Cmp(b.Center[0]) == 0 &&
		a.Center[1].Cmp(b.Center[1]) == 0 && a.Center[2].Cmp(b.Center[2]) == 0
}

func circleSense(c Circle, a, b [3]*big.Rat) (bool, error) {
	y, normal := 1, 2
	if c.Meridian {
		y, normal = 2, 1
	}
	if a[normal].Cmp(c.Center[normal]) != 0 || b[normal].Cmp(c.Center[normal]) != 0 {
		return false, geom.ErrConvergence
	}
	delta := func(p [3]*big.Rat) (*big.Rat, *big.Rat) {
		return new(big.Rat).Sub(p[0], c.Center[0]), new(big.Rat).Sub(p[y], c.Center[y])
	}
	ax, ay := delta(a)
	bx, by := delta(b)
	square := func(x, y *big.Rat) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	}
	r2 := new(big.Rat).Mul(c.Radius, c.Radius)
	dot := new(big.Rat).Add(new(big.Rat).Mul(ax, bx), new(big.Rat).Mul(ay, by))
	if dot.Sign() != 0 || square(ax, ay).Cmp(r2) != 0 || square(bx, by).Cmp(r2) != 0 {
		return false, geom.ErrConvergence
	}
	return new(big.Rat).Sub(new(big.Rat).Mul(ax, by), new(big.Rat).Mul(ay, bx)).Sign() > 0, nil
}
