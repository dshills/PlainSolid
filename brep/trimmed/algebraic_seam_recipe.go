package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// VerifyAlgebraicSeamRecipe reproves the source plane or graph contact, rejects an
// endpoint or a different numeric root, and independently verifies both
// original model-space coedges before accepting an interior split.
func (s Solid) VerifyAlgebraicSeamRecipe(ctx context.Context, recipe AlgebraicSeamRecipe, o AlgebraicSeamOptions) (AlgebraicSeamProof, error) {
	var empty AlgebraicSeamProof
	if ctx == nil || recipe.Version < 1 || recipe.Version > 4 {
		return empty, geom.ErrInvalid
	}
	var plane [4]*big.Rat
	var graph *geom.SurfacePatch
	var affine *geom.ExactSplineCurve
	var paired *geom.ExactAlgebraicRoot
	if recipe.Version != 4 && (recipe.Projection != nil || recipe.GraphBounds != nil) ||
		recipe.Version == 1 && (len(recipe.Graph) != 0 || recipe.Affine != ([2][3]string{}) || recipe.Paired != nil) ||
		recipe.Version == 2 && (recipe.Plane != ([4]string{}) || recipe.Affine != ([2][3]string{}) || recipe.Paired != nil) ||
		recipe.Version == 3 && (recipe.Plane != ([4]string{}) || len(recipe.Graph) != 0 || recipe.Paired == nil) ||
		recipe.Version == 4 && (recipe.Plane != ([4]string{}) || recipe.Affine != ([2][3]string{}) || recipe.Paired != nil || recipe.Projection == nil || recipe.GraphBounds == nil) {
		return empty, geom.ErrInvalid
	}
	if recipe.Version == 2 || recipe.Version == 4 {
		if len(recipe.Graph) == 0 || len(recipe.Graph) > 4<<20 {
			return empty, geom.ErrInvalid
		}
		s, e := parametric.DecodeSurface(recipe.Graph, 4096)
		if e != nil {
			return empty, e
		}
		graph, e = exactFaceSupportPatch(Face{Surface: s}, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		if recipe.Version == 4 {
			graph, e = graph.Trim(*recipe.GraphBounds)
			if e != nil {
				return empty, e
			}
		}
	}
	if recipe.Version == 3 {
		net := make([][4]*big.Rat, 2)
		for i, row := range recipe.Affine {
			for j, raw := range row {
				x, e := parseAlgebraicSeamRational(raw)
				if e != nil {
					return empty, e
				}
				net[i][j] = x
			}
			net[i][3] = big.NewRat(1, 1)
		}
		var e error
		affine, e = geom.NewExactHomogeneousBezier(ctx, net, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		paired, e = geom.ImportExactRoot(ctx, *recipe.Paired, o.Root)
		if e != nil {
			return empty, e
		}
	}
	for i, raw := range recipe.Plane {
		if graph != nil || affine != nil {
			break
		}
		v, e := parseAlgebraicSeamRational(raw)
		if e != nil {
			return empty, e
		}
		plane[i] = v
	}
	root, e := geom.ImportExactRoot(ctx, recipe.Parameter, o.Root)
	if e != nil {
		return empty, e
	}
	model, owners, e := algebraicSeamSource(ctx, s, recipe.Edge, o.Composition)
	if e != nil {
		return empty, e
	}
	out := AlgebraicSeamProof{Edge: recipe.Edge, Faces: [2]int{owners[0].face, owners[1].face}, Parameter: root, PairedParameter: paired}
	find := func(owner algebraicSeamOwner) (geom.AlgebraicCurveEndpointBounds, error) {
		var empty geom.AlgebraicCurveEndpointBounds
		cells, e := owner.curve.ModelCurveCells(ctx, owner.patch, o.Composition)
		if e != nil {
			return empty, e
		}
		if !cells.Complete || len(cells.Cells) == 0 {
			return empty, geom.ErrConvergence
		}
		for i, cell := range cells.Cells {
			var event *geom.ExactComposedPlaneEvent
			if graph != nil && recipe.Version == 4 {
				event, e = bindSeamProjectedGraphRoot(ctx, cell, graph, *recipe.Projection, root, o)
			} else if graph != nil {
				event, e = bindSeamGraphRoot(ctx, cell, graph, root, o)
			} else if affine != nil {
				event, e = bindSeamAffineRoot(ctx, cell, affine, root, paired, o)
			} else {
				event, e = cell.BindPlaneRoot(ctx, plane, root, o.Root.ExactCurvePlaneEventOptions)
			}
			if errors.Is(e, geom.ErrDomain) {
				continue
			}
			if e != nil {
				return empty, e
			}
			end := event.EndpointMask()
			if graph != nil || affine != nil {
				bound, e := cell.BindRoot(ctx, root, o.Root.ExactCurvePlaneEventOptions)
				if e != nil {
					return empty, e
				}
				end = bound.EndpointMask()
			}
			if i == 0 && end&1 != 0 || i == len(cells.Cells)-1 && end&2 != 0 {
				return empty, geom.ErrDomain
			}
			return event.Enclose(ctx, o.Enclosure)
		}
		return empty, geom.ErrDomain
	}
	out.Model, e = find(model)
	if e != nil {
		return empty, e
	}
	for i, owner := range owners {
		trim, err := ValidateFaceTrimLoops(ctx, s.faces[owner.face], trimvalidate.Options{})
		if err != nil {
			return empty, err
		}
		if trim.Status != trimvalidate.Certified {
			return empty, geom.ErrConvergence
		}
		out.Owners[i], e = find(owner)
		if e != nil {
			return empty, e
		}
		if _, ok := out.Model.Bounds.Intersect(out.Owners[i].Bounds); !ok {
			return empty, geom.ErrConvergence
		}
		uv, e := owner.curve.modelParameterCurve(ctx, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		uvCell, e := geom.NewExactComposedBezierCell(ctx, uv, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		event, e := uvCell.BindRoot(ctx, root, o.Root.ExactCurvePlaneEventOptions)
		if e != nil {
			return empty, e
		}
		out.OwnerUV[i], e = event.Enclose(ctx, o.Enclosure)
		if e != nil {
			return empty, e
		}
	}
	return out, nil
}

func parseAlgebraicSeamRational(raw string) (*big.Rat, error) {
	if len(raw) == 0 || len(raw) > 2500 {
		return nil, geom.ErrInvalid
	}
	for _, c := range raw {
		if (c < '0' || c > '9') && c != '-' && c != '/' {
			return nil, geom.ErrInvalid
		}
	}
	v, ok := new(big.Rat).SetString(raw)
	if !ok || v.RatString() != raw || v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
		return nil, geom.ErrInvalid
	}
	return v, nil
}

func bindSeamAffineRoot(ctx context.Context, cell *geom.ExactComposedCurveCell, line *geom.ExactSplineCurve,
	root, paired *geom.ExactAlgebraicRoot, o AlgebraicSeamOptions) (*geom.ExactComposedPlaneEvent, error) {
	if _, e := cell.BindRoot(ctx, root, o.Root.ExactCurvePlaneEventOptions); e != nil {
		return nil, e
	}
	r, e := geom.IntersectExactComposedCurveAffineEdge(ctx, cell, line, geom.ComposedCurveLineOptions{ExactCurvePlaneEventOptions: o.Root.ExactCurvePlaneEventOptions})
	if e != nil {
		return nil, e
	}
	if !r.Complete {
		return nil, geom.ErrConvergence
	}
	for _, h := range r.Hits {
		cmp, e := root.Compare(ctx, h.CurveEvent.Root(), o.Root.ExactCurvePlaneEventOptions)
		if e != nil {
			return nil, e
		}
		if cmp.Proven && cmp.Order == 0 {
			q, e := paired.Compare(ctx, h.LineRoot, o.Root.ExactCurvePlaneEventOptions)
			if e != nil {
				return nil, e
			}
			if !q.Proven || q.Order != 0 {
				return nil, geom.ErrDomain
			}
			return h.CurveEvent, nil
		}
	}
	return nil, geom.ErrDomain
}

func bindSeamGraphRoot(ctx context.Context, cell *geom.ExactComposedCurveCell, graph *geom.SurfacePatch,
	root *geom.ExactAlgebraicRoot, o AlgebraicSeamOptions) (*geom.ExactComposedPlaneEvent, error) {
	if _, e := cell.BindRoot(ctx, root, o.Root.ExactCurvePlaneEventOptions); e != nil {
		return nil, e
	}
	r, e := geom.IntersectExactComposedCurveIdentityGraph(ctx, cell, graph, o.Composition)
	if e != nil {
		return nil, e
	}
	if !r.Complete {
		return nil, geom.ErrConvergence
	}
	for _, q := range r.Cells {
		if q.Intersection.Coincident {
			continue
		}
		for _, event := range q.Intersection.Events {
			cmp, e := root.Compare(ctx, event.Root(), o.Root.ExactCurvePlaneEventOptions)
			if e != nil {
				return nil, e
			}
			if cmp.Proven && cmp.Order == 0 {
				return event, nil
			}
		}
	}
	return nil, geom.ErrDomain
}

func bindSeamProjectedGraphRoot(ctx context.Context, cell *geom.ExactComposedCurveCell, graph *geom.SurfacePatch,
	axes [2]int, root *geom.ExactAlgebraicRoot, o AlgebraicSeamOptions) (*geom.ExactComposedCurveEvent, error) {
	if _, e := cell.BindRoot(ctx, root, o.Root.ExactCurvePlaneEventOptions); e != nil {
		return nil, e
	}
	r, e := geom.IntersectExactComposedCurveProjectedGraph(ctx, cell, graph, axes, o.Composition)
	if e != nil {
		return nil, e
	}
	if !r.Complete {
		return nil, geom.ErrConvergence
	}
	for _, c := range r.Cells {
		if c.Intersection.Coincident {
			continue
		}
		for _, event := range c.Intersection.Events {
			cmp, e := root.Compare(ctx, event.Root(), o.Root.ExactCurvePlaneEventOptions)
			if e != nil {
				return nil, e
			}
			if !cmp.Proven {
				return nil, geom.ErrConvergence
			}
			if cmp.Order == 0 && event.Multiplicity() > 0 {
				return event, nil
			}
		}
	}
	return nil, geom.ErrDomain
}

// AlgebraicSeamRecipe is a versioned split recipe in the original normalized
// parameter of one shared edge. A raw model plane (version 1), exact identity
// graph (version 2), finite affine edge (version 3), or bounded affine
// coordinate-plane graph (version 4) must reproduce Parameter on reload.
// Version 3 also reproves the affine edge's paired parameter.
type AlgebraicSeamRecipe struct {
	Version     int                      `json:"version"`
	Edge        int                      `json:"edge"`
	Plane       [4]string                `json:"plane"`
	Parameter   geom.ExactRootRecipe     `json:"parameter"`
	Graph       json.RawMessage          `json:"graph,omitempty"`
	Affine      [2][3]string             `json:"affine,omitempty"`
	Paired      *geom.ExactRootRecipe    `json:"paired_parameter,omitempty"`
	Projection  *[2]int                  `json:"projection,omitempty"`
	GraphBounds *geom.ParameterRectangle `json:"graph_bounds,omitempty"`
}

// PrepareAlgebraicAffineSeamRecipe stores an exact finite affine edge as the
// independent constraint. Its paired contact must rebind to the selected
// original seam and both owning charts.
func (s Solid) PrepareAlgebraicAffineSeamRecipe(ctx context.Context, edge int, affine [2][3]*big.Rat,
	event geom.ComposedAffineEdgeEvent, o AlgebraicSeamOptions) (AlgebraicSeamRecipe, AlgebraicSeamProof, error) {
	var empty AlgebraicSeamRecipe
	if ctx == nil || event.CurveEvent == nil || event.LineRoot == nil {
		return empty, AlgebraicSeamProof{}, geom.ErrInvalid
	}
	r := AlgebraicSeamRecipe{Version: 3, Edge: edge}
	for i, row := range affine {
		for j, v := range row {
			if v == nil {
				return empty, AlgebraicSeamProof{}, geom.ErrInvalid
			}
			r.Affine[i][j] = v.RatString()
		}
	}
	p, e := event.CurveEvent.RootRecipe(ctx, o.Root)
	if e != nil {
		return empty, AlgebraicSeamProof{}, e
	}
	r.Parameter = p
	q, e := event.LineRoot.Export(ctx, o.Root)
	if e != nil {
		return empty, AlgebraicSeamProof{}, e
	}
	r.Paired = &q
	proof, e := s.VerifyAlgebraicSeamRecipe(ctx, r, o)
	if e != nil {
		return empty, AlgebraicSeamProof{}, e
	}
	return r, proof, nil
}

// PrepareAlgebraicGraphSeamRecipe persists a graph contact against its full
// original support and independently rebinds it to both seam owner charts.
// The supplied graph is a support, not a trimmed face or a Boolean result.
func (s Solid) PrepareAlgebraicGraphSeamRecipe(ctx context.Context, edge int, graph geom.Surface,
	event *geom.ExactComposedPlaneEvent, o AlgebraicSeamOptions) (AlgebraicSeamRecipe, AlgebraicSeamProof, error) {
	var empty AlgebraicSeamRecipe
	if ctx == nil || event == nil || graph == nil {
		return empty, AlgebraicSeamProof{}, geom.ErrInvalid
	}
	raw, e := parametric.EncodeSurface(graph)
	if e != nil {
		return empty, AlgebraicSeamProof{}, e
	}
	root, e := event.RootRecipe(ctx, o.Root)
	if e != nil {
		return empty, AlgebraicSeamProof{}, e
	}
	recipe := AlgebraicSeamRecipe{Version: 2, Edge: edge, Graph: raw, Parameter: root}
	proof, e := s.VerifyAlgebraicSeamRecipe(ctx, recipe, o)
	if e != nil {
		return empty, AlgebraicSeamProof{}, e
	}
	return recipe, proof, nil
}

// AlgebraicSeamOptions bounds reconstruction and both charts' independent
// model composition. Tolerance in Enclosure bounds display error.
type AlgebraicSeamOptions struct {
	Composition geom.SurfaceCurveAgreementOptions
	Root        geom.ExactRootCodecOptions
	Enclosure   geom.ComposedCurveEndpointOptions
}

// AlgebraicSeamProof binds the split root to a selected edge and to both
// original owning spline charts, with an outward enclosure on each source.
// The parent edge and normalized parameter stay authoritative.
type AlgebraicSeamProof struct {
	Edge            int
	Faces           [2]int
	Parameter       *geom.ExactAlgebraicRoot
	PairedParameter *geom.ExactAlgebraicRoot
	Model           geom.AlgebraicCurveEndpointBounds
	Owners          [2]geom.AlgebraicCurveEndpointBounds
	OwnerUV         [2]geom.AlgebraicCurveEndpointBounds
}

// PrepareAlgebraicSeamRecipe takes an isolated contact reported by the exact
// raw-plane query, exports its numeric recipe, then independently reconstructs
// the event and both original owners before returning the persistent recipe.
func (s Solid) PrepareAlgebraicSeamRecipe(ctx context.Context, edge int, plane [4]*big.Rat,
	event *geom.ExactComposedPlaneEvent, o AlgebraicSeamOptions) (AlgebraicSeamRecipe, AlgebraicSeamProof, error) {
	var recipe AlgebraicSeamRecipe
	if ctx == nil || event == nil {
		return recipe, AlgebraicSeamProof{}, geom.ErrInvalid
	}
	recipe.Version, recipe.Edge = 1, edge
	for i, v := range plane {
		if v == nil {
			v = new(big.Rat)
		}
		recipe.Plane[i] = v.RatString()
	}
	r, e := event.RootRecipe(ctx, o.Root)
	if e != nil {
		return AlgebraicSeamRecipe{}, AlgebraicSeamProof{}, e
	}
	recipe.Parameter = r
	proof, e := s.VerifyAlgebraicSeamRecipe(ctx, recipe, o)
	if e != nil {
		return AlgebraicSeamRecipe{}, AlgebraicSeamProof{}, e
	}
	return recipe, proof, nil
}

type algebraicSeamOwner struct {
	face  int
	patch *geom.SurfacePatch
	curve ExactQuadraticUV // in canonical model-edge traversal
	use   Use
}

func algebraicSeamSource(ctx context.Context, s Solid, edge int, o geom.SurfaceCurveAgreementOptions) (algebraicSeamOwner, [2]algebraicSeamOwner, error) {
	var model algebraicSeamOwner
	var owners [2]algebraicSeamOwner
	if ctx == nil || edge < 0 || edge >= len(s.edges) {
		return model, owners, geom.ErrInvalid
	}
	m := s.edges[edge]
	c, ok := m.Curve.(surfaceTrimCurve)
	if !ok || m.Range != (parametric.Interval{Max: 1}) {
		return model, owners, geom.ErrConvergence
	}
	getCurve := func(p PCurve, a, b geom.Vec2) (ExactQuadraticUV, error) {
		switch q := p.(type) {
		case nil:
			// Express the original linear chart with exact degree elevation.
			// Its model evaluation and canonical parameter are unchanged.
			if !uvFinite(a) || !uvFinite(b) || a == b {
				return ExactQuadraticUV{}, geom.ErrInvalid
			}
			var h [3][3]*big.Rat
			for i := range h {
				h[i] = [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
			}
			for j, p := range [2][2]float64{{a.X, b.X}, {a.Y, b.Y}} {
				h[0][j], h[2][j] = ratInset(p[0]), ratInset(p[1])
				h[1][j].Quo(new(big.Rat).Add(h[0][j], h[2][j]), big.NewRat(2, 1))
			}
			return NewExactQuadraticUV(h)
		case ExactQuadraticUV:
			return q, nil
		case SplineUV:
			h, e := quadraticTrimSourceControls(q)
			if e != nil {
				return ExactQuadraticUV{}, e
			}
			return NewExactQuadraticUV(h)
		}
		return ExactQuadraticUV{}, geom.ErrConvergence
	}
	var e error
	model.curve, e = getCurve(c.curve, c.a, c.b)
	if e != nil {
		return model, owners, e
	}
	model.patch, e = exactFaceSupportPatch(Face{Surface: c.surface}, o.SurfacePatchOptions)
	if e != nil {
		return model, owners, e
	}
	n := 0
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return model, owners, e
		}
		for _, ring := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range ring {
				if u.Edge != edge {
					continue
				}
				if n == 2 || n == 1 && owners[0].face == i {
					return model, owners, geom.ErrConvergence
				}
				check := compareSplineRationalTrimUse
				if c.curve == nil && u.Curve == nil {
					check = compareSplineEdgeUse
				}
				proof, e := check(ctx, m, f.Surface, u, edgecheck.Options{
					MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits, MaxIntervals: o.MaxEvents,
				})
				if e != nil || !proof {
					if e == nil {
						e = geom.ErrConvergence
					}
					return model, owners, e
				}
				q, e := getCurve(u.Curve, u.UVStart, u.UVEnd)
				if e != nil {
					return model, owners, e
				}
				if u.Reversed {
					q = q.reversed()
				}
				p, e := exactFaceSupportPatch(f, o.SurfacePatchOptions)
				if e != nil {
					return model, owners, e
				}
				owners[n] = algebraicSeamOwner{face: i, patch: p, curve: q, use: u}
				n++
			}
		}
	}
	if n != 2 {
		return model, owners, geom.ErrConvergence
	}
	return model, owners, nil
}
