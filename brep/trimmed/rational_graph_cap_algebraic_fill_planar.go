//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AlgebraicGraphPlanarMeshOptions limits shared boundary approximation,
// planar-face triangulation, and the final exact display embedding check.
// Every original retained stock support must be proved planar.
type AlgebraicGraphPlanarMeshOptions struct {
	Boundary      AlgebraicGraphStockBoundaryOptions
	Polygon       trimvalidate.Options
	MaxPredicates int
	Collision     collision.Options
}

// CertifiedAlgebraicGraphPlanarMesh reports the common exact edge network,
// original topology per triangle, and a two-sided source-fill bound.
// All triangles are checked for unintended contact and closed incidence.
type CertifiedAlgebraicGraphPlanarMesh struct {
	Mesh             mesh.Mesh
	Boundary         CertifiedAlgebraicGraphStockBoundary
	IndexedFaces     []int
	Predicates       int
	SignedVolume     *big.Rat
	MaximumFillError float64
}

// TessellatePlanarStockCertified fills the closed shell using the single
// shared boundary network. It accepts only strictly planar source patches
// and represented samples exactly in their owning planes. Each indexed face
// must have a single occupied region, optionally with holes. Planar winding
// through the certified edge homotopies bounds the fill in both directions.
func (c AlgebraicGraphCapPair) TessellatePlanarStockCertified(ctx context.Context, o AlgebraicGraphPlanarMeshOptions) (CertifiedAlgebraicGraphPlanarMesh, error) {
	var zero CertifiedAlgebraicGraphPlanarMesh
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 2000000
	}
	if o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return zero, geom.ErrInvalid
	}
	net, e := c.TessellateStockBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return zero, e
	}
	out := CertifiedAlgebraicGraphPlanarMesh{Boundary: net, MaximumFillError: net.MaximumBoundaryError}
	out.Mesh.Vertices = append([]geom.Vec3(nil), net.Vertices...)
	for fi, face := range net.Indexed.Faces {
		var plane polynomialPlaneEquation
		var e error
		if face.Cap >= 0 {
			plane = c.caps[face.Cap].regions.plane
		} else {
			original := c.caps[0].recipe.stock.faces[face.StockFace]
			patch, e := exactFaceSupportPatch(original, o.Boundary.Section.Spans.Faces.Patches)
			if e != nil {
				return zero, e
			}
			uv, e := faceUVBounds(original)
			if e != nil {
				return zero, e
			}
			patch, e = patch.Trim(uv)
			if e != nil {
				return zero, e
			}
			plane, e = exactGraphStockFacePlane(ctx, patch, original.Reversed, o.Boundary.Section.Spans.Faces)
			if e != nil {
				return zero, e
			}
		}
		remaining := o.MaxPredicates - out.Predicates
		if remaining < 1 {
			return zero, geom.ErrConvergence
		}
		tri, used, e := fillAlgebraicGraphPlanarFace(ctx, net, face, plane, o.Polygon, remaining)
		if e != nil {
			return zero, e
		}
		out.Predicates += used
		out.Mesh.Triangles = append(out.Mesh.Triangles, tri...)
		for range tri {
			out.IndexedFaces = append(out.IndexedFaces, fi)
		}
	}
	out.SignedVolume, _, e = checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if e != nil {
		return zero, e
	}
	return out, nil
}

func exactGraphStockFacePlane(ctx context.Context, patch *geom.SurfacePatch, reversed bool, o SplineRectangleEmbeddingOptions) (polynomialPlaneEquation, error) {
	var zero polynomialPlaneEquation
	d := patch.Domain()
	var p [3]geom.Vec3
	for i, uv := range [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}} {
		j, e := patch.EvaluateRational(ctx, ratInset(uv[0]), ratInset(uv[1]), geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
		})
		if e != nil {
			return zero, e
		}
		p[i], e = geom.RepresentedExactPoint(j.Point)
		if e != nil {
			return zero, e
		}
	}
	if reversed {
		p[1], p[2] = p[2], p[1]
	}
	proof, e := geom.CertifySurfaceSupportingPlane(ctx, patch, p[0], p[1], p[2], o.Plane)
	if e != nil || !proof.Complete || !proof.Coincident {
		if e == nil {
			e = geom.ErrUnrepresentable
		}
		return zero, e
	}
	return polynomialPlaneFromPoints(p)
}

func fillAlgebraicGraphPlanarFace(ctx context.Context, net CertifiedAlgebraicGraphStockBoundary, face AlgebraicGraphShellFace,
	plane polynomialPlaneEquation, o trimvalidate.Options, limit int) ([][3]int, int, error) {
	axes := orientedRationalGraphProjection(plane)
	return fillAlgebraicGraphProjectedFace(ctx, net, face, axes, func(p [3]*big.Rat) bool {
		return plane.dot(p).Cmp(plane.offset) == 0
	}, o, limit)
}

func fillAlgebraicGraphProjectedFace(ctx context.Context, net CertifiedAlgebraicGraphStockBoundary, face AlgebraicGraphShellFace,
	axes [2]int, validate func([3]*big.Rat) bool, o trimvalidate.Options, limit int) ([][3]int, int, error) {
	var points [][2]*big.Rat
	var rings []trimvalidate.Loop
	var local, global [][]int
	for _, loop := range face.Loops {
		var path []int
		for _, use := range loop {
			p := net.Edges[use.Edge]
			for i := 0; i < len(p)-1; i++ {
				k := i
				if use.Reversed {
					k = len(p) - 1 - i
				}
				path = append(path, p[k])
			}
		}
		if len(path) < 3 {
			return nil, 0, geom.ErrDegenerate
		}
		row := make([]int, len(path))
		f := make([]geom.Vec2, len(path))
		for j, idx := range path {
			p := exactBoundPoint(net.Vertices[idx])
			if validate != nil && !validate(p) {
				return nil, 0, geom.ErrUnrepresentable
			}
			row[j] = len(points)
			points = append(points, [2]*big.Rat{p[axes[0]], p[axes[1]]})
			f[j].X, _ = p[axes[0]].Float64()
			f[j].Y, _ = p[axes[1]].Float64()
		}
		var ring trimvalidate.Loop
		for i := range f {
			ring = append(ring, trimvalidate.Segment{Start: f[i], End: f[(i+1)%len(f)]})
		}
		rings = append(rings, ring)
		local, global = append(local, row), append(global, path)
	}
	_, forest, report, e := trimvalidate.CertifyRingForest(ctx, rings, o)
	if e != nil || report.Status != trimvalidate.Certified || len(forest) != len(global) {
		if e == nil {
			e = geom.ErrConvergence
		}
		return nil, 0, e
	}
	outer := -1
	for i, q := range forest {
		if q.Depth == 0 && q.Orientation == 1 && outer < 0 {
			outer = i
		} else if q.Depth != 1 || q.Orientation != -1 {
			return nil, 0, geom.ErrConvergence
		}
	}
	if outer < 0 {
		return nil, 0, geom.ErrConvergence
	}
	local[0], local[outer] = local[outer], local[0]
	list := make([]int, 0, len(points))
	for _, row := range global {
		list = append(list, row...)
	}
	tri, used, e := exactHoledFootprintTriangulation(ctx, points, local, limit)
	if e != nil {
		return nil, used, e
	}
	for i, tr := range tri {
		tri[i] = [3]int{list[tr[0]], list[tr[1]], list[tr[2]]}
	}
	return tri, used, nil
}
