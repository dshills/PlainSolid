package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// AffineSplineShellBooleanOptions bounds exact chart proofs and planar Boolean
// work. The planar output conversion always requires exact representation.
type AffineSplineShellBooleanOptions struct {
	Patches geom.SurfacePatchOptions
	Charts  geom.AffineSurfaceChartOptions
	Edges   edgecheck.Options
	Planar  poly.Options
}

// PlanarBoundaryFromAffineSplines proves that all faces have affine spline
// support, straight exact UV edges and vertices, and an embedded planar
// boundary with consistent shell orientation. It returns the same indexed
// geometry as planar polygons and preserves holes and cavity shells.
func PlanarBoundaryFromAffineSplines(ctx context.Context, s Solid, o AffineSplineShellBooleanOptions) (brep.Solid, error) {
	if ctx == nil {
		return brep.Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return brep.Solid{}, e
	}
	r, _, e := affineSplinePlanarBoundary(ctx, s, o)
	if e == nil {
		e = poly.ValidateEmbeddedBoundary(ctx, r, o.Planar)
	}
	if e != nil {
		return brep.Solid{}, e
	}
	return r, nil
}

// CombineAffineSplineShells combines embedded spline solids whose faces all
// have provably affine charts with straight UV boundary uses. Polygonal trims,
// holes and multiple shells are accepted when the planar kernel proves
// embedding and nesting. It preserves each retained original support and
// exactly inverts its output UV coordinates, so eligible results can be used
// as operands in subsequent calls. Nonaffine faces are rejected.
func CombineAffineSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o AffineSplineShellBooleanOptions) (Solid, SplineShellCombineHistory, error) {
	var h SplineShellCombineHistory
	if ctx == nil || op > parametric.Difference {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	pa, ac, e := affineSplinePlanarBoundary(ctx, a, o)
	if e != nil {
		return Solid{}, h, e
	}
	pb, bc, e := affineSplinePlanarBoundary(ctx, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	o.Planar.RequireExactConversion = true
	r, history, e := poly.CombineWithTopologyHistory(ctx, pa, pb, poly.Operation(op), o.Planar)
	if e != nil {
		return Solid{}, h, e
	}
	h.Method = "affine-planar-arrangement"
	h.PlanarEdges = history.Edges
	h.FacesA, h.FacesB = make([][]int, len(a.faces)), make([][]int, len(b.faces))
	h.EdgesA, h.EdgesB = make([][]int, len(a.edges)), make([][]int, len(b.edges))
	charts := [2][]*geom.AffineSurfaceChart{ac, bc}
	inputs := [2]Solid{a, b}
	faceRows := [2][][]int{h.FacesA, h.FacesB}
	edgeRows := [2][][]int{h.EdgesA, h.EdgesB}
	result, e := liftAffinePlanarBoundary(ctx, r, max(a.tolerance, b.tolerance), o,
		func(i int, _ []geom.Vec3) (Face, *geom.AffineSurfaceChart, error) {
			src := history.Faces[i]
			operand, face := int(src.Operand), src.Face
			faceRows[operand][face] = append(faceRows[operand][face], i)
			original := inputs[operand].faces[face]
			return Face{Surface: original.Surface, Reversed: original.Reversed != src.Reversed}, charts[operand][face], nil
		})
	if e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	for i, entry := range history.Edges {
		seen := map[[2]int]bool{}
		for _, overlap := range entry.Overlaps {
			key := [2]int{int(overlap.Operand), overlap.Edge}
			if !seen[key] {
				edgeRows[key[0]][key[1]] = append(edgeRows[key[0]][key[1]], i)
				seen[key] = true
			}
		}
		if entry.HasGeneratedPortion {
			h.GeneratedEdges = append(h.GeneratedEdges, i)
		}
	}
	return result, h, nil
}

type affineFaceSource func(face int, corners []geom.Vec3) (Face, *geom.AffineSurfaceChart, error)

func liftAffinePlanarBoundary(ctx context.Context, r brep.Solid, tolerance float64, o AffineSplineShellBooleanOptions, source affineFaceSource) (Solid, error) {
	vertices, modelEdges, boundary := r.Vertices(), r.Edges(), r.Faces()
	if len(boundary) == 0 {
		return Solid{}, nil
	}
	edges := make([]parametric.Edge, len(modelEdges))
	faces := make([]Face, len(boundary))
	for i, f := range boundary {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		corners := make([]geom.Vec3, len(f.Boundary))
		for j, u := range f.Boundary {
			v := modelEdges[u.Edge].Start
			if u.Reversed {
				v = modelEdges[u.Edge].End
			}
			corners[j] = vertices[v]
		}
		out, inverse, e := source(i, corners)
		if e != nil {
			return Solid{}, e
		}
		uv := map[int]geom.Vec2{}
		at := func(id int) (geom.Vec2, error) {
			if q, ok := uv[id]; ok {
				return q, nil
			}
			p := vertices[id]
			r, e := inverse.InvertExactContext(ctx, [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)},
				geom.AffineChartInverseOptions{MaxOperations: o.Charts.MaxOperations, MaxCoefficientBits: o.Charts.MaxCoefficientBits})
			if e != nil || !r.OnPlane || !r.Inside {
				if e == nil {
					e = geom.ErrConvergence
				}
				return geom.Vec2{}, e
			}
			q, e := r.RepresentedUV()
			if e == nil {
				uv[id] = q
			}
			return q, e
		}
		for k, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			lifted := make([]Use, len(loop))
			for j, use := range loop {
				if out.Reversed {
					use = loop[len(loop)-1-j]
					use.Reversed = !use.Reversed
				}
				id := use.Edge
				me := modelEdges[id]
				p, e := at(me.Start)
				if e != nil {
					return Solid{}, e
				}
				q, e := at(me.End)
				if e != nil {
					return Solid{}, e
				}
				if edges[id].Curve == nil {
					edges[id] = parametric.Edge{Start: me.Start, End: me.End,
						Curve: surfaceTrimCurve{out.Surface, p, q, nil}, Range: parametric.Interval{Max: 1}}
				}
				if use.Reversed {
					p, q = q, p
				}
				lifted[j] = Use{Edge: id, Reversed: use.Reversed, UVStart: p, UVEnd: q}
			}
			if k == 0 {
				out.Outer = lifted
			} else {
				out.Holes = append(out.Holes, lifted)
			}
		}
		faces[i] = out
	}
	result, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, e
	}
	proof, e := result.ValidateEdgeAgreement(ctx, o.Edges)
	if e != nil || !proof.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, e
	}
	return result, nil
}

func affineSplinePlanarBoundary(ctx context.Context, s Solid, o AffineSplineShellBooleanOptions) (brep.Solid, []*geom.AffineSurfaceChart, error) {
	charts := make([]*geom.AffineSurfaceChart, len(s.faces))
	faces := make([]brep.Face, len(s.faces))
	edges := make([]brep.Edge, len(s.edges))
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return brep.Solid{}, nil, e
		}
		base, m := parametric.SurfaceMapping(f.Surface)
		bs, ok := base.(*geom.BSplineSurface)
		if !ok {
			return brep.Solid{}, nil, geom.ErrConvergence
		}
		p, e := geom.NewSurfacePatch(bs, o.Patches)
		if e != nil {
			return brep.Solid{}, nil, e
		}
		d, e := faceUVBounds(f)
		if e != nil {
			return brep.Solid{}, nil, e
		}
		p, e = p.Trim(d)
		if e != nil {
			return brep.Solid{}, nil, e
		}
		if m != geom.Identity() {
			p, e = p.Transformed(m)
			if e != nil {
				return brep.Solid{}, nil, e
			}
		}
		c, e := geom.NewAffineSurfaceChart(ctx, p, o.Charts)
		if e != nil {
			if errors.Is(e, geom.ErrDegenerate) {
				e = geom.ErrConvergence
			}
			return brep.Solid{}, nil, e
		}
		charts[i] = c
		for j, loop := range append([][]Use{f.Outer}, f.Holes...) {
			out := make([]brep.EdgeUse, 0, len(loop))
			for k, u := range loop {
				if u.Curve != nil {
					return brep.Solid{}, nil, geom.ErrConvergence
				}
				v := s.edges[u.Edge].Start
				if u.Reversed {
					v = s.edges[u.Edge].End
				}
				x, e := c.EvalExact(ratInset(u.UVStart.X), ratInset(u.UVStart.Y))
				if e != nil {
					return brep.Solid{}, nil, e
				}
				q, e := geom.RepresentedExactPoint(x)
				if e != nil || q != s.vertices[v] {
					return brep.Solid{}, nil, geom.ErrUnrepresentable
				}
				if f.Reversed {
					u = loop[len(loop)-1-k]
					u.Reversed = !u.Reversed
				}
				out = append(out, brep.EdgeUse{Edge: u.Edge, Reversed: u.Reversed})
			}
			if j == 0 {
				faces[i].Boundary = out
			} else {
				faces[i].Holes = append(faces[i].Holes, out)
			}
		}
	}
	proof, e := s.ValidateEdgeAgreement(ctx, o.Edges)
	if e != nil || !proof.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return brep.Solid{}, nil, e
	}
	for i, e := range s.edges {
		edges[i] = brep.Edge{Start: e.Start, End: e.End}
	}
	planar, e := brep.NewSolid(s.vertices, edges, faces)
	if e != nil {
		return brep.Solid{}, nil, fmt.Errorf("trimmed: affine spline boundary: %w", e)
	}
	return planar, charts, nil
}
