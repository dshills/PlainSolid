package trimmed

import (
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Bounds encloses the original surfaces within conservative UV rectangles.
// Supported trims are lines, quadratic arcs, angular arcs, rational spline
// arcs and certified spline–plane and spline-pair arcs. Spline restrictions use positive
// control hulls. Unknown evaluators fail explicitly.
func (s Solid) Bounds() (geom.Box, error) {
	if len(s.faces) == 0 {
		return geom.Box{}, geom.ErrDegenerate
	}
	box, e := geom.Bounds(s.vertices)
	if e != nil {
		return geom.Box{}, e
	}
	for i, f := range s.faces {
		d, e := faceUVBounds(f)
		if e != nil {
			return geom.Box{}, fmt.Errorf("trimmed: bounds face %d: %w", i, e)
		}
		b, e := parametric.SurfaceBounds(f.Surface, parametric.Interval{Min: d.UMin, Max: d.UMax}, parametric.Interval{Min: d.VMin, Max: d.VMax})
		if e != nil {
			return geom.Box{}, e
		}
		box, e = box.Union(b)
		if e != nil {
			return geom.Box{}, e
		}
	}
	pad := func(lo, hi float64) (float64, float64) {
		a := s.tolerance + 64*0x1p-52*max(math.Abs(lo), math.Abs(hi))
		return max(-math.MaxFloat64, math.Nextafter(lo-a, math.Inf(-1))), min(math.MaxFloat64, math.Nextafter(hi+a, math.Inf(1)))
	}
	box.Min.X, box.Max.X = pad(box.Min.X, box.Max.X)
	box.Min.Y, box.Max.Y = pad(box.Min.Y, box.Max.Y)
	box.Min.Z, box.Max.Z = pad(box.Min.Z, box.Max.Z)
	return box, nil
}

func faceUVBounds(f Face) (geom.ParameterRectangle, error) {
	if len(f.Outer) == 0 {
		return geom.ParameterRectangle{}, geom.ErrDomain
	}
	lo, hi := f.Outer[0].UVStart, f.Outer[0].UVStart
	for _, l := range append([][]Use{f.Outer}, f.Holes...) {
		for _, u := range l {
			a, b, e := trimBounds(u)
			if e != nil {
				return geom.ParameterRectangle{}, e
			}
			lo.X, lo.Y = min(lo.X, a.X), min(lo.Y, a.Y)
			hi.X, hi.Y = max(hi.X, b.X), max(hi.Y, b.Y)
		}
	}
	return geom.ParameterRectangle{UMin: lo.X, UMax: hi.X, VMin: lo.Y, VMax: hi.Y}, nil
}

type uvBounded interface{ BoundsUV() (geom.UVBox, error) }

func trimBounds(u Use) (geom.Vec2, geom.Vec2, error) {
	lo, hi := u.UVStart, u.UVStart
	add := func(v geom.Vec2) {
		lo.X, lo.Y = min(lo.X, v.X), min(lo.Y, v.Y)
		hi.X, hi.Y = max(hi.X, v.X), max(hi.Y, v.Y)
	}
	add(u.UVEnd)
	c := u.Curve
	for {
		switch x := c.(type) {
		case nil:
		case *QuadraticUV:
			if x == nil {
				return geom.Vec2{}, geom.Vec2{}, geom.ErrInvalid
			}
			c = *x
			continue
		case QuadraticUV:
			add(x.Control)
		case *EllipseUV:
			if x == nil {
				return geom.Vec2{}, geom.Vec2{}, geom.ErrInvalid
			}
			c = *x
			continue
		case EllipseUV:
			bound := func(center, cosine, sine, linear float64) (float64, float64) {
				a, b := linear*x.From, linear*x.To
				reach := math.Nextafter(math.Hypot(cosine, sine), math.Inf(1))
				if cosine == 0 && sine == 0 {
					reach = 0
				}
				l, r := center+min(a, b)-reach, center+max(a, b)+reach
				return math.Nextafter(l, math.Inf(-1)), math.Nextafter(r, math.Inf(1))
			}
			a, b := bound(x.Center.X, x.Cos.X, x.Sin.X, x.Linear.X)
			d, e := bound(x.Center.Y, x.Cos.Y, x.Sin.Y, x.Linear.Y)
			add(geom.Vec2{X: a, Y: d})
			add(geom.Vec2{X: b, Y: e})
		case *SplineUV:
			if x == nil {
				return geom.Vec2{}, geom.Vec2{}, geom.ErrInvalid
			}
			c = *x
			continue
		case SplineUV:
			if x.Curve == nil {
				return geom.Vec2{}, geom.Vec2{}, geom.ErrInvalid
			}
			a, b := x.Curve.Domain()
			if x.Range.Min < a || x.Range.Max > b || x.Range.Min >= x.Range.Max {
				return geom.Vec2{}, geom.Vec2{}, geom.ErrDomain
			}
			if x.Range.Min != a || x.Range.Max != b {
				p, e := geom.NewExactSplineCurve(x.Curve, geom.SurfacePatchOptions{})
				if e == nil {
					p, e = p.Restrict(x.Range.Min, x.Range.Max, geom.SurfacePatchOptions{})
				}
				if e == nil {
					box, e := p.ProjectionBounds()
					if e == nil {
						add(geom.Vec2{X: box.Min.X, Y: box.Min.Y})
						add(geom.Vec2{X: box.Max.X, Y: box.Max.Y})
						break
					}
				}
			}
			for _, p := range x.Curve.Poles() {
				add(geom.Vec2{X: p.X, Y: p.Y})
			}
		case subTrim:
			c = x.base
			continue
		case reverseTrim:
			c = x.differentiableTrim
			continue
		case uvBounded:
			b, e := x.BoundsUV()
			if e != nil {
				return geom.Vec2{}, geom.Vec2{}, e
			}
			add(b.Min)
			add(b.Max)
		default:
			return geom.Vec2{}, geom.Vec2{}, fmt.Errorf("unsupported p-curve %T", c)
		}
		break
	}
	if !uvFinite(lo) || !uvFinite(hi) {
		return geom.Vec2{}, geom.Vec2{}, geom.ErrInvalid
	}
	return lo, hi, nil
}
