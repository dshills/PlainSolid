package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SubdivideOriginalEdges transfers every proved open terminal to both coedges
// of its original shell edge. An opposing face must supply an exact affine
// plane or an affine coordinate-plane graph as an independent constraint
// for its persistent seam recipe. The original face supports and loops are
// retained, with new indexed subedges; this does not select Boolean fragments.
// Both results are returned only after their complete incidence is checked.
func (r AlgebraicShellSectionResult) SubdivideOriginalEdges(ctx context.Context, o AlgebraicSeamOptions) (result [2]AlgebraicSubdivision, err error) {
	if ctx == nil || r.source == nil {
		return result, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			result = [2]AlgebraicSubdivision{}
		}
	}()
	if e := ctx.Err(); e != nil {
		return result, e
	}
	shells := [2]Solid{r.source.a, r.source.b}
	type cut struct {
		recipe AlgebraicSeamRecipe
		root   *geom.ExactAlgebraicRoot
	}
	var cuts [2][]cut
	constraints := map[[2]int]algebraicShellSeamConstraint{}
	for _, b := range r.source.branches {
		if b.Closed {
			continue
		}
		for end, t := range b.Ends {
			if e := ctx.Err(); e != nil {
				return result, e
			}
			op, edge, root := t.Operand, b.EndEdges[end], b.EdgeParameters[end]
			if root == nil || t.Certificate.ModelEvent == nil {
				return result, geom.ErrConvergence
			}
			found := false
			for _, c := range cuts[op] {
				if c.recipe.Edge != edge {
					continue
				}
				q, e := root.Compare(ctx, c.root, o.Root.ExactCurvePlaneEventOptions)
				if e != nil {
					return result, e
				}
				if !q.Proven {
					return result, geom.ErrConvergence
				}
				if q.Order == 0 {
					found = true
					break
				}
			}
			if found {
				continue
			}
			if len(cuts[op]) >= 32 {
				return result, geom.ErrConvergence
			}
			owner := [2]int{op ^ 1, b.Faces[op^1]}
			constraint, ok := constraints[owner]
			if !ok {
				f := shells[owner[0]].faces[owner[1]]
				p, e := exactFaceSupportPatch(f, o.Composition.SurfacePatchOptions)
				if e != nil {
					return result, e
				}
				d, e := faceUVBounds(f)
				if e != nil {
					return result, e
				}
				p, e = p.Trim(d)
				if e != nil {
					return result, e
				}
				constraint, e = newAlgebraicShellSeamConstraint(ctx, f, p, o.Composition)
				if e != nil {
					return result, e
				}
				constraints[owner] = constraint
			}
			recipe, e := shellSectionSeamRecipe(ctx, shells[op], edge, root, t.Certificate.ModelEvent, constraint, o)
			if e != nil {
				return result, e
			}
			cuts[op] = append(cuts[op], cut{recipe, root})
		}
	}
	for i, s := range shells {
		recipes := make([]AlgebraicSeamRecipe, len(cuts[i]))
		for j, c := range cuts[i] {
			recipes[j] = c.recipe
		}
		result[i], err = s.subdivideAlgebraicSeams(ctx, recipes, o)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

type algebraicShellSeamConstraint struct {
	plane [4]*big.Rat
	graph *geom.SurfacePatch
	raw   json.RawMessage
	axes  [2]int
}

func newAlgebraicShellSeamConstraint(ctx context.Context, f Face, p *geom.SurfacePatch,
	o geom.SurfaceCurveAgreementOptions) (out algebraicShellSeamConstraint, err error) {
	plane, e := exactAffineSectionPlane(ctx, p, o)
	if e == nil {
		out.plane = plane
		return out, nil
	}
	if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) {
		return out, e
	}
	for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		_, e := geom.NewAffineProjectedGraph(ctx, p, axes, o.SurfacePatchOptions)
		if e == nil {
			raw, e := parametric.EncodeSurface(f.Surface)
			if e != nil {
				return out, e
			}
			return algebraicShellSeamConstraint{graph: p, raw: raw, axes: axes}, nil
		}
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) {
			return out, e
		}
	}
	return out, geom.ErrConvergence
}

func exactAffineSectionPlane(ctx context.Context, p *geom.SurfacePatch, o geom.SurfaceCurveAgreementOptions) (plane [4]*big.Rat, err error) {
	c, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return plane, e
	}
	d := c.Domain()
	var q [3][3]*big.Rat
	for i, uv := range [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}} {
		q[i], e = c.EvalExact(ratInset(uv[0]), ratInset(uv[1]))
		if e != nil {
			return plane, e
		}
	}
	var u, v [3]*big.Rat
	for i := range u {
		u[i] = new(big.Rat).Sub(q[1][i], q[0][i])
		v[i] = new(big.Rat).Sub(q[2][i], q[0][i])
	}
	plane[3] = new(big.Rat)
	for i := range u {
		j, k := (i+1)%3, (i+2)%3
		plane[i] = new(big.Rat).Sub(new(big.Rat).Mul(u[j], v[k]), new(big.Rat).Mul(u[k], v[j]))
		plane[3].Sub(plane[3], new(big.Rat).Mul(plane[i], q[0][i]))
	}
	return plane, nil
}

func shellSectionSeamRecipe(ctx context.Context, s Solid, edge int, root *geom.ExactAlgebraicRoot,
	point *geom.ExactComposedCurveEvent, constraint algebraicShellSeamConstraint, o AlgebraicSeamOptions) (AlgebraicSeamRecipe, error) {
	var empty AlgebraicSeamRecipe
	m, _, e := algebraicSeamSource(ctx, s, edge, o.Composition)
	if e != nil {
		return empty, e
	}
	cells, e := m.curve.ModelCurveCells(ctx, m.patch, o.Composition)
	if e != nil {
		return empty, e
	}
	if !cells.Complete {
		return empty, geom.ErrConvergence
	}
	for _, c := range cells.Cells {
		var v *geom.ExactComposedCurveEvent
		if constraint.graph == nil {
			v, e = c.BindPlaneRoot(ctx, constraint.plane, root, o.Root.ExactCurvePlaneEventOptions)
		} else {
			v, e = bindSeamProjectedGraphRoot(ctx, c, constraint.graph, constraint.axes, root, o)
		}
		if errors.Is(e, geom.ErrDomain) {
			continue
		}
		if e != nil {
			return empty, e
		}
		match, _, e := v.CompareModelPoint(ctx, point, o.Root.ExactCurvePlaneEventOptions)
		if e != nil {
			return empty, e
		}
		if !match {
			return empty, geom.ErrConvergence
		}
		out := AlgebraicSeamRecipe{Version: 1, Edge: edge}
		if constraint.graph == nil {
			for i, x := range constraint.plane {
				out.Plane[i] = x.RatString()
			}
		} else {
			d := constraint.graph.Domain()
			out.Version, out.Graph = 4, constraint.raw
			out.Projection, out.GraphBounds = &constraint.axes, &d
		}
		out.Parameter, e = v.RootRecipe(ctx, o.Root)
		return out, e
	}
	return empty, geom.ErrConvergence
}
