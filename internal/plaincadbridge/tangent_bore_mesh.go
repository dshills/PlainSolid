package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type tangentSample struct {
	angle     float64
	direction geom.Vec2
	outer     geom.Vec2
}

func (h *tangentBlindBore) samples(o parametric.TessellationOptions) ([]tangentSample, error) {
	lo, hi := h.box.Min, h.box.Max
	// Cardinal and corner rays divide the entry cap into eight sectors.
	// Adjacent perimeter points in a sector always lie on one straight side.
	ends := []tangentSample{
		{0, geom.Vec2{X: 1}, geom.Vec2{X: hi.X}},
		{math.Atan2(hi.Y, hi.X), geom.Vec2{}, geom.Vec2{X: hi.X, Y: hi.Y}},
		{math.Pi / 2, geom.Vec2{Y: 1}, geom.Vec2{Y: hi.Y}},
		{math.Atan2(hi.Y, lo.X), geom.Vec2{}, geom.Vec2{X: lo.X, Y: hi.Y}},
		{math.Pi, geom.Vec2{X: -1}, geom.Vec2{X: lo.X}},
		{math.Atan2(lo.Y, lo.X) + 2*math.Pi, geom.Vec2{}, geom.Vec2{X: lo.X, Y: lo.Y}},
		{3 * math.Pi / 2, geom.Vec2{Y: -1}, geom.Vec2{Y: lo.Y}},
		{math.Atan2(lo.Y, hi.X) + 2*math.Pi, geom.Vec2{}, geom.Vec2{X: hi.X, Y: lo.Y}},
		{2 * math.Pi, geom.Vec2{X: 1}, geom.Vec2{X: hi.X}},
	}
	var samples []tangentSample
	for i := 0; i+1 < len(ends); i++ {
		a, b := ends[i], ends[i+1]
		span := b.angle - a.angle
		if span <= 0 {
			return nil, geom.ErrUnrepresentable
		}
		n := 1
		for {
			half := span / (2 * float64(n))
			if 2*h.radius*math.Pow(math.Sin(half/2), 2) <= o.Deflection && half <= o.Angle {
				break
			}
			if n > o.MaxSubdivisions/2 {
				return nil, parametric.ErrBudget
			}
			n *= 2
		}
		if len(samples)+n > o.MaxSubdivisions || len(samples)+n > o.MaxTriangles/9 {
			return nil, parametric.ErrBudget
		}
		if a.direction == (geom.Vec2{}) {
			length := math.Hypot(a.outer.X, a.outer.Y)
			a.direction = geom.Vec2{X: a.outer.X / length, Y: a.outer.Y / length}
		}
		samples = append(samples, a)
		for j := 1; j < n; j++ {
			angle := a.angle + span*float64(j)/float64(n)
			sn, cs := math.Sincos(angle)
			outer := geom.Vec2{}
			if a.outer.X == b.outer.X {
				outer = geom.Vec2{X: a.outer.X, Y: a.outer.X * (sn / cs)}
			} else {
				outer = geom.Vec2{X: a.outer.Y * (cs / sn), Y: a.outer.Y}
			}
			samples = append(samples, tangentSample{angle, geom.Vec2{X: cs, Y: sn}, outer})
		}
	}
	return samples, nil
}

func (h *tangentBlindBore) tessellate(o parametric.TessellationOptions) (mesh.Mesh, error) {
	samples, err := h.samples(o)
	if err != nil {
		return mesh.Mesh{}, err
	}
	var m mesh.Mesh
	add := func(p geom.Vec2, z float64) int {
		id := len(m.Vertices)
		m.Vertices = append(m.Vertices, h.placement.Apply(geom.Vec3{X: p.X, Y: p.Y, Z: z}))
		return id
	}
	type section struct{ top, back, rim, floor int }
	ring := make([]section, len(samples))
	contact := [2]int{}
	for i, s := range samples {
		radial := geom.Vec2{X: s.direction.X * h.radius, Y: s.direction.Y * h.radius}
		row := section{top: add(s.outer, 0), back: add(s.outer, h.box.Max.Z), floor: add(radial, h.depth)}
		if s.direction == (geom.Vec2{Y: -1}) {
			row.rim = row.top
			contact = [2]int{row.top, row.floor}
		} else {
			row.rim = add(radial, 0)
		}
		ring[i] = row
	}
	backCenter, floorCenter := add(geom.Vec2{}, h.box.Max.Z), add(geom.Vec2{}, h.depth)
	reflected := h.placement.Determinant() < 0
	triangle := func(a, b, c int) {
		if reflected {
			b, c = c, b
		}
		m.Triangles = append(m.Triangles, [3]int{a, b, c})
	}
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		// Entry cap runs from the inscribed bore polygon to the box perimeter.
		if b.top != b.rim {
			triangle(a.top, b.rim, b.top)
		}
		if a.top != a.rim {
			triangle(a.top, a.rim, b.rim)
		}
		// Split the two side patches incident to the contact generator at
		// the blind floor so both share precisely the same contact edge.
		switch {
		case a.top == a.rim:
			triangle(a.top, b.top, a.floor)
			triangle(a.floor, b.top, b.back)
			triangle(a.floor, b.back, a.back)
		case b.top == b.rim:
			triangle(a.top, b.top, b.floor)
			triangle(a.top, b.floor, a.back)
			triangle(a.back, b.floor, b.back)
		default:
			triangle(a.top, b.top, b.back)
			triangle(a.top, b.back, a.back)
		}
		triangle(a.rim, b.floor, b.rim)
		triangle(a.rim, a.floor, b.floor)
		triangle(floorCenter, b.floor, a.floor)
		triangle(backCenter, a.back, b.back)
	}
	if err := validateTangentDisplay(m, contact); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}

func validateTangentDisplay(m mesh.Mesh, contact [2]int) error {
	if err := m.Validate(); err != nil {
		return err
	}
	type incidence struct{ count, direction int }
	edges := make(map[[2]int]incidence)
	seen := make(map[geom.Vec3]bool, len(m.Vertices))
	for _, p := range m.Vertices {
		if seen[p] {
			return geom.ErrUnrepresentable
		}
		seen[p] = true
	}
	for _, t := range m.Triangles {
		for i, a := range t {
			b := t[(i+1)%3]
			k := [2]int{min(a, b), max(a, b)}
			u := edges[k]
			u.count++
			if a < b {
				u.direction++
			} else {
				u.direction--
			}
			edges[k] = u
		}
	}
	if edges[contact] != (incidence{count: 4}) {
		return errors.New("tangent display did not preserve the exact contact generator")
	}
	for e, u := range edges {
		if e != contact && u != (incidence{count: 2}) {
			return errors.New("tangent display has an open or inconsistently oriented edge")
		}
	}
	return nil
}

func (h *tangentBlindBore) displayEdges(maxUses, maxSegments int) (displayEdgeResult, error) {
	// At each bore ring the side contact is the seam of one complete circle.
	// The contact generator itself belongs to the side and cylinder patches.
	// The top stock edge splits at the circle contact. Its two uses
	// become four, in addition to the four uses of the contact generator.
	out := displayEdgeResult{Segments: []float64{}, EdgeUses: 34}
	o, err := tessellationOptions(request{})
	if err != nil {
		return out, err
	}
	samples, err := h.samples(o)
	if err != nil {
		return out, err
	}
	if out.EdgeUses > maxUses || 14+2*len(samples) > maxSegments {
		return out, &failureError{"LIMIT", "native edges exceed the display segment budget"}
	}
	line := func(a, b geom.Vec3) {
		for _, p := range []geom.Vec3{a, b} {
			q := h.placement.Apply(p)
			out.Segments = append(out.Segments, q.X, q.Y, q.Z)
		}
	}
	lo, hi := h.box.Min, h.box.Max
	corners := [4]geom.Vec3{{X: lo.X, Y: lo.Y}, {X: hi.X, Y: lo.Y}, {X: hi.X, Y: hi.Y}, {X: lo.X, Y: hi.Y}}
	back := geom.Vec3{Z: hi.Z}
	for i, a := range corners {
		b := corners[(i+1)%len(corners)]
		if i == 0 {
			contact := geom.Vec3{Y: lo.Y}
			line(a, contact)
			line(contact, b)
		} else {
			line(a, b)
		}
		line(a.Add(back), b.Add(back))
		line(a, a.Add(back))
	}
	for i, a := range samples {
		b := samples[(i+1)%len(samples)]
		for _, z := range []float64{0, h.depth} {
			line(geom.Vec3{X: a.direction.X * h.radius, Y: a.direction.Y * h.radius, Z: z},
				geom.Vec3{X: b.direction.X * h.radius, Y: b.direction.Y * h.radius, Z: z})
		}
	}
	line(geom.Vec3{Y: -h.radius}, geom.Vec3{Y: -h.radius, Z: h.depth})
	return out, nil
}
