package parametric

import (
	"math/big"
	"plainsolid/geom"
)

// CardinalQuarterCylinder constructs the positive X-to-Y quarter of a cylinder.
// The frame axes must be signed coordinate axes. All chart poles and vertices
// must be exactly representable; no trigonometric endpoint rounding is used.
// Rational chart parameter t is tan(angle/2), not angle in radians.
func CardinalQuarterCylinder(frame geom.Frame, radius, height float64) (Solid, error) {
	if !finite(radius) || !finite(height) || radius <= 0 || height <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	axes := []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()}
	for _, a := range axes {
		n := 0
		for _, v := range []float64{a.X, a.Y, a.Z} {
			if v == 1 || v == -1 {
				n++
			} else if v != 0 {
				return Solid{}, geom.ErrInvalid
			}
		}
		if n != 1 {
			return Solid{}, geom.ErrInvalid
		}
	}
	if axes[0].Cross(axes[1]) != axes[2] {
		return Solid{}, geom.ErrInvalid
	}
	point := func(x, y, z float64) (geom.Vec3, error) {
		o := frame.Origin()
		out := [3]float64{}
		av := [3][3]float64{{axes[0].X, axes[0].Y, axes[0].Z}, {axes[1].X, axes[1].Y, axes[1].Z}, {axes[2].X, axes[2].Y, axes[2].Z}}
		for i, b := range []float64{o.X, o.Y, o.Z} {
			if !finite(b) {
				return geom.Vec3{}, geom.ErrInvalid
			}
			q := new(big.Rat).SetFloat64(b)
			for j, v := range []float64{x, y, z} {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(v), new(big.Rat).SetFloat64(av[j][i])))
			}
			f, ok := q.Float64()
			if !ok || !finite(f) {
				return geom.Vec3{}, geom.ErrUnrepresentable
			}
			out[i] = f
		}
		return geom.Vec3{X: out[0], Y: out[1], Z: out[2]}, nil
	}
	patch := func(kind int) (geom.Surface, error) {
		poles := []geom.Vec3{}
		for _, xy := range [][2]float64{{radius, 0}, {radius, radius}, {0, radius}} {
			for j := 0; j < 2; j++ {
				x, y, z := xy[0], xy[1], float64(j)*height
				if kind != 0 {
					z = float64(kind-1) * height
					if j == 0 {
						x, y = 0, 0
					}
				}
				p, e := point(x, y, z)
				if e != nil {
					return nil, e
				}
				poles = append(poles, p)
			}
		}
		return geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 1, 1, 2, 2})
	}
	ws, e := patch(0)
	if e != nil {
		return Solid{}, e
	}
	ls, e := patch(1)
	if e != nil {
		return Solid{}, e
	}
	hs, e := patch(2)
	if e != nil {
		return Solid{}, e
	}
	radial := func(x geom.Vec3) (geom.PlaneSurface, error) {
		f, e := geom.NewFrame(frame.Origin(), x, x.Cross(frame.ZAxis()))
		if e != nil {
			return geom.PlaneSurface{}, e
		}
		return geom.NewPlaneSurface(f)
	}
	start, e := radial(frame.XAxis())
	if e != nil {
		return Solid{}, e
	}
	end, e := radial(frame.YAxis())
	if e != nil {
		return Solid{}, e
	}
	wall := Face{Surface: ws, U: normalized, V: normalized, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {3, false}}}
	lo := Face{Surface: ls, U: normalized, V: normalized, Boundary: [4]Use{{4, false}, {5, false}, {0, true}, {6, false}}}
	hi := Face{Surface: hs, U: normalized, V: normalized, Boundary: [4]Use{{7, false}, {8, false}, {2, false}, {9, false}}, Reversed: true}
	first := Face{Surface: start, U: Interval{Max: radius}, V: Interval{Max: height}, Boundary: [4]Use{{6, true}, {3, true}, {9, false}, {10, false}}}
	last := Face{Surface: end, U: Interval{Max: radius}, V: Interval{Max: height}, Boundary: [4]Use{{5, false}, {1, false}, {8, true}, {10, false}}, Reversed: true}
	v := []geom.Vec3{}
	for _, p := range [][3]float64{{0, 0, 0}, {0, 0, height}, {radius, 0, 0}, {0, radius, 0}, {0, radius, height}, {radius, 0, height}} {
		q, e := point(p[0], p[1], p[2])
		if e != nil {
			return Solid{}, e
		}
		v = append(v, q)
	}
	edges := []Edge{
		{Start: 2, End: 3, Curve: iso{wall, 0}, Range: normalized},
		{Start: 3, End: 4, Curve: iso{wall, 1}, Range: normalized},
		{Start: 4, End: 5, Curve: iso{wall, 2}, Range: normalized},
		{Start: 5, End: 2, Curve: iso{wall, 3}, Range: normalized},
		{Start: 0, End: 0},
		{Start: 0, End: 3, Curve: iso{lo, 1}, Range: normalized},
		{Start: 2, End: 0, Curve: iso{lo, 3}, Range: normalized},
		{Start: 1, End: 1},
		{Start: 1, End: 4, Curve: iso{hi, 1}, Range: normalized},
		{Start: 5, End: 1, Curve: iso{hi, 3}, Range: normalized},
		{Start: 1, End: 0, Curve: iso{first, 3}, Range: normalized},
	}
	return NewSolid(v, edges, []Face{wall, lo, hi, first, last}, max(radius, height)*1e-12)
}
