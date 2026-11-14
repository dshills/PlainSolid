package trimmed

import (
	"fmt"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// StopPerpendicularBore closes a proven perpendicular through-bore at an interior depth.
// It preserves shared analytic edges, restores the far stock face, and adds
// an oppositely oriented planar floor. delta is the distance pulled backward
// along the bore axis from the original exit support.
func StopPerpendicularBore(s Solid, h BoreHistory, axis geom.Vec3, delta float64) (Solid, int, error) {
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	if !axis.IsFinite() || axis.Length() != 1 || !finite(delta) || delta <= s.Tolerance() || h.Entry < 0 || h.Exit < 0 || h.Entry >= len(f) || h.Exit >= len(f) || h.Entry == h.Exit {
		return Solid{}, 0, geom.ErrInvalid
	}
	for _, id := range h.ExitEdges {
		if id < 0 || id >= len(e) {
			return Solid{}, 0, geom.ErrInvalid
		}
	}
	for _, id := range h.WallSeamEdges {
		if id < 0 || id >= len(e) || e[id].Range.Max-e[id].Range.Min <= delta+s.Tolerance() {
			return Solid{}, 0, geom.ErrDomain
		}
	}
	for _, id := range h.WallFaces {
		if id < 0 || id >= len(f) {
			return Solid{}, 0, geom.ErrInvalid
		}
	}
	shift := axis.Scale(-delta)
	m := geom.Translation(shift)
	ids := map[int]bool{}
	for _, id := range h.ExitEdges {
		ids[id] = true
		v[e[id].Start] = v[e[id].Start].Add(shift)
		e[id].Curve = parametric.MapCurve(e[id].Curve, m)
	}
	for _, id := range h.WallSeamEdges {
		e[id].Range.Max -= delta
	}
	for _, id := range h.WallFaces {
		// Perpendicular rims have constant V, and thus straight wall p-curves.
		for j := range f[id].Outer {
			u := &f[id].Outer[j]
			if u.Curve != nil {
				return Solid{}, 0, fmt.Errorf("blind bore requires perpendicular circular rims")
			}
			if j == 1 || j == 2 {
				u.UVEnd.Y -= delta
			}
			if j == 2 || j == 3 {
				u.UVStart.Y -= delta
			}
		}
	}
	var ring []Use
	holes := f[h.Exit].Holes[:0]
	for _, hole := range f[h.Exit].Holes {
		if len(hole) > 0 && ids[hole[0].Edge] {
			ring = hole
		} else {
			holes = append(holes, hole)
		}
	}
	f[h.Exit].Holes = holes
	if len(ring) != 4 {
		return Solid{}, 0, fmt.Errorf("missing blind rim")
	}
	floor := Face{Reversed: true}
	plane, ok := f[h.Exit].Surface.(geom.PlaneSurface)
	if !ok {
		return Solid{}, 0, fmt.Errorf("blind exit is not planar")
	}
	frame := plane.Frame()
	moved, err := geom.NewFrame(frame.Origin().Add(shift), frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Solid{}, 0, err
	}
	floor.Surface, err = geom.NewPlaneSurface(moved)
	if err != nil {
		return Solid{}, 0, err
	}
	for j := len(ring) - 1; j >= 0; j-- {
		u := ring[j]
		u.Reversed = !u.Reversed
		u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
		arc, ok := u.Curve.(EllipseUV)
		if !ok {
			return Solid{}, 0, fmt.Errorf("blind rim is not an analytic ellipse")
		}
		arc.From, arc.To = arc.To, arc.From
		u.Curve = arc
		floor.Outer = append(floor.Outer, u)
	}
	id := len(f)
	f = append(f, floor)
	out, err := NewSolid(v, e, f, s.Tolerance())
	return out, id, err
}
