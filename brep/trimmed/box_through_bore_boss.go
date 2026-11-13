package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// BoxBoreAndBoss retains a rectangular stock, a through or bottom-entry blind bore and a
// circular boss seated on its top face. Both disks are strictly interior and
// disjoint. The boss consumes only its matching cap footprint.
func BoxBoreAndBoss(ctx context.Context, frame geom.Frame, size geom.Vec3, bore, boss ExtrusionCircle, boreDepth, height float64) (Solid, error) {
	if ctx == nil || !size.IsFinite() || min(size.X, size.Y, size.Z) <= 0 || !finite(height) || height <= 0 || !finite(boreDepth) || boreDepth <= 0 || boreDepth > size.Z {
		return Solid{}, geom.ErrInvalid
	}
	tol := max(size.X, size.Y, size.Z, height) * 1e-11
	gap := 8 * tol
	for _, disk := range []ExtrusionCircle{bore, boss} {
		c, r := disk.Center, disk.Radius
		if !uvFinite(c) || !finite(r) || r <= gap || min(c.X, c.Y, size.X-c.X, size.Y-c.Y)-r <= gap {
			return Solid{}, geom.ErrDomain
		}
	}
	sum := new(big.Rat).Add(new(big.Rat).Add(ratInset(bore.Radius), ratInset(boss.Radius)), ratInset(gap))
	if extrusionCenterDistanceSquared(bore, boss).Cmp(sum.Mul(sum, sum)) <= 0 || height <= gap {
		return Solid{}, geom.ErrDomain
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	outer := []geom.Vec2{{}, {X: size.X}, {X: size.X, Y: size.Y}, {Y: size.Y}}
	for i, z := range []float64{0, size.Z} {
		if err := b.horizontal(outer, nil, z, i == 0, 0); err != nil {
			return Solid{}, err
		}
	}
	for i, p := range outer {
		if err := b.wall(p, outer[(i+1)%4], 0, size.Z); err != nil {
			return Solid{}, err
		}
	}
	if boreDepth == size.Z {
		if err := b.addBoxBore(bore, size.Z); err != nil {
			return Solid{}, err
		}
	} else {
		if size.Z-boreDepth <= gap {
			return Solid{}, geom.ErrDomain
		}
		if err := b.addBoxBottomBlindBore(bore, boreDepth); err != nil {
			return Solid{}, err
		}
	}
	if err := b.addBoxBoss(boss, size.Z, size.Z+height); err != nil {
		return Solid{}, err
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tol)
}

func (b *cornerCylinderBuilder) addBoxBoss(h ExtrusionCircle, lo, hi float64) error {
	rim, err := mixedCardinalPoints(h.Center, h.Radius)
	if err != nil {
		return err
	}
	for i, z := range []float64{lo, hi} {
		plane := b.faces[1].Surface
		if i == 1 {
			f, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{Z: z}), b.frame.XAxis(), b.frame.ZAxis())
			if err != nil {
				return err
			}
			plane, err = geom.NewPlaneSurface(f)
			if err != nil {
				return err
			}
		}
		uv := rim
		if i == 0 {
			uv = [4]geom.Vec2{rim[0], rim[3], rim[2], rim[1]}
		}
		points := make([]geom.Vec3, 4)
		curves := map[int]PCurve{}
		for j, p := range uv {
			points[j] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
			curves[j], err = extrusionArc(h, j, p, uv[(j+1)%4], i == 0)
			if err != nil {
				return err
			}
		}
		wire, err := b.wire(plane, points, uv[:], curves)
		if err != nil {
			return err
		}
		if i == 0 {
			b.faces[1].Holes = append(b.faces[1].Holes, wire)
		} else {
			b.faces = append(b.faces, Face{Surface: plane, Outer: wire})
		}
	}
	f, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y}), b.frame.XAxis(), b.frame.ZAxis())
	if err != nil {
		return err
	}
	wall, err := geom.NewCylinderSurface(f, h.Radius)
	if err != nil {
		return err
	}
	for i, p := range rim {
		q := rim[(i+1)%4]
		a, z := float64(i)*(math.Pi/2), float64(i+1)*(math.Pi/2)
		points := []geom.Vec3{{X: p.X, Y: p.Y, Z: lo}, {X: q.X, Y: q.Y, Z: lo}, {X: q.X, Y: q.Y, Z: hi}, {X: p.X, Y: p.Y, Z: hi}}
		uv := []geom.Vec2{{X: a, Y: lo}, {X: z, Y: lo}, {X: z, Y: hi}, {X: a, Y: hi}}
		if err := b.face(wall, points, uv, nil, false); err != nil {
			return err
		}
	}
	return nil
}

func (b *cornerCylinderBuilder) addBoxBottomBlindBore(h ExtrusionCircle, depth float64) error {
	rim, err := mixedCardinalPoints(h.Center, h.Radius)
	if err != nil {
		return err
	}
	for i, z := range []float64{0, depth} {
		plane := b.faces[0].Surface
		if i == 1 {
			f, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{Z: z}), b.frame.XAxis(), b.frame.ZAxis())
			if err != nil {
				return err
			}
			plane, err = geom.NewPlaneSurface(f)
			if err != nil {
				return err
			}
		}
		uv := rim
		if i == 0 {
			uv = [4]geom.Vec2{rim[0], rim[3], rim[2], rim[1]}
		}
		points := make([]geom.Vec3, 4)
		curves := map[int]PCurve{}
		for j, p := range uv {
			points[j] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
			curves[j], err = extrusionArc(h, j, p, uv[(j+1)%4], i == 0)
			if err != nil {
				return err
			}
		}
		wire, err := b.wire(plane, points, uv[:], curves)
		if err != nil {
			return err
		}
		if i == 0 {
			b.faces[0].Holes = append(b.faces[0].Holes, wire)
		} else {
			b.faces = append(b.faces, Face{Surface: plane, Outer: wire, Reversed: true})
		}
	}
	f, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y}), b.frame.XAxis(), b.frame.ZAxis())
	if err != nil {
		return err
	}
	wall, err := geom.NewCylinderSurface(f, h.Radius)
	if err != nil {
		return err
	}
	for i, p := range rim {
		q := rim[(i+1)%4]
		a, z := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		points := []geom.Vec3{{X: p.X, Y: p.Y}, {X: q.X, Y: q.Y}, {X: q.X, Y: q.Y, Z: depth}, {X: p.X, Y: p.Y, Z: depth}}
		uv := []geom.Vec2{{X: a}, {X: z}, {X: z, Y: depth}, {X: a, Y: depth}}
		if err := b.face(wall, points, uv, nil, true); err != nil {
			return err
		}
	}
	return nil
}
