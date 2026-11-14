package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AffineQuadraticTrimMeshOptions bounds the boundary proof and final triangles.
// MaxTriangles defaults to 100000 (maximum 1000000). Region's polygon edge
// budget and brep.MaxHoledFaceVertices also bound the triangulation proposal.
type AffineQuadraticTrimMeshOptions struct {
	Region       QuadraticTrimLoopOptions
	MaxTriangles int
}

// CertifiedAffineQuadraticTrimMesh covers the validated exact chord polygon by
// exact rational triangles; Mesh contains their rounded model display vertices.
// ExactUV and ExactPoints correspond to Mesh.Vertices. BoundaryLoops has outer
// CCW followed by holes CW, with no duplicated closing vertex. OriginalFace is
// -1 for this standalone face. Every triangle has that same ancestry.
// DisplaySquaredDistance bounds exact-triangle/display-triangle correspondence.
// FillSquaredDistance bounds both directed distances from the original curved
// closed face region to the displayed triangle union, including model rounding.
// No cross-face sewing or rounded-mesh embeddedness guarantee is implied.
type CertifiedAffineQuadraticTrimMesh struct {
	Mesh                                        mesh.Mesh
	ExactUV                                     [][2]*big.Rat
	ExactPoints                                 [][3]*big.Rat
	BoundaryLoops                               [][]int
	OriginalFace                                []int
	Region                                      AffineQuadraticTrimRegionArea
	DisplaySquaredDistance, FillSquaredDistance *big.Rat
}

// TessellateAffineQuadraticTrimRegionCertified uses rounded UV coordinates only
// to propose triangle indices. It certifies those indices against original
// rational vertices: positive exact triangle orientation, cancellation of all
// interior edges, exact equality of directed boundary chains and signed area.
// Winding additivity then proves these triangles cover precisely the validated
// chord polygon region, including its holes. No rounded UV geometry is accepted
// as a replacement. Ambiguous/degenerate proposals reject without partial output.
func TessellateAffineQuadraticTrimRegionCertified(ctx context.Context, patch *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, o AffineQuadraticTrimMeshOptions) (CertifiedAffineQuadraticTrimMesh, error) {
	empty := CertifiedAffineQuadraticTrimMesh{}
	if ctx == nil || patch == nil {
		return empty, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	region, e := BoundAffineQuadraticTrimRegionArea(ctx, patch, outer, holes, o.Region)
	if e != nil {
		return empty, e
	}
	return triangulateCertifiedAffineTrimRegion(ctx, region, len(holes), o.MaxTriangles)
}

func triangulateCertifiedAffineTrimRegion(ctx context.Context, region AffineQuadraticTrimRegionArea, holes, maximum int) (CertifiedAffineQuadraticTrimMesh, error) {
	return triangulateCertifiedAffineTrimRegionBounded(ctx, region, holes, maximum, 0, 0)
}

func triangulateCertifiedAffineTrimRegionBounded(ctx context.Context, region AffineQuadraticTrimRegionArea, holes, maximum, boundaryLimit, workLimit int) (CertifiedAffineQuadraticTrimMesh, error) {
	empty := CertifiedAffineQuadraticTrimMesh{}
	o := AffineQuadraticTrimMeshOptions{MaxTriangles: maximum}
	if boundaryLimit == 0 {
		boundaryLimit = brep.MaxHoledFaceVertices
	}
	if workLimit == 0 {
		workLimit = 2000000
	}
	if boundaryLimit < 1 || boundaryLimit > 2048 || workLimit < 1 {
		return empty, geom.ErrInvalid
	}
	count := 0
	for _, loop := range region.Loops {
		count += len(loop.Boundary.Points)
	}
	if count > boundaryLimit || count+2*holes-2 > o.MaxTriangles {
		return empty, geom.ErrConvergence
	}
	result := CertifiedAffineQuadraticTrimMesh{Region: region, DisplaySquaredDistance: new(big.Rat)}
	ids := map[geom.Vec2]int{}
	var proposedLoops [][]geom.Vec3
	for li, loop := range region.Loops {
		var boundary []int
		var proposal []geom.Vec3
		for j := range loop.Boundary.ExactUV {
			index := j
			if li > 0 {
				index = len(loop.Boundary.ExactUV) - 1 - j
			}
			uv := loop.Boundary.ExactUV[index]
			point := loop.Boundary.ExactPoints[index]
			u, _ := uv[0].Float64()
			v, _ := uv[1].Float64()
			roundedUV := geom.Vec2{X: u, Y: v}
			if !finite(u) || !finite(v) {
				return empty, geom.ErrConvergence
			}
			if _, exists := ids[roundedUV]; exists {
				return empty, geom.ErrConvergence
			}
			id := len(result.Mesh.Vertices)
			ids[roundedUV] = id
			boundary = append(boundary, id)
			proposal = append(proposal, geom.Vec3{X: u, Y: v})
			display := loop.Boundary.Points[index]
			result.Mesh.Vertices = append(result.Mesh.Vertices, display)
			result.ExactUV = append(result.ExactUV, uv)
			result.ExactPoints = append(result.ExactPoints, point)
			errorSquared := new(big.Rat)
			for k, value := range []float64{display.X, display.Y, display.Z} {
				delta := new(big.Rat).Sub(point[k], ratInset(value))
				errorSquared.Add(errorSquared, new(big.Rat).Mul(delta, delta))
			}
			if errorSquared.Cmp(result.DisplaySquaredDistance) > 0 {
				result.DisplaySquaredDistance = errorSquared
			}
		}
		result.BoundaryLoops = append(result.BoundaryLoops, boundary)
		proposedLoops = append(proposedLoops, proposal)
	}
	var proposed [][3]geom.Vec2
	if count <= brep.MaxHoledFaceVertices {
		var e error
		proposed, e = triangulateUV(proposedLoops)
		if e != nil {
			return empty, e
		}
	} else {
		exact, _, e := exactHoledFootprintTriangulation(ctx, result.ExactUV, result.BoundaryLoops, workLimit)
		if e != nil {
			return empty, e
		}
		for _, tri := range exact {
			var q [3]geom.Vec2
			for j, id := range tri {
				x, _ := result.ExactUV[id][0].Float64()
				y, _ := result.ExactUV[id][1].Float64()
				q[j] = geom.Vec2{X: x, Y: y}
			}
			proposed = append(proposed, q)
		}
	}
	if len(proposed) > o.MaxTriangles {
		return empty, geom.ErrConvergence
	}
	type edge [2]int
	key := func(a, b int) (edge, int) {
		if a > b {
			return edge{b, a}, -1
		}
		return edge{a, b}, 1
	}
	expected := map[edge]int{}
	for _, loop := range result.BoundaryLoops {
		for i, a := range loop {
			k, sign := key(a, loop[(i+1)%len(loop)])
			expected[k] += sign
		}
	}
	actual, incidence := map[edge]int{}, map[edge]int{}
	area := new(big.Rat)
	for _, tri := range proposed {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		var t [3]int
		for j, uv := range tri {
			id, ok := ids[uv]
			if !ok {
				return empty, geom.ErrConvergence
			}
			t[j] = id
		}
		a, b, c := result.ExactUV[t[0]], result.ExactUV[t[1]], result.ExactUV[t[2]]
		determinant := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
		if determinant.Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		area.Add(area, new(big.Rat).Quo(determinant, big.NewRat(2, 1)))
		for j, a := range t {
			k, sign := key(a, t[(j+1)%3])
			actual[k] += sign
			incidence[k]++
		}
		result.Mesh.Triangles = append(result.Mesh.Triangles, t)
		result.OriginalFace = append(result.OriginalFace, -1)
	}
	if area.Cmp(region.PolygonUVArea) != 0 {
		return empty, geom.ErrConvergence
	}
	for k, n := range incidence {
		wanted := expected[k]
		if actual[k] != wanted || wanted == 0 && n != 2 || wanted != 0 && n != 1 {
			return empty, geom.ErrConvergence
		}
	}
	for k, wanted := range expected {
		if wanted == 0 || actual[k] != wanted {
			return empty, geom.ErrConvergence
		}
	}
	if e := result.Mesh.Validate(); e != nil {
		return empty, e
	}
	result.FillSquaredDistance = new(big.Rat).Set(region.FillSquaredDistance)
	if result.DisplaySquaredDistance.Sign() > 0 {
		result.FillSquaredDistance.Add(result.FillSquaredDistance, result.DisplaySquaredDistance)
		result.FillSquaredDistance.Mul(result.FillSquaredDistance, big.NewRat(2, 1))
	}
	return result, nil
}

// TessellateAffineQuadraticFaceCertified adapts one existing spline face with
// straight or SplineUV trims. It proves that face's chart/trim region, not model
// edge agreement with neighboring faces. Curved nonaffine supports and other
// p-curve representations reject; face reversal changes triangle winding.
func (s Solid) TessellateAffineQuadraticFaceCertified(ctx context.Context, face int, o AffineQuadraticTrimMeshOptions) (CertifiedAffineQuadraticTrimMesh, error) {
	empty := CertifiedAffineQuadraticTrimMesh{}
	if ctx == nil || face < 0 || face >= len(s.faces) {
		return empty, geom.ErrInvalid
	}
	f := s.faces[face]
	base, m := parametric.SurfaceMapping(f.Surface)
	var patch *geom.SurfacePatch
	var e error
	switch v := base.(type) {
	case *geom.BSplineSurface:
		patch, e = geom.NewSurfacePatch(v, geom.SurfacePatchOptions{})
	case *geom.WrappedSurfacePatch:
		patch = v.FundamentalPatch()
	default:
		return empty, geom.ErrConvergence
	}
	if e != nil {
		return empty, e
	}
	if m != geom.Identity() {
		patch, e = patch.Transformed(m)
		if e != nil {
			return empty, e
		}
	}
	var loops [][]SplineUV
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		var trims []SplineUV
		for _, u := range loop {
			var trim SplineUV
			switch c := u.Curve.(type) {
			case nil:
				curve, e := geom.NewBSpline(1, []geom.Vec3{{X: u.UVStart.X, Y: u.UVStart.Y}, {X: u.UVEnd.X, Y: u.UVEnd.Y}}, []float64{0, 0, 1, 1}, nil)
				if e != nil {
					return empty, e
				}
				trim = SplineUV{Curve: curve, Range: UVInterval{Max: 1}}
			case SplineUV:
				trim = c
			case *SplineUV:
				if c == nil {
					return empty, geom.ErrInvalid
				}
				trim = *c
			default:
				return empty, geom.ErrConvergence
			}
			trims = append(trims, trim)
		}
		loops = append(loops, trims)
	}
	if len(loops) == 0 {
		return empty, geom.ErrInvalid
	}
	result, e := TessellateAffineQuadraticTrimRegionCertified(ctx, patch, loops[0], loops[1:], o)
	if e != nil {
		return empty, e
	}
	for i := range result.Mesh.Triangles {
		result.OriginalFace[i] = face
		if f.Reversed {
			result.Mesh.Triangles[i][1], result.Mesh.Triangles[i][2] = result.Mesh.Triangles[i][2], result.Mesh.Triangles[i][1]
		}
	}
	return result, nil
}
