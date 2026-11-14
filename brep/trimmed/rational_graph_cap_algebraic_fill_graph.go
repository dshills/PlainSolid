package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphStockFillOptions extends the planar shell mesher to stock
// supports with a certified affine projection and bounded Bézier residuals.
type AlgebraicGraphStockFillOptions struct {
	AlgebraicGraphPlanarMeshOptions
	Deflection    float64 // maximum two-sided source-fill error
	MaxOperations int
	MaxBits       int
}

// TessellateAffineProjectedStockCertified fills each face from the same
// certified edge indices. A curved face must be a graph over an affine
// projection of its original chart. Exact projected winding gives two-sided
// planar fill correspondence, and the original-surface affine lift and
// triangle residual then bound model error. Unsupported graphs, insufficient
// fixed-edge sampling, or an invalid displayed shell fail atomically.
func (c AlgebraicGraphCapPair) TessellateAffineProjectedStockCertified(ctx context.Context, o AlgebraicGraphStockFillOptions) (CertifiedAlgebraicGraphPlanarMesh, error) {
	var zero CertifiedAlgebraicGraphPlanarMesh
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 {
		return zero, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 120000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 2000000
	}
	if o.MaxOperations < 1 || o.MaxBits < 64 || o.MaxBits > 65536 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return zero, geom.ErrInvalid
	}
	if o.Boundary.Display.Deflection == 0 {
		o.Boundary.Display.Deflection = o.Deflection / 16
	}
	net, e := c.TessellateStockBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return zero, e
	}
	out := CertifiedAlgebraicGraphPlanarMesh{Boundary: net, MaximumFillError: net.MaximumBoundaryError}
	out.Mesh.Vertices = append(out.Mesh.Vertices, net.Vertices...)
	budget := capClipBudget{ctx, o.MaxOperations, 1, o.MaxBits}
	for fi, face := range net.Indexed.Faces {
		remaining := o.MaxPredicates - out.Predicates
		if remaining < 1 {
			return zero, geom.ErrConvergence
		}
		if face.Cap >= 0 {
			t, ops, e := fillAlgebraicGraphPlanarFace(ctx, net, face, c.caps[face.Cap].regions.plane, o.Polygon, remaining)
			if e != nil {
				return zero, fmt.Errorf("clipped cap %d: %w", face.Cap, e)
			}
			out.Predicates += ops
			out.appendIndexedGraphTriangles(fi, t)
			continue
		}
		original := c.caps[0].recipe.stock.faces[face.StockFace]
		patch, e := exactFaceSupportPatch(original, o.Boundary.Section.Spans.Faces.Patches)
		if e != nil {
			return zero, e
		}
		d, e := faceUVBounds(original)
		if e != nil {
			return zero, e
		}
		patch, e = patch.Trim(d)
		if e != nil {
			return zero, e
		}
		plane, e := exactGraphStockFacePlane(ctx, patch, original.Reversed, o.Boundary.Section.Spans.Faces)
		if e == nil {
			t, ops, e := fillAlgebraicGraphPlanarFace(ctx, net, face, plane, o.Polygon, remaining)
			if e != nil {
				return zero, fmt.Errorf("planar stock face %d: %w", face.StockFace, e)
			}
			out.Predicates += ops
			out.appendIndexedGraphTriangles(fi, t)
			continue
		}
		if !errors.Is(e, geom.ErrUnrepresentable) && !errors.Is(e, geom.ErrConvergence) {
			return zero, e
		}
		var graph *geom.AffineProjectedGraph
		var axes [2]int
		for _, a := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
			for _, g := range [][2]int{a, {a[1], a[0]}} {
				h, work, e := geom.NewAffineProjectedGraphWithWork(ctx, patch, g, geom.SurfacePatchOptions{MaxOperations: budget.work, MaxCoefficientBits: budget.bits})
				if e := budget.charge(work, 0); e != nil {
					return zero, e
				}
				if e == nil && (h.Orientation() < 0) == original.Reversed {
					graph, axes = h, g
					break
				}
			}
			if graph != nil {
				break
			}
		}
		if graph == nil {
			return zero, fmt.Errorf("stock face %d affine chart: %w", face.StockFace, geom.ErrUnrepresentable)
		}
		t, ops, e := fillAlgebraicGraphProjectedFace(ctx, net, face, axes, nil, o.Polygon, remaining)
		if e != nil {
			return zero, fmt.Errorf("curved stock face %d projection: %w", face.StockFace, e)
		}
		out.Predicates += ops
		inv, e := graph.InverseCoefficients()
		if e != nil {
			return zero, e
		}
		// L1 bounds for the source Jacobian and affine inverse multiply to
		// an (intentionally conservative) projected-space Lipschitz bound.
		_, jac, ok := patch.RationalBezierDerivativeBounds(4)
		if !ok {
			return zero, geom.ErrUnrepresentable
		}
		m := new(big.Rat)
		for _, row := range inv {
			m.Add(m, new(big.Rat).Abs(row[0])).Add(m, new(big.Rat).Abs(row[1]))
		}
		b := new(big.Rat).Mul(jac, m)
		for _, tr := range t {
			var model [3][3]*big.Rat
			var uv [3][2]*big.Rat
			for i, id := range tr {
				model[i] = exactBoundPoint(out.Mesh.Vertices[id])
				target := [2]*big.Rat{model[i][axes[0]], model[i][axes[1]]}
				q, e := graph.InvertRational(ctx, target, geom.SurfacePatchEvaluationOptions{MaxOperations: budget.work, MaxCoefficientBits: budget.bits})
				if e != nil {
					return zero, e
				}
				if !q.Inside {
					return zero, fmt.Errorf("stock face %d projected sample outside chart: %w", face.StockFace, geom.ErrConvergence)
				}
				if e := budget.charge(q.Operations, 0); e != nil {
					return zero, e
				}
				uv[i] = q.UV
			}
			r, e := BoundRationalBezierTriangleDeviation(ctx, patch, uv, model, BezierTriangleBoundOptions{MaxOperations: min(budget.work, 100000000), MaxBits: budget.bits})
			if e != nil || !r.BothDirections {
				if e == nil {
					e = geom.ErrConvergence
				}
				return zero, e
			}
			if e := budget.charge(r.Operations, 0); e != nil {
				return zero, e
			}
			_, residual, e := graphAreaSqrt(r.SquaredDistance, 128, budget.bits)
			if e != nil {
				return zero, e
			}
			bound := new(big.Rat).Add(residual, new(big.Rat).Mul(b, ratInset(net.MaximumBoundaryError)))
			f, _ := bound.Float64()
			if ratInset(f).Cmp(bound) < 0 {
				f = math.Nextafter(f, math.Inf(1))
			}
			if !finite(f) || f > o.Deflection {
				return zero, fmt.Errorf("stock face %d residual %.5g: %w", face.StockFace, f, geom.ErrConvergence)
			}
			out.MaximumFillError = max(out.MaximumFillError, f)
		}
		out.appendIndexedGraphTriangles(fi, t)
	}
	if out.MaximumFillError > o.Deflection {
		return zero, geom.ErrConvergence
	}
	out.SignedVolume, _, e = checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if e != nil {
		return zero, e
	}
	return out, nil
}

func (o *CertifiedAlgebraicGraphPlanarMesh) appendIndexedGraphTriangles(face int, t [][3]int) {
	o.Mesh.Triangles = append(o.Mesh.Triangles, t...)
	for range t {
		o.IndexedFaces = append(o.IndexedFaces, face)
	}
}
