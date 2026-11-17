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

type ChamferedAnnularExtensionResult struct {
	Solid                              Solid
	LowerFace, ShoulderFace, UpperFace int
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
	h := p.Height
	profile := p.profile()
	top := 0
	for i, q := range profile {
		if q.Y == h && profile[(i+1)%len(profile)].Y == h {
			top = i
			break
		}
	}
	solid, e := buildCappedAnnularRevolution(ctx, p.Frame, profile, []int{0, top})
	if e != nil {
		return out, e
	}
	return ChamferedAnnularResult{Solid: solid, LowerFace: 0, UpperFace: top}, nil
}

// BuildUpperExtension extends the original bore through a reduced outer
// radius above the unchamfered upper flange. The original lower and
// upper flange stations remain the planar caps for a later through cut.
func (p ChamferedAnnularPrism) BuildUpperExtension(ctx context.Context, radius, height float64) (ChamferedAnnularExtensionResult, error) {
	var out ChamferedAnnularExtensionResult
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if e := p.validate(); e != nil {
		return out, e
	}
	if !finite(radius) || !finite(height) || radius <= p.InnerRadius || radius >= p.OuterRadius ||
		height <= p.Height || p.UpperChamfer != 0 {
		return out, geom.ErrDomain
	}
	original := p.profile()
	shoulder := 2
	if p.LowerChamfer > 0 {
		shoulder++
	}
	profile := append([]geom.Vec2(nil), original[:shoulder+1]...)
	profile = append(profile,
		geom.Vec2{X: radius, Y: p.Height},
		geom.Vec2{X: radius, Y: height},
		geom.Vec2{X: p.InnerRadius, Y: height},
	)
	profile = append(profile, original[shoulder+2:]...)
	solid, e := buildCappedAnnularRevolution(ctx, p.Frame, profile, []int{0, shoulder, shoulder + 2})
	if e != nil {
		return out, e
	}
	return ChamferedAnnularExtensionResult{Solid: solid, LowerFace: 0, ShoulderFace: shoulder, UpperFace: shoulder + 2}, nil
}

func buildCappedAnnularRevolution(ctx context.Context, frame geom.Frame, profile []geom.Vec2, caps []int) (Solid, error) {
	patches, e := parametric.Revolve(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	s, e := FromParametric(patches)
	if e != nil {
		return Solid{}, e
	}
	rings := make([][4]int, len(profile))
	for ring := range profile {
		current := ring
		for j := 0; j < 3; j++ {
			var split EdgeSplit
			s, split, e = s.splitEdgeDraft(current, float64(j+1)/4)
			if e != nil {
				return Solid{}, e
			}
			rings[ring][j] = split.First
			current = split.Second
		}
		rings[ring][3] = current
	}
	for _, cap := range caps {
		if e = ctx.Err(); e != nil {
			return Solid{}, e
		}
		inner, outer := cap, cap+1
		reversed := profile[inner].X < profile[outer].X
		if !reversed {
			inner, outer = outer, inner
		}
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: profile[cap].Y}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
		loops := [2][]Use{}
		for which, ring := range []struct {
			id     int
			radius float64
		}{{outer, profile[outer].X}, {inner, profile[inner].X}} {
			ids := rings[ring.id]
			points := [4]geom.Vec2{{X: ring.radius}, {Y: ring.radius}, {X: -ring.radius}, {Y: -ring.radius}}
			for j, id := range ids {
				u, v := points[j], points[(j+1)%4]
				arc, e := extrusionArc(ExtrusionCircle{Radius: ring.radius}, j, u, v, false)
				if e != nil {
					return Solid{}, e
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
					return Solid{}, e
				}
				use := Use{Edge: id, UVStart: u, UVEnd: v, Curve: arc}
				if which == 1 {
					id = ids[3-j]
					u, v = points[(4-j)%4], points[3-j]
					arc, e = extrusionArc(ExtrusionCircle{Radius: ring.radius}, j, u, v, true)
					if e != nil {
						return Solid{}, e
					}
					use = Use{Edge: id, Reversed: true, UVStart: u, UVEnd: v, Curve: arc}
				}
				loops[which] = append(loops[which], use)
			}
		}
		s.faces[cap] = Face{Surface: plane, Outer: loops[0], Holes: [][]Use{loops[1]}, Reversed: reversed}
	}
	return compactSteppedTopology(ctx, s.vertices, s.edges, s.faces, s.tolerance)
}
