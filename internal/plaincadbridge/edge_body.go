package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// edgeBody retains a canonical rectangular-stock recipe, not a display mesh.
// Its frame maps canonical XYZ to world space; all added features are replayed
// on the actual treated analytic boundary by the qualified core constructor.
type edgeBody struct {
	frame                 geom.Frame
	length, width, height float64
	radius, chamfer       float64
	bottomFront           bool
	bore                  *trimmed.ExtrusionCircle
	pocket                *trimmed.ExtrusionRectangle
	boss                  *trimmed.ExtrusionBoss
	chain                 fillet.LongEdgeChain
}

func frameMap(f geom.Frame) geom.Affine {
	x, y, z, o := f.XAxis(), f.YAxis(), f.ZAxis(), f.Origin()
	return geom.Affine{{x.X, y.X, z.X, o.X}, {x.Y, y.Y, z.Y, o.Y}, {x.Z, y.Z, z.Z, o.Z}}
}
func nearPoint(a, b geom.Vec3) bool { return a.Sub(b).Length() <= 1e-7 }
func sameEnds(a, b, c, d geom.Vec3) bool {
	return nearPoint(a, c) && nearPoint(b, d) || nearPoint(a, d) && nearPoint(b, c)
}
func unsupportedEdge(s string) error { return &failureError{"UNSUPPORTED", s} }
func rebuildEdge(body edgeBody) (*edgeBody, error) {
	chain, err := fillet.TreatLongCapEdgesOptional(context.Background(), body.length, body.width, body.height, body.radius, body.chamfer, body.chamfer, body.bottomFront, body.bore, body.pocket, body.boss, fillet.Options{})
	if err != nil {
		return nil, unsupportedEdge("edge-treated stock recipe is outside the qualified domain: " + err.Error())
	}
	body.chain = chain
	return &body, nil
}
func (b *Bridge) putEdge(body *edgeBody) (Shape, error) {
	if !cardinalFrame(body.frame) {
		return Shape{}, unsupportedEdge("treated stock requires a signed-cardinal frame")
	}
	// Every axis extremum survives the strictly interior feature clearances.
	// Cardinal placement makes these exact recipe extrema also world extrema.
	top := body.height
	if body.boss != nil {
		top += body.boss.Height
	}
	if !finite(top) {
		return Shape{}, errors.New("treated-stock bound exceeds numeric range")
	}
	bounds := geom.Box{Max: geom.Vec3{X: body.length, Y: body.width, Z: top}}
	// Preserve represented corners/features under the admitted placement; a
	// finite but huge translation must not collapse an extent or shift a rim.
	for _, p := range body.chain.Solid.Vertices() {
		world := body.frame.ToWorld(p)
		if !world.IsFinite() || body.frame.ToLocal(world).Sub(p).Length() > 1e-9 {
			return Shape{}, unsupportedEdge("placement cannot preserve represented treated-stock vertices")
		}
	}
	for _, x := range []float64{0, body.length} {
		for _, y := range []float64{0, body.width} {
			for _, z := range []float64{0, top} {
				p := geom.Vec3{X: x, Y: y, Z: z}
				world := body.frame.ToWorld(p)
				if !world.IsFinite() || body.frame.ToLocal(world) != p {
					return Shape{}, unsupportedEdge("placement cannot preserve represented treated-stock bounds")
				}
			}
		}
	}
	box, err := frameMap(body.frame).ApplyBox(bounds)
	if err != nil {
		return Shape{}, err
	}
	if box.Max.X <= box.Min.X || box.Max.Y <= box.Min.Y || box.Max.Z <= box.Min.Z {
		return Shape{}, unsupportedEdge("placement collapses treated-stock extent")
	}
	volume, area := body.chain.NominalVolume, body.chain.NominalSurfaceArea
	if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
		return Shape{}, errors.New("invalid treated-stock measures")
	}
	return b.publish(stored{treated: body, result: Shape{GeometryAssertions: Assertions{true, volume, area, 1}, Bounds: Bounds{coordinates(box.Min), coordinates(box.Max)}}})
}
func (body *edgeBody) tessellate(o parametric.TessellationOptions) (mesh.Mesh, error) {
	m, err := body.chain.Solid.Tessellate(context.Background(), o)
	if err != nil {
		return mesh.Mesh{}, err
	}
	f := frameMap(body.frame)
	for i, p := range m.Vertices {
		m.Vertices[i] = f.Apply(p)
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}
func (body *edgeBody) transformed(m geom.Affine) (*edgeBody, error) {
	if err := rigid(m); err != nil {
		return nil, err
	}
	if m.Determinant() <= 0 {
		return nil, unsupportedEdge("treated stock placement requires an orientation-preserving rigid transform")
	}
	f, err := geom.NewFrame(m.Apply(body.frame.Origin()), m.ApplyVector(body.frame.XAxis()), m.ApplyVector(body.frame.ZAxis()))
	if err != nil {
		return nil, err
	}
	if f.XAxis() != m.ApplyVector(body.frame.XAxis()) || f.YAxis() != m.ApplyVector(body.frame.YAxis()) || f.ZAxis() != m.ApplyVector(body.frame.ZAxis()) {
		return nil, unsupportedEdge("placement cannot preserve the represented treated-stock frame")
	}
	if !cardinalFrame(f) {
		return nil, unsupportedEdge("treated stock placement requires a signed-cardinal frame")
	}
	result := *body
	result.frame = f
	return &result, nil
}

func planarEdge(s brep.Solid, start, end geom.Vec3) (geom.Vec3, geom.Vec3, bool) {
	v := s.Vertices()
	var first, last geom.Vec3
	found := false
	for id, e := range s.Edges() {
		if !sameEnds(start, end, v[e.Start], v[e.End]) {
			continue
		}
		var ns []geom.Vec3
		for i, f := range s.Faces() {
			for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
				for _, u := range loop {
					if u.Edge == id {
						p, err := s.FacePlane(i)
						if err != nil {
							return geom.Vec3{}, geom.Vec3{}, false
						}
						ns = append(ns, p.Normal())
					}
				}
			}
		}
		if len(ns) == 2 && ns[0].Dot(ns[1]) < 1-1e-10 {
			if found {
				return geom.Vec3{}, geom.Vec3{}, false
			}
			first, last, found = v[e.Start], v[e.End], true
		}
	}
	return first, last, found
}
func trimmedEdge(s trimmed.Solid, start, end geom.Vec3) (geom.Vec3, geom.Vec3, bool) {
	v := s.Vertices()
	var first, last geom.Vec3
	found := false
	for id, e := range s.Edges() {
		base, _ := parametric.CurveMapping(e.Curve)
		if _, ok := base.(geom.Line); !ok || !sameEnds(start, end, v[e.Start], v[e.End]) {
			continue
		}
		var ns []geom.Vec3
		for _, f := range s.Faces() {
			for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				for _, u := range loop {
					if u.Edge != id {
						continue
					}
					uv := geom.Vec2{X: (u.UVStart.X + u.UVEnd.X) / 2, Y: (u.UVStart.Y + u.UVEnd.Y) / 2}
					if u.Curve != nil {
						var err error
						uv, err = u.Curve.EvalUV(.5)
						if err != nil {
							return geom.Vec3{}, geom.Vec3{}, false
						}
					}
					du, dv, err := f.Surface.Partials(uv.X, uv.Y)
					if err != nil {
						return geom.Vec3{}, geom.Vec3{}, false
					}
					n, err := du.Cross(dv).Unit()
					if err != nil {
						return geom.Vec3{}, geom.Vec3{}, false
					}
					if f.Reversed {
						n = n.Scale(-1)
					}
					ns = append(ns, n)
				}
			}
		}
		if len(ns) == 2 && ns[0].Dot(ns[1]) < 1-1e-10 {
			if found {
				return geom.Vec3{}, geom.Vec3{}, false
			}
			first, last, found = v[e.Start], v[e.End], true
		}
	}
	return first, last, found
}
func (s stored) edgeAvailable(start, end geom.Vec3) bool {
	if !start.IsFinite() || !end.IsFinite() || nearPoint(start, end) {
		return false
	}
	if s.treated != nil {
		_, _, ok := trimmedEdge(s.treated.chain.Solid, s.treated.frame.ToLocal(start), s.treated.frame.ToLocal(end))
		return ok
	}
	if s.bore != nil {
		_, _, ok := trimmedEdge(s.bore.solid, start, end)
		return ok
	}
	if s.cylinder != nil {
		return false
	}
	_, _, ok := planarEdge(s.solid, start, end)
	return ok
}

func inferEdgeStock(s brep.Solid, start, end, normal geom.Vec3, top bool) (edgeBody, error) {
	fail := func() (edgeBody, error) {
		return edgeBody{}, unsupportedEdge("edge treatment requires a represented orthogonal rectangular stock and an original cap edge")
	}
	start, end, ok := planarEdge(s, start, end)
	if !ok {
		return fail()
	}
	if len(s.Vertices()) != 8 || len(s.Edges()) != 12 || len(s.Faces()) != 6 {
		return fail()
	}
	for _, f := range s.Faces() {
		if len(f.Boundary) != 4 || len(f.Holes) != 0 {
			return fail()
		}
	}
	axis, err := normal.Unit()
	if err != nil || exactDotDifference(end, start, axis) != 0 {
		return fail()
	}
	frame, err := geom.NewFrame(start, end.Sub(start), axis)
	if err != nil {
		return fail()
	}
	extent := func(f geom.Frame) (geom.Vec3, geom.Vec3) {
		lo, hi := geom.Vec3{}, geom.Vec3{}
		for _, p := range s.Vertices() {
			q := f.ToLocal(p)
			lo.X = math.Min(lo.X, q.X)
			lo.Y = math.Min(lo.Y, q.Y)
			lo.Z = math.Min(lo.Z, q.Z)
			hi.X = math.Max(hi.X, q.X)
			hi.Y = math.Max(hi.Y, q.Y)
			hi.Z = math.Max(hi.Z, q.Z)
		}
		return lo, hi
	}
	lo, hi := extent(frame)
	if lo.Z != 0 && hi.Z != 0 {
		return fail()
	}
	if (top && hi.Z > 0) || (!top && lo.Z < 0) {
		axis = axis.Scale(-1)
		frame, err = geom.NewFrame(start, end.Sub(start), axis)
		if err != nil {
			return fail()
		}
	}
	lo, hi = extent(frame)
	if lo.Y != 0 && hi.Y != 0 {
		return fail()
	}
	if lo.Y < 0 {
		start, end = end, start
		frame, err = geom.NewFrame(start, end.Sub(start), axis)
		if err != nil {
			return fail()
		}
	}
	lo, hi = extent(frame)
	if lo.X != 0 || lo.Y != 0 || hi.X <= 0 || hi.Y <= 0 || hi.Z-lo.Z <= 0 {
		return fail()
	}
	frame, err = geom.NewFrame(frame.Origin().Add(axis.Scale(lo.Z)), frame.XAxis(), axis)
	if err != nil {
		return fail()
	}
	if !cardinalFrame(frame) {
		return fail()
	}
	length, width, height := hi.X, hi.Y, hi.Z-lo.Z
	for _, p := range s.Vertices() {
		q := frame.ToLocal(p)
		if (q.X != 0 && q.X != length) || (q.Y != 0 && q.Y != width) || (q.Z != 0 && q.Z != height) || frame.ToWorld(q) != p {
			return fail()
		}
	}
	if !rectangularFaces(s, frame, geom.Vec3{}, geom.Vec3{X: length, Y: width, Z: height}) {
		return fail()
	}
	return edgeBody{frame: frame, length: length, width: width, height: height}, nil
}
func (b *Bridge) treatEdge(s stored, q request) (Shape, error) {
	if q.EdgeStart == nil || q.EdgeEnd == nil || q.Normal == nil {
		return Shape{}, errors.New("edge treatment requires edge endpoints and extrusion normal")
	}
	start, end := q.EdgeStart.vec(), q.EdgeEnd.vec()
	if !s.edgeAvailable(start, end) {
		return Shape{}, unsupportedEdge(q.Op + " failed: edge reference no longer identifies a complete sharp edge")
	}
	size := float64(q.Radius)
	if q.Op == "chamfer" {
		size = float64(q.Distance)
	}
	if !finite(size) || size <= 0 {
		return Shape{}, errors.New(q.Op + " failed: size must be positive")
	}
	var body edgeBody
	if s.treated != nil {
		body = *s.treated
	} else {
		if s.bore != nil || s.cylinder != nil {
			return Shape{}, unsupportedEdge(q.Op + " failed: stock type is unsupported")
		}
		var err error
		body, err = inferEdgeStock(s.solid, start, end, q.Normal.vec(), q.Op == "fillet")
		if err != nil {
			return Shape{}, fmt.Errorf("%s failed: %w", q.Op, err)
		}
	}
	a, c, ok := trimmedEdge(body.chain.Solid, body.frame.ToLocal(start), body.frame.ToLocal(end))
	if s.treated == nil {
		a, c = body.frame.ToLocal(start), body.frame.ToLocal(end)
		ok = true
	}
	if !ok || math.Abs(a.X-c.X-body.length) > 1e-7 && math.Abs(c.X-a.X-body.length) > 1e-7 || math.Abs(a.Y-c.Y) > 1e-7 || math.Abs(a.Z-c.Z) > 1e-7 {
		return Shape{}, unsupportedEdge(q.Op + " failed: edge must span the retained stock length")
	}
	at := func(y, z float64) bool { return math.Abs(a.Y-y) <= 1e-7 && math.Abs(a.Z-z) <= 1e-7 }
	if q.Op == "fillet" {
		if body.radius != 0 || !at(0, body.height) {
			return Shape{}, unsupportedEdge("fillet failed: selected corner is outside the qualified recipe")
		}
		body.radius = size
	} else {
		if body.chamfer != 0 {
			return Shape{}, unsupportedEdge("chamfer failed: a chamfer is already present")
		}
		if at(0, 0) {
			body.bottomFront = true
		} else if at(body.width, body.height) {
			body.bottomFront = false
		} else {
			return Shape{}, unsupportedEdge("chamfer failed: selected corner is outside the qualified recipe")
		}
		body.chamfer = size
	}
	next, err := rebuildEdge(body)
	if err != nil {
		return Shape{}, fmt.Errorf("%s failed: %w", q.Op, err)
	}
	return b.putEdge(next)
}
