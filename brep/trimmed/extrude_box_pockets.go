package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// BoxBlindPocket removes a rectangular footprint from an axial cap or from
// the complete floor of the immediately preceding pocket.
type BoxBlindPocket struct {
	Footprint ExtrusionRectangle
	Floor     float64
}

// ExtrudeBoxWithPocketsAndBore builds a rectangular stock with successively
// nested top pockets and an optional disjoint cylindrical through bore.
func ExtrudeBoxWithPocketsAndBore(ctx context.Context, frame geom.Frame, size geom.Vec3, pockets []BoxBlindPocket, bore *ExtrusionCircle) (Solid, error) {
	var bores []ExtrusionCircle
	if bore != nil {
		bores = []ExtrusionCircle{*bore}
	}
	return ExtrudeBoxWithPocketsAndBores(ctx, frame, size, pockets, bores)
}

// ExtrudeBoxWithPocketsAndBores retains disjoint through bores around one
// nested rectangular pocket chain.
func ExtrudeBoxWithPocketsAndBores(ctx context.Context, frame geom.Frame, size geom.Vec3, pockets []BoxBlindPocket, bores []ExtrusionCircle) (Solid, error) {
	if ctx == nil || !size.IsFinite() || size.X <= 0 || size.Y <= 0 || size.Z <= 0 || len(pockets) == 0 || len(pockets) > 8 {
		return Solid{}, geom.ErrInvalid
	}
	tol := max(size.X, size.Y, size.Z) * 1e-11
	gap := 8 * tol
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	stock := ExtrusionRectangle{Max: geom.Vec2{X: size.X, Y: size.Y}}
	bound, capHeight := stock, size.Z
	for _, p := range pockets {
		r := p.Footprint
		if !uvFinite(r.Min) || !uvFinite(r.Max) || !finite(p.Floor) ||
			r.Min.X-bound.Min.X <= gap || r.Min.Y-bound.Min.Y <= gap ||
			bound.Max.X-r.Max.X <= gap || bound.Max.Y-r.Max.Y <= gap ||
			r.Max.X-r.Min.X <= gap || r.Max.Y-r.Min.Y <= gap ||
			capHeight-p.Floor <= gap || p.Floor <= gap {
			return Solid{}, geom.ErrDomain
		}
		bound, capHeight = r, p.Floor
	}
	for i := range bores {
		bore := &bores[i]
		c, r := bore.Center, bore.Radius
		if !uvFinite(c) || !finite(r) || r <= gap || min(c.X, c.Y, size.X-c.X, size.Y-c.Y)-r <= gap {
			return Solid{}, geom.ErrDomain
		}
		p := pockets[0].Footprint
		closest := geom.Vec2{X: max(p.Min.X, min(p.Max.X, c.X)), Y: max(p.Min.Y, min(p.Max.Y, c.Y))}
		pad := new(big.Rat).Add(ratInset(r), ratInset(gap))
		if extrusionCenterDistanceSquared(*bore, ExtrusionCircle{Center: closest}).Cmp(pad.Mul(pad, pad)) <= 0 {
			return Solid{}, geom.ErrDomain
		}
		for _, prior := range bores[:i] {
			sep := new(big.Rat).Add(new(big.Rat).Add(ratInset(r), ratInset(prior.Radius)), ratInset(gap))
			if extrusionCenterDistanceSquared(*bore, prior).Cmp(sep.Mul(sep, sep)) <= 0 {
				return Solid{}, geom.ErrDomain
			}
		}
	}
	ring := extrusionRectangleRing(stock)
	// The rectangle helper supplies clockwise hole wires.
	outer := []geom.Vec2{ring[0], ring[3], ring[2], ring[1]}
	if err := b.horizontal(outer, nil, 0, true, 0); err != nil {
		return Solid{}, err
	}
	if err := b.horizontal(outer, nil, size.Z, false, 0); err != nil {
		return Solid{}, err
	}
	for i, p := range outer {
		if err := b.wall(p, outer[(i+1)%4], 0, size.Z); err != nil {
			return Solid{}, err
		}
	}
	capIndex, top := 1, size.Z
	for _, pocket := range pockets {
		inner := extrusionRectangleRing(pocket.Footprint)
		if err := b.addRectanglePocket(capIndex, inner, pocket.Floor, top); err != nil {
			return Solid{}, err
		}
		capIndex, top = len(b.faces)-1, pocket.Floor
	}
	for _, bore := range bores {
		if err := b.addBoxBore(bore, size.Z); err != nil {
			return Solid{}, err
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tol)
}

func (b *cornerCylinderBuilder) addRectanglePocket(cap int, rim []geom.Vec2, floor, top float64) error {
	points := make([]geom.Vec3, len(rim))
	for i, p := range rim {
		points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: top}
	}
	loop, err := b.wire(b.faces[cap].Surface, points, rim, nil)
	if err != nil {
		return err
	}
	b.faces[cap].Holes = append(b.faces[cap].Holes, loop)
	for i, p := range rim {
		if err := b.wall(p, rim[(i+1)%len(rim)], floor, top); err != nil {
			return err
		}
	}
	return b.horizontal([]geom.Vec2{rim[0], rim[3], rim[2], rim[1]}, nil, floor, false, 0)
}

func (b *cornerCylinderBuilder) addBoxBore(h ExtrusionCircle, height float64) error {
	return b.addBoxBoreAtCaps(h, height, [2]int{0, 1})
}

func (b *cornerCylinderBuilder) addBoxBoreAtCaps(h ExtrusionCircle, height float64, caps [2]int) error {
	r, c := h.Radius, h.Center
	rim, err := mixedCardinalPoints(c, r)
	if err != nil {
		return err
	}
	for i, z := range []float64{0, height} {
		id := caps[i]
		points := make([]geom.Vec3, 4)
		uv := []geom.Vec2{rim[0], rim[3], rim[2], rim[1]}
		curves := map[int]PCurve{}
		for i, p := range uv {
			points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
			curves[i], err = extrusionArc(h, i, p, uv[(i+1)%4], true)
			if err != nil {
				return err
			}
		}
		wire, err := b.wire(b.faces[id].Surface, points, uv, curves)
		if err != nil {
			return err
		}
		b.faces[id].Holes = append(b.faces[id].Holes, wire)
	}
	frame, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{X: c.X, Y: c.Y}), b.frame.XAxis(), b.frame.ZAxis())
	if err != nil {
		return err
	}
	wall, err := geom.NewCylinderSurface(frame, r)
	if err != nil {
		return err
	}
	for i, p := range rim {
		q := rim[(i+1)%4]
		a, z := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		points := []geom.Vec3{{X: p.X, Y: p.Y}, {X: q.X, Y: q.Y}, {X: q.X, Y: q.Y, Z: height}, {X: p.X, Y: p.Y, Z: height}}
		uv := []geom.Vec2{{X: a}, {X: z}, {X: z, Y: height}, {X: a, Y: height}}
		if err := b.face(wall, points, uv, nil, true); err != nil {
			return err
		}
	}
	return nil
}
