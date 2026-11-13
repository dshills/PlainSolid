package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicSubdivision is an exact seam subdivision of an original trimmed
// solid. Parent supports and trim geometry are unchanged. The result owns
// its recipes and topology; it is separate from float-only Solid edits.
type AlgebraicSubdivision struct {
	source   Solid
	recipes  []AlgebraicSeamRecipe
	vertices []AlgebraicSubdivisionVertex
	edges    []AlgebraicSubdivisionEdge
	faces    []AlgebraicSubdivisionFace
}

type AlgebraicSubdivisionVertex struct {
	Original, Recipe int // -1 when absent
	Display          geom.Vec3
	ErrorUpper       float64 // geometric enclosure error for an inserted vertex
}

// AlgebraicSubdivisionEdge uses the increasing original normalized parameter.
// An endpoint root is nil when it is the original rational endpoint 0 or 1.
type AlgebraicSubdivisionEdge struct {
	Original, Start, End int
	Lower, Upper         *geom.ExactAlgebraicRoot
}

// AlgebraicSubdivisionUse maps a subdivided topological edge back to one
// original face loop use. Reversed is the original incidence orientation.
type AlgebraicSubdivisionUse struct {
	Edge, OriginalUse int
	Reversed          bool
}

// AlgebraicSubdivisionChart retains both exact local chart domains for a
// coedge. UV and Model use increasing original model-edge parameter; Reverse
// tells callers when to traverse them in decreasing order for the face loop.
type AlgebraicSubdivisionChart struct {
	UV      *geom.ExactComposedCurveCell
	Model   geom.SurfaceCurveCompositionCells
	Reverse bool
}

type AlgebraicSubdivisionFace struct {
	Original int
	Outer    []AlgebraicSubdivisionUse
	Holes    [][]AlgebraicSubdivisionUse
	Reversed bool
}

// AlgebraicSubdivisionTopology counts the indexed arrangement after the
// seam splits. It reports incidence and Euler characteristic only; the
// original boundary's embedding proof, if any, remains separate.
type AlgebraicSubdivisionTopology struct {
	Vertices, Edges, Faces, InnerWires, CollapsedEdges, EulerCharacteristic int
}

func (s AlgebraicSubdivision) Topology() AlgebraicSubdivisionTopology {
	r := AlgebraicSubdivisionTopology{Vertices: len(s.vertices), Edges: len(s.edges), Faces: len(s.faces)}
	for _, f := range s.faces {
		r.InnerWires += len(f.Holes)
	}
	for _, e := range s.edges {
		if s.source.edges[e.Original].Curve == nil {
			r.CollapsedEdges++
		}
	}
	r.EulerCharacteristic = r.Vertices - r.Edges + r.Faces - r.InnerWires + r.CollapsedEdges
	return r
}

func (s AlgebraicSubdivision) Source() Solid { return s.source }
func (s AlgebraicSubdivision) Vertices() []AlgebraicSubdivisionVertex {
	return append([]AlgebraicSubdivisionVertex(nil), s.vertices...)
}
func (s AlgebraicSubdivision) Edges() []AlgebraicSubdivisionEdge {
	return append([]AlgebraicSubdivisionEdge(nil), s.edges...)
}
func (s AlgebraicSubdivision) Faces() []AlgebraicSubdivisionFace {
	out := make([]AlgebraicSubdivisionFace, len(s.faces))
	for i, f := range s.faces {
		out[i] = f
		out[i].Outer = append([]AlgebraicSubdivisionUse(nil), f.Outer...)
		out[i].Holes = make([][]AlgebraicSubdivisionUse, len(f.Holes))
		for j, h := range f.Holes {
			out[i].Holes[j] = append([]AlgebraicSubdivisionUse(nil), h...)
		}
	}
	return out
}
func (s AlgebraicSubdivision) Recipes() []AlgebraicSeamRecipe {
	out := append([]AlgebraicSeamRecipe(nil), s.recipes...)
	for i := range out {
		out[i] = cloneAlgebraicSeamRecipe(out[i])
	}
	return out
}

func cloneAlgebraicSeamRecipe(r AlgebraicSeamRecipe) AlgebraicSeamRecipe {
	r.Parameter.Coefficients = append([]string(nil), r.Parameter.Coefficients...)
	r.Graph = append([]byte(nil), r.Graph...)
	if r.Projection != nil {
		p := *r.Projection
		r.Projection = &p
	}
	if r.GraphBounds != nil {
		p := *r.GraphBounds
		r.GraphBounds = &p
	}
	if r.Paired != nil {
		p := *r.Paired
		p.Coefficients = append([]string(nil), p.Coefficients...)
		r.Paired = &p
	}
	return r
}

// SubdivideAlgebraicSeams verifies up to 32 independent source-plane recipes,
// orders roots exactly on each edge, and rewrites both coedge loops. Duplicate
// roots and rounded display collisions do not weld distinct topology. The
// returned result preserves the entire original boundary as exact subdomains.
func (s Solid) SubdivideAlgebraicSeams(ctx context.Context, recipes []AlgebraicSeamRecipe, o AlgebraicSeamOptions) (AlgebraicSubdivision, error) {
	if len(recipes) == 0 {
		return AlgebraicSubdivision{}, geom.ErrInvalid
	}
	return s.subdivideAlgebraicSeams(ctx, recipes, o)
}

func (s Solid) subdivideAlgebraicSeams(ctx context.Context, recipes []AlgebraicSeamRecipe, o AlgebraicSeamOptions) (AlgebraicSubdivision, error) {
	var empty AlgebraicSubdivision
	if ctx == nil || len(recipes) > 32 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	type cut struct {
		id    int
		proof AlgebraicSeamProof
	}
	byEdge := make(map[int][]cut)
	out := AlgebraicSubdivision{source: s, recipes: append([]AlgebraicSeamRecipe(nil), recipes...)}
	for i, p := range s.vertices {
		out.vertices = append(out.vertices, AlgebraicSubdivisionVertex{Original: i, Recipe: -1, Display: p})
	}
	for i, recipe := range recipes {
		p, e := s.VerifyAlgebraicSeamRecipe(ctx, recipe, o)
		if e != nil {
			return empty, e
		}
		if !p.Model.Converged || !p.Model.HasRepresentative {
			return empty, geom.ErrConvergence
		}
		out.recipes[i] = cloneAlgebraicSeamRecipe(recipe)
		a := byEdge[recipe.Edge]
		pos := len(a)
		for pos > 0 {
			cmp, e := a[pos-1].proof.Parameter.Compare(ctx, p.Parameter, o.Root.ExactCurvePlaneEventOptions)
			if e != nil {
				return empty, e
			}
			if !cmp.Proven {
				return empty, geom.ErrConvergence
			}
			if cmp.Order == 0 {
				return empty, geom.ErrInvalid
			}
			if cmp.Order < 0 {
				break
			}
			pos--
		}
		a = append(a, cut{})
		copy(a[pos+1:], a[pos:])
		a[pos] = cut{id: i, proof: p}
		byEdge[recipe.Edge] = a
	}
	ranges := make([][2]int, len(s.edges))
	for i, e := range s.edges {
		ranges[i][0] = len(out.edges)
		start := e.Start
		var lower *geom.ExactAlgebraicRoot
		for _, c := range byEdge[i] {
			v := len(out.vertices)
			p := c.proof.Model
			out.vertices = append(out.vertices, AlgebraicSubdivisionVertex{Original: -1, Recipe: c.id, Display: p.Point, ErrorUpper: p.ErrorUpper})
			out.edges = append(out.edges, AlgebraicSubdivisionEdge{i, start, v, lower, c.proof.Parameter})
			start, lower = v, c.proof.Parameter
		}
		out.edges = append(out.edges, AlgebraicSubdivisionEdge{i, start, e.End, lower, nil})
		ranges[i][1] = len(out.edges)
	}
	loop := func(in []Use) []AlgebraicSubdivisionUse {
		var out []AlgebraicSubdivisionUse
		for i, u := range in {
			a, b := ranges[u.Edge][0], ranges[u.Edge][1]
			for j := a; j < b; j++ {
				id := j
				if u.Reversed {
					id = b - 1 - (j - a)
				}
				out = append(out, AlgebraicSubdivisionUse{id, i, u.Reversed})
			}
		}
		return out
	}
	for i, f := range s.faces {
		q := AlgebraicSubdivisionFace{Original: i, Outer: loop(f.Outer), Reversed: f.Reversed}
		for _, h := range f.Holes {
			q.Holes = append(q.Holes, loop(h))
		}
		out.faces = append(out.faces, q)
	}
	if e := out.checkIncidence(ctx); e != nil {
		return empty, e
	}
	return out, nil
}

func (s AlgebraicSubdivision) checkIncidence(ctx context.Context) error {
	count, orientation := make([]int, len(s.edges)), make([]int, len(s.edges))
	for _, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return e
		}
		for _, ring := range append([][]AlgebraicSubdivisionUse{f.Outer}, f.Holes...) {
			start, end := -1, -1
			for _, u := range ring {
				x := s.edges[u.Edge]
				a, b := x.Start, x.End
				if u.Reversed {
					a, b = b, a
				}
				if start == -1 {
					start = a
				} else if end != a {
					return geom.ErrConvergence
				}
				end = b
				count[u.Edge]++
				if u.Reversed == f.Reversed {
					orientation[u.Edge]++
				} else {
					orientation[u.Edge]--
				}
			}
			if start != end {
				return geom.ErrConvergence
			}
		}
	}
	for i, e := range s.edges {
		if s.source.edges[e.Original].Curve == nil {
			if count[i] != 1 {
				return geom.ErrConvergence
			}
		} else if count[i] != 2 || orientation[i] != 0 {
			return geom.ErrConvergence
		}
	}
	return nil
}

// ParameterRange gives outward exact rational endpoint intervals in the
// parent edge's normalized parameter. The intervals are not domain cuts.
func (e AlgebraicSubdivisionEdge) ParameterRange() [2]geom.ExactParameterInterval {
	var out [2]geom.ExactParameterInterval
	for i, r := range [2]*geom.ExactAlgebraicRoot{e.Lower, e.Upper} {
		if r != nil {
			out[i] = r.Interval()
		} else {
			out[i] = geom.ExactParameterInterval{Lower: big.NewRat(int64(i), 1), Upper: big.NewRat(int64(i), 1)}
		}
	}
	return out
}

// OriginalModelEdge returns the original edge evaluator and range. The
// algebraic limits in Edges further restrict the returned original edge.
func (s AlgebraicSubdivision) OriginalModelEdge(edge int) (parametric.Edge, error) {
	if edge < 0 || edge >= len(s.edges) {
		return parametric.Edge{}, geom.ErrInvalid
	}
	return s.source.edges[s.edges[edge].Original], nil
}

// ModelCurveCells proves the original spline edge restricted to one exact
// subdivided domain. Distinct source knot cells stay separate.
func (s AlgebraicSubdivision) ModelCurveCells(ctx context.Context, edge int, o AlgebraicSeamOptions) (geom.SurfaceCurveCompositionCells, error) {
	var empty geom.SurfaceCurveCompositionCells
	if ctx == nil || edge < 0 || edge >= len(s.edges) {
		return empty, geom.ErrInvalid
	}
	x := s.edges[edge]
	model, _, e := algebraicSeamSource(ctx, s.source, x.Original, o.Composition)
	if e != nil {
		return empty, e
	}
	c, e := model.curve.ModelCurveCells(ctx, model.patch, o.Composition)
	if e != nil {
		return empty, e
	}
	if !c.Complete || len(c.Cells) == 0 {
		return empty, geom.ErrConvergence
	}
	return restrictSubdivisionCells(ctx, c, x, o.Root.ExactCurvePlaneEventOptions)
}

// TessellateModelEdgeCertified certifies a display for an exact subdivided
// model edge. A collapsed display is reported independently of the exact
// topology. Optional endpoint displays are accepted only after a new proof.
func (s AlgebraicSubdivision) TessellateModelEdgeCertified(ctx context.Context, edge int, o AlgebraicSeamOptions,
	display geom.ComposedCollectionPolylineOptions) (geom.CertifiedComposedCollectionPolyline, error) {
	c, e := s.ModelCurveCells(ctx, edge, o)
	if e != nil {
		return geom.CertifiedComposedCollectionPolyline{}, e
	}
	if display.JunctionPoints == nil && display.EndpointPoints == nil {
		x := s.edges[edge]
		display.EndpointPoints = &[2]geom.Vec3{s.vertices[x.Start].Display, s.vertices[x.End].Display}
	}
	return c.TessellateCertified(ctx, display)
}

// CoedgeChart independently rederives the exact chart and surface restriction
// for an original owning face of a subdivided spline model edge.
func (s AlgebraicSubdivision) CoedgeChart(ctx context.Context, edge, face int, o AlgebraicSeamOptions) (AlgebraicSubdivisionChart, error) {
	var empty AlgebraicSubdivisionChart
	if ctx == nil || edge < 0 || edge >= len(s.edges) || face < 0 || face >= len(s.faces) {
		return empty, geom.ErrInvalid
	}
	x := s.edges[edge]
	_, owners, e := algebraicSeamSource(ctx, s.source, x.Original, o.Composition)
	if e != nil {
		return empty, e
	}
	for _, owner := range owners {
		if owner.face != face {
			continue
		}
		curve, e := owner.curve.modelParameterCurve(ctx, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		uv, e := geom.NewExactComposedBezierCell(ctx, curve, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		options := o.Root.ExactCurvePlaneEventOptions
		lo, hi := x.Lower, x.Upper
		if lo == nil {
			lo, e = uv.EndpointRoot(0)
			if e != nil {
				return empty, e
			}
		}
		if hi == nil {
			hi, e = uv.EndpointRoot(1)
			if e != nil {
				return empty, e
			}
		}
		cut, e := uv.RestrictAlgebraic(ctx, lo, hi, options)
		if e != nil {
			return empty, e
		}
		if !cut.Proven || cut.Cell == nil {
			return empty, geom.ErrConvergence
		}
		model, e := owner.curve.ModelCurveCells(ctx, owner.patch, o.Composition)
		if e != nil {
			return empty, e
		}
		if !model.Complete || len(model.Cells) == 0 {
			return empty, geom.ErrConvergence
		}
		model, e = restrictSubdivisionCells(ctx, model, x, options)
		if e != nil {
			return empty, e
		}
		return AlgebraicSubdivisionChart{UV: cut.Cell, Model: model, Reverse: owner.use.Reversed}, nil
	}
	return empty, geom.ErrDomain
}

func restrictSubdivisionCells(ctx context.Context, c geom.SurfaceCurveCompositionCells, x AlgebraicSubdivisionEdge, o geom.ExactCurvePlaneEventOptions) (geom.SurfaceCurveCompositionCells, error) {
	lo, hi := x.Lower, x.Upper
	var e error
	if lo == nil {
		lo, e = c.Cells[0].EndpointRoot(0)
		if e != nil {
			return geom.SurfaceCurveCompositionCells{}, e
		}
	}
	if hi == nil {
		hi, e = c.Cells[len(c.Cells)-1].EndpointRoot(1)
		if e != nil {
			return geom.SurfaceCurveCompositionCells{}, e
		}
	}
	return c.RestrictAlgebraic(ctx, lo, hi, o)
}
