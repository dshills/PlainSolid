package scene

import (
	"context"
	"fmt"
	"math"
	"sort"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// DrawingEdge is an explicitly selected model edge. IDs are returned unchanged.
type DrawingEdge struct {
	ID         string
	Start, End geom.Vec3
}

// VisibleSpan is an interval of a projected edge. T0 and T1 refer to the
// original 3D edge, from Start at 0 to End at 1.
type VisibleSpan struct {
	ID         string
	Start, End geom.Vec2
	T0, T1     float64
}

// HiddenLineOptions controls coplanar depth tolerance and the maximum number
// of edge/triangle candidate tests. MaxTests defaults to one million.
type HiddenLineOptions struct {
	DepthTolerance float64
	MaxTests       int
}

type projected struct{ x, y, z float64 }
type span struct{ lo, hi float64 }

func (p projected) at(q projected, t float64) projected {
	return projected{(1-t)*p.x + t*q.x, (1-t)*p.y + t*q.y, (1-t)*p.z + t*q.z}
}
func (p projected) isFinite() bool { return finite(p.x) && finite(p.y) && finite(p.z) }

// HiddenLines emits the visible portions of explicit edges behind a triangle
// mesh. It clips edge segments to either camera frustum and subtracts
// intervals covered by closer triangles. Coplanar triangles leave the edge
// visible. Sub-ULP span gaps are discarded. Silhouette extraction and exact
// curved occlusion are outside this algorithm. Output follows input order.
func HiddenLines(ctx context.Context, c Camera, occluder mesh.Mesh, edges []DrawingEdge, o HiddenLineOptions) ([]VisibleSpan, error) {
	if ctx == nil || !c.valid() || !finite(o.DepthTolerance) || o.DepthTolerance < 0 || o.MaxTests < 0 {
		return nil, fmt.Errorf("scene: invalid hidden-line options")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.MaxTests == 0 {
		o.MaxTests = 1000000
	}
	if len(occluder.Triangles) > 0 && len(edges) > o.MaxTests/len(occluder.Triangles) {
		return nil, fmt.Errorf("scene: hidden-line work limit exceeded")
	}
	if e := occluder.Validate(); e != nil {
		return nil, e
	}
	if c.perspective {
		return perspectiveHidden(ctx, c, occluder, edges, o)
	}
	project := func(p geom.Vec3) projected {
		v := c.frame.ToLocal(p)
		return projected{2 * (v.X / c.width), 2 * (v.Y / c.height), -v.Z}
	}
	vertices := make([]projected, len(occluder.Vertices))
	for i, p := range occluder.Vertices {
		vertices[i] = project(p)
		if !vertices[i].isFinite() {
			return nil, fmt.Errorf("scene: projection overflow")
		}
	}
	var result []VisibleSpan
	for _, edge := range edges {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		p, q := project(edge.Start), project(edge.End)
		if !p.isFinite() || !q.isFinite() {
			return nil, fmt.Errorf("scene: invalid drawing edge")
		}
		if p.x == q.x && p.y == q.y {
			continue
		}
		valid := span{0, 1}
		for _, g := range [][2]float64{{p.x + 1, q.x + 1}, {1 - p.x, 1 - q.x}, {p.y + 1, q.y + 1}, {1 - p.y, 1 - q.y}, {p.z - c.near, q.z - c.near}, {c.far - p.z, c.far - q.z}} {
			ok, err := keepPositive(&valid, g[0], g[1])
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
		}
		if valid.lo >= valid.hi {
			continue
		}
		a, b := p.at(q, valid.lo), p.at(q, valid.hi)
		var hidden []span
		for i, tri := range occluder.Triangles {
			if i%1024 == 0 {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
			}
			t := [3]projected{vertices[tri[0]], vertices[tri[1]], vertices[tri[2]]}
			h, ok, e := hiddenBy(a, b, t, c.near, o.DepthTolerance)
			if e != nil {
				return nil, e
			}
			if ok {
				hidden = append(hidden, h)
			}
		}
		sort.Slice(hidden, func(i, j int) bool { return hidden[i].lo < hidden[j].lo })
		add := func(start, end float64) {
			if end-start <= 8*(math.Nextafter(1, 2)-1) {
				return
			}
			u, v := a.at(b, start), a.at(b, end)
			t0, t1 := valid.lo+(valid.hi-valid.lo)*start, valid.lo+(valid.hi-valid.lo)*end
			if t1 <= t0 {
				return
			}
			result = append(result, VisibleSpan{edge.ID, geom.Vec2{X: u.x, Y: u.y}, geom.Vec2{X: v.x, Y: v.y}, t0, t1})
		}
		at := 0.
		for _, h := range hidden {
			if h.lo > at {
				add(at, h.lo)
			}
			at = math.Max(at, h.hi)
		}
		add(at, 1)
	}
	return result, nil
}

func keepPositive(s *span, a, b float64) (bool, error) {
	if !finite(a) || !finite(b) {
		return false, fmt.Errorf("scene: clipping arithmetic overflow")
	}
	if a < 0 && b < 0 {
		s.hi = s.lo
		return false, nil
	}
	if a >= 0 && b >= 0 {
		return s.lo < s.hi, nil
	}
	scale := math.Max(math.Abs(a), math.Abs(b))
	t := (a / scale) / ((a / scale) - (b / scale))
	if a < 0 {
		s.lo = math.Max(s.lo, t)
	} else {
		s.hi = math.Min(s.hi, t)
	}
	return s.lo < s.hi, nil
}

func turn(a, b, c projected) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }
func hiddenBy(a, b projected, t [3]projected, near, tolerance float64) (span, bool, error) {
	s := span{0, 1}
	if math.Max(a.x, b.x) < math.Min(t[0].x, math.Min(t[1].x, t[2].x)) ||
		math.Min(a.x, b.x) > math.Max(t[0].x, math.Max(t[1].x, t[2].x)) ||
		math.Max(a.y, b.y) < math.Min(t[0].y, math.Min(t[1].y, t[2].y)) ||
		math.Min(a.y, b.y) > math.Max(t[0].y, math.Max(t[1].y, t[2].y)) {
		return s, false, nil
	}
	d := turn(t[0], t[1], t[2])
	if !finite(d) {
		return s, false, fmt.Errorf("scene: projected triangle outside numerical range")
	}
	if d == 0 {
		return s, false, nil
	}
	sign := math.Copysign(1, d)
	var da, db float64
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		x, y := turn(t[j], t[k], a), turn(t[j], t[k], b)
		ok, err := keepPositive(&s, sign*x, sign*y)
		if err != nil || !ok {
			return s, false, err
		}
		da += (x / d) * t[i].z
		db += (y / d) * t[i].z
	}
	ok, err := keepPositive(&s, da-near, db-near)
	if err != nil || !ok {
		return s, false, err
	}
	x, y := a.z-da-tolerance, b.z-db-tolerance
	if x <= 0 && y <= 0 {
		return s, false, nil
	}
	ok, err = keepPositive(&s, x, y)
	return s, ok, err
}
