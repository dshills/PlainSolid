package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// cutPocketCylinder operates on the retained pocket boundary and finite tool.
// It never infers a historical floor owner from newly generated geometry.
func cutPocketCylinder(a stored, tool *cylinderBody) (*analyticBody, float64, bool, error) {
	if a.analytic == nil || a.analytic.pockets == nil || tool == nil {
		return nil, 0, false, nil
	}
	p := *a.analytic.pockets
	fail := func() (*analyticBody, float64, bool, error) {
		return nil, 0, true, unsupportedEdge("pocket circular cut requires a separated axial footprint and a finite entry-open interval")
	}
	axis := tool.frame.ZAxis()
	if axis != p.frame.ZAxis() && axis != p.frame.ZAxis().Scale(-1) {
		return fail()
	}
	start, end := tool.frame.Origin(), tool.frame.Origin().Add(axis.Scale(tool.height))
	lo, hi := p.frame.ToLocal(start), p.frame.ToLocal(end)
	if p.frame.ToWorld(lo) != start || p.frame.ToWorld(hi) != end || lo.X != hi.X || lo.Y != hi.Y {
		return fail()
	}
	center := geom.Vec2{X: lo.X, Y: lo.Y}
	r := tool.radius
	top := p.size.Z
	for _, pocket := range p.pockets {
		box := pocket.Footprint
		if center.X-r > box.Min.X && center.X+r < box.Max.X && center.Y-r > box.Min.Y && center.Y+r < box.Max.Y {
			top = pocket.Floor
		} else if center.X+r < box.Min.X || center.X-r > box.Max.X || center.Y+r < box.Min.Y || center.Y-r > box.Max.Y {
			break
		} else {
			return fail()
		}
	}
	bottom, upper := math.Max(0, math.Min(lo.Z, hi.Z)), math.Min(top, math.Max(lo.Z, hi.Z))
	if bottom >= upper {
		return a.analytic, 0, true, nil
	}
	if upper != top {
		return fail()
	}
	depth := top - bottom
	oldRadius := 0.
	kept := make([]trimmed.ExtrusionCircle, 0, len(p.bores))
	for _, old := range p.bores {
		if old.Center == center {
			if oldRadius != 0 {
				return fail()
			}
			oldRadius = old.Radius
			if r <= oldRadius {
				return a.analytic, 0, true, nil
			}
		} else {
			kept = append(kept, old)
		}
	}
	native, e := trimmed.ExtrudeBoxWithPocketsAndBores(context.Background(), p.frame, p.size, p.pockets, kept)
	if e != nil {
		return nil, 0, true, e
	}
	lower, entry := -1, -1
	for i, f := range native.Faces() {
		plane, ok := f.Surface.(geom.PlaneSurface)
		if !ok {
			continue
		}
		normal := plane.Frame().ZAxis()
		if f.Reversed {
			normal = normal.Scale(-1)
		}
		z := p.frame.ToLocal(plane.Frame().Origin()).Z
		if normal == p.frame.ZAxis() && z == top {
			if entry >= 0 {
				return fail()
			}
			entry = i
		}
		if normal == p.frame.ZAxis().Scale(-1) && z == 0 {
			if lower >= 0 {
				return fail()
			}
			lower = i
		}
	}
	if entry < 0 || lower < 0 {
		return fail()
	}
	steps := []trimmed.AxialBoreStep{}
	if oldRadius > 0 {
		steps = append(steps, trimmed.AxialBoreStep{Radius: oldRadius})
	}
	through := bottom == 0
	if through {
		steps = append(steps, trimmed.AxialBoreStep{Radius: r})
	} else {
		steps = append(steps, trimmed.AxialBoreStep{Radius: r, Depth: depth})
		if oldRadius == 0 {
			lower = -1
		}
	}
	// A counterbore is sewn as one stepped union so the old cylindrical wall is
	// replaced only over the new finite depth; its deeper rim remains shared.
	solid, e := trimmed.SewPlanarSteppedBore(context.Background(), native, lower, entry, center, steps)
	if e != nil {
		return nil, 0, true, e
	}
	removed := math.Pi * (r*r - oldRadius*oldRadius) * depth
	delta := 2 * math.Pi * (r - oldRadius) * depth
	if through {
		delta -= 2 * math.Pi * (r*r - oldRadius*oldRadius)
	}
	body := &analyticBody{solid: solid, volume: a.analytic.volume - removed, area: a.analytic.area + delta}
	if through && top == p.size.Z && oldRadius == 0 {
		p.bores = append(append([]trimmed.ExtrusionCircle(nil), p.bores...), trimmed.ExtrusionCircle{Center: center, Radius: r})
		body.pockets = &p
	}
	return body, removed, true, nil
}
