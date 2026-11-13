package faceclip

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// RegionOptions bounds the initial Clip pass and subsequent rational window
// clipping independently. MaxHalfspaces defaults to 32 and may not exceed128.
// Clip.MaxWork is a cumulative budget for all subsequent clipping stages.
type RegionOptions struct {
	MaxHalfspaces int
	Clip          Options
}

// ClipRegion intersects a planar face region with all supplied halfspaces.
// An empty list returns the complete face. Intermediate rational coordinates
// are never converted to floats. Hole topology is inherited from source
// tessellation; MergeLoops can reconstruct the final region's open trim loops.
func ClipRegion(ctx context.Context, s brep.Solid, face int, halfspaces []Halfspace, o RegionOptions) (Result, error) {
	empty := Result{}
	if ctx == nil || o.MaxHalfspaces < 0 || o.MaxHalfspaces > 128 {
		return empty, ErrInvalid
	}
	if o.MaxHalfspaces == 0 {
		o.MaxHalfspaces = 32
	}
	if len(halfspaces) > o.MaxHalfspaces {
		return empty, ErrLimit
	}
	for _, h := range halfspaces {
		if !h.Normal.IsFinite() || h.Normal == (geom.Vec3{}) || math.IsNaN(h.Offset) || math.IsInf(h.Offset, 0) {
			return empty, ErrInvalid
		}
	}
	first := Halfspace{Normal: geom.Vec3{X: 1}, Offset: math.MaxFloat64}
	if len(halfspaces) > 0 {
		first = halfspaces[0]
	}
	r, e := Clip(ctx, s, face, first, o.Clip)
	if e != nil {
		return empty, e
	}
	if len(halfspaces) <= 1 {
		return r, nil
	}
	bits, limit := o.Clip.MaxBits, o.Clip.MaxWork
	if bits == 0 {
		bits = 16384
	}
	if limit == 0 {
		limit = 100000
	}
	work := 0
	check := func(x *big.Rat) bool { return x.Num().BitLen() <= bits && x.Denom().BitLen() <= bits }
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > limit {
			return ErrLimit
		}
		return nil
	}
	for _, h := range halfspaces[1:] {
		if e := tick(); e != nil {
			return empty, e
		}
		normal, offset := vec(h.Normal), n(h.Offset)
		for _, x := range normal {
			if !check(x) {
				return empty, ErrLimit
			}
		}
		if !check(offset) {
			return empty, ErrLimit
		}
		if len(r.Fragments) == 0 {
			continue
		}
		var sourceNormal Point
		found := false
		for _, f := range r.Fragments {
			for i := 1; i+1 < len(f.Vertices); i++ {
				nn := cross(diff(f.Vertices[i], f.Vertices[0]), diff(f.Vertices[i+1], f.Vertices[0]))
				if nn[r.Axis].Sign() != 0 {
					sourceNormal = nn
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return empty, ErrInvalid
		}
		next := Result{Axis: r.Axis, ProjectedArea: new(big.Rat)}
		moments := Point{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, fragment := range r.Fragments {
			var vertices []Point
			for i, a := range fragment.Vertices {
				if e := tick(); e != nil {
					return empty, e
				}
				b := fragment.Vertices[(i+1)%len(fragment.Vertices)]
				da, db := sub(dot(normal, a), offset), sub(dot(normal, b), offset)
				if da.Sign() <= 0 {
					vertices = append(vertices, clone(a))
				}
				if da.Sign()*db.Sign() < 0 {
					t := div(da, sub(da, db))
					var p Point
					for k := range p {
						p[k] = add(a[k], mul(t, sub(b[k], a[k])))
						if !check(p[k]) {
							return empty, ErrLimit
						}
					}
					vertices = append(vertices, p)
				}
			}
			var unique []Point
			for _, p := range vertices {
				if len(unique) == 0 || !same(unique[len(unique)-1], p) {
					unique = append(unique, p)
				}
			}
			if len(unique) > 1 && same(unique[0], unique[len(unique)-1]) {
				unique = unique[:len(unique)-1]
			}
			if len(unique) < 3 {
				continue
			}
			area := new(big.Rat)
			for i := 1; i+1 < len(unique); i++ {
				if e := tick(); e != nil {
					return empty, e
				}
				a, b, c := unique[0], unique[i], unique[i+1]
				weight := div(cross(diff(b, a), diff(c, a))[r.Axis], big.NewRat(2, 1))
				if sourceNormal[r.Axis].Sign() < 0 {
					weight.Neg(weight)
				}
				if weight.Sign() < 0 {
					return empty, ErrInvalid
				}
				area.Add(area, weight)
				for k := range moments {
					moments[k].Add(moments[k], mul(weight, div(add(add(a[k], b[k]), c[k]), big.NewRat(3, 1))))
					if !check(moments[k]) {
						return empty, ErrLimit
					}
				}
			}
			if area.Sign() == 0 {
				continue
			}
			next.ProjectedArea.Add(next.ProjectedArea, area)
			if !check(next.ProjectedArea) {
				return empty, ErrLimit
			}
			next.Fragments = append(next.Fragments, Fragment{unique, fragment.Face, fragment.Triangle})
		}
		if next.ProjectedArea.Sign() != 0 {
			for k := range next.Centroid {
				next.Centroid[k] = div(moments[k], next.ProjectedArea)
				if !check(next.Centroid[k]) {
					return empty, ErrLimit
				}
			}
			ratio := div(dot(sourceNormal, sourceNormal), mul(sourceNormal[r.Axis], sourceNormal[r.Axis]))
			sq := mul(mul(next.ProjectedArea, next.ProjectedArea), ratio)
			if !check(sq) {
				return empty, ErrLimit
			}
			value := new(big.Float).SetPrec(256).SetRat(sq)
			value.Sqrt(value)
			next.Area, _ = value.Float64()
			if math.IsInf(next.Area, 0) || next.Area == 0 {
				return empty, ErrRange
			}
		}
		r = next
	}
	return r, nil
}
