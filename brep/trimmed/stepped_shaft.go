package trimmed

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/brep"
	"plainsolid/geom"
)

// SteppedShaft builds an analytic shaft below axial zero and a wider head
// above it. An optional simple polygon opens into the head from its top cap.
// SocketFloor is an axial coordinate strictly inside the head.
type SteppedShaft struct {
	Frame                   geom.Frame
	ShaftRadius, HeadRadius float64
	ShaftLength, HeadHeight float64
	Socket                  []geom.Vec2
	SocketFloor             float64
}

func (p SteppedShaft) Solid(ctx context.Context) (Solid, error) {
	tol := max(p.HeadHeight, p.ShaftLength, p.HeadRadius) * 1e-11
	gap := 8 * tol
	if ctx == nil || !finite(tol) || tol <= 0 || !finite(p.ShaftRadius) ||
		p.ShaftRadius <= gap || p.HeadRadius-p.ShaftRadius <= gap || min(p.ShaftLength, p.HeadHeight) <= gap {
		return Solid{}, geom.ErrDomain
	}
	socket, err := p.socketLoop(gap)
	if err != nil {
		return Solid{}, err
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: p.Frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	circle := func(r float64) []geom.Vec2 {
		return []geom.Vec2{{X: r}, {Y: r}, {X: -r}, {Y: -r}}
	}
	shaft, head := circle(p.ShaftRadius), circle(p.HeadRadius)
	arcs := map[int][2]float64{}
	for i := range 4 {
		arcs[i] = [2]float64{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
	}
	if err := b.horizontal(shaft, arcs, -p.ShaftLength, true, p.ShaftRadius); err != nil {
		return Solid{}, err
	}
	if err := b.horizontal(head, arcs, 0, true, p.HeadRadius); err != nil {
		return Solid{}, err
	}
	if err := b.addShaftShoulderHole(p.ShaftRadius); err != nil {
		return Solid{}, err
	}
	if err := b.horizontal(head, arcs, p.HeadHeight, false, p.HeadRadius); err != nil {
		return Solid{}, err
	}
	if len(socket) > 0 {
		if err := b.addShaftSocket(socket, p.SocketFloor, p.HeadHeight); err != nil {
			return Solid{}, err
		}
	}
	for i := range 4 {
		a := arcs[i]
		j := (i + 1) % 4
		if err := b.cylinderBand(a[0], a[1], -p.ShaftLength, 0, p.ShaftRadius, false, shaft[i], shaft[j]); err != nil {
			return Solid{}, err
		}
		if err := b.cylinderBand(a[0], a[1], 0, p.HeadHeight, p.HeadRadius, false, head[i], head[j]); err != nil {
			return Solid{}, err
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tol)
}

func (p SteppedShaft) socketLoop(gap float64) ([]geom.Vec2, error) {
	if len(p.Socket) == 0 && p.SocketFloor == 0 {
		return nil, nil
	}
	if len(p.Socket) < 3 || len(p.Socket) > 64 || !finite(p.SocketFloor) ||
		min(p.SocketFloor, p.HeadHeight-p.SocketFloor) <= gap {
		return nil, geom.ErrDomain
	}
	r := new(big.Rat).Sub(ratInset(p.HeadRadius), ratInset(gap))
	r.Mul(r, r)
	points := make([]geom.Vec3, len(p.Socket))
	loop := append([]geom.Vec2(nil), p.Socket...)
	for i, q := range loop {
		if !uvFinite(q) {
			return nil, geom.ErrInvalid
		}
		x, y := ratInset(q.X), ratInset(q.Y)
		if new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y)).Cmp(r) >= 0 {
			return nil, geom.ErrDomain
		}
		points[i] = geom.Vec3{X: q.X, Y: q.Y}
	}
	if _, err := brep.Prism(points, geom.Vec3{Z: 1}); err != nil {
		return nil, err
	}
	winding, err := polygonWinding(loop)
	if err != nil {
		return nil, err
	}
	if winding > 0 {
		slices.Reverse(loop)
	}
	return loop, nil
}

func polygonWinding(points []geom.Vec2) (int, error) {
	left := 0
	for i, p := range points {
		q := points[left]
		if p.X < q.X || p.X == q.X && p.Y < q.Y {
			left = i
		}
	}
	n := len(points)
	return geom.Orient2D(points[(left+n-1)%n], points[left], points[(left+1)%n])
}

func (b *cornerCylinderBuilder) addShaftShoulderHole(radius float64) error {
	f := &b.faces[1]
	uv := []geom.Vec2{{X: radius}, {Y: -radius}, {X: -radius}, {Y: radius}}
	points := make([]geom.Vec3, len(uv))
	curves := map[int]PCurve{}
	for i, p := range uv {
		points[i] = geom.Vec3{X: p.X, Y: p.Y}
		c, err := extrusionArc(ExtrusionCircle{Radius: radius}, i, p, uv[(i+1)%len(uv)], true)
		if err != nil {
			return err
		}
		curves[i] = c
	}
	loop, err := b.wire(f.Surface, points, uv, curves)
	if err != nil {
		return err
	}
	f.Holes = append(f.Holes, loop)
	return nil
}

func (b *cornerCylinderBuilder) addShaftSocket(socket []geom.Vec2, floor, top float64) error {
	f := &b.faces[2]
	points := make([]geom.Vec3, len(socket))
	for i, p := range socket {
		points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: top}
	}
	loop, err := b.wire(f.Surface, points, socket, nil)
	if err != nil {
		return err
	}
	f.Holes = append(f.Holes, loop)
	for i, p := range socket {
		if err := b.wall(p, socket[(i+1)%len(socket)], floor, top); err != nil {
			return err
		}
	}
	cap := append([]geom.Vec2(nil), socket...)
	slices.Reverse(cap)
	return b.horizontal(cap, nil, floor, false, 0)
}
