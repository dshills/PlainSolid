package draft

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// QuadraticInsetScalar denotes Rational + Coefficient*sqrt(result.Radicand).
// Values own their rational coefficients. The coefficient can be zero.
type QuadraticInsetScalar struct {
	Rational, Coefficient *big.Rat
}

// QuadraticInsetPlane is an inward-shifted halfspace with its original face
// sources. The normal uses the dominant-component scaling of Halfspace.
type QuadraticInsetPlane struct {
	Normal [3]*big.Rat
	Offset QuadraticInsetScalar
}

// EuclideanInsetResult records the exact convex erosion and its bounded
// binary64 realization. ExactVertices and FacePlanes align with Solid.
// MaxSquaredDisplacement is an outward rational bound on real 3D rounding.
type EuclideanInsetResult struct {
	Solid                  brep.Solid
	Radicand               *big.Rat
	ExactVertices          [][3]QuadraticInsetScalar
	ExactVolume            QuadraticInsetScalar
	FacePlanes             []QuadraticInsetPlane
	FaceSources            [][]int
	OldToNew               []int
	MaxSquaredDisplacement *big.Rat
}

// InsetStableEuclidean erodes a convex planar solid by a Euclidean ball.
// It shifts each original supporting plane inward by the exact normal length
// times distance, and proves the final entire halfspace intersection has the
// same simple vertex incidence. Source support groups can contain coplanar
// faces. Normal lengths must be rational or share one rational square class.
// A topology change, unresolved arithmetic or invalid rounded result fails
// atomically. The ideal erosion and rounded Solid have separate coordinates.
func InsetStableEuclidean(ctx context.Context, s brep.Solid, distance float64, o Options) (EuclideanInsetResult, error) {
	var empty EuclideanInsetResult
	if ctx == nil || math.IsInf(distance, 0) || math.IsNaN(distance) || distance < 0 {
		return empty, ErrInvalid
	}
	o, err := options(o)
	if err != nil {
		return empty, err
	}
	base, err := Apply(ctx, s, nil, o)
	if err != nil {
		return empty, err
	}
	if len(base.FacePlanes) < 4 || len(base.ExactVertices) < 4 {
		return empty, ErrUnsupported
	}
	field := insetQuadraticField{work: &budget{ctx: ctx, o: o}}
	planes, err := insetEuclideanPlanes(base.FacePlanes, num(distance), &field)
	if err != nil {
		return empty, err
	}
	points, err := insetStableQuadraticVertices(base, planes, &field)
	if err != nil {
		return empty, err
	}
	volume := insetStableVolume(base.Solid, points, &field)
	if field.work.err != nil {
		return empty, field.work.err
	}
	volumeSign := field.sign(volume)
	if field.work.err != nil {
		return empty, field.work.err
	}
	if volumeSign <= 0 {
		return empty, ErrUnsupported
	}
	nativePoints, displacement, err := insetQuadraticNativeVertices(points, &field)
	if err != nil {
		return empty, err
	}
	native, err := brep.NewSolid(nativePoints, base.Solid.Edges(), base.Solid.Faces())
	if err != nil {
		return empty, fmt.Errorf("%w: %v", ErrPrecision, err)
	}
	if err = validate(ctx, native, o); err != nil {
		return empty, err
	}
	return EuclideanInsetResult{Solid: native, Radicand: field.radical, ExactVertices: points,
		ExactVolume: volume, FacePlanes: planes, FaceSources: base.FaceSources,
		OldToNew: base.OldToNew, MaxSquaredDisplacement: displacement}, nil
}

type insetQuadraticField struct {
	radical *big.Rat
	work    *budget
}

func (f *insetQuadraticField) pair(a, b *big.Rat) QuadraticInsetScalar {
	f.work.check(a)
	f.work.check(b)
	return QuadraticInsetScalar{a, b}
}

func (f *insetQuadraticField) plus(a, b QuadraticInsetScalar) QuadraticInsetScalar {
	return f.pair(add(a.Rational, b.Rational), add(a.Coefficient, b.Coefficient))
}

func (f *insetQuadraticField) minus(a, b QuadraticInsetScalar) QuadraticInsetScalar {
	return f.pair(sub(a.Rational, b.Rational), sub(a.Coefficient, b.Coefficient))
}

func (f *insetQuadraticField) scaled(a QuadraticInsetScalar, s *big.Rat) QuadraticInsetScalar {
	return f.pair(mul(a.Rational, s), mul(a.Coefficient, s))
}

func (f *insetQuadraticField) product(a, b QuadraticInsetScalar) QuadraticInsetScalar {
	constant := add(mul(a.Rational, b.Rational), mul(mul(a.Coefficient, b.Coefficient), f.radical))
	coefficient := add(mul(a.Rational, b.Coefficient), mul(a.Coefficient, b.Rational))
	return f.pair(constant, coefficient)
}

func (f *insetQuadraticField) sign(a QuadraticInsetScalar) int {
	sx, sy := a.Rational.Sign(), a.Coefficient.Sign()
	if sx == 0 {
		return sy
	}
	if sy == 0 || sx == sy {
		return sx
	}
	x := mul(a.Rational, a.Rational)
	y := mul(mul(a.Coefficient, a.Coefficient), f.radical)
	f.work.check(x)
	f.work.check(y)
	return sx * x.Cmp(y)
}

func insetEuclideanPlanes(source []Halfspace, distance *big.Rat, f *insetQuadraticField) ([]QuadraticInsetPlane, error) {
	var planes []QuadraticInsetPlane
	for i, h := range source {
		if !f.work.tick() {
			return nil, f.work.err
		}
		constant := new(big.Rat).Set(h.Offset)
		coefficient := new(big.Rat)
		if distance.Sign() != 0 {
			squared := dot(h.Normal, h.Normal)
			f.work.check(squared)
			if exact, ok := rationalSqrt(squared); ok {
				constant.Sub(constant, mul(distance, exact))
			} else {
				if f.radical == nil {
					f.radical = squared
				}
				ratio := div(squared, f.radical)
				f.work.check(ratio)
				scale, ok := rationalSqrt(ratio)
				if !ok {
					return nil, fmt.Errorf("%w: support %d has a different irrational normal class", ErrUnsupported, i)
				}
				coefficient.Neg(mul(distance, scale))
			}
		}
		planes = append(planes, QuadraticInsetPlane{Normal: h.Normal, Offset: f.pair(constant, coefficient)})
	}
	if f.radical == nil {
		f.radical = big.NewRat(1, 1)
	}
	return planes, f.work.err
}

func insetStableQuadraticVertices(base Result, planes []QuadraticInsetPlane, f *insetQuadraticField) ([][3]QuadraticInsetScalar, error) {
	active := make(map[[3]int]int, len(base.ExactVertices))
	for i, v := range base.ExactVertices {
		var incident []int
		for j, p := range base.FacePlanes {
			if !f.work.tick() {
				return nil, f.work.err
			}
			if dot(p.Normal, v).Cmp(p.Offset) == 0 {
				incident = append(incident, j)
			}
		}
		if len(incident) != 3 {
			return nil, fmt.Errorf("%w: original vertex %d is not simple", ErrUnsupported, i)
		}
		active[[3]int(incident)] = i
	}
	points := make([][3]QuadraticInsetScalar, len(base.ExactVertices))
	matched := 0
	for i, a := range planes {
		for j := i + 1; j < len(planes); j++ {
			for k := j + 1; k < len(planes); k++ {
				if !f.work.tick() {
					return nil, f.work.err
				}
				b, c := planes[j], planes[k]
				t := [3]int{i, j, k}
				id, original := active[t]
				cofactors := [3]point{cross(b.Normal, c.Normal), cross(c.Normal, a.Normal), cross(a.Normal, b.Normal)}
				det := dot(a.Normal, cofactors[0])
				f.work.check(det)
				if det.Sign() == 0 {
					continue
				}
				var vertex [3]QuadraticInsetScalar
				for axis := range vertex {
					vertex[axis] = f.scaled(a.Offset, div(cofactors[0][axis], det))
					vertex[axis] = f.plus(vertex[axis], f.scaled(b.Offset, div(cofactors[1][axis], det)))
					vertex[axis] = f.plus(vertex[axis], f.scaled(c.Offset, div(cofactors[2][axis], det)))
				}
				feasible, strict := true, true
				for l, p := range planes {
					if l == i || l == j || l == k {
						continue
					}
					if !f.work.tick() {
						return nil, f.work.err
					}
					value := f.scaled(vertex[0], p.Normal[0])
					for axis := 1; axis < 3; axis++ {
						value = f.plus(value, f.scaled(vertex[axis], p.Normal[axis]))
					}
					sign := f.sign(f.minus(value, p.Offset))
					if f.work.err != nil {
						return nil, f.work.err
					}
					if sign > 0 {
						feasible = false
						break
					}
					strict = strict && sign < 0
				}
				if original != feasible || (feasible && !strict) {
					return nil, fmt.Errorf("%w: Euclidean inset changes vertex incidence at %v", ErrUnsupported, t)
				}
				if original {
					points[id] = vertex
					matched++
				}
			}
		}
	}
	if matched != len(points) {
		return nil, ErrUnsupported
	}
	return points, f.work.err
}

func insetStableVolume(s brep.Solid, points [][3]QuadraticInsetScalar, f *insetQuadraticField) QuadraticInsetScalar {
	total := QuadraticInsetScalar{new(big.Rat), new(big.Rat)}
	edges := s.Edges()
	for _, face := range s.Faces() {
		var ids []int
		for _, use := range face.Boundary {
			e := edges[use.Edge]
			id := e.Start
			if use.Reversed {
				id = e.End
			}
			ids = append(ids, id)
		}
		a := points[ids[0]]
		for k := 1; k+1 < len(ids); k++ {
			if !f.work.tick() {
				return total
			}
			b, c := points[ids[k]], points[ids[k+1]]
			for i := 0; i < 3; i++ {
				j, l := (i+1)%3, (i+2)%3
				cofactor := f.minus(f.product(b[j], c[l]), f.product(b[l], c[j]))
				total = f.plus(total, f.product(a[i], cofactor))
			}
		}
	}
	return f.scaled(total, big.NewRat(1, 6))
}

func insetQuadraticNativeVertices(points [][3]QuadraticInsetScalar, f *insetQuadraticField) ([]geom.Vec3, *big.Rat, error) {
	precision := min(512, max(64, f.work.o.MaxBits/8))
	den := new(big.Int).Lsh(big.NewInt(1), uint(precision))
	n := new(big.Int).Lsh(f.radical.Num(), uint(2*precision))
	n.Quo(n, f.radical.Denom()).Sqrt(n)
	lower := new(big.Rat).SetFrac(n, den)
	upper := new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), den)
	maxError := num(f.work.o.MaxCoordinateError)
	maxError.Mul(maxError, maxError)
	var out []geom.Vec3
	maxSquared := new(big.Rat)
	seen := make(map[geom.Vec3]bool)
	for _, point := range points {
		if !f.work.tick() {
			return nil, nil, f.work.err
		}
		var coords [3]float64
		squared := new(big.Rat)
		for i, v := range point {
			lo := add(v.Rational, mul(v.Coefficient, lower))
			hi := add(v.Rational, mul(v.Coefficient, upper))
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			value, _ := div(add(lo, hi), big.NewRat(2, 1)).Float64()
			if math.IsInf(value, 0) {
				return nil, nil, ErrPrecision
			}
			coords[i] = value
			d1, d2 := new(big.Rat).Abs(sub(num(value), lo)), new(big.Rat).Abs(sub(num(value), hi))
			if d1.Cmp(d2) < 0 {
				d1 = d2
			}
			squared.Add(squared, mul(d1, d1))
			f.work.check(lo)
			f.work.check(hi)
			f.work.check(squared)
		}
		if f.work.err != nil {
			return nil, nil, f.work.err
		}
		p := geom.Vec3{X: coords[0], Y: coords[1], Z: coords[2]}
		if seen[p] || squared.Cmp(maxError) > 0 {
			return nil, nil, ErrPrecision
		}
		seen[p] = true
		out = append(out, p)
		if squared.Cmp(maxSquared) > 0 {
			maxSquared = squared
		}
	}
	return out, maxSquared, nil
}
