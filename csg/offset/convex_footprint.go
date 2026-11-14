package offset

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ConvexOffsetPrimitive records a finite original offset segment. A line has
// an OriginalEdge and a circular corner has an OriginalVertex. The unused
// source index is -1. Start, End and Center are exact represented section
// coordinates in the supplied frame. Circular arcs run counterclockwise.
type ConvexOffsetPrimitive struct {
	Start, End, Center           geom.Vec2
	OriginalEdge, OriginalVertex int
	Radius                       float64
}

// ConvexFootprintDilation records the original ideal support profile and its
// native analytic realization. Steiner area and volume have the form
// Constant + PiCoefficient*pi. Their coefficients are in model units.
// The realization uses the bounded arc anchoring of trimmed.ExtrudeArcLoop.
type ConvexFootprintDilation struct {
	Native                              trimmed.ArcLoopExtrusion
	Primitives                          []ConvexOffsetPrimitive
	AreaConstant, AreaPiCoefficient     *big.Rat
	VolumeConstant, VolumePiCoefficient *big.Rat
	source                              *convexFootprintSource
}

// DilateConvexPrismFootprint offsets a strictly convex straight polygon by a
// Euclidean disk and extrudes the rounded section between the original axial
// planes. Signed height is supported. Edge lengths and shifted endpoints
// must have exact representations for the requested radius. The exact convex
// support proof uses represented coordinates, with bounded analytic anchoring
// of the native circular arcs. The frame uses proper signed coordinate axes.
// The domain is three to 32 original sides.
func DilateConvexPrismFootprint(ctx context.Context, frame geom.Frame, profile []geom.Vec2, height, radius, tolerance float64) (ConvexFootprintDilation, error) {
	var empty ConvexFootprintDilation
	if ctx == nil || len(profile) < 3 || len(profile) > 32 || !finite(height) || height == 0 || !finite(radius) || radius < 0 || !finite(tolerance) || tolerance <= 0 {
		return empty, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	placement, err := footprintPlacement(frame)
	if err != nil {
		return empty, err
	}
	points := make([]pt, len(profile))
	original := make([]int, len(profile))
	for i, p := range profile {
		if !finite(p.X) || !finite(p.Y) {
			return empty, ErrInvalid
		}
		points[i], original[i] = pt{num(p.X), num(p.Y)}, i
	}
	doubleArea := new(big.Rat)
	for i, a := range points {
		b := points[(i+1)%len(points)]
		doubleArea.Add(doubleArea, footprintCross(a, b))
	}
	if doubleArea.Sign() == 0 {
		return empty, ErrInvalid
	}
	if doubleArea.Sign() < 0 {
		slices.Reverse(points)
		slices.Reverse(original)
		doubleArea.Neg(doubleArea)
	}
	r := num(radius)
	normals := make([]pt, len(points))
	shift := make([]pt, len(points))
	perimeter := new(big.Rat)
	for i, a := range points {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		j := (i + 1) % len(points)
		d := footprintDifference(points[j], a)
		for k, p := range points {
			if k != i && k != j && footprintCross(d, footprintDifference(p, a)).Sign() <= 0 {
				return empty, ErrInvalid
			}
		}
		if radius == 0 {
			shift[i] = pt{new(big.Rat), new(big.Rat)}
			continue
		}
		lengthSquared := add(mul(d.x, d.x), mul(d.y, d.y))
		length, ok := footprintSquareRoot(lengthSquared)
		if !ok {
			return empty, geom.ErrUnrepresentable
		}
		perimeter.Add(perimeter, length)
		normals[i] = pt{new(big.Rat).Quo(d.y, length), new(big.Rat).Quo(new(big.Rat).Neg(d.x), length)}
		shift[i] = pt{mul(normals[i].x, r), mul(normals[i].y, r)}
	}
	edgeIDs := make([]int, len(points))
	var primitives []ConvexOffsetPrimitive
	var segments []trimmed.ExtrusionSegment
	for i, p := range points {
		j := (i + 1) % len(points)
		a, err := footprintShiftPoint(p, shift[i])
		if err != nil {
			return empty, err
		}
		b, err := footprintShiftPoint(points[j], shift[i])
		if err != nil {
			return empty, err
		}
		edge := original[i]
		if (edge+1)%len(points) != original[j] {
			edge = original[j]
		}
		edgeIDs[i] = edge
		primitives = append(primitives, ConvexOffsetPrimitive{Start: a, End: b, OriginalEdge: edge, OriginalVertex: -1})
		segments = append(segments, trimmed.ExtrusionSegment{Start: a, End: b})
		if radius == 0 {
			continue
		}
		c, err := footprintShiftPoint(points[j], shift[j])
		if err != nil {
			return empty, err
		}
		n0, n1 := normals[i], normals[j]
		cross, dot := footprintCross(n0, n1), add(mul(n0.x, n1.x), mul(n0.y, n1.y))
		x, _ := n0.x.Float64()
		y, _ := n0.y.Float64()
		tangentCross, _ := cross.Float64()
		tangentDot, _ := dot.Float64()
		angle, sweep := math.Atan2(y, x), math.Atan2(tangentCross, tangentDot)
		center := profile[original[j]]
		primitives = append(primitives, ConvexOffsetPrimitive{Start: b, End: c, Center: center, Radius: radius, OriginalEdge: -1, OriginalVertex: original[j]})
		segments = append(segments, trimmed.ExtrusionSegment{Start: b, End: c, Center: center, Radius: radius, StartAngle: angle, Sweep: sweep})
	}
	local, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return empty, err
	}
	native, err := trimmed.ExtrudeArcLoop(ctx, local, segments, height, tolerance)
	if err != nil {
		return empty, err
	}
	if placement != geom.Identity() {
		native.Solid, err = native.Solid.TransformedContext(ctx, placement)
		if err != nil {
			return empty, err
		}
		native.Bounds, err = native.Solid.Bounds()
		if err != nil {
			return empty, err
		}
	}
	area := add(new(big.Rat).Quo(doubleArea, big.NewRat(2, 1)), mul(r, perimeter))
	areaPi, length := mul(r, r), num(math.Abs(height))
	return ConvexFootprintDilation{Native: native, Primitives: primitives, AreaConstant: area, AreaPiCoefficient: areaPi,
		VolumeConstant: mul(length, area), VolumePiCoefficient: mul(length, areaPi),
		source: &convexFootprintSource{points: points, originalVertex: original, originalEdge: edgeIDs, radiusSquared: new(big.Rat).Set(areaPi),
			primitives: slices.Clone(primitives), placement: placement, z: [2]*big.Rat{num(min(0, height)), num(max(0, height))}}}, nil
}

func footprintCross(a, b pt) *big.Rat { return sub(mul(a.x, b.y), mul(a.y, b.x)) }
func footprintDifference(a, b pt) pt  { return pt{sub(a.x, b.x), sub(a.y, b.y)} }

func footprintSquareRoot(q *big.Rat) (*big.Rat, bool) {
	if q.Sign() <= 0 {
		return nil, false
	}
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(n, d), true
}

func footprintShiftPoint(point, shift pt) (geom.Vec2, error) {
	x, err := geom.RepresentedExactScalar(add(point.x, shift.x))
	if err != nil {
		return geom.Vec2{}, err
	}
	y, err := geom.RepresentedExactScalar(add(point.y, shift.y))
	if err != nil {
		return geom.Vec2{}, err
	}
	return geom.Vec2{X: x, Y: y}, nil
}

func footprintPlacement(frame geom.Frame) (geom.Affine, error) {
	x, y, z, p := frame.XAxis(), frame.YAxis(), frame.ZAxis(), frame.Origin()
	for _, v := range [3]geom.Vec3{x, y, z} {
		switch v {
		case geom.Vec3{X: 1}, geom.Vec3{X: -1}, geom.Vec3{Y: 1}, geom.Vec3{Y: -1}, geom.Vec3{Z: 1}, geom.Vec3{Z: -1}:
		default:
			return geom.Affine{}, geom.ErrUnrepresentable
		}
	}
	matrix := geom.Affine{{x.X, y.X, z.X, p.X}, {x.Y, y.Y, z.Y, p.Y}, {x.Z, y.Z, z.Z, p.Z}}
	if !p.IsFinite() || matrix.Determinant() != 1 {
		return geom.Affine{}, geom.ErrInvalid
	}
	return matrix, nil
}

// ExactSolid builds the rounded footprint with exact rational circular
// supports and cap trims. It retains the ideal geometry when arc controls
// cannot fit binary64, and supports the result's original signed placement.
func (q ConvexFootprintDilation) ExactSolid(ctx context.Context, options trimmed.ExactCircularLoopOptions) (trimmed.Solid, error) {
	if ctx == nil || q.source == nil {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	segments := make([]trimmed.ExactCircularLoopSegment, len(q.source.primitives))
	for i, source := range q.source.primitives {
		segments[i] = trimmed.ExactCircularLoopSegment{Start: source.Start, End: source.End, Center: source.Center, Radius: source.Radius}
	}
	height, _ := q.source.z[1].Float64()
	if height == 0 {
		height, _ = q.source.z[0].Float64()
	}
	result, err := trimmed.ExtrudeExactCircularLoop(ctx, segments, height, options)
	if err != nil || q.source.placement == geom.Identity() {
		return result, err
	}
	return result.TransformedContext(ctx, q.source.placement)
}
