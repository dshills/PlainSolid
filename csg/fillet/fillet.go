// Package fillet constructs analytically rounded convex XY prisms. Circular
// corner edges and cylindrical side faces are retained, not replaced by facets.
package fillet

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("fillet: invalid convex prism or options")
var ErrLimit = errors.New("fillet: resource limit exceeded")
var ErrRadius = errors.New("fillet: radius violates separation or tangency limits")

// Options bounds corners (default 64, maximum 128) and support/pair checks
// (default 100000). Tolerance defaults to 1e-9 model units. Radius and height
// must exceed eight times tolerance. Construction uses float square roots and
// trigonometry; Tolerance controls represented endpoint/tangency agreement.
type Options struct {
	MaxCorners, MaxWork int
	Tolerance           float64
}

// Corner reports original polygon-corner ancestry and analytic arc topology.
type Corner struct {
	Center, Incoming, Outgoing      geom.Vec2
	Sweep, Setback                  float64
	CylinderFace, BottomArc, TopArc int
}

// Result retains analytic topology. Nominal measures use the standard ideal
// constant-radius polygon fillet formulas; represented float construction can
// differ within its tangency tolerance. Use Solid.MassProperties for integration
// of the returned trimmed surfaces themselves.
type Result struct {
	Solid                                                trimmed.Solid
	Corners                                              []Corner
	StraightFaces                                        []int
	Caps                                                 [2]int
	Radius, NominalArea, NominalPerimeter, NominalVolume float64
}

// CornerRadius selects a vertical prism corner and its fillet radius. Corner
// is the zero-based index of the corner in the original CCW profile.
type CornerRadius struct {
	Corner int
	Radius float64
}

// SelectiveResult records source corner and side ancestry. A nil corner
// means its vertical edge remains sharp. StraightFaces follows the original
// polygon side order. Caps[0] is the lower cap and Caps[1] is the upper cap.
type SelectiveResult struct {
	Solid                                        trimmed.Solid
	Corners                                      []*Corner
	StraightFaces                                []int
	Caps                                         [2]int
	NominalArea, NominalPerimeter, NominalVolume float64
}

func finite(x float64) bool                  { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func sub(a, b geom.Vec2) geom.Vec2           { return geom.Vec2{X: a.X - b.X, Y: a.Y - b.Y} }
func add(a, b geom.Vec2) geom.Vec2           { return geom.Vec2{X: a.X + b.X, Y: a.Y + b.Y} }
func scale(a geom.Vec2, t float64) geom.Vec2 { return geom.Vec2{X: a.X * t, Y: a.Y * t} }
func cross(a, b geom.Vec2) float64           { return a.X*b.Y - a.Y*b.X }
func dot(a, b geom.Vec2) float64             { return a.X*b.X + a.Y*b.Y }

type q = *big.Rat

func num(x float64) q { return new(big.Rat).SetFloat64(x) }
func qa(a, b q) q     { return new(big.Rat).Add(a, b) }
func qs(a, b q) q     { return new(big.Rat).Sub(a, b) }
func qm(a, b q) q     { return new(big.Rat).Mul(a, b) }
func squaredDistance(a, b geom.Vec2) q {
	x, y := qs(num(a.X), num(b.X)), qs(num(a.Y), num(b.Y))
	return qa(qm(x, x), qm(y, y))
}

// support compares the exact represented signed supporting-line distance with
// the requested radius interval by squaring nonnegative rational quantities.
func support(a, b, c geom.Vec2, lo, hi q) bool {
	dx, dy := qs(num(b.X), num(a.X)), qs(num(b.Y), num(a.Y))
	x, y := qs(num(c.X), num(a.X)), qs(num(c.Y), num(a.Y))
	side := qs(qm(dx, y), qm(dy, x))
	if side.Sign() <= 0 {
		return false
	}
	length := qa(qm(dx, dx), qm(dy, dy))
	squared := qm(side, side)
	if squared.Cmp(qm(qm(lo, lo), length)) < 0 {
		return false
	}
	return hi == nil || squared.Cmp(qm(qm(hi, hi), length)) <= 0
}

// retainedEdge certifies endpoint support-line and circle tangency residuals
// using rational comparisons on represented coordinates.
func retainedSelectedEdge(a, b, p, qpoint geom.Vec2, centers [2]*geom.Vec2, tolerance float64) bool {
	dx, dy := qs(num(b.X), num(a.X)), qs(num(b.Y), num(a.Y))
	length := qa(qm(dx, dx), qm(dy, dy))
	tol2 := qm(num(tolerance), num(tolerance))
	var projections [2]q
	for i, x := range []geom.Vec2{p, qpoint} {
		vx, vy := qs(num(x.X), num(a.X)), qs(num(x.Y), num(a.Y))
		side := qs(qm(dx, vy), qm(dy, vx))
		if qm(side, side).Cmp(qm(tol2, length)) > 0 {
			return false
		}
		projections[i] = qa(qm(dx, vx), qm(dy, vy))
		if centers[i] == nil {
			if i == 0 && x != a || i == 1 && x != b {
				return false
			}
		} else if projections[i].Sign() <= 0 || projections[i].Cmp(length) >= 0 {
			return false
		}
	}
	if projections[0].Cmp(projections[1]) >= 0 {
		return false
	}
	ux, uy := qs(num(qpoint.X), num(p.X)), qs(num(qpoint.Y), num(p.Y))
	span := qa(qm(ux, ux), qm(uy, uy))
	if span.Sign() == 0 {
		return false
	}
	for i, c := range centers {
		if c == nil {
			continue
		}
		x := p
		if i == 1 {
			x = qpoint
		}
		rx, ry := qs(num(x.X), num(c.X)), qs(num(x.Y), num(c.Y))
		residual := qa(qm(rx, ux), qm(ry, uy))
		if qm(residual, residual).Cmp(qm(tol2, span)) > 0 {
			return false
		}
	}
	return true
}

// Prism rounds every corner of a strictly convex counterclockwise polygon by
// the same radius and extrudes it from zMin to zMax. Holes, concavities, collapsed
// straight edges, and overlapping corner disks are unsupported. The disk test
// is deliberately conservative: some valid large fillets are rejected before
// the theoretical edge-collapse radius. No input coordinate is welded.
func Prism(ctx context.Context, polygon []geom.Vec2, zMin, zMax, radius float64, o Options) (Result, error) {
	if !finite(radius) || radius <= 0 {
		return Result{}, ErrInvalid
	}
	if len(polygon) > 128 {
		return Result{}, ErrLimit
	}
	selection := make([]CornerRadius, len(polygon))
	for i := range selection {
		selection[i] = CornerRadius{Corner: i, Radius: radius}
	}
	x, e := PrismCorners(ctx, polygon, zMin, zMax, selection, o)
	if e != nil {
		return Result{}, e
	}
	r := Result{Solid: x.Solid, Corners: make([]Corner, len(x.Corners)), StraightFaces: x.StraightFaces,
		Caps: x.Caps, Radius: radius, NominalArea: x.NominalArea, NominalPerimeter: x.NominalPerimeter, NominalVolume: x.NominalVolume}
	for i, c := range x.Corners {
		r.Corners[i] = *c
	}
	return r, nil
}

// PrismCorners rounds selected vertical edges of a strictly convex CCW
// polygonal prism. Each selected corner can have a different positive
// radius. Unselected corners remain sharp. The same conservative disk and
// tangent-clearance checks as Prism apply to all selected corners.
func PrismCorners(ctx context.Context, polygon []geom.Vec2, zMin, zMax float64, selection []CornerRadius, o Options) (SelectiveResult, error) {
	empty := SelectiveResult{}
	if ctx == nil || o.MaxCorners < 0 || o.MaxCorners > 128 || o.MaxWork < 0 || !finite(o.Tolerance) || o.Tolerance < 0 ||
		!finite(zMin) || !finite(zMax) || !finite(zMax-zMin) || len(selection) == 0 {
		return empty, ErrInvalid
	}
	if o.MaxCorners == 0 {
		o.MaxCorners = 64
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100000
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	if zMax-zMin <= 8*o.Tolerance || len(polygon) < 3 || len(selection) > len(polygon) {
		return empty, ErrInvalid
	}
	if len(polygon) > o.MaxCorners {
		return empty, ErrLimit
	}
	work := o.MaxWork
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if work == 0 {
			return ErrLimit
		}
		work--
		return nil
	}
	n := len(polygon)
	if n+len(selection) > brep.MaxHoledFaceVertices {
		return empty, ErrLimit
	}
	radii := make([]float64, n)
	for _, s := range selection {
		if s.Corner < 0 || s.Corner >= n || !finite(s.Radius) || s.Radius <= 8*o.Tolerance || radii[s.Corner] != 0 {
			return empty, ErrInvalid
		}
		radii[s.Corner] = s.Radius
	}
	for _, p := range polygon {
		if !finite(p.X) || !finite(p.Y) {
			return empty, ErrInvalid
		}
	}
	directions := make([]geom.Vec2, n)
	lengths := make([]float64, n)
	area, perimeter := 0., 0.
	origin := polygon[0]
	for i, a := range polygon {
		b := polygon[(i+1)%n]
		d := sub(b, a)
		length := math.Hypot(d.X, d.Y)
		if !finite(length) || length <= 8*o.Tolerance {
			return empty, ErrInvalid
		}
		directions[i] = scale(d, 1/length)
		lengths[i] = length
		perimeter += length
		area += cross(sub(a, origin), sub(b, origin)) / 2
		for j, p := range polygon {
			if j == i || j == (i+1)%n {
				continue
			}
			if e := tick(); e != nil {
				return empty, e
			}
			sign, e := geom.Orient2D(a, b, p)
			if e != nil || sign <= 0 {
				return empty, fmt.Errorf("%w: profile must be strictly convex and counterclockwise", ErrInvalid)
			}
		}
	}
	r := SelectiveResult{Corners: make([]*Corner, n), StraightFaces: make([]int, n), Caps: [2]int{0, 1}}
	frames := make([]geom.Frame, n)
	in, out := append([]geom.Vec2(nil), polygon...), append([]geom.Vec2(nil), polygon...)
	setbacks := make([]float64, n)
	for i, p := range polygon {
		radius := radii[i]
		if radius == 0 {
			continue
		}
		prev, next := directions[(i+n-1)%n], directions[i]
		turn := math.Atan2(cross(prev, next), dot(prev, next))
		if !finite(turn) || turn <= 0 || turn >= math.Pi {
			return empty, ErrInvalid
		}
		setback := radius * math.Tan(turn/2)
		center := add(add(p, scale(prev, -setback)), geom.Vec2{X: -prev.Y * radius, Y: prev.X * radius})
		if !finite(setback) || !finite(center.X) || !finite(center.Y) {
			return empty, ErrRadius
		}
		frame, e := geom.NewFrame(geom.Vec3{X: center.X, Y: center.Y, Z: zMin}, geom.Vec3{X: prev.Y, Y: -prev.X}, geom.Vec3{Z: 1})
		if e != nil {
			return empty, e
		}
		x, y := frame.XAxis().Scale(radius), frame.YAxis().Scale(radius)
		uv := trimmed.EllipseUV{Center: center, Cos: geom.Vec2{X: x.X, Y: x.Y}, Sin: geom.Vec2{X: y.X, Y: y.Y}, From: 0, To: turn}
		a, e := uv.EvalUV(0)
		if e != nil {
			return empty, e
		}
		b, e := uv.EvalUV(1)
		if e != nil {
			return empty, e
		}
		frames[i], in[i], out[i], setbacks[i] = frame, a, b, setback
		r.Corners[i] = &Corner{Center: center, Incoming: a, Outgoing: b, Sweep: turn, Setback: setback}
		area += -radius*setback + radius*radius*turn/2
		perimeter += -2*setback + radius*turn
	}
	for i, c := range r.Corners {
		if lengths[i]-setbacks[i]-setbacks[(i+1)%n] <= 8*o.Tolerance {
			return empty, fmt.Errorf("%w: short retained edge", ErrRadius)
		}
		if c == nil {
			continue
		}
		radius := radii[i]
		low, high := qs(num(radius), num(o.Tolerance)), qa(num(radius), num(o.Tolerance))
		clear := qa(num(radius), qm(num(4), num(o.Tolerance)))
		for j := 0; j < n; j++ {
			if e := tick(); e != nil {
				return empty, e
			}
			a, b := polygon[j], polygon[(j+1)%n]
			if j == i || j == (i+n-1)%n {
				if !support(a, b, c.Center, low, high) {
					return empty, fmt.Errorf("%w: tangent construction exceeds tolerance", ErrRadius)
				}
			} else if !support(a, b, c.Center, clear, nil) {
				return empty, fmt.Errorf("%w: nonadjacent supporting-line clearance", ErrRadius)
			}
		}
		for j := 0; j < i; j++ {
			if r.Corners[j] == nil {
				continue
			}
			if e := tick(); e != nil {
				return empty, e
			}
			sep := qa(qa(num(radius), num(radii[j])), qm(num(8), num(o.Tolerance)))
			if squaredDistance(c.Center, r.Corners[j].Center).Cmp(qm(sep, sep)) <= 0 {
				return empty, fmt.Errorf("%w: corner disks are not strictly separated", ErrRadius)
			}
		}
	}
	for i, c := range r.Corners {
		j := (i + 1) % n
		var centers [2]*geom.Vec2
		if c != nil {
			centers[0] = &c.Center
		}
		if r.Corners[j] != nil {
			centers[1] = &r.Corners[j].Center
		}
		if !retainedSelectedEdge(polygon[i], polygon[j], out[i], in[j], centers, o.Tolerance) {
			return empty, fmt.Errorf("%w: represented straight-edge tangency exceeds tolerance", ErrRadius)
		}
	}
	type piece struct {
		start  geom.Vec2
		corner int
		arc    bool
	}
	var pieces []piece
	for i := range polygon {
		if r.Corners[i] != nil {
			pieces = append(pieces, piece{in[i], i, true})
		}
		pieces = append(pieces, piece{out[i], i, false})
	}
	height, count := zMax-zMin, len(pieces)
	vertices := make([]geom.Vec3, 2*count)
	for i, p := range pieces {
		vertices[i] = geom.Vec3{X: p.start.X, Y: p.start.Y, Z: zMin}
		vertices[count+i] = geom.Vec3{X: p.start.X, Y: p.start.Y, Z: zMax}
	}
	edges := make([]parametric.Edge, 0, 3*count)
	var capUses [2][]trimmed.Use
	for layer, z := range [2]float64{zMin, zMax} {
		for k, piece := range pieces {
			if e := tick(); e != nil {
				return empty, e
			}
			a, b := layer*count+k, layer*count+(k+1)%count
			var curve geom.Curve
			var interval parametric.Interval
			var uv trimmed.PCurve
			if piece.arc {
				i := piece.corner
				c, radius := r.Corners[i], radii[i]
				frame, e := geom.NewFrame(geom.Vec3{X: c.Center.X, Y: c.Center.Y, Z: z}, frames[i].XAxis(), geom.Vec3{Z: 1})
				if e != nil {
					return empty, e
				}
				curve, e = geom.NewEllipse(frame, radius, radius)
				if e != nil {
					return empty, e
				}
				interval.Max = c.Sweep
				x, y := frames[i].XAxis().Scale(radius), frames[i].YAxis().Scale(radius)
				uv = trimmed.EllipseUV{Center: c.Center, Cos: geom.Vec2{X: x.X, Y: x.Y}, Sin: geom.Vec2{X: y.X, Y: y.Y}, To: c.Sweep}
				if layer == 0 {
					c.BottomArc = len(edges)
				} else {
					c.TopArc = len(edges)
				}
			} else {
				d := vertices[b].Sub(vertices[a])
				line, e := geom.NewLine(vertices[a], d)
				if e != nil {
					return empty, e
				}
				curve, interval.Max = line, d.Length()
			}
			capUses[layer] = append(capUses[layer], trimmed.Use{Edge: len(edges), UVStart: pieces[k].start, UVEnd: pieces[(k+1)%count].start, Curve: uv})
			edges = append(edges, parametric.Edge{Start: a, End: b, Curve: curve, Range: interval})
		}
	}
	for k := range pieces {
		line, e := geom.NewLine(vertices[k], geom.Vec3{Z: 1})
		if e != nil {
			return empty, e
		}
		edges = append(edges, parametric.Edge{Start: k, End: count + k, Curve: line, Range: parametric.Interval{Max: height}})
	}
	faces := make([]trimmed.Face, 2, 2+count)
	for i, z := range [2]float64{zMin, zMax} {
		frame, e := geom.NewFrame(geom.Vec3{Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return empty, e
		}
		s, e := geom.NewPlaneSurface(frame)
		if e != nil {
			return empty, e
		}
		faces[i] = trimmed.Face{Surface: s, Outer: capUses[i], Reversed: i == 0}
	}
	for k, piece := range pieces {
		i := piece.corner
		var surface geom.Surface
		var end float64
		if piece.arc {
			c, e := geom.NewCylinderSurface(frames[i], radii[i])
			if e != nil {
				return empty, e
			}
			surface, end = c, r.Corners[i].Sweep
			r.Corners[i].CylinderFace = len(faces)
		} else {
			a, b := vertices[k], vertices[(k+1)%count]
			d := b.Sub(a)
			frame, e := geom.NewFrame(a, d, geom.Vec3{X: d.Y, Y: -d.X})
			if e != nil {
				return empty, e
			}
			plane, e := geom.NewPlaneSurface(frame)
			if e != nil {
				return empty, e
			}
			surface, end = plane, d.Length()
			r.StraightFaces[i] = len(faces)
		}
		faces = append(faces, trimmed.Face{Surface: surface, Outer: []trimmed.Use{
			{Edge: k, UVEnd: geom.Vec2{X: end}},
			{Edge: 2*count + (k+1)%count, UVStart: geom.Vec2{X: end}, UVEnd: geom.Vec2{X: end, Y: height}},
			{Edge: count + k, Reversed: true, UVStart: geom.Vec2{X: end, Y: height}, UVEnd: geom.Vec2{Y: height}},
			{Edge: 2*count + k, Reversed: true, UVStart: geom.Vec2{Y: height}},
		}})
	}
	s, e := trimmed.NewSolidContext(ctx, vertices, edges, faces, o.Tolerance)
	if e != nil {
		return empty, e
	}
	if e := tick(); e != nil {
		return empty, e
	}
	r.Solid, r.NominalArea, r.NominalPerimeter, r.NominalVolume = s, area, perimeter, area*height
	if !finite(area) || !finite(perimeter) || !finite(r.NominalVolume) || area <= 0 {
		return empty, ErrInvalid
	}
	return r, nil
}
