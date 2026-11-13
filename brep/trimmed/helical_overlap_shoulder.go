package trimmed

import (
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type entrySectionUse struct {
	edge    int
	reverse bool
	a, b    geom.Vec2
	av, bv  int
}

func (c *entryClipper) section(phase float64) ([]entrySectionUse, error) {
	counts := make([]int, len(c.edges))
	for _, f := range c.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				counts[u.Edge]++
			}
		}
	}
	adjacency := map[int][]int{}
	var selected []int
	for i, e := range c.edges {
		if counts[i] != 1 {
			continue
		}
		a, b := c.r.Frame.ToLocal(c.vertices[e.Start]), c.r.Frame.ToLocal(c.vertices[e.End])
		if math.Abs(a.Z-c.depth) > c.source.tolerance || math.Abs(b.Z-c.depth) > c.source.tolerance {
			continue
		}
		adjacency[e.Start] = append(adjacency[e.Start], i)
		adjacency[e.End] = append(adjacency[e.End], i)
		selected = append(selected, i)
	}
	wall := 0
	if c.r.BoreRadius > 0 {
		wall = 1
	}
	frequency := map[int]int{}
	seam := -1
	for _, u := range c.source.faces[wall].Outer {
		frequency[u.Edge]++
		if frequency[u.Edge] == 2 {
			cut, ok := c.cuts[u.Edge]
			if !ok {
				return nil, geom.ErrDomain
			}
			seam = cut.id
		}
	}
	if seam < 0 || len(adjacency[seam]) != 2 || len(selected) < 3 {
		return nil, geom.ErrDomain
	}
	for _, edges := range adjacency {
		if len(edges) != 2 {
			return nil, geom.ErrDomain
		}
	}
	uv := func(p geom.Vec3) geom.Vec2 {
		v := c.r.Frame.ToLocal(p)
		turn := math.Atan2(v.Y, v.X)/(2*math.Pi) - phase
		turn -= math.Floor(turn)
		radius := math.Hypot(v.X, v.Y)
		stockRadius := c.r.OuterRadius
		if c.r.BoreRadius > 0 {
			stockRadius = c.r.BoreRadius
		}
		if math.Abs(radius-stockRadius) <= c.source.tolerance {
			radius = stockRadius
		}
		return geom.Vec2{X: turn, Y: radius}
	}
	first := -1
	for _, id := range adjacency[seam] {
		e := c.edges[id]
		p, err := e.Curve.Eval((e.Range.Min + e.Range.Max) / 2)
		if err != nil {
			return nil, err
		}
		if uv(p).X < .5 {
			if first >= 0 {
				return nil, geom.ErrDomain
			}
			first = id
		}
	}
	if first < 0 {
		return nil, geom.ErrDomain
	}
	seen := map[int]bool{}
	current, edge := seam, first
	var out []entrySectionUse
	last := geom.Vec2{X: 0, Y: uv(c.vertices[seam]).Y}
	for len(out) < len(selected) {
		if seen[edge] {
			return nil, geom.ErrDomain
		}
		seen[edge] = true
		e := c.edges[edge]
		next := e.End
		rev := false
		if e.Start != current {
			if e.End != current {
				return nil, geom.ErrDomain
			}
			next = e.Start
			rev = true
		}
		end := uv(c.vertices[next])
		if next == seam {
			end.X = 1
		}
		if end.X+1e-12 < last.X {
			return nil, geom.ErrDomain
		}
		out = append(out, entrySectionUse{edge, rev, last, end, current, next})
		last = end
		current = next
		if current == seam {
			break
		}
		adj := adjacency[current]
		edge = adj[0]
		if seen[edge] {
			edge = adj[1]
		}
	}
	if current != seam || len(out) != len(selected) {
		return nil, geom.ErrDomain
	}
	return out, nil
}
func (c *entryClipper) appendEntryRelief(radius float64) (int, int, error) {
	wall := 0
	if c.r.BoreRadius > 0 {
		wall = 1
	}
	twisted, ok := c.source.faces[wall].Surface.(geom.TwistedCylinderSurface)
	if !ok {
		return 0, 0, geom.ErrDomain
	}
	frame, stockRadius, twist, phase := twisted.Parameters()
	phaseDepth := twist*c.depth + phase
	section, e := c.section(phaseDepth)
	if e != nil {
		return 0, 0, e
	}
	seam := section[0].av
	theta := 2 * math.Pi * math.Remainder(phaseDepth, 1)
	x := frame.XAxis().Scale(math.Cos(theta)).Add(frame.YAxis().Scale(math.Sin(theta)))
	oriented, e := geom.NewFrame(frame.Origin(), x, frame.ZAxis())
	if e != nil {
		return 0, 0, e
	}
	relief, e := geom.NewTwistedCylinderSurface(oriented, radius, 0, 0)
	if e != nil {
		return 0, 0, e
	}
	lo, hi := len(c.vertices), len(c.vertices)+1
	for _, z := range []float64{0, c.depth} {
		p, e := relief.Eval(0, z)
		if e != nil {
			return 0, 0, e
		}
		c.vertices = append(c.vertices, p)
	}
	add := func(a, b int, s geom.Surface, p, q geom.Vec2) int {
		id := len(c.edges)
		c.edges = append(c.edges, parametric.Edge{Start: a, End: b, Curve: surfaceTrimCurve{s, p, q, nil}, Range: parametric.Interval{Max: 1}})
		return id
	}
	bottom := add(lo, lo, relief, geom.Vec2{}, geom.Vec2{X: 1})
	axial := add(lo, hi, relief, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: c.depth})
	top := add(hi, hi, relief, geom.Vec2{Y: c.depth}, geom.Vec2{X: 1, Y: c.depth})
	reliefID := len(c.faces)
	c.faces = append(c.faces, Face{Surface: relief, Reversed: c.r.BoreRadius > 0, Outer: []Use{{Edge: bottom, UVEnd: geom.Vec2{X: 1}}, {Edge: axial, UVStart: geom.Vec2{X: 1}, UVEnd: geom.Vec2{X: 1, Y: c.depth}}, {Edge: top, Reversed: true, UVStart: geom.Vec2{X: 1, Y: c.depth}, UVEnd: geom.Vec2{Y: c.depth}}, {Edge: axial, Reversed: true, UVStart: geom.Vec2{Y: c.depth}}}})
	lower, upper := math.Min(radius, stockRadius), math.Max(radius, stockRadius)
	shoulder, e := geom.NewTwistedAnnularSurface(oriented, lower, upper, c.depth, 0)
	if e != nil {
		return 0, 0, e
	}
	radial := add(hi, seam, shoulder, geom.Vec2{X: 1, Y: radius}, geom.Vec2{X: 1, Y: stockRadius})
	face := Face{Surface: shoulder}
	if c.r.BoreRadius == 0 {
		face.Outer = append(face.Outer, Use{Edge: top, UVStart: geom.Vec2{Y: radius}, UVEnd: geom.Vec2{X: 1, Y: radius}}, Use{Edge: radial, UVStart: geom.Vec2{X: 1, Y: radius}, UVEnd: geom.Vec2{X: 1, Y: stockRadius}})
		for i := len(section) - 1; i >= 0; i-- {
			s := section[i]
			face.Outer = append(face.Outer, Use{Edge: s.edge, Reversed: !s.reverse, UVStart: s.b, UVEnd: s.a})
		}
		face.Outer = append(face.Outer, Use{Edge: radial, Reversed: true, UVStart: geom.Vec2{Y: stockRadius}, UVEnd: geom.Vec2{Y: radius}})
	} else {
		for _, s := range section {
			face.Outer = append(face.Outer, Use{Edge: s.edge, Reversed: s.reverse, UVStart: s.a, UVEnd: s.b})
		}
		face.Outer = append(face.Outer, Use{Edge: radial, Reversed: true, UVStart: geom.Vec2{X: 1, Y: stockRadius}, UVEnd: geom.Vec2{X: 1, Y: radius}}, Use{Edge: top, Reversed: true, UVStart: geom.Vec2{X: 1, Y: radius}, UVEnd: geom.Vec2{Y: radius}}, Use{Edge: radial, UVStart: geom.Vec2{Y: radius}, UVEnd: geom.Vec2{Y: stockRadius}})
	}
	shoulderID := len(c.faces)
	c.faces = append(c.faces, face)
	if c.r.BoreRadius == 0 {
		diskBody, e := parametric.Cylinder(oriented, radius, 1)
		if e != nil {
			return 0, 0, e
		}
		disk := diskBody.Faces()[1].Surface
		center := len(c.vertices)
		c.vertices = append(c.vertices, oriented.Origin())
		pole := len(c.edges)
		c.edges = append(c.edges, parametric.Edge{Start: center, End: center})
		seamEdge := add(center, lo, disk, geom.Vec2{X: 2 * math.Pi}, geom.Vec2{X: 2 * math.Pi, Y: radius})
		c.faces = append(c.faces, Face{Surface: disk, Outer: []Use{{Edge: pole, UVEnd: geom.Vec2{X: 2 * math.Pi}}, {Edge: seamEdge, UVStart: geom.Vec2{X: 2 * math.Pi}, UVEnd: geom.Vec2{X: 2 * math.Pi, Y: radius}}, {Edge: bottom, Reversed: true, UVStart: geom.Vec2{X: 2 * math.Pi, Y: radius}, UVEnd: geom.Vec2{Y: radius}}, {Edge: seamEdge, Reversed: true, UVStart: geom.Vec2{Y: radius}}}})
	} else {
		cap, e := geom.NewTwistedAnnularSurface(frame, radius, c.r.OuterRadius, 0, math.Remainder(phaseDepth, 1))
		if e != nil {
			return 0, 0, e
		}
		outerBottom, ok := c.edgeIDs[0]
		if !ok {
			return 0, 0, geom.ErrDomain
		}
		outerVertex := c.edges[outerBottom].Start
		seamEdge := add(lo, outerVertex, cap, geom.Vec2{X: 1, Y: radius}, geom.Vec2{X: 1, Y: c.r.OuterRadius})
		c.faces = append(c.faces, Face{Surface: cap, Outer: []Use{{Edge: bottom, UVStart: geom.Vec2{Y: radius}, UVEnd: geom.Vec2{X: 1, Y: radius}}, {Edge: seamEdge, UVStart: geom.Vec2{X: 1, Y: radius}, UVEnd: geom.Vec2{X: 1, Y: c.r.OuterRadius}}, {Edge: outerBottom, Reversed: true, UVStart: geom.Vec2{X: 1, Y: c.r.OuterRadius}, UVEnd: geom.Vec2{Y: c.r.OuterRadius}}, {Edge: seamEdge, Reversed: true, UVStart: geom.Vec2{Y: c.r.OuterRadius}, UVEnd: geom.Vec2{Y: radius}}}})
	}
	return reliefID, shoulderID, nil
}
func (c *entryClipper) solid() (Solid, error) {
	used := make([]bool, len(c.vertices))
	for _, edge := range c.edges {
		used[edge.Start] = true
		used[edge.End] = true
	}
	ids := make([]int, len(c.vertices))
	var vertices []geom.Vec3
	for i, yes := range used {
		if yes {
			ids[i] = len(vertices)
			vertices = append(vertices, c.vertices[i])
		}
	}
	for i := range c.edges {
		c.edges[i].Start = ids[c.edges[i].Start]
		c.edges[i].End = ids[c.edges[i].End]
	}
	return NewSolidContext(c.ctx, vertices, c.edges, c.faces, c.source.tolerance)
}
