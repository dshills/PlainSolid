package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// planarBoredStock retains an independently validated planar material domain.
// The certificate is created only after exact shell nesting and strictly
// separated, fully contained through-bore envelope proofs.
type planarBoredStock struct {
	base      brep.Solid
	frame     geom.Frame
	height    float64
	bores     []trimmed.ExtrusionCircle
	connected bool
}

func retainedPlanarBase(a stored) (*planarBoredStock, error) {
	if a.analytic == nil {
		return nil, nil
	}
	if a.analytic.planarBores != nil {
		p := *a.analytic.planarBores
		p.bores = append([]trimmed.ExtrusionCircle(nil), p.bores...)
		return &p, nil
	}
	if a.analytic.pockets == nil || len(a.analytic.pockets.bores) == 0 {
		return nil, nil
	}
	p := *a.analytic.pockets
	native, e := trimmed.ExtrudeBoxWithPocketsAndBores(context.Background(), p.frame, p.size, p.pockets, nil)
	if e != nil {
		return nil, e
	}
	original := p.bores
	p.bores = nil
	base, _, e := (stored{analytic: &analyticBody{solid: native, pockets: &p}}).planarCutBoundary()
	return &planarBoredStock{base: base, frame: p.frame, height: p.size.Z, bores: append([]trimmed.ExtrusionCircle(nil), original...)}, e
}

func normalizeBoreCaps(s trimmed.Solid, frame geom.Frame, height float64) (trimmed.Solid, int, int, error) {
	faces := s.Faces()
	vertices := s.Vertices()
	edges := s.Edges()
	low, high := -1, -1
	for i, f := range faces {
		plane, ok := f.Surface.(geom.PlaneSurface)
		if !ok {
			return s, 0, 0, geom.ErrDomain
		}
		n := plane.Frame().ZAxis()
		if f.Reversed {
			n = n.Scale(-1)
		}
		z := frame.ToLocal(plane.Frame().Origin()).Z
		bottom := n == frame.ZAxis().Scale(-1) && z == 0
		top := n == frame.ZAxis() && z == height
		if !bottom && !top {
			continue
		}
		if bottom {
			if low >= 0 {
				return s, 0, 0, geom.ErrDomain
			}
			low = i
		} else {
			if high >= 0 {
				return s, 0, 0, geom.ErrDomain
			}
			high = i
		}
		chart, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return s, 0, 0, e
		}
		surface, e := geom.NewPlaneSurface(chart)
		if e != nil {
			return s, 0, 0, e
		}
		reverse := plane.Frame().ZAxis() != frame.ZAxis()
		convert := func(loop []trimmed.Use) []trimmed.Use {
			out := make([]trimmed.Use, len(loop))
			for j, u := range loop {
				if reverse {
					u.Reversed = !u.Reversed
				}
				ed := edges[u.Edge]
				a, b := ed.Start, ed.End
				if u.Reversed {
					a, b = b, a
				}
				p, q := chart.ToLocal(vertices[a]), chart.ToLocal(vertices[b])
				u.UVStart = geom.Vec2{X: p.X, Y: p.Y}
				u.UVEnd = geom.Vec2{X: q.X, Y: q.Y}
				u.Curve = nil
				k := j
				if reverse {
					k = len(loop) - 1 - j
				}
				out[k] = u
			}
			return out
		}
		faces[i].Surface = surface
		faces[i].Reversed = bottom
		faces[i].Outer = convert(f.Outer)
		for j, l := range f.Holes {
			faces[i].Holes[j] = convert(l)
		}
	}
	if low < 0 || high < 0 {
		return s, 0, 0, geom.ErrDomain
	}
	out, e := trimmed.NewSolid(vertices, edges, faces, 1e-8)
	return out, low, high, e
}

func buildPlanarBores(p *planarBoredStock) (*analyticBody, error) {
	ctx := context.Background()
	if !cardinalFrame(p.frame) || !finite(p.height) || p.height <= 0 || len(p.bores) > 32 {
		return nil, geom.ErrDomain
	}
	locator, e := collision.NewPointLocator(ctx, p.base, collision.LocationOptions{})
	if e != nil {
		return nil, e
	}
	roots := 0
	for _, s := range locator.Shells() {
		if s.Depth == 0 && s.SignedVolume.Sign() > 0 {
			roots++
		} else if s.Depth != 1 || s.SignedVolume.Sign() >= 0 {
			return nil, geom.ErrDomain
		}
	}
	if roots != 1 {
		return nil, geom.ErrDomain
	}
	shells, e := p.base.Shells()
	if e != nil {
		return nil, e
	}
	var outer brep.Solid
	var cavities []trimmed.Solid
	for _, shell := range shells {
		mass, e := shell.MassProperties()
		if e != nil {
			return nil, e
		}
		if mass.Volume > 0 {
			outer = shell
			continue
		}
		// A cavity may not meet any complete bore envelope. The conservative local
		// AABB proof is exact for the represented signed-cardinal coordinates.
		for _, hole := range p.bores {
			minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
			for _, v := range shell.Vertices() {
				q := p.frame.ToLocal(v)
				minX = min(minX, q.X)
				maxX = max(maxX, q.X)
				minY = min(minY, q.Y)
				maxY = max(maxY, q.Y)
			}
			if !(hole.Center.X+hole.Radius < minX || hole.Center.X-hole.Radius > maxX || hole.Center.Y+hole.Radius < minY || hole.Center.Y-hole.Radius > maxY) {
				return nil, geom.ErrDomain
			}
		}
		native, e := trimmed.CoalesceCardinalPlanar(ctx, shell, 1e-8)
		if e != nil {
			return nil, e
		}
		cavities = append(cavities, native)
	}
	native, e := trimmed.CoalesceCardinalPlanar(ctx, outer, 1e-8)
	if e != nil {
		return nil, e
	}
	if len(p.bores) > 0 {
		var low, high int
		native, low, high, e = normalizeBoreCaps(native, p.frame, p.height)
		if e != nil {
			return nil, e
		}
		for _, hole := range p.bores {
			native, e = trimmed.SewPlanarSteppedBore(ctx, native, low, high, hole.Center, []trimmed.AxialBoreStep{{Radius: hole.Radius}})
			if e != nil {
				return nil, e
			}
		}
	}
	vertices, edges, faces := native.Vertices(), native.Edges(), native.Faces()
	for _, cavity := range cavities {
		vo, eo := len(vertices), len(edges)
		vertices = append(vertices, cavity.Vertices()...)
		for _, ed := range cavity.Edges() {
			ed.Start += vo
			ed.End += vo
			edges = append(edges, ed)
		}
		for _, f := range cavity.Faces() {
			for j := range f.Outer {
				f.Outer[j].Edge += eo
			}
			for h := range f.Holes {
				for j := range f.Holes[h] {
					f.Holes[h][j].Edge += eo
				}
			}
			faces = append(faces, f)
		}
	}
	if len(cavities) > 0 {
		native, e = trimmed.NewSolid(vertices, edges, faces, 1e-8)
		if e != nil {
			return nil, e
		}
	}
	m, e := p.base.MassProperties()
	if e != nil {
		return nil, e
	}
	volume, area := m.Volume, m.SurfaceArea
	for _, h := range p.bores {
		volume -= math.Pi * h.Radius * h.Radius * p.height
		area += 2*math.Pi*h.Radius*p.height - 2*math.Pi*h.Radius*h.Radius
	}
	q := *p
	q.connected = true
	q.bores = append([]trimmed.ExtrusionCircle(nil), p.bores...)
	return &analyticBody{solid: native, volume: volume, area: area, planarBores: &q}, nil
}

func retainedPlanarBoolean(a, c stored, op string) (*analyticBody, float64, bool, error) {
	p, e := retainedPlanarBase(a)
	if e != nil {
		return nil, 0, true, e
	}
	if p == nil {
		return nil, 0, false, nil
	}
	fail := func() (*analyticBody, float64, bool, error) {
		return nil, 0, true, unsupportedEdge("retained planar stock requires a separated planar edit or a full axial bore")
	}
	if c.planar() {
		for _, hole := range p.bores {
			lo, hi := geom.Vec3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}, geom.Vec3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
			for _, v := range c.solid.Vertices() {
				v = p.frame.ToLocal(v)
				lo.X = min(lo.X, v.X)
				lo.Y = min(lo.Y, v.Y)
				lo.Z = min(lo.Z, v.Z)
				hi.X = max(hi.X, v.X)
				hi.Y = max(hi.Y, v.Y)
				hi.Z = max(hi.Z, v.Z)
			}
			if !(hi.X < hole.Center.X-hole.Radius || lo.X > hole.Center.X+hole.Radius || hi.Y < hole.Center.Y-hole.Radius || lo.Y > hole.Center.Y+hole.Radius || hi.Z < 0 || lo.Z > p.height) {
				return fail()
			}
		}
		operation := poly.Difference
		if op == "fuse" {
			operation = poly.Union
		}
		p.base, e = poly.Combine(context.Background(), p.base, c.solid, operation, poly.Options{})
		if e != nil {
			return nil, 0, true, e
		}
	} else if c.cylinder != nil {
		// Preserve the richer pocket-floor path for ordinary circular cuts.
		if op == "cut" && a.analytic.pockets != nil {
			return nil, 0, false, nil
		}
		tool := c.cylinder
		axis := tool.frame.ZAxis()
		if axis != p.frame.ZAxis() && axis != p.frame.ZAxis().Scale(-1) {
			return fail()
		}
		start, end := p.frame.ToLocal(tool.frame.Origin()), p.frame.ToLocal(tool.frame.Origin().Add(axis.Scale(tool.height)))
		if start.X != end.X || start.Y != end.Y || min(start.Z, end.Z) > 0 || max(start.Z, end.Z) < p.height {
			return fail()
		}
		center := geom.Vec2{X: start.X, Y: start.Y}
		radius := tool.radius
		if op == "fuse" {
			if min(start.Z, end.Z) != 0 || max(start.Z, end.Z) != p.height {
				return fail()
			}
			found := -1
			for i, h := range p.bores {
				if h.Center == center && radius >= h.Radius {
					found = i
				}
			}
			if found < 0 {
				return fail()
			}
			// Prove the complete plug is contained in the unbored material, including
			// separation from every retained cavity, before discarding the old bore.
			proof := *p
			proof.bores = []trimmed.ExtrusionCircle{{Center: center, Radius: radius}}
			if _, e = buildPlanarBores(&proof); e != nil {
				return nil, 0, true, e
			}
			p.bores = append(p.bores[:found], p.bores[found+1:]...)
		} else {
			p.bores = append(p.bores, trimmed.ExtrusionCircle{Center: center, Radius: radius})
		}
	} else {
		return nil, 0, false, nil
	}
	body, e := buildPlanarBores(p)
	if e != nil {
		return nil, 0, true, e
	}
	removed := a.analytic.volume - body.volume
	return body, removed, true, nil
}
