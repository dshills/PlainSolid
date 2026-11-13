package trimmed

import (
	"context"
	"fmt"
	"plainsolid/brep"
	"plainsolid/geom"
)

// AxialBoreStep specifies a circular cut measured inward from the selected
// upper cap. Depth zero means through to the selected lower cap.
type AxialBoreStep struct{ Radius, Depth float64 }

// SewPlanarSteppedBore sews a coaxial union of finite circular cuts into an
// existing solid. lowCap=-1 selects a blind operation. The common maximum-radius
// envelope must be strictly separated from all other faces and existing holes.
// Cap charts must be parallel signed-cardinal XY charts. Source face IDs survive.
func SewPlanarSteppedBore(ctx context.Context, stock Solid, lowCap, highCap int, center geom.Vec2, steps []AxialBoreStep) (Solid, error) {
	return sewPlanarSteppedBore(ctx, stock, lowCap, highCap, center, steps, true, false)
}

// ProvePlanarSteppedBore checks the conservative cutter envelope without
// allocating a resulting stock boundary. The cut constructor still validates
// the sewn topology before publication.
func ProvePlanarSteppedBore(ctx context.Context, stock Solid, lowCap, highCap int, center geom.Vec2, steps []AxialBoreStep) error {
	_, err := sewPlanarSteppedBore(ctx, stock, lowCap, highCap, center, steps, false, false)
	return err
}

func sewPlanarSteppedBore(ctx context.Context, stock Solid, lowCap, highCap int, center geom.Vec2, steps []AxialBoreStep, build, draft bool) (Solid, error) {
	fail := func() (Solid, error) { return Solid{}, geom.ErrDomain }
	if ctx == nil || highCap < 0 || highCap >= len(stock.faces) || lowCap < -1 || lowCap >= len(stock.faces) || lowCap == highCap || len(steps) == 0 || len(steps) > 32 {
		return Solid{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Solid{}, err
	}
	plane, ok := stock.faces[highCap].Surface.(geom.PlaneSurface)
	if !ok || stock.faces[highCap].Reversed || !prismCardinal(plane.Frame()) {
		return fail()
	}
	frame := plane.Frame()
	span, maxR, maxDepth := 0., 0., 0.
	if lowCap >= 0 {
		p, ok := stock.faces[lowCap].Surface.(geom.PlaneSurface)
		if !ok {
			return fail()
		}
		q, e := prismExactLocal(frame, p.Frame().Origin())
		if e != nil || q.Z >= 0 {
			return fail()
		}
		span = -q.Z
	}
	for _, s := range steps {
		if !finite(s.Radius) || !finite(s.Depth) || s.Radius <= stock.tolerance || s.Depth < 0 || s.Depth == 0 && lowCap < 0 {
			return fail()
		}
		d := s.Depth
		if d == 0 {
			d = span
		}
		if lowCap >= 0 && d > span {
			return fail()
		}
		maxR = max(maxR, s.Radius)
		maxDepth = max(maxDepth, d)
	}
	if maxDepth <= stock.tolerance {
		return fail()
	}
	bottom := maxDepth
	if lowCap >= 0 {
		bottom = span
	}
	toolFrame, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: center.X, Y: center.Y, Z: -bottom}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return Solid{}, e
	}
	if !draft {
		envelope, e := ExtrudeDiskWithHoles(ctx, toolFrame, ExtrusionCircle{Radius: maxR}, nil, nil, bottom)
		if e != nil {
			return Solid{}, e
		}
		// Reuse the strict whole-envelope proof, including actual finite-face and
		// existing-hole separation, before building the stepped cavity.
		if lowCap >= 0 {
			_, e = sewPlanarThroughPrism(ctx, stock, envelope, lowCap, highCap, 0, 1, false)
		} else {
			_, _, e = sewPlanarCapPrism(ctx, stock, envelope, highCap, 0, 1, true, false)
		}
		if e != nil {
			return Solid{}, fmt.Errorf("stepped bore envelope: %w", e)
		}
	}
	if !build {
		return Solid{}, nil
	}
	donorSpan := span
	if lowCap < 0 {
		donorSpan = 2 * maxDepth
	}
	if !finite(donorSpan) || donorSpan <= maxDepth && lowCap < 0 {
		return fail()
	}
	margin := 2*maxR + 1
	points := []geom.Vec3{{X: center.X - margin, Y: center.Y - margin, Z: -donorSpan}, {X: center.X + margin, Y: center.Y - margin, Z: -donorSpan}, {X: center.X + margin, Y: center.Y + margin, Z: -donorSpan}, {X: center.X - margin, Y: center.Y + margin, Z: -donorSpan}}
	for i := range points {
		points[i] = frame.ToWorld(points[i])
	}
	base, e := brep.Prism(points, frame.ZAxis().Scale(donorSpan))
	if e != nil {
		return Solid{}, e
	}
	var cuts []CylinderCut
	var depths []float64
	for _, s := range steps {
		cuts = append(cuts, CylinderCut{Entry: 1, Exit: 0, AxisPoint: frame.ToWorld(geom.Vec3{X: center.X, Y: center.Y}), Direction: frame.ZAxis().Scale(-1), Radius: s.Radius})
		d := s.Depth
		if lowCap >= 0 && d == span {
			d = 0
		}
		depths = append(depths, d)
	}
	donor, _, e := BoreCylinderDepthUnion(base, cuts, depths, stock.tolerance)
	if e != nil {
		return Solid{}, e
	}
	vertices := append(stock.Vertices(), donor.Vertices()...)
	edges, faces := stock.Edges(), stock.Faces()
	eo, vo := len(edges), len(stock.vertices)
	for _, edge := range donor.edges {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	for _, pair := range [][2]int{{highCap, 1}, {lowCap, 0}} {
		if pair[0] < 0 {
			continue
		}
		target := faces[pair[0]]
		source := donor.faces[pair[1]]
		from := source.Surface.(geom.PlaneSurface).Frame()
		to := target.Surface.(geom.PlaneSurface).Frame()
		project := func(p geom.Vec2) geom.Vec2 {
			q := to.ToLocal(from.ToWorld(geom.Vec3{X: p.X, Y: p.Y}))
			return geom.Vec2{X: q.X, Y: q.Y}
		}
		vector := func(p geom.Vec2) geom.Vec2 {
			q := from.XAxis().Scale(p.X).Add(from.YAxis().Scale(p.Y))
			return geom.Vec2{X: q.Dot(to.XAxis()), Y: q.Dot(to.YAxis())}
		}
		for _, loop := range source.Holes {
			out := append([]Use(nil), loop...)
			for i := range out {
				u := &out[i]
				c, ok := u.Curve.(unionArcUV)
				if !ok {
					return fail()
				}
				c.Center = project(c.Center)
				c.Cos = vector(c.Cos)
				c.Sin = vector(c.Sin)
				c.Linear = vector(c.Linear)
				c.Start = project(c.Start)
				c.End = project(c.End)
				u.Edge += eo
				u.UVStart, u.UVEnd = c.Start, c.End
				u.Curve = c
			}
			if target.Reversed != source.Reversed {
				out = reverseUnionLoop(out)
			}
			faces[pair[0]].Holes = append(faces[pair[0]].Holes, out)
		}
	}
	for _, f := range donor.faces[6:] {
		f.Outer = append([]Use(nil), f.Outer...)
		for i := range f.Outer {
			f.Outer[i].Edge += eo
		}
		for i := range f.Holes {
			f.Holes[i] = append([]Use(nil), f.Holes[i]...)
			for j := range f.Holes[i] {
				f.Holes[i][j].Edge += eo
			}
		}
		faces = append(faces, f)
	}
	if draft {
		// Private append-only graph: all old cap indices stay stable, and no
		// caller can receive this graph before the aggregate validation below.
		return Solid{vertices: vertices, edges: edges, faces: faces, tolerance: stock.tolerance}, nil
	}
	return compactSteppedTopology(ctx, vertices, edges, faces, stock.tolerance)
}

// PlanarSteppedBore is one coaxial site whose maximum radial envelope is
// disjoint from the other sites in a batch. LowerCap=-1 selects blind cuts.
type PlanarSteppedBore struct {
	LowerCap, UpperCap int
	Center             geom.Vec2
	Steps              []AxialBoreStep
}

// SewPlanarSteppedBores proves every finite envelope against the original
// stock and proves strict pairwise radial separation, then validates the
// combined retained boundary once. No intermediate draft escapes on failure.
func SewPlanarSteppedBores(ctx context.Context, stock Solid, sites []PlanarSteppedBore) (Solid, error) {
	if ctx == nil || len(sites) == 0 || len(sites) > 32 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	centers := make([]geom.Vec2, len(sites))
	radii := make([]float64, len(sites))
	var reference geom.Frame
	for i, site := range sites {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if e := ProvePlanarSteppedBore(ctx, stock, site.LowerCap, site.UpperCap, site.Center, site.Steps); e != nil {
			return Solid{}, e
		}
		frame := stock.faces[site.UpperCap].Surface.(geom.PlaneSurface).Frame()
		if i == 0 {
			reference = frame
		} else if frame.ZAxis() != reference.ZAxis() {
			return Solid{}, geom.ErrDomain
		}
		offset, e := prismExactLocal(reference, frame.Origin())
		if e != nil {
			return Solid{}, e
		}
		projected := geom.Vec2{
			X: frame.XAxis().Dot(reference.XAxis())*site.Center.X + frame.YAxis().Dot(reference.XAxis())*site.Center.Y,
			Y: frame.XAxis().Dot(reference.YAxis())*site.Center.X + frame.YAxis().Dot(reference.YAxis())*site.Center.Y,
		}
		centers[i], e = prismExactAdd(geom.Vec2{X: offset.X, Y: offset.Y}, projected)
		if e != nil {
			return Solid{}, e
		}
		for _, step := range site.Steps {
			radii[i] = max(radii[i], step.Radius)
		}
		for j := 0; j < i; j++ {
			if !(prismCircle{center: centers[i], radius: radii[i]}).outside(centers[j], ratInset(radii[j]), 8*stock.tolerance) {
				return Solid{}, geom.ErrDomain
			}
		}
	}
	result := stock
	for _, site := range sites {
		var e error
		result, e = sewPlanarSteppedBore(ctx, result, site.LowerCap, site.UpperCap, site.Center, site.Steps, true, true)
		if e != nil {
			return Solid{}, e
		}
	}
	return compactSteppedTopology(ctx, result.vertices, result.edges, result.faces, stock.tolerance)
}
