package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativeAlgebraicBooleanOptions bounds reconstruction of a Boolean whose
// exact junctions and source-edge subranges have binary64 representations.
// The implicit section interiors remain original paired tube evaluators.
type NativeAlgebraicBooleanOptions struct {
	Exact    geom.ExactCurvePlaneEventOptions
	Edge     edgecheck.Options
	MaxEdges int
}

func combineSplineShellsAlgebraicNative(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o SplineShellAlgebraicNativeOptions) (Solid, SplineShellCombineHistory, error) {
	var empty SplineShellCombineHistory
	if !finite(o.Tolerance) || o.Tolerance < 0 {
		return Solid{}, empty, geom.ErrInvalid
	}
	if o.Tolerance == 0 {
		o.Tolerance = max(a.tolerance, b.tolerance)
		if o.Tolerance == 0 {
			o.Tolerance = 1e-8
		}
	}
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o.Section)
	if e != nil {
		return Solid{}, empty, e
	}
	s, e := r.CombineAlgebraic(ctx, op, o.Boolean)
	if e != nil {
		return Solid{}, empty, e
	}
	native, e := s.NativeRepresentable(ctx, o.Tolerance, o.Native)
	if e != nil {
		return Solid{}, empty, e
	}
	h := SplineShellCombineHistory{Method: "algebraic-native", FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	for i, f := range s.faces {
		row := [2][][]int{h.FacesA, h.FacesB}[f.Operand]
		row[f.Original] = append(row[f.Original], i)
	}
	for i, x := range s.index {
		if x.SectionPiece >= 0 {
			h.SectionEdges = append(h.SectionEdges, i)
			continue
		}
		row := [2][][]int{h.EdgesA, h.EdgesB}[x.Operand]
		source := s.parts[x.Operand].Subdivision.edges[x.OriginalSubdivisionEdge].Original
		row[source] = append(row[source], i)
	}
	return native, h, nil
}

// NativeRepresentable converts the exact Boolean graph to the ordinary native
// Solid when all stored vertex and chart coordinates and retained source
// curve limits are exactly representable. Face, edge and vertex indices
// remain those of the immutable Boolean boundary. It independently binds
// both chart endpoints of each implicit tube and rechecks continuous edge
// ownership and closed incidence. Unrepresentable junctions reject the whole
// conversion; the algebraic Boolean remains usable in that case.
func (s AlgebraicShellBoolean) NativeRepresentable(ctx context.Context, tolerance float64, o NativeAlgebraicBooleanOptions) (Solid, error) {
	if ctx == nil || s.source == nil || !finite(tolerance) || tolerance <= 0 || o.MaxEdges < 0 || o.MaxEdges > 8192 ||
		o.Exact.MaxOperations < 0 || o.Exact.MaxSubdivisions < 0 || o.Exact.MaxCoefficientBits != 0 && o.Exact.MaxCoefficientBits < 64 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if len(s.faces) == 0 {
		return Solid{}, nil
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 1024
	}
	if len(s.index) > o.MaxEdges {
		return Solid{}, geom.ErrConvergence
	}
	if o.Exact.MaxOperations == 0 {
		o.Exact.MaxOperations = 64000000
	}
	w := nativeAlgebraicBooleanWork{s: s, exact: o.Exact}
	vertices := make([]geom.Vec3, len(s.vertices))
	vertexSet := make([]bool, len(vertices))
	vertex := func(i int, p geom.Vec3) error {
		if vertexSet[i] && vertices[i] != p {
			return geom.ErrConvergence
		}
		vertices[i], vertexSet[i] = p, true
		return nil
	}
	for i, v := range s.vertices {
		if v.OriginalSubdivisionVertex < 0 {
			continue
		}
		sub := s.parts[v.Operand].Subdivision
		x := sub.vertices[v.OriginalSubdivisionVertex]
		if x.Original >= 0 {
			if e := vertex(i, sub.source.vertices[x.Original]); e != nil {
				return Solid{}, e
			}
		}
	}
	var edges []parametric.Edge
	generated := make([]nativeRepresentedTube, len(s.index))
	sourceLimits := make([][2]*big.Rat, len(s.index))
	for i, e := range s.index {
		if err := ctx.Err(); err != nil {
			return Solid{}, err
		}
		var model parametric.Edge
		if e.SectionPiece >= 0 {
			tube, err := w.representedTube(ctx, i)
			if err != nil {
				return Solid{}, fmt.Errorf("trimmed: represent section edge %d: %w", i, err)
			}
			generated[i] = tube
			for j, id := range [2]int{e.Start, e.End} {
				if err = vertex(id, tube.xyz[j]); err != nil {
					return Solid{}, err
				}
			}
			trim := tube.seg.TrimA()
			model = parametric.Edge{Range: parametric.Interval{Max: 1}, Curve: surfaceTrimCurve{
				trim.SourceSurface(), tube.uv[0][0], tube.uv[0][1], trim,
			}}
		} else {
			sub := s.parts[e.Operand].Subdivision
			x := sub.edges[e.OriginalSubdivisionEdge]
			model = sub.source.edges[x.Original]
			limits := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
			for j, r := range [2]*geom.ExactAlgebraicRoot{x.Lower, x.Upper} {
				if r != nil {
					q, err := w.root(ctx, r)
					if err != nil {
						return Solid{}, fmt.Errorf("trimmed: represent original edge %d: %w", i, err)
					}
					limits[j] = q
				}
			}
			minT, err := geom.RepresentedExactScalar(nativeAlgebraicLerp(model.Range.Min, model.Range.Max, limits[0]))
			if err != nil {
				return Solid{}, err
			}
			maxT, err := geom.RepresentedExactScalar(nativeAlgebraicLerp(model.Range.Min, model.Range.Max, limits[1]))
			if err != nil {
				return Solid{}, err
			}
			model.Range = parametric.Interval{Min: minT, Max: maxT}
			sourceLimits[i] = limits
		}
		model.Start, model.End = e.Start, e.End
		edges = append(edges, model)
	}
	for _, b := range vertexSet {
		if !b {
			return Solid{}, geom.ErrConvergence
		}
	}
	var faces []Face
	for _, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		original := [2]Solid{s.source.a, s.source.b}[f.Operand].faces[f.Original]
		native := Face{Surface: original.Surface, Reversed: f.Reversed}
		for j, loop := range append([][]AlgebraicShellBooleanUse{f.Outer}, f.Holes...) {
			var uses []Use
			for _, u := range loop {
				source := s.index[u.Edge]
				x := Use{Edge: u.Edge, Reversed: u.Reversed}
				if source.SectionPiece >= 0 {
					g := generated[u.Edge]
					x.UVStart, x.UVEnd = g.uv[f.Operand][0], g.uv[f.Operand][1]
					x.Curve = [2]intersect.TubeTrim{g.seg.TrimA(), g.seg.TrimB()}[f.Operand]
					if u.Reversed {
						x.UVStart, x.UVEnd, x.Curve = x.UVEnd, x.UVStart, reversePCurve(x.Curve)
					}
				} else {
					originalEdge := s.parts[f.Operand].Subdivision.edges[source.OriginalSubdivisionEdge].Original
					retained, err := nativeAlgebraicSourceUse(original, originalEdge, u.Reversed, sourceLimits[u.Edge])
					if err != nil {
						return Solid{}, err
					}
					x.UVStart, x.UVEnd, x.Curve = retained.UVStart, retained.UVEnd, retained.Curve
				}
				uses = append(uses, x)
			}
			if j == 0 {
				native.Outer = uses
			} else {
				native.Holes = append(native.Holes, uses)
			}
		}
		faces = append(faces, native)
	}
	result, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, e
	}
	agreement, e := result.ValidateEdgeAgreement(ctx, o.Edge)
	if e != nil {
		return Solid{}, e
	}
	if !agreement.Complete {
		return Solid{}, geom.ErrConvergence
	}
	if result.Topology().EulerCharacteristic != len(s.vertices)-len(s.index)+len(s.faces)-nativeAlgebraicHoles(s.faces) {
		return Solid{}, geom.ErrConvergence
	}
	return result, nil
}

func nativeAlgebraicHoles(faces []AlgebraicShellBooleanFace) int {
	n := 0
	for _, f := range faces {
		n += len(f.Holes)
	}
	return n
}

type nativeAlgebraicBooleanWork struct {
	s     AlgebraicShellBoolean
	exact geom.ExactCurvePlaneEventOptions
}

func (w *nativeAlgebraicBooleanWork) root(ctx context.Context, r *geom.ExactAlgebraicRoot) (*big.Rat, error) {
	if w.exact.MaxOperations < 1 {
		return nil, geom.ErrConvergence
	}
	q, e := r.Classify(ctx, w.exact)
	w.exact.MaxOperations -= q.Operations
	if e != nil {
		return nil, e
	}
	if q.Kind != geom.ExactRootKindRational || q.Rational == nil {
		return nil, geom.ErrUnrepresentable
	}
	return q.Rational, nil
}

type nativeRepresentedTube struct {
	seg intersect.TubeSegment
	uv  [2][2]geom.Vec2 // original chart, model-edge end
	xyz [2]geom.Vec3
}

func (w *nativeAlgebraicBooleanWork) representedTube(ctx context.Context, edge int) (nativeRepresentedTube, error) {
	var out nativeRepresentedTube
	seg, _, e := w.s.SectionSegment(edge)
	if e != nil {
		return out, e
	}
	recipe, i, _ := seg.TrimA().Recipe()
	d := recipe.Data()
	axis, _, _, e := seg.ExactFreeDomain()
	if e != nil {
		return out, e
	}
	d.Cells = d.Cells[i : i+1]
	if len(d.Oriented) != 0 {
		d.Oriented = d.Oriented[i : i+1]
	}
	d.FreeAxis, d.Closed, d.ExactEndpoints = axis, false, nil
	for j := range 2 {
		if w.exact.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		q, e := discoverExactTubeSectionEndpoint(ctx, seg, j, d.A, d.B, geom.TubeAlgebraicGraphEndpointOptions{
			SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{
				SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits},
				MaxSubdivisions:     w.exact.MaxSubdivisions,
			},
		})
		w.exact.MaxOperations -= q.ExactOperations
		if e != nil {
			return out, e
		}
		if !q.Proven || q.ModelEvent == nil {
			return out, geom.ErrConvergence
		}
		h := intersect.TubeExactEndpoint{End: j}
		for k, root := range q.Parameters {
			r, e := w.root(ctx, root)
			if e != nil {
				return out, e
			}
			h.Parameters[k], e = geom.RepresentedExactScalar(r)
			if e != nil {
				return out, e
			}
		}
		for k := range 2 {
			out.uv[k][j] = geom.Vec2{X: h.Parameters[2*k], Y: h.Parameters[2*k+1]}
		}
		param, e := w.root(ctx, q.ModelEvent.Root())
		if e != nil {
			return out, e
		}
		if w.exact.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		p, e := q.ModelEvent.EvaluateRationalWitness(ctx, param, w.exact)
		w.exact.MaxOperations -= p.ExactOperations
		if e != nil {
			return out, e
		}
		var xyz [3]float64
		for k, r := range p.Point {
			xyz[k], e = geom.RepresentedExactScalar(r)
			if e != nil {
				return out, e
			}
		}
		out.xyz[j] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		d.ExactEndpoints = append(d.ExactEndpoints, h)
	}
	if w.exact.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	// Recipe rebuild has independent geometry budgets. It is bounded by
	// MaxEdges and caps its own per-cell evaluation/endpoint limits.
	rebuilt, e := intersect.RebuildTubeCurves(ctx, d)
	if e != nil {
		return out, e
	}
	out.seg = rebuilt[0]
	return out, nil
}

func nativeAlgebraicLerp(a, b float64, q *big.Rat) *big.Rat {
	x := new(big.Rat).Sub(ratInset(b), ratInset(a))
	return x.Add(x.Mul(x, q), ratInset(a))
}

func nativeAlgebraicSourceUV(f Face, edge int, reverse bool, limits [2]*big.Rat) (geom.Vec2, geom.Vec2, error) {
	for _, u := range f.Outer {
		if u.Edge != edge || u.Curve != nil {
			continue
		}
		if u.Reversed {
			u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
		}
		var points [2]geom.Vec2
		for j, t := range limits {
			x, e := geom.RepresentedExactScalar(nativeAlgebraicLerp(u.UVStart.X, u.UVEnd.X, t))
			if e != nil {
				return geom.Vec2{}, geom.Vec2{}, e
			}
			y, e := geom.RepresentedExactScalar(nativeAlgebraicLerp(u.UVStart.Y, u.UVEnd.Y, t))
			if e != nil {
				return geom.Vec2{}, geom.Vec2{}, e
			}
			points[j] = geom.Vec2{X: x, Y: y}
		}
		if reverse {
			points[0], points[1] = points[1], points[0]
		}
		return points[0], points[1], nil
	}
	return geom.Vec2{}, geom.Vec2{}, geom.ErrConvergence
}
