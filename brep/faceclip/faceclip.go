// Package faceclip clips a planar BRep face region, including holes, into exact
// rational polygon fragments without constructing rounded trim loops.
package faceclip

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("faceclip: invalid face, plane or options")
var ErrLimit = errors.New("faceclip: resource limit exceeded")
var ErrRange = errors.New("faceclip: area cannot be represented as float64")

type Point [3]*big.Rat

// Halfspace is Normal dot x <= Offset. Coefficients are interpreted exactly;
// arbitrary finite unnormalized normals avoid intermediate plane normalization.
type Halfspace struct {
	Normal geom.Vec3
	Offset float64
}
type Options struct{ MaxTriangles, MaxWork, MaxBits int }

// Fragment retains source polygon winding and original tessellation ancestry.
// Vertices and all exact result rationals are caller-owned.
type Fragment struct {
	Vertices       []Point
	Face, Triangle int
}

// Result's ProjectedArea is exact area after dropping Axis (0=X,1=Y,2=Z),
// chosen by the largest absolute face-normal component. Centroid is the exact
// spatial area centroid, undefined (all nil) for an empty result. Area is a
// rounded Euclidean area convenience; exact geometry remains authoritative.
type Result struct {
	Fragments     []Fragment
	Axis          int
	ProjectedArea *big.Rat
	Centroid      Point
	Area          float64
}

func n(x float64) *big.Rat       { return new(big.Rat).SetFloat64(x) }
func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func div(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
func vec(v geom.Vec3) Point      { return Point{n(v.X), n(v.Y), n(v.Z)} }
func diff(a, b Point) Point      { return Point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func cross(a, b Point) Point {
	return Point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b Point) *big.Rat { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }
func same(a, b Point) bool {
	for i := range a {
		if a[i].Cmp(b[i]) != 0 {
			return false
		}
	}
	return true
}
func clone(p Point) Point {
	return Point{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1]), new(big.Rat).Set(p[2])}
}

// Clip retains the closed halfspace, but omits zero-area contacts. Triangulation
// already encodes holes and concavities, so clips cannot fill excluded regions.
// Adjacent fragments retain their common boundaries; they are not merged.
func Clip(ctx context.Context, s brep.Solid, face int, h Halfspace, o Options) (Result, error) {
	empty := Result{}
	if ctx == nil || face < 0 || face >= len(s.Faces()) || !h.Normal.IsFinite() || h.Normal == (geom.Vec3{}) || math.IsNaN(h.Offset) || math.IsInf(h.Offset, 0) || o.MaxTriangles < 0 || o.MaxWork < 0 || o.MaxBits < 0 {
		return empty, ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 8192
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	m := s.Tessellate()
	if len(m.Triangles) > o.MaxTriangles {
		return empty, ErrLimit
	}
	sources := s.TriangleFaces()
	normal := vec(h.Normal)
	offset := n(h.Offset)
	work := 0
	check := func(x *big.Rat) bool { return x.Num().BitLen() <= o.MaxBits && x.Denom().BitLen() <= o.MaxBits }
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > o.MaxWork {
			return ErrLimit
		}
		return nil
	}
	for _, x := range normal {
		if !check(x) {
			return empty, ErrLimit
		}
	}
	if !check(offset) {
		return empty, ErrLimit
	}
	result := Result{ProjectedArea: new(big.Rat)}
	moments := Point{new(big.Rat), new(big.Rat), new(big.Rat)}
	var faceNormal Point
	var support *big.Rat
	axis := 0
	for index, tr := range m.Triangles {
		if e := tick(); e != nil {
			return empty, e
		}
		if sources[index] != face {
			continue
		}
		triangle := []Point{vec(m.Vertices[tr[0]]), vec(m.Vertices[tr[1]]), vec(m.Vertices[tr[2]])}
		if support == nil {
			faceNormal = cross(diff(triangle[1], triangle[0]), diff(triangle[2], triangle[0]))
			for i := 1; i < 3; i++ {
				if new(big.Rat).Abs(faceNormal[i]).Cmp(new(big.Rat).Abs(faceNormal[axis])) > 0 {
					axis = i
				}
			}
			if faceNormal[axis].Sign() == 0 {
				return empty, ErrInvalid
			}
			support = dot(faceNormal, triangle[0])
			result.Axis = axis
		}
		for _, p := range triangle {
			for _, x := range p {
				if !check(x) {
					return empty, ErrLimit
				}
			}
			if dot(faceNormal, p).Cmp(support) != 0 {
				return empty, ErrInvalid
			}
		}
		var clipped []Point
		for i, a := range triangle {
			if e := tick(); e != nil {
				return empty, e
			}
			b := triangle[(i+1)%3]
			da, db := sub(dot(normal, a), offset), sub(dot(normal, b), offset)
			if da.Sign() <= 0 {
				clipped = append(clipped, clone(a))
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
				clipped = append(clipped, p)
			}
		}
		var unique []Point
		for _, p := range clipped {
			if len(unique) == 0 || !same(p, unique[len(unique)-1]) {
				unique = append(unique, p)
			}
		}
		if len(unique) > 1 && same(unique[0], unique[len(unique)-1]) {
			unique = unique[:len(unique)-1]
		}
		if len(unique) < 3 {
			continue
		}
		fragmentArea := new(big.Rat)
		for i := 1; i+1 < len(unique); i++ {
			a, b, c := unique[0], unique[i], unique[i+1]
			area := div(cross(diff(b, a), diff(c, a))[axis], big.NewRat(2, 1))
			if faceNormal[axis].Sign() < 0 {
				area.Neg(area)
			}
			if area.Sign() < 0 {
				return empty, ErrInvalid
			}
			fragmentArea.Add(fragmentArea, area)
			for k := range moments {
				cent := div(add(add(a[k], b[k]), c[k]), big.NewRat(3, 1))
				moments[k].Add(moments[k], mul(area, cent))
				if !check(moments[k]) {
					return empty, ErrLimit
				}
			}
		}
		if fragmentArea.Sign() == 0 {
			continue
		}
		result.ProjectedArea.Add(result.ProjectedArea, fragmentArea)
		if !check(result.ProjectedArea) {
			return empty, ErrLimit
		}
		result.Fragments = append(result.Fragments, Fragment{unique, face, index})
	}
	if support == nil {
		return empty, ErrInvalid
	}
	if result.ProjectedArea.Sign() == 0 {
		return result, nil
	}
	for k := range result.Centroid {
		result.Centroid[k] = div(moments[k], result.ProjectedArea)
		if !check(result.Centroid[k]) {
			return empty, ErrLimit
		}
	}
	scale2 := div(dot(faceNormal, faceNormal), mul(faceNormal[axis], faceNormal[axis]))
	sq := mul(mul(result.ProjectedArea, result.ProjectedArea), scale2)
	if !check(sq) {
		return empty, ErrLimit
	}
	value := new(big.Float).SetPrec(256).SetRat(sq)
	value.Sqrt(value)
	result.Area, _ = value.Float64()
	if math.IsInf(result.Area, 0) || result.Area == 0 {
		return empty, ErrRange
	}
	return result, nil
}
