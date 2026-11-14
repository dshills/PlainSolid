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
	result, err := insetStableEuclidean(ctx, s, distance, o, 1)
	if err != nil {
		return EuclideanInsetResult{}, err
	}
	radical := big.NewRat(1, 1)
	if len(result.radicals) > 1 {
		radical = new(big.Rat).Set(result.radicals[1])
	}
	convert := func(value insetAlgebraic) QuadraticInsetScalar {
		return QuadraticInsetScalar{value.coefficient(0), value.coefficient(1)}
	}
	vertices := make([][3]QuadraticInsetScalar, len(result.vertices))
	for i, v := range result.vertices {
		for j, coordinate := range v {
			vertices[i][j] = convert(coordinate)
		}
	}
	planes := make([]QuadraticInsetPlane, len(result.planes))
	for i, plane := range result.planes {
		planes[i] = QuadraticInsetPlane{Normal: plane.Normal, Offset: convert(plane.Offset)}
	}
	return EuclideanInsetResult{Solid: result.solid, Radicand: radical, ExactVertices: vertices,
		ExactVolume: convert(result.volume), FacePlanes: planes, FaceSources: result.base.FaceSources,
		OldToNew: result.base.OldToNew, MaxSquaredDisplacement: result.displacement}, nil
}

// AlgebraicInsetScalar denotes the sum of Coefficients[i]*sqrt(Radicands[i]).
// The basis belongs to the corresponding AlgebraicEuclideanInsetResult.
type AlgebraicInsetScalar struct{ Coefficients []*big.Rat }

// AlgebraicInsetPlane is an original support moved by its exact normal length.
type AlgebraicInsetPlane struct {
	Normal [3]*big.Rat
	Offset AlgebraicInsetScalar
}

// AlgebraicEuclideanInsetResult records the ideal erosion and a validated
// rounded native realization. Radicands[0] is 1. The other radicands have
// distinct rational square classes. The output coefficients are caller-owned.
type AlgebraicEuclideanInsetResult struct {
	Solid                  brep.Solid
	Radicands              []*big.Rat
	ExactVertices          [][3]AlgebraicInsetScalar
	ExactVolume            AlgebraicInsetScalar
	FacePlanes             []AlgebraicInsetPlane
	FaceSources            [][]int
	OldToNew               []int
	MaxSquaredDisplacement *big.Rat
}

// InsetStableEuclideanAlgebraic erodes a simple convex planar solid by a
// Euclidean ball while preserving proved vertex incidence. It admits up to four
// independent irrational normal classes and their products. Exact arithmetic
// tracks each basis coefficient. Outward rational bounds decide the signs of
// sums with more than two terms within the configured work and bit limits.
func InsetStableEuclideanAlgebraic(ctx context.Context, s brep.Solid, distance float64, o Options) (AlgebraicEuclideanInsetResult, error) {
	result, err := insetStableEuclidean(ctx, s, distance, o, 4)
	if err != nil {
		return AlgebraicEuclideanInsetResult{}, err
	}
	convert := func(value insetAlgebraic) AlgebraicInsetScalar {
		coefficients := make([]*big.Rat, len(result.radicals))
		for i := range coefficients {
			coefficients[i] = value.coefficient(i)
		}
		return AlgebraicInsetScalar{coefficients}
	}
	vertices := make([][3]AlgebraicInsetScalar, len(result.vertices))
	for i, v := range result.vertices {
		for j, coordinate := range v {
			vertices[i][j] = convert(coordinate)
		}
	}
	planes := make([]AlgebraicInsetPlane, len(result.planes))
	for i, plane := range result.planes {
		planes[i] = AlgebraicInsetPlane{Normal: plane.Normal, Offset: convert(plane.Offset)}
	}
	return AlgebraicEuclideanInsetResult{Solid: result.solid, Radicands: result.radicals, ExactVertices: vertices,
		ExactVolume: convert(result.volume), FacePlanes: planes, FaceSources: result.base.FaceSources,
		OldToNew: result.base.OldToNew, MaxSquaredDisplacement: result.displacement}, nil
}

type insetStableResult struct {
	solid        brep.Solid
	base         Result
	radicals     []*big.Rat
	vertices     [][3]insetAlgebraic
	volume       insetAlgebraic
	planes       []insetRadicalPlane
	displacement *big.Rat
}

func insetStableEuclidean(ctx context.Context, s brep.Solid, distance float64, o Options, maxGenerators int) (insetStableResult, error) {
	var empty insetStableResult
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
	field := insetRadicalField{work: &budget{ctx: ctx, o: o}, radicals: []*big.Rat{big.NewRat(1, 1)}}
	planes, err := insetEuclideanPlanes(base.FacePlanes, num(distance), &field, maxGenerators)
	if err != nil {
		return empty, err
	}
	points, err := insetStableAlgebraicVertices(base, planes, &field)
	if err != nil {
		return empty, err
	}
	volume := insetStableVolume(base.Solid, points, &field)
	volumeSign := field.sign(volume)
	if field.work.err != nil {
		return empty, field.work.err
	}
	if volumeSign <= 0 {
		return empty, ErrUnsupported
	}
	nativePoints, displacement, err := insetAlgebraicNativeVertices(points, &field)
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
	return insetStableResult{solid: native, base: base, radicals: field.radicals, vertices: points,
		volume: volume, planes: planes, displacement: displacement}, nil
}

type insetRadicalPlane struct {
	Normal [3]*big.Rat
	Offset insetAlgebraic
}

func insetEuclideanPlanes(source []Halfspace, distance *big.Rat, f *insetRadicalField, maxGenerators int) ([]insetRadicalPlane, error) {
	var planes []insetRadicalPlane
	for i, h := range source {
		if !f.work.tick() {
			return nil, f.work.err
		}
		var offset insetAlgebraic
		f.addTerm(&offset, 0, h.Offset)
		if distance.Sign() != 0 {
			idx, scale, err := f.includeRoot(dot(h.Normal, h.Normal), maxGenerators)
			if err != nil {
				return nil, fmt.Errorf("inset support %d: %w", i, err)
			}
			f.addTerm(&offset, idx, new(big.Rat).Neg(mul(distance, scale)))
		}
		planes = append(planes, insetRadicalPlane{Normal: h.Normal, Offset: offset})
	}
	return planes, f.work.err
}

func insetStableAlgebraicVertices(base Result, planes []insetRadicalPlane, f *insetRadicalField) ([][3]insetAlgebraic, error) {
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
	points := make([][3]insetAlgebraic, len(base.ExactVertices))
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
				var vertex [3]insetAlgebraic
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

func insetStableVolume(s brep.Solid, points [][3]insetAlgebraic, f *insetRadicalField) insetAlgebraic {
	var total insetAlgebraic
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

func insetAlgebraicNativeVertices(points [][3]insetAlgebraic, f *insetRadicalField) ([]geom.Vec3, *big.Rat, error) {
	precision := min(512, max(64, f.work.o.MaxBits/8))
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
			lo, hi := f.bounds(v, precision)
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
