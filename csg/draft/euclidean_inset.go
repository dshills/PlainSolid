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
	result, err := insetConvexEuclidean(ctx, s, distance, nil, o, 1, true)
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

// AlgebraicInsetPlane is an original support with an exact normal offset.
type AlgebraicInsetPlane struct {
	Normal [3]*big.Rat
	Offset AlgebraicInsetScalar
}

// AlgebraicEuclideanInsetResult records the ideal shifted halfspaces and a validated
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
	return insetEuclideanAlgebraic(ctx, s, distance, nil, o, true)
}

// InsetConvexEuclideanAlgebraic erodes a convex planar solid by a Euclidean
// ball and reconstructs every feasible displaced halfspace facet. Supports
// can disappear and vertices can change incidence. Lower-dimensional and
// empty results produce an empty regularized solid with exact zero volume.
// At most four independent normal classes and the native conversion limits
// of InsetStableEuclideanAlgebraic apply.
func InsetConvexEuclideanAlgebraic(ctx context.Context, s brep.Solid, distance float64, o Options) (AlgebraicEuclideanInsetResult, error) {
	return insetEuclideanAlgebraic(ctx, s, distance, nil, o, false)
}

// FaceNormalOffset moves one original convex support by Distance measured
// along its unit outward normal. Negative distances move inward.
type FaceNormalOffset struct {
	Face     int
	Distance float64
}

// OffsetConvexEuclideanSupportsAlgebraic reconstructs a convex solid from
// individually shifted original support planes. Unselected supports keep
// their original positions. All subfaces of one coplanar support must have
// the same shift. The exact result may change which faces and vertices exist.
// Parallel inward offsets of all footprint walls can shrink its section with
// both original caps fixed. Outward offsets use mitered support intersections.
// Arithmetic, conversion, empty output, and ancestry follow the same contract
// as InsetConvexEuclideanAlgebraic.
func OffsetConvexEuclideanSupportsAlgebraic(ctx context.Context, s brep.Solid, offsets []FaceNormalOffset, o Options) (AlgebraicEuclideanInsetResult, error) {
	return insetEuclideanAlgebraic(ctx, s, 0, offsets, o, false)
}

func insetEuclideanAlgebraic(ctx context.Context, s brep.Solid, distance float64, offsets []FaceNormalOffset, o Options, stable bool) (AlgebraicEuclideanInsetResult, error) {
	result, err := insetConvexEuclidean(ctx, s, distance, offsets, o, 4, stable)
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

type insetEuclideanResult struct {
	solid        brep.Solid
	base         Result
	radicals     []*big.Rat
	vertices     [][3]insetAlgebraic
	volume       insetAlgebraic
	planes       []insetRadicalPlane
	displacement *big.Rat
}

func insetConvexEuclidean(ctx context.Context, s brep.Solid, distance float64, offsets []FaceNormalOffset, o Options, maxGenerators int, stable bool) (insetEuclideanResult, error) {
	var empty insetEuclideanResult
	if ctx == nil || math.IsInf(distance, 0) || math.IsNaN(distance) || distance < 0 {
		return empty, ErrInvalid
	}
	o, err := options(o)
	if err != nil {
		return empty, err
	}
	if len(offsets) > o.MaxFaces {
		return empty, ErrLimit
	}
	base, err := Apply(ctx, s, nil, o)
	if err != nil {
		return empty, err
	}
	shifts := make(map[int]float64, len(offsets))
	for _, offset := range offsets {
		if offset.Face < 0 || offset.Face >= len(base.OldToNew) || math.IsInf(offset.Distance, 0) || math.IsNaN(offset.Distance) {
			return empty, ErrInvalid
		}
		if _, duplicate := shifts[offset.Face]; duplicate {
			return empty, ErrInvalid
		}
		shifts[offset.Face] = offset.Distance
	}
	if !stable && len(base.FacePlanes) == 0 && len(s.Faces()) == 0 {
		return insetEuclideanResult{base: base, radicals: []*big.Rat{big.NewRat(1, 1)}, displacement: new(big.Rat)}, nil
	}
	if len(base.FacePlanes) < 4 || len(base.ExactVertices) < 4 {
		return empty, ErrUnsupported
	}
	field := insetRadicalField{work: &budget{ctx: ctx, o: o}, radicals: []*big.Rat{big.NewRat(1, 1)}}
	planes, err := insetEuclideanPlanes(base, distance, shifts, &field, maxGenerators)
	if err != nil {
		return empty, err
	}
	var boundary insetAlgebraicBoundary
	if stable {
		points, err := insetStableAlgebraicVertices(base, planes, &field)
		if err != nil {
			return empty, err
		}
		boundary = insetAlgebraicBoundary{points: points, edges: base.Solid.Edges(), faces: base.Solid.Faces(), planes: planes}
	} else {
		boundary, err = insetConvexAlgebraicBoundary(planes, &field)
		if err != nil {
			return empty, err
		}
		for i := range base.OldToNew {
			base.OldToNew[i] = -1
		}
		sources := make([][]int, len(boundary.planes))
		for i, p := range boundary.sources {
			sources[i] = base.FaceSources[p]
			for _, id := range sources[i] {
				base.OldToNew[id] = i
			}
		}
		base.FaceSources = sources
		if len(boundary.points) == 0 {
			return insetEuclideanResult{base: base, radicals: field.radicals, displacement: new(big.Rat)}, nil
		}
	}
	volume := insetVolume(boundary, &field)
	volumeSign := field.sign(volume)
	if field.work.err != nil {
		return empty, field.work.err
	}
	if volumeSign <= 0 {
		return empty, ErrUnsupported
	}
	nativePoints, displacement, err := insetAlgebraicNativeVertices(boundary.points, &field)
	if err != nil {
		return empty, err
	}
	native, err := brep.NewSolid(nativePoints, boundary.edges, boundary.faces)
	if err != nil {
		return empty, fmt.Errorf("%w: %v", ErrPrecision, err)
	}
	if err = validate(ctx, native, o); err != nil {
		return empty, err
	}
	return insetEuclideanResult{solid: native, base: base, radicals: field.radicals, vertices: boundary.points,
		volume: volume, planes: boundary.planes, displacement: displacement}, nil
}

type insetRadicalPlane struct {
	Normal [3]*big.Rat
	Offset insetAlgebraic
}

func insetEuclideanPlanes(base Result, distance float64, shifts map[int]float64, f *insetRadicalField, maxGenerators int) ([]insetRadicalPlane, error) {
	var planes []insetRadicalPlane
	for i, h := range base.FacePlanes {
		if !f.work.tick() {
			return nil, f.work.err
		}
		shift := -distance
		if len(shifts) > 0 {
			sources := base.FaceSources[i]
			shift = shifts[sources[0]]
			for _, face := range sources[1:] {
				if shifts[face] != shift {
					return nil, fmt.Errorf("%w: differing normal offsets on one coplanar support", ErrUnsupported)
				}
			}
		}
		var offset insetAlgebraic
		f.addTerm(&offset, 0, h.Offset)
		if shift != 0 {
			idx, scale, err := f.includeRoot(dot(h.Normal, h.Normal), maxGenerators)
			if err != nil {
				return nil, fmt.Errorf("inset support %d: %w", i, err)
			}
			f.addTerm(&offset, idx, mul(num(shift), scale))
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
	for i := range planes {
		for j := i + 1; j < len(planes); j++ {
			for k := j + 1; k < len(planes); k++ {
				triple := [3]int{i, j, k}
				id, original := active[triple]
				candidate, err := insetPlaneIntersection(planes, triple, f)
				if err != nil {
					return nil, err
				}
				if original != (candidate != nil) || (candidate != nil && len(candidate.supports) != 3) {
					return nil, fmt.Errorf("%w: Euclidean inset changes vertex incidence at %v", ErrUnsupported, triple)
				}
				if original {
					points[id] = candidate.point
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

func insetVolume(boundary insetAlgebraicBoundary, f *insetRadicalField) insetAlgebraic {
	var total insetAlgebraic
	for _, face := range boundary.faces {
		var ids []int
		for _, use := range face.Boundary {
			e := boundary.edges[use.Edge]
			id := e.Start
			if use.Reversed {
				id = e.End
			}
			ids = append(ids, id)
		}
		a := boundary.points[ids[0]]
		for k := 1; k+1 < len(ids); k++ {
			if !f.work.tick() {
				return total
			}
			b, c := boundary.points[ids[k]], boundary.points[ids[k+1]]
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
