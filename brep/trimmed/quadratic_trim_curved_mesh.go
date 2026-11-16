package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CurvedQuadraticTrimMeshOptions requests a total model-space deflection.
// Boundary topology/work options are retained, but its Deflection is chosen
// conservatively from the certified chart Lipschitz bound. MaxSubdivisions
// defaults to 32 (power of two, maximum128); MaxTriangleChecks to100000 and
// MaxTriangles to100000 (maximum1M). Evaluation bounds each exact sample.
type CurvedQuadraticTrimMeshOptions struct {
	Deflection                                       float64
	Boundary                                         QuadraticTrimLoopOptions
	MaxSubdivisions, MaxTriangleChecks, MaxTriangles int
	Evaluation                                       geom.SurfacePatchEvaluationOptions
}

// CertifiedCurvedQuadraticTrimMesh bounds both directed distances between the
// surface image of the original closed trimmed UV region and the display mesh.
// LipschitzSquared transports the exact UV fill bound; SurfaceSquaredDistance
// is the largest exact common-parameter triangle residual, including display
// rounding. FillSquaredDistance combines both and is <= requested deflection².
// Boundary stores the independently certified UV triangulation proposal. This
// certifies image distance, not surface injectivity, mesh embedding or sewing.
type CertifiedCurvedQuadraticTrimMesh struct {
	// SquaredNormalTangent is the upper bound for an explicitly certified
	// mapped roof. It is nil on the existing distance-only paths.
	SquaredNormalTangent                                                                          *big.Rat `json:",omitempty"`
	InteriorRefinementPasses                                                                      int      // Fixed-boundary refinement only; Subdivisions stays 1.
	SpanCellChecks, SpanPieceChecks                                                               int      // Zero for the single-span API.
	Mesh                                                                                          mesh.Mesh
	ExactUV                                                                                       [][2]*big.Rat
	ExactPoints                                                                                   [][3]*big.Rat
	BoundaryLoops                                                                                 [][]int
	OriginalFace                                                                                  []int
	SquaredErrors                                                                                 []*big.Rat
	Boundary                                                                                      CertifiedAffineQuadraticTrimMesh
	LipschitzSquared, TransportedTrimSquaredDistance, SurfaceSquaredDistance, FillSquaredDistance *big.Rat
	Subdivisions, TriangleChecks                                                                  int
}

// TessellateCurvedQuadraticTrimRegionCertified accepts single-span positive
// rational patches of degree<=2 in each coordinate, with rational quadratic or
// linear UV trims and holes. Exact rational UV triangles are refined uniformly
// with shared edge identities. It composes a planar fill homotopy bound with a
// rigorous chart Jacobian bound and exact rational interpolation residuals.
// Atomic errors distinguish unsupported/budget cases through existing errors.
func TessellateCurvedQuadraticTrimRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, o CurvedQuadraticTrimMeshOptions) (CertifiedCurvedQuadraticTrimMesh, error) {
	return tessellateCurvedQuadraticTrimRegionCertified(ctx, p, outer, holes, o, 1)
}

func tessellateCurvedQuadraticTrimRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, o CurvedQuadraticTrimMeshOptions, minimum int) (CertifiedCurvedQuadraticTrimMesh, error) {
	return tessellateCurvedQuadraticTrimWithSpans(ctx, p, outer, holes, o, minimum, nil)
}

func tessellateCurvedQuadraticTrimWithSpans(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, o CurvedQuadraticTrimMeshOptions, minimum int, spans *multiSpanMeshWork) (CertifiedCurvedQuadraticTrimMesh, error) {
	empty := CertifiedCurvedQuadraticTrimMesh{}
	if ctx == nil || p == nil || !finite(o.Deflection) || o.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 32
	}
	if o.MaxTriangleChecks == 0 {
		o.MaxTriangleChecks = 100000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 128 || o.MaxSubdivisions&(o.MaxSubdivisions-1) != 0 || o.MaxTriangleChecks < 1 || o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	l2, l1, ok := quadraticPatchLipschitz(p)
	if spans != nil {
		var e error
		l2, l1, e = spans.lipschitz(ctx, p)
		if e != nil {
			return empty, e
		}
		ok = true
	}
	if !ok || l1.Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	uvTolerance := new(big.Rat).Quo(ratInset(o.Deflection), new(big.Rat).Mul(big.NewRat(4, 1), l1))
	deflection, _ := uvTolerance.Float64()
	if ratInset(deflection) != nil && ratInset(deflection).Cmp(uvTolerance) > 0 {
		deflection = math.Nextafter(deflection, 0)
	}
	if !finite(deflection) || deflection <= 0 {
		return empty, geom.ErrConvergence
	}
	d := p.Domain()
	ku := []float64{d.UMin, d.UMin, d.UMax, d.UMax}
	kv := []float64{d.VMin, d.VMin, d.VMax, d.VMax}
	identity, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}}, ku, kv, nil)
	if e != nil {
		return empty, e
	}
	uvPatch, e := geom.NewSurfacePatch(identity, geom.SurfacePatchOptions{})
	if e != nil {
		return empty, e
	}
	o.Boundary.Tessellation.Deflection = deflection
	boundary, e := TessellateAffineQuadraticTrimRegionCertified(ctx, uvPatch, outer, holes, AffineQuadraticTrimMeshOptions{Region: o.Boundary, MaxTriangles: o.MaxTriangles})
	if e != nil {
		return empty, e
	}
	return tessellateCertifiedTrimBoundary(ctx, p, boundary, o, minimum, spans, l2)
}

func tessellateCertifiedTrimBoundary(ctx context.Context, p *geom.SurfacePatch, boundary CertifiedAffineQuadraticTrimMesh, o CurvedQuadraticTrimMeshOptions, minimum int, spans *multiSpanMeshWork, l2 *big.Rat) (CertifiedCurvedQuadraticTrimMesh, error) {
	return tessellateCertifiedTrimBoundaryMode(ctx, p, boundary, o, minimum, spans, l2, false)
}
func tessellateCertifiedTrimBoundaryMode(ctx context.Context, p *geom.SurfacePatch, boundary CertifiedAffineQuadraticTrimMesh, o CurvedQuadraticTrimMeshOptions, minimum int, spans *multiSpanMeshWork, l2 *big.Rat, fixedBoundary bool) (CertifiedCurvedQuadraticTrimMesh, error) {
	return tessellateCertifiedTrimBoundaryDisplayMode(ctx, p, boundary, o, minimum, spans, l2, fixedBoundary, nil)
}

// Prescribed display samples participate in the exact triangle residual;
// ExactPoints still records the unmodified rational surface evaluation.
func tessellateCertifiedTrimBoundaryDisplayMode(ctx context.Context, p *geom.SurfacePatch, boundary CertifiedAffineQuadraticTrimMesh, o CurvedQuadraticTrimMeshOptions, minimum int, spans *multiSpanMeshWork, l2 *big.Rat, fixedBoundary bool, display map[int]geom.Vec3) (CertifiedCurvedQuadraticTrimMesh, error) {
	empty := CertifiedCurvedQuadraticTrimMesh{}
	target := ratInset(o.Deflection)
	target.Mul(target, target)
	quarter := new(big.Rat).Quo(target, big.NewRat(4, 1))
	transported := new(big.Rat).Mul(l2, boundary.Region.FillSquaredDistance)
	if transported.Cmp(quarter) > 0 {
		return empty, geom.ErrConvergence
	}
	result := CertifiedCurvedQuadraticTrimMesh{Boundary: boundary, LipschitzSquared: l2, TransportedTrimSquaredDistance: transported, ExactUV: append([][2]*big.Rat(nil), boundary.ExactUV...), BoundaryLoops: boundary.BoundaryLoops}
	result.Mesh.Triangles = append([][3]int(nil), boundary.Mesh.Triangles...)
	evaluate := func(uv [2]*big.Rat) ([3]*big.Rat, geom.Vec3, error) {
		jet, e := p.EvaluateRational(ctx, uv[0], uv[1], o.Evaluation)
		if e != nil {
			return [3]*big.Rat{}, geom.Vec3{}, e
		}
		x, _ := jet.Point[0].Float64()
		y, _ := jet.Point[1].Float64()
		z, _ := jet.Point[2].Float64()
		point := geom.Vec3{X: x, Y: y, Z: z}
		if !point.IsFinite() {
			return [3]*big.Rat{}, geom.Vec3{}, geom.ErrConvergence
		}
		return jet.Point, point, nil
	}
	for id, uv := range result.ExactUV {
		exact, point, e := evaluate(uv)
		if e != nil {
			return empty, e
		}
		if override, ok := display[id]; ok {
			if !override.IsFinite() {
				return empty, geom.ErrInvalid
			}
			point = override
		}
		result.ExactPoints = append(result.ExactPoints, exact)
		result.Mesh.Vertices = append(result.Mesh.Vertices, point)
	}
	type edge [2]int
	key := func(a, b int) edge {
		if a > b {
			a, b = b, a
		}
		return edge{a, b}
	}
	boundaryEdges := map[edge]bool{}
	if fixedBoundary {
		for _, loop := range result.BoundaryLoops {
			for j, a := range loop {
				boundaryEdges[key(a, loop[(j+1)%len(loop)])] = true
			}
		}
	}
	passes := 0
	var quality *mappedRoofQuality
	var normals *mappedRoofNormals
	var distanceByTriangle map[[3]int]*big.Rat
	if spans != nil && spans.maxKnotCellFlips > 0 {
		var err error
		quality, err = newMappedRoofQuality(ctx, p, spans.maxKnotCellFlips)
		if err != nil {
			return empty, fmt.Errorf("mapped roof projected quality: %w", err)
		}
		if spans.mappedNormalAngle > 0 {
			distanceByTriangle = make(map[[3]int]*big.Rat)
			trimDistance := boundary.Region.FillSquaredDistance
			if spans.mappedStraightTrimImage {
				trimDistance = new(big.Rat)
			}
			normals, err = newMappedRoofNormals(ctx, p, spans.mappedNormalAngle, trimDistance)
			if err != nil {
				return empty, fmt.Errorf("mapped roof normal cells: %w", err)
			}
		}
	}
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		if len(result.Mesh.Triangles) > o.MaxTriangleChecks-result.TriangleChecks {
			return empty, geom.ErrConvergence
		}
		if quality != nil {
			if err := quality.improve(ctx, &result); err != nil {
				return empty, fmt.Errorf("mapped roof projected quality: %w", err)
			}
		}
		result.SquaredErrors = nil
		result.OriginalFace = nil
		maximum := new(big.Rat)
		for _, tri := range result.Mesh.Triangles {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			uv := [3][2]*big.Rat{result.ExactUV[tri[0]], result.ExactUV[tri[1]], result.ExactUV[tri[2]]}
			model := [3]geom.Vec3{result.Mesh.Vertices[tri[0]], result.Mesh.Vertices[tri[1]], result.Mesh.Vertices[tri[2]]}
			bound := distanceByTriangle[tri]
			if bound == nil {
				if spans == nil {
					var ok bool
					bound, ok = rationalBiquadraticExactCorrespondenceBound(p, uv, model)
					if !ok {
						return empty, geom.ErrConvergence
					}
				} else {
					var e error
					bound, e = spans.bound(ctx, p, uv, model)
					if e != nil {
						return empty, e
					}
					result.SpanCellChecks, result.SpanPieceChecks = spans.cells, spans.pieces
					if distanceByTriangle != nil {
						distanceByTriangle[tri] = bound
					}
				}
			}
			result.TriangleChecks++
			result.SquaredErrors = append(result.SquaredErrors, bound)
			result.OriginalFace = append(result.OriginalFace, -1)
			if bound.Cmp(maximum) > 0 {
				maximum = bound
			}
		}
		distanceReady := maximum.Cmp(quarter) <= 0 && n >= minimum
		var angleRetries []bool
		if distanceReady && normals != nil {
			var e error
			result.SquaredNormalTangent, angleRetries, e = normals.certify(ctx, p, result)
			if e != nil {
				return empty, fmt.Errorf("mapped roof normal cone: %w", e)
			}
		}
		if distanceReady && (normals == nil || result.SquaredNormalTangent != nil) {
			if e := result.Mesh.Validate(); e != nil {
				return empty, e
			}
			result.Subdivisions = n
			if fixedBoundary {
				result.Subdivisions = 1
				result.InteriorRefinementPasses = passes
			}
			result.SurfaceSquaredDistance = maximum
			result.FillSquaredDistance = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(transported, maximum))
			if result.FillSquaredDistance.Cmp(target) > 0 {
				return empty, geom.ErrConvergence
			}
			return result, nil
		}
		if n == o.MaxSubdivisions || len(result.Mesh.Triangles) > o.MaxTriangles/4 || len(result.Mesh.Triangles) > (o.MaxTriangleChecks-result.TriangleChecks)/4 {
			if distanceReady && normals != nil {
				unresolved := 0
				for _, retry := range angleRetries {
					if retry {
						unresolved++
					}
				}
				return empty, fmt.Errorf("mapped roof angular refinement after %d passes, %d of %d triangles unresolved: %w",
					passes, unresolved, len(result.Mesh.Triangles), geom.ErrConvergence)
			}
			return empty, geom.ErrConvergence
		}
		mids := map[edge]int{}
		midpoint := func(a, b int) (int, error) {
			k := key(a, b)
			if id, ok := mids[k]; ok {
				return id, nil
			}
			var uv [2]*big.Rat
			for j := range uv {
				uv[j] = new(big.Rat).Quo(new(big.Rat).Add(result.ExactUV[a][j], result.ExactUV[b][j]), big.NewRat(2, 1))
			}
			exact, point, e := evaluate(uv)
			if e != nil {
				return 0, e
			}
			id := len(result.Mesh.Vertices)
			mids[k] = id
			result.ExactUV = append(result.ExactUV, uv)
			result.ExactPoints = append(result.ExactPoints, exact)
			result.Mesh.Vertices = append(result.Mesh.Vertices, point)
			return id, nil
		}
		next := make([][3]int, 0, 4*len(result.Mesh.Triangles))
		if fixedBoundary {
			selected := map[edge]bool{}
			for i, needsRefinement := range angleRetries {
				if needsRefinement {
					t := result.Mesh.Triangles[i]
					for j, a := range t {
						k := key(a, t[(j+1)%3])
						if !boundaryEdges[k] {
							selected[k] = true
						}
					}
				}
			}
			for i, t := range result.Mesh.Triangles {
				if err := ctx.Err(); err != nil {
					return empty, err
				}
				m := [3]int{-1, -1, -1}
				count := 0
				for j, a := range t {
					k := key(a, t[(j+1)%3])
					if !boundaryEdges[k] && (angleRetries == nil || selected[k]) {
						id, err := midpoint(a, t[(j+1)%3])
						if err != nil {
							return empty, err
						}
						m[j] = id
						count++
					}
				}
				switch count {
				case 0:
					if angleRetries != nil && !angleRetries[i] {
						next = append(next, t)
						continue
					}
					var uv [2]*big.Rat
					for k := range uv {
						uv[k] = new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Add(result.ExactUV[t[0]][k], result.ExactUV[t[1]][k]), result.ExactUV[t[2]][k]), big.NewRat(3, 1))
					}
					exact, point, err := evaluate(uv)
					if err != nil {
						return empty, err
					}
					id := len(result.Mesh.Vertices)
					result.ExactUV = append(result.ExactUV, uv)
					result.ExactPoints = append(result.ExactPoints, exact)
					result.Mesh.Vertices = append(result.Mesh.Vertices, point)
					for j, a := range t {
						next = append(next, [3]int{a, t[(j+1)%3], id})
					}
				case 1:
					for j, id := range m {
						if id >= 0 {
							next = append(next, [3]int{t[j], id, t[(j+2)%3]}, [3]int{id, t[(j+1)%3], t[(j+2)%3]})
							break
						}
					}
				case 2:
					for j := range m {
						if m[j] >= 0 && m[(j+1)%3] >= 0 {
							a, b, c := t[j], t[(j+1)%3], t[(j+2)%3]
							x, y := m[j], m[(j+1)%3]
							next = append(next, [3]int{b, y, x}, [3]int{a, x, c}, [3]int{x, y, c})
							break
						}
					}
				case 3:
					next = append(next, [3]int{t[0], m[0], m[2]}, [3]int{m[0], t[1], m[1]}, [3]int{m[2], m[1], t[2]}, [3]int{m[0], m[1], m[2]})
				}
			}
			result.Mesh.Triangles = next
			passes++
			continue
		}
		for _, t := range result.Mesh.Triangles {
			a, e := midpoint(t[0], t[1])
			if e != nil {
				return empty, e
			}
			b, e := midpoint(t[1], t[2])
			if e != nil {
				return empty, e
			}
			c, e := midpoint(t[2], t[0])
			if e != nil {
				return empty, e
			}
			next = append(next, [3]int{t[0], a, c}, [3]int{a, t[1], b}, [3]int{c, b, t[2]}, [3]int{a, b, c})
		}
		result.Mesh.Triangles = next
		var loops [][]int
		for _, loop := range result.BoundaryLoops {
			next := make([]int, 0, 2*len(loop))
			for i, a := range loop {
				mid, ok := mids[key(a, loop[(i+1)%len(loop)])]
				if !ok {
					return empty, geom.ErrConvergence
				}
				next = append(next, a, mid)
			}
			loops = append(loops, next)
		}
		result.BoundaryLoops = loops
	}
	return empty, geom.ErrConvergence
}

// Frobenius norm bounds the Jacobian operator norm on the convex rectangle.
// Absolute power coefficient sums bound H,H_u,H_v,W,W_u,W_v on [0,1]^2;
// positive Bernstein weights provide a strict W lower bound. Quotient rule
// then bounds every component of both original-parameter derivatives.
func quadraticPatchLipschitz(p *geom.SurfacePatch) (squared, sum *big.Rat, ok bool) {
	c, minW, ok := p.RationalBiquadraticCoefficients()
	if !ok {
		return nil, nil, false
	}
	var h, du, dv [4]*big.Rat
	for k := 0; k < 4; k++ {
		h[k] = new(big.Rat)
		du[k] = new(big.Rat)
		dv[k] = new(big.Rat)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 4; k++ {
				a := new(big.Rat).Abs(c[i][j][k])
				h[k].Add(h[k], a)
				du[k].Add(du[k], new(big.Rat).Mul(big.NewRat(int64(i), 1), a))
				dv[k].Add(dv[k], new(big.Rat).Mul(big.NewRat(int64(j), 1), a))
			}
		}
	}
	d := p.Domain()
	widths := [2]*big.Rat{new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))}
	denominator := new(big.Rat).Mul(minW, minW)
	squared, sum = new(big.Rat), new(big.Rat)
	for axis, derivative := range [][4]*big.Rat{du, dv} {
		for k := 0; k < 3; k++ {
			bound := new(big.Rat).Add(new(big.Rat).Mul(derivative[k], h[3]), new(big.Rat).Mul(h[k], derivative[3]))
			bound.Quo(bound, new(big.Rat).Mul(denominator, widths[axis]))
			sum.Add(sum, bound)
			squared.Add(squared, new(big.Rat).Mul(bound, bound))
		}
	}
	return squared, sum, true
}
