// Package support certifies exact represented planar face supports without
// mutating topology or treating tolerance-planar geometry as exactly planar.
package support

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("support: invalid input or options")
var ErrNonplanar = errors.New("support: face is not exactly planar")
var ErrNonconvex = errors.New("support: vertex lies outside an outward support")
var ErrLimit = errors.New("support: computation limit exceeded")

// Violation identifies a source face and vertex that failed certification.
// Residual is exact Normal dot vertex-Offset. It is caller-owned.
type Violation struct {
	Kind         error
	Face, Vertex int
	Residual     *big.Rat
}

func (v *Violation) Error() string {
	return fmt.Sprintf("%v (face %d vertex %d residual %s)", v.Kind, v.Face, v.Vertex, v.Residual.RatString())
}
func (v *Violation) Unwrap() error { return v.Kind }

// Plane represents Normal dot point=Offset. The largest absolute normal
// component is one; this is not Euclidean normalization. Orientation follows
// the source face boundary. All rationals are caller-owned.
type Plane struct {
	Normal [3]*big.Rat
	Offset *big.Rat
}
type Options struct {
	MaxFaces, MaxVertices, MaxWork, MaxBits int
	Convex                                  bool
}
type Report struct {
	Planes []Plane
	Convex bool
	Work   int
}
type point [3]*big.Rat

func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func vec(v geom.Vec3) point {
	return point{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}
func diff(a, b point) point { return point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func cross(a, b point) point {
	return point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b point) *big.Rat { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }

// Certify checks exact support incidence on every outer and inner wire. Convex
// additionally proves all original vertices satisfy every oriented halfspace.
// It does not prove shell embedding/orientation; use collision validation for
// that separate requirement. Empty input certifies vacuously. Defaults:4096
// faces,16384 vertices,2000000 work units,16384 rational bits.
func Certify(ctx context.Context, s brep.Solid, o Options) (Report, error) {
	empty := Report{}
	if ctx == nil || o.MaxFaces < 0 || o.MaxVertices < 0 || o.MaxWork < 0 || o.MaxBits < 0 {
		return empty, ErrInvalid
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 4096
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 16384
	}
	if o.MaxWork == 0 {
		o.MaxWork = 2000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	fs, es, vs := s.Faces(), s.Edges(), s.Vertices()
	if len(fs) > o.MaxFaces || len(vs) > o.MaxVertices {
		return empty, ErrLimit
	}
	r := Report{Convex: o.Convex}
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		r.Work++
		if r.Work > o.MaxWork {
			return ErrLimit
		}
		return nil
	}
	check := func(x *big.Rat) bool { return x.Num().BitLen() <= o.MaxBits && x.Denom().BitLen() <= o.MaxBits }
	points := make([]point, len(vs))
	for i, v := range vs {
		if e := tick(); e != nil {
			return empty, e
		}
		points[i] = vec(v)
		for _, x := range points[i] {
			if !check(x) {
				return empty, ErrLimit
			}
		}
	}
	for fi, f := range fs {
		if len(f.Boundary) < 3 {
			return empty, ErrInvalid
		}
		ids := make([]int, len(f.Boundary))
		for i, u := range f.Boundary {
			if e := tick(); e != nil {
				return empty, e
			}
			edge := es[u.Edge]
			ids[i] = edge.Start
			if u.Reversed {
				ids[i] = edge.End
			}
		}
		// Oriented area normal handles a concave polygon whose first corner is reflex.
		origin := points[ids[0]]
		normal := point{new(big.Rat), new(big.Rat), new(big.Rat)}
		for i, a := range ids {
			if e := tick(); e != nil {
				return empty, e
			}
			c := ids[(i+1)%len(ids)]
			n := cross(diff(points[a], origin), diff(points[c], origin))
			for k := range normal {
				normal[k].Add(normal[k], n[k])
				if !check(normal[k]) {
					return empty, ErrLimit
				}
			}
		}
		scale := new(big.Rat)
		for _, x := range normal {
			if new(big.Rat).Abs(x).Cmp(scale) > 0 {
				scale.Abs(x)
			}
		}
		if scale.Sign() == 0 {
			return empty, ErrInvalid
		}
		for k := range normal {
			normal[k].Quo(normal[k], scale)
		}
		offset := dot(normal, origin)
		if !check(offset) {
			return empty, ErrLimit
		}
		plane := Plane{Normal: normal, Offset: offset}
		loops := append([][]brep.EdgeUse{f.Boundary}, f.Holes...)
		for _, loop := range loops {
			for _, u := range loop {
				if e := tick(); e != nil {
					return empty, e
				}
				id := es[u.Edge].Start
				residual := sub(dot(normal, points[id]), offset)
				if !check(residual) {
					return empty, ErrLimit
				}
				if residual.Sign() != 0 {
					return empty, &Violation{ErrNonplanar, fi, id, residual}
				}
			}
		}
		if o.Convex {
			for id, p := range points {
				if e := tick(); e != nil {
					return empty, e
				}
				residual := sub(dot(normal, p), offset)
				if !check(residual) {
					return empty, ErrLimit
				}
				if residual.Sign() > 0 {
					return empty, &Violation{ErrNonconvex, fi, id, residual}
				}
			}
		}
		r.Planes = append(r.Planes, plane)
	}
	return r, nil
}
