package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// OffsetDualDiskPoint preserves exposed source support ownership. Stock axis
// masks use original local coordinates; hidden cutter circles have no mask.
type OffsetDualDiskPoint struct {
	Location parametric.Location
	Support  geom.BoxDualCircleSupport
}

// ClassifyRational classifies against the immutable original source using exact
// rational stock, cap and circle equations. A blind floor is retained boundary;
// the through-cut footprint is absent even on either original axial cap.
func (c OffsetSideOpenDualDiskCut) ClassifyRational(ctx context.Context, point [3]*big.Rat) (OffsetDualDiskPoint, error) {
	zero := OffsetDualDiskPoint{Location: parametric.Outside}
	s := c.source
	if ctx == nil || s == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, q := range point {
		if q == nil {
			return zero, geom.ErrInvalid
		}
	}
	origin := s.frame.Origin()
	var local [3]*big.Rat
	for i, a := range [3]geom.Vec3{s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()} {
		for j, v := range [3]float64{a.X, a.Y, a.Z} {
			if v != 0 {
				local[i] = new(big.Rat).Mul(new(big.Rat).Sub(point[j], ratInset([3]float64{origin.X, origin.Y, origin.Z}[j])), ratInset(v))
				break
			}
		}
	}
	var mask geom.BoxDualCircleSupport
	for i, lo := range [3]float64{s.min.X, s.min.Y, 0} {
		hi := [3]float64{s.max.X, s.max.Y, s.height}[i]
		l, h := local[i].Cmp(ratInset(lo)), local[i].Cmp(ratInset(hi))
		if l < 0 || h > 0 {
			return zero, nil
		}
		if l == 0 || h == 0 {
			mask |= geom.BoxDualCircleStockX << i
		}
	}
	if z := local[2].Cmp(ratInset(s.depth)); z <= 0 {
		signs := [2]int{}
		u := 1
		for i, d := range s.discs {
			dx := new(big.Rat).Sub(local[0], ratInset(d.Center.X))
			dy := new(big.Rat).Sub(local[1], ratInset(d.Center.Y))
			v := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
			r := ratInset(d.Radius)
			signs[i] = v.Cmp(new(big.Rat).Mul(r, r))
			if signs[i] < u {
				u = signs[i]
			}
		}
		if u < 0 && (z < 0 || s.depth == s.height) {
			return zero, nil
		}
		if u == 0 {
			for i, sign := range signs {
				if sign == 0 {
					mask |= geom.BoxDualCircleFirstWall << i
				}
			}
		}
		if u <= 0 && z == 0 && s.depth < s.height {
			mask |= geom.BoxDualCircleBlindFloor
		}
	}
	out := OffsetDualDiskPoint{Location: parametric.Inside, Support: mask}
	if mask != 0 {
		out.Location = parametric.Boundary
	}
	return out, nil
}
func (c OffsetSideOpenDualDiskCut) LocateRational(ctx context.Context, point [3]*big.Rat) (parametric.Location, error) {
	r, e := c.ClassifyRational(ctx, point)
	return r.Location, e
}
func (c OffsetSideOpenDualDiskCut) Locate(ctx context.Context, point geom.Vec3) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	return c.LocateRational(ctx, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)})
}

// Measures evaluates closed-form circular integrals of the actual offset
// section. Values are rounded numerical analytic measures, not exact rational
// quantities or certified enclosures. No sampling or display mesh is used.
func (c OffsetSideOpenDualDiskCut) Measures() (SideOpenDualDiskMeasures, error) {
	var zero SideOpenDualDiskMeasures
	s := c.source
	if s == nil {
		return zero, geom.ErrInvalid
	}
	left, right := s.discs[0], s.discs[1]
	if left.Center.X > right.Center.X {
		left, right = right, left
	}
	d := right.Center.X - left.Center.X
	join := (d*d + left.Radius*left.Radius - right.Radius*right.Radius) / (2 * d)
	offset := left.Center.Y - s.min.Y
	l := math.Sqrt(left.Radius*left.Radius - offset*offset)
	r := math.Sqrt(right.Radius*right.Radius - offset*offset)
	var removed, mx, my, arc float64
	for _, seg := range []struct {
		disk ExtrusionCircle
		a, b float64
	}{{left, -l, join}, {right, join - d, r}} {
		radius := seg.disk.Radius
		a, b := seg.a, seg.b
		ha, hb := math.Sqrt(radius*radius-a*a), math.Sqrt(radius*radius-b*b)
		angle := math.Asin(b/radius) - math.Asin(a/radius)
		f := (b*hb - a*ha + radius*radius*angle) / 2
		area := offset*(b-a) + f
		removed += area
		mx += seg.disk.Center.X*area + offset*(b*b-a*a)/2 - (hb*hb*hb-ha*ha*ha)/3
		my += s.min.Y*area + ((offset*offset+radius*radius)*(b-a)-(b*b*b-a*a*a)/3+2*offset*f)/2
		arc += radius * angle
	}
	w, h := s.max.X-s.min.X, s.max.Y-s.min.Y
	stock := w * h * s.height
	v := stock - s.depth*removed
	area := 2*(w*h+(w+h)*s.height) + s.depth*(arc-(d+l+r))
	if s.depth == s.height {
		area -= 2 * removed
	}
	centroid := s.frame.ToWorld(geom.Vec3{X: (stock*(s.min.X+w/2) - s.depth*mx) / v, Y: (stock*(s.min.Y+h/2) - s.depth*my) / v, Z: (stock*s.height/2 - s.depth*s.depth*removed/2) / v})
	if v <= 0 || area <= 0 || !finite(v) || !finite(area) || !centroid.IsFinite() {
		return zero, geom.ErrUnrepresentable
	}
	return SideOpenDualDiskMeasures{Volume: v, Area: area, Centroid: centroid}, nil
}
