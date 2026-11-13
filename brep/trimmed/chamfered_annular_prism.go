package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ChamferedAnnularPrism retains a coaxial washer and equal radial/axial cap
// setbacks. Each selected cap treats both its original outer and inner rim.
// Lower/Upper refer to local axial coordinates 0/Height in Frame.
type ChamferedAnnularPrism struct {
	Frame                            geom.Frame
	OuterRadius, InnerRadius, Height float64
	LowerChamfer, UpperChamfer       float64
}

type ChamferedAnnularResult struct {
	Solid                Solid
	LowerFace, UpperFace int
}

func (p ChamferedAnnularPrism) validate() error {
	R, r, h, a, b := p.OuterRadius, p.InnerRadius, p.Height, p.LowerChamfer, p.UpperChamfer
	if !finite(R) || !finite(r) || !finite(h) || !finite(a) || !finite(b) || r <= 0 || R <= r || h <= 0 || a < 0 || b < 0 {
		return geom.ErrInvalid
	}
	if 2*max(a, b) >= R-r || a+b >= h {
		return geom.ErrDomain
	}
	if p.Frame.ZAxis().Length() == 0 {
		return geom.ErrInvalid
	}
	return nil
}

func (p ChamferedAnnularPrism) NominalVolume() (float64, error) {
	if e := p.validate(); e != nil {
		return 0, e
	}
	R, r, h, a, b := p.OuterRadius, p.InnerRadius, p.Height, p.LowerChamfer, p.UpperChamfer
	v := math.Pi * ((R*R-r*r)*h - (R+r)*(a*a+b*b))
	if !finite(v) || v <= 0 {
		return 0, geom.ErrUnrepresentable
	}
	return v, nil
}
func (p ChamferedAnnularPrism) SurfaceArea() (float64, error) {
	if e := p.validate(); e != nil {
		return 0, e
	}
	R, r, h, a, b := p.OuterRadius, p.InnerRadius, p.Height, p.LowerChamfer, p.UpperChamfer
	area := 2 * math.Pi * ((R+r)*h + R*R - r*r + (R+r)*(math.Sqrt2-2)*(a+b))
	if !finite(area) || area <= 0 {
		return 0, geom.ErrUnrepresentable
	}
	return area, nil
}
func (p ChamferedAnnularPrism) Solid(ctx context.Context) (Solid, error) {
	result, e := p.Build(ctx)
	return result.Solid, e
}

func (p ChamferedAnnularPrism) profile() []geom.Vec2 {
	R, r, h, a, b := p.OuterRadius, p.InnerRadius, p.Height, p.LowerChamfer, p.UpperChamfer
	profile := []geom.Vec2{{X: r + a}, {X: R - a}}
	if a > 0 {
		profile = append(profile, geom.Vec2{X: R, Y: a})
	}
	profile = append(profile, geom.Vec2{X: R, Y: h - b})
	if b > 0 {
		profile = append(profile, geom.Vec2{X: R - b, Y: h})
	}
	profile = append(profile, geom.Vec2{X: r + b, Y: h})
	if b > 0 {
		profile = append(profile, geom.Vec2{X: r, Y: h - b})
	}
	profile = append(profile, geom.Vec2{X: r, Y: a})
	if a == 0 {
		profile = profile[:len(profile)-1]
	}
	return profile
}

// Build returns one planar annular face per cap, with shared exact circular
// trim edges. Subsequent qualified through holes can modify those cap loops;
// the cylinder and cone surfaces remain analytic.
func (p ChamferedAnnularPrism) Build(ctx context.Context) (ChamferedAnnularResult, error) {
	var out ChamferedAnnularResult
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if e := p.validate(); e != nil {
		return out, e
	}
	R, r, h, a, b := p.OuterRadius, p.InnerRadius, p.Height, p.LowerChamfer, p.UpperChamfer
	profile := p.profile()
	top := 0
	for i, q := range profile {
		if q.Y == h && profile[(i+1)%len(profile)].Y == h {
			top = i
			break
		}
	}
	patches, e := parametric.Revolve(p.Frame, profile)
	if e != nil {
		return out, e
	}
	s, e := FromParametric(patches)
	if e != nil {
		return out, e
	}
	rings := make([][4]int, len(profile))
	for ring := range profile {
		current := ring
		for j := 0; j < 3; j++ {
			var split EdgeSplit
			s, split, e = s.splitEdgeDraft(current, float64(j+1)/4)
			if e != nil {
				return out, e
			}
			rings[ring][j] = split.First
			current = split.Second
		}
		rings[ring][3] = current
	}
	out.LowerFace, out.UpperFace = 0, top
	for _, cap := range []struct {
		face, inner, outer int
		z, ri, ro          float64
		reversed           bool
	}{{0, 0, 1, 0, r + a, R - a, true}, {top, top + 1, top, h, r + b, R - b, false}} {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		f, e := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{Z: cap.z}), p.Frame.XAxis(), p.Frame.ZAxis())
		if e != nil {
			return out, e
		}
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return out, e
		}
		loops := [2][]Use{}
		for which, ring := range []struct {
			id     int
			radius float64
		}{{cap.outer, cap.ro}, {cap.inner, cap.ri}} {
			ids := rings[ring.id]
			points := [4]geom.Vec2{{X: ring.radius}, {Y: ring.radius}, {X: -ring.radius}, {Y: -ring.radius}}
			for j, id := range ids {
				u, v := points[j], points[(j+1)%4]
				arc, e := extrusionArc(ExtrusionCircle{Radius: ring.radius}, j, u, v, false)
				if e != nil {
					return out, e
				}
				edge := s.edges[id]
				s.vertices[edge.Start] = f.ToWorld(geom.Vec3{X: u.X, Y: u.Y})
				s.vertices[edge.End] = f.ToWorld(geom.Vec3{X: v.X, Y: v.Y})
				edge.Curve = surfaceTrimCurve{surface: plane, a: u, b: v, curve: arc}
				edge.Range = parametric.Interval{Max: 1}
				s.edges[id] = edge
			}
			for j := 0; j < 4; j++ {
				id := ids[j]
				u, v := points[j], points[(j+1)%4]
				arc, e := extrusionArc(ExtrusionCircle{Radius: ring.radius}, j, u, v, false)
				if e != nil {
					return out, e
				}
				use := Use{Edge: id, UVStart: u, UVEnd: v, Curve: arc}
				if which == 1 {
					id = ids[3-j]
					u, v = points[(4-j)%4], points[3-j]
					arc, e = extrusionArc(ExtrusionCircle{Radius: ring.radius}, j, u, v, true)
					if e != nil {
						return out, e
					}
					use = Use{Edge: id, Reversed: true, UVStart: u, UVEnd: v, Curve: arc}
				}
				loops[which] = append(loops[which], use)
			}
		}
		s.faces[cap.face] = Face{Surface: plane, Outer: loops[0], Holes: [][]Use{loops[1]}, Reversed: cap.reversed}
	}
	out.Solid, e = compactSteppedTopology(ctx, s.vertices, s.edges, s.faces, s.tolerance)
	return out, e
}
