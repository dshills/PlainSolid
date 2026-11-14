package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AlgebraicGraphStockOccurrence is a proper rational rigid placement of the
// original exact clipped stock. Native edge and face IDs remain those of the
// local model; no source geometry or parameter is rebuilt in world space.
type AlgebraicGraphStockOccurrence struct {
	pair  AlgebraicGraphCapPair
	model AlgebraicGraphVerticalMaterial
	pose  placement.RationalRigid
}

// PlaceVerticalMaterialRigid prepares the exact source and binds a proper
// rigid pose for world material queries. Rigidly placed vertical fibers are
// measured along the original stock Z direction after the pose.
func (c AlgebraicGraphCapPair) PlaceVerticalMaterialRigid(ctx context.Context, pose placement.RationalRigid,
	o RationalGraphShellSectionOptions, roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockOccurrence, error) {
	var zero AlgebraicGraphStockOccurrence
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return zero, e
	}
	p, e := placement.NewRationalRigid(matrix, 4096)
	if e != nil {
		return zero, e
	}
	m, e := c.PrepareVerticalMaterial(ctx, o, roots)
	if e != nil {
		return zero, e
	}
	return AlgebraicGraphStockOccurrence{pair: c, model: m, pose: p}, nil
}

func (o AlgebraicGraphStockOccurrence) Pose() ([3][4]*big.Rat, error) { return o.pose.Matrix() }

// ClassifyRational classifies a world point by exact inverse transformation.
func (o AlgebraicGraphStockOccurrence) ClassifyRational(ctx context.Context, p [3]*big.Rat) (parametric.Location, error) {
	local, e := o.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return parametric.Outside, e
	}
	return o.model.ClassifyRational(ctx, local)
}

// IntersectCapLineRational pulls a world line back to the original cap plane.
// Its exact line parameter, finite window and original trim owner are unchanged.
func (o AlgebraicGraphStockOccurrence) IntersectCapLineRational(ctx context.Context, cap int, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapLine, error) {
	var zero AlgebraicGraphCapLine
	if cap < 0 || cap > 1 {
		return zero, geom.ErrInvalid
	}
	a, e := o.pose.TransformPoint(ctx, origin, true, 4096)
	if e != nil {
		return zero, e
	}
	d, e := o.pose.TransformVector(ctx, direction, true, 4096)
	if e != nil {
		return zero, e
	}
	return o.pair.caps[cap].IntersectRationalLine(ctx, a, d, extent, options)
}

// IntersectLineRational classifies a finite world line against both caps and
// all retained vertical stock walls. The exact parameter and original face
// owners are invariant under the rigid placement.
func (o AlgebraicGraphStockOccurrence) IntersectLineRational(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalLine, error) {
	return o.intersectLineRational(ctx, origin, direction, extent, options, false, false)
}

// IntersectLineRationalWithTouches also retains isolated regular tangencies
// on the original swept stock walls, without toggling material.
func (o AlgebraicGraphStockOccurrence) IntersectLineRationalWithTouches(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalLine, error) {
	return o.intersectLineRational(ctx, origin, direction, extent, options, true, false)
}

// IntersectLineRationalWithJunctions also retains all original owners of an
// isolated shared boundary event. Rigid placement preserves its parameter.
func (o AlgebraicGraphStockOccurrence) IntersectLineRationalWithJunctions(ctx context.Context, origin, direction [3]*big.Rat,
	extent [2]*big.Rat, options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalLine, error) {
	return o.intersectLineRational(ctx, origin, direction, extent, options, true, true)
}

func (o AlgebraicGraphStockOccurrence) intersectLineRational(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions, includeTouches, includeJunctions bool) (AlgebraicGraphVerticalLine, error) {
	var zero AlgebraicGraphVerticalLine
	a, e := o.pose.TransformPoint(ctx, origin, true, 4096)
	if e != nil {
		return zero, e
	}
	d, e := o.pose.TransformVector(ctx, direction, true, 4096)
	if e != nil {
		return zero, e
	}
	return o.model.intersectRationalLine(ctx, a, d, extent, options, includeTouches, includeJunctions)
}

// BoundVolume returns the unchanged original certified volume. Only proper
// rigid poses are admitted, so no numeric determinant estimate is needed.
func (o AlgebraicGraphStockOccurrence) BoundVolume(ctx context.Context, width *big.Rat,
	options RationalGraphShellSectionOptions, roots geom.ExactCurvePlaneEventOptions) (geom.ExactParameterInterval, error) {
	return o.pair.BoundVerticalVolume(ctx, width, options, roots)
}

// BoundSurfaceArea returns the same exact source area for a proper rational
// rigid occurrence. No model curve is rebuilt in world coordinates.
func (o AlgebraicGraphStockOccurrence) BoundSurfaceArea(ctx context.Context, width *big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalArea, error) {
	return o.model.BoundSurfaceArea(ctx, width, options)
}

// CertifiedAlgebraicGraphStockOccurrenceMesh has world-space mesh and
// error bounds. Source is the independent local certificate and supplies the
// original face/edge ancestry. World vertices keep the same shared indices.
type CertifiedAlgebraicGraphStockOccurrenceMesh struct {
	Mesh                                   mesh.Mesh
	Source                                 CertifiedAlgebraicGraphPlanarMesh
	MaximumFillError, MaximumBoundaryError float64
	SignedVolume                           *big.Rat
}

// TessellateCertified transports an independently certified closed local
// mesh under the exact placement and rounds once. Convex barycentric
// interpolation bounds the extra model error by the largest vertex motion.
// World display contacts and closed incidence are rechecked after rounding.
func (o AlgebraicGraphStockOccurrence) TessellateCertified(ctx context.Context, options AlgebraicGraphStockFillOptions) (CertifiedAlgebraicGraphStockOccurrenceMesh, error) {
	var zero CertifiedAlgebraicGraphStockOccurrenceMesh
	if ctx == nil || o.pair.caps[0].recipe == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	target := options.Deflection
	options.Deflection = target * 0.875
	m, e := o.pair.TessellateAffineProjectedStockCertified(ctx, options)
	if e != nil {
		return zero, e
	}
	world := mesh.Mesh{Vertices: make([]geom.Vec3, len(m.Mesh.Vertices)), Triangles: append([][3]int(nil), m.Mesh.Triangles...)}
	upper := new(big.Rat)
	for i, p := range m.Mesh.Vertices {
		exact, e := o.pose.TransformPoint(ctx, exactBoundPoint(p), false, 4096)
		if e != nil {
			return zero, e
		}
		var q [3]float64
		delta := new(big.Rat)
		for j, x := range exact {
			q[j], _ = x.Float64()
			if !finite(q[j]) {
				return zero, geom.ErrUnrepresentable
			}
			v := new(big.Rat).Sub(x, ratInset(q[j]))
			delta.Add(delta, v.Mul(v, v))
		}
		if delta.Cmp(upper) > 0 {
			upper.Set(delta)
		}
		world.Vertices[i] = geom.Vec3{X: q[0], Y: q[1], Z: q[2]}
	}
	_, rounding, e := graphAreaSqrt(upper, 128, 16384)
	if e != nil {
		return zero, e
	}
	bound := func(x float64) float64 {
		value := new(big.Rat).Add(ratInset(x), rounding)
		q, _ := value.Float64()
		if finite(q) && ratInset(q).Cmp(value) < 0 {
			q = math.Nextafter(q, math.Inf(1))
		}
		return q
	}
	fill, boundary := bound(m.MaximumFillError), bound(m.Boundary.MaximumBoundaryError)
	if fill > target {
		return zero, geom.ErrConvergence
	}
	volume, _, e := checkClosedPeriodicMesh(ctx, world, options.Collision)
	if e != nil {
		return zero, e
	}
	return CertifiedAlgebraicGraphStockOccurrenceMesh{Mesh: world, Source: m, MaximumFillError: fill, MaximumBoundaryError: boundary, SignedVolume: volume}, nil
}
