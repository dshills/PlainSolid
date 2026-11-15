package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CertifiedPolygonPatchMesh triangulates one polygonally trimmed patch. Loops
// preserves outer-first/hole order as mesh vertex indices, including inserted
// samples. VertexUV corresponds to Mesh.Vertices. OriginalFace is -1 for a
// standalone patch. Each triangle's certificate covers its actual UV interior;
// direction semantics follow BothDirections. No global embedding or cross-face
// sewing guarantee is implied by this open single-face mesh.
type CertifiedPolygonPatchMesh struct {
	CertifiedPatchMesh
	VertexUV     []geom.Vec2
	Loops        [][]int
	OriginalFace int
	// KnotCells/KnotPieces include all attempted refinements; zero when the
	// optional original-knot proof was not requested.
	KnotCells, KnotPieces int
	// Uses records exact original straight-UV traversal parameters for stored
	// faces only. It does not certify correspondence to stored model edges.
	Uses []PolygonFaceUseSamples
}

// PolygonFaceUseSamples preserves original UV coedge lineage independently
// of face orientation. Vertices index the final display; Parameters are exact
// original normalized UV-use parameters. No model-edge parameter is inferred.
type PolygonFaceUseSamples struct {
	Face, Loop, Use, Edge int
	FaceReversed          bool
	Vertices              []int
	Parameters            []*big.Rat
}

// TessellatePolygonPatchCertified validates straight polygonal UV trims with
// holes, triangulates them exactly, then uniformly subdivides all triangles.
// Exact represented UV midpoint checks prevent silent parameter movement;
// shared UV vertices are evaluated once, preserving conforming interior edges
// and complete hole boundaries. Model rounding is covered by residual bounds.
// Outer is CCW and holes CW. Unsupported/degenerate trims or exhausted budgets
// return no partial mesh. Limits retain CertifiedPatchMeshOptions semantics.
func TessellatePolygonPatchCertified(ctx context.Context, p *geom.SurfacePatch, outer []geom.Vec2, holes [][]geom.Vec2, o CertifiedPatchMeshOptions) (CertifiedPolygonPatchMesh, error) {
	return tessellatePolygonPatchCertified(ctx, p, outer, holes, o, 1)
}

func tessellatePolygonPatchCertified(ctx context.Context, p *geom.SurfacePatch, outer []geom.Vec2, holes [][]geom.Vec2, o CertifiedPatchMeshOptions, minimum int) (CertifiedPolygonPatchMesh, error) {
	empty := CertifiedPolygonPatchMesh{}
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
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 128 || o.MaxSubdivisions&(o.MaxSubdivisions-1) != 0 || o.MaxTriangleChecks < 1 {
		return empty, geom.ErrInvalid
	}
	var rings [][]geom.Vec2
	var flat [][]geom.Vec3
	count := 0
	domain := p.Domain()
	for _, loop := range append([][]geom.Vec2{outer}, holes...) {
		if len(loop) < 3 || len(loop) > brep.MaxHoledFaceVertices-count {
			return empty, geom.ErrInvalid
		}
		count += len(loop)
		ring := append([]geom.Vec2(nil), loop...)
		var points []geom.Vec3
		for _, uv := range ring {
			if !finite(uv.X) || !finite(uv.Y) || uv.X < domain.UMin || uv.X > domain.UMax || uv.Y < domain.VMin || uv.Y > domain.VMax {
				return empty, geom.ErrDomain
			}
			points = append(points, geom.Vec3{X: uv.X, Y: uv.Y})
		}
		rings = append(rings, ring)
		flat = append(flat, points)
	}
	triangles, e := triangulateUV(flat)
	if e != nil {
		return empty, e
	}
	tolerance := ratInset(o.Deflection)
	tolerance.Mul(tolerance, tolerance)
	checks := 0
	var knot *polygonKnotBudget
	if o.PolygonMultiSpan != nil {
		knot, e = newPolygonKnotBudget(*o.PolygonMultiSpan)
		if e != nil {
			return empty, e
		}
	}
	midpoint := func(a, b geom.Vec2) (geom.Vec2, error) {
		var result [2]float64
		for i, pair := range [][2]float64{{a.X, b.X}, {a.Y, b.Y}} {
			r := new(big.Rat).Quo(new(big.Rat).Add(ratInset(pair[0]), ratInset(pair[1])), big.NewRat(2, 1))
			f, _ := r.Float64()
			if !finite(f) || ratInset(f).Cmp(r) != 0 {
				return geom.Vec2{}, geom.ErrConvergence
			}
			result[i] = f
		}
		return geom.Vec2{X: result[0], Y: result[1]}, nil
	}
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		if len(triangles) > o.MaxTriangleChecks-checks {
			return empty, geom.ErrConvergence
		}
		result := CertifiedPolygonPatchMesh{CertifiedPatchMesh: CertifiedPatchMesh{Subdivisions: n, BothDirections: true}, OriginalFace: -1}
		ids := map[geom.Vec2]int{}
		vertex := func(uv geom.Vec2) (int, error) {
			if id, ok := ids[uv]; ok {
				return id, nil
			}
			jet, e := p.EvaluateRational(ctx, ratInset(uv.X), ratInset(uv.Y), o.Evaluation)
			if e != nil {
				return 0, e
			}
			x, _ := jet.Point[0].Float64()
			y, _ := jet.Point[1].Float64()
			z, _ := jet.Point[2].Float64()
			point := geom.Vec3{X: x, Y: y, Z: z}
			if !point.IsFinite() {
				return 0, geom.ErrConvergence
			}
			id := len(result.Mesh.Vertices)
			ids[uv] = id
			result.Mesh.Vertices = append(result.Mesh.Vertices, point)
			result.VertexUV = append(result.VertexUV, uv)
			return id, nil
		}
		accepted := true
		for _, uv := range triangles {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			var index [3]int
			var model [3]geom.Vec3
			for k := range uv {
				id, e := vertex(uv[k])
				if e != nil {
					return empty, e
				}
				index[k] = id
				model[k] = result.Mesh.Vertices[id]
			}
			bound, e := polygonTriangleDeviation(ctx, p, uv, model, knot)
			checks++
			if e != nil {
				return empty, e
			}
			result.BothDirections = result.BothDirections && bound.BothDirections
			if bound.SquaredDistance.Cmp(tolerance) > 0 {
				accepted = false
			}
			result.Mesh.Triangles = append(result.Mesh.Triangles, index)
			result.UVTriangles = append(result.UVTriangles, uv)
			result.SquaredErrors = append(result.SquaredErrors, bound.SquaredDistance)
		}
		if accepted && n >= minimum {
			for _, ring := range rings {
				loop := make([]int, len(ring))
				for i, uv := range ring {
					id, ok := ids[uv]
					if !ok {
						return empty, geom.ErrConvergence
					}
					loop[i] = id
				}
				result.Loops = append(result.Loops, loop)
			}
			if e := result.Mesh.Validate(); e != nil {
				return empty, e
			}
			result.TriangleChecks = checks
			if knot != nil {
				result.KnotCells = knot.cells
				result.KnotPieces = knot.pieces
			}
			return result, nil
		}
		if n == o.MaxSubdivisions || len(triangles) > (o.MaxTriangleChecks-checks)/4 {
			return empty, geom.ErrConvergence
		}
		next := make([][3]geom.Vec2, 0, 4*len(triangles))
		for _, t := range triangles {
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
			next = append(next, [3]geom.Vec2{t[0], a, c}, [3]geom.Vec2{a, t[1], b}, [3]geom.Vec2{c, b, t[2]}, [3]geom.Vec2{a, b, c})
		}
		triangles = next
		for j, ring := range rings {
			next := make([]geom.Vec2, 0, 2*len(ring))
			for i, a := range ring {
				b, e := midpoint(a, ring[(i+1)%len(ring)])
				if e != nil {
					return empty, e
				}
				next = append(next, a, b)
			}
			rings[j] = next
		}
	}
	return empty, geom.ErrConvergence
}

// TessellatePolygonFaceCertified meshes one existing straight-trim spline face.
// It retains original face identity and boundary loops. Curved trims reject;
// independent output faces are not claimed to share boundary samples.
func (s Solid) TessellatePolygonFaceCertified(ctx context.Context, face int, o CertifiedPatchMeshOptions) (CertifiedPolygonPatchMesh, error) {
	return s.tessellatePolygonFaceCertified(ctx, face, o, 1)
}

func (s Solid) tessellatePolygonFaceCertified(ctx context.Context, face int, o CertifiedPatchMeshOptions, minimum int) (CertifiedPolygonPatchMesh, error) {
	empty := CertifiedPolygonPatchMesh{}
	if face < 0 || face >= len(s.faces) {
		return empty, geom.ErrInvalid
	}
	f := s.faces[face]
	if len(f.Outer) < 3 {
		return empty, geom.ErrInvalid
	}
	base, m := parametric.SurfaceMapping(f.Surface)
	var p *geom.SurfacePatch
	var e error
	switch v := base.(type) {
	case *geom.BSplineSurface:
		p, e = geom.NewSurfacePatch(v, geom.SurfacePatchOptions{})
	case *geom.WrappedSurfacePatch:
		p = v.FundamentalPatch()
	case *geom.ExactBezierSurface, *geom.ExactTensorSurface:
		p, e = exactSplineSupportPatch(base, geom.SurfacePatchOptions{})
	default:
		return empty, geom.ErrConvergence
	}
	if e != nil {
		return empty, e
	}
	if m != geom.Identity() {
		p, e = p.Transformed(m)
		if e != nil {
			return empty, e
		}
	}
	var loops [][]geom.Vec2
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		var uv []geom.Vec2
		for i, u := range loop {
			if u.Curve != nil || u.UVEnd != loop[(i+1)%len(loop)].UVStart {
				return empty, geom.ErrConvergence
			}
			uv = append(uv, u.UVStart)
		}
		loops = append(loops, uv)
	}
	result, e := tessellatePolygonPatchCertified(ctx, p, loops[0], loops[1:], o, minimum)
	if e != nil {
		return empty, e
	}
	result.OriginalFace = face
	result.Uses, e = polygonOriginalUseSamples(ctx, f, face, result)
	if e != nil {
		return empty, e
	}
	if f.Reversed {
		for i := range result.Mesh.Triangles {
			t := &result.Mesh.Triangles[i]
			t[1], t[2] = t[2], t[1]
			uv := &result.UVTriangles[i]
			uv[1], uv[2] = uv[2], uv[1]
		}
	}
	return result, nil
}
