package trimmed

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeAlignedPolynomialEnvelope = "trimmed-aligned-polynomial-envelope"

// AlignedPolynomialEnvelope retains an original-source Common or Fuse with
// an order-three crossing and an order-two tangency in independent charts.
type AlignedPolynomialEnvelope struct {
	solid     Solid
	history   PolynomialCubicCrossingTangencyEnvelopeHistory
	contacts  [2]PolynomialMixedEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *AlignedPolynomialEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History relates exact contact branches and regions to native topology.
// RoofOwners indexes the roofs in the original input order.
func (r *AlignedPolynomialEnvelope) History() PolynomialCubicCrossingTangencyEnvelopeHistory {
	if r == nil {
		return PolynomialCubicCrossingTangencyEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns the contacts in increasing physical free order. The
// parameters and derivative gaps use the common unit base chart.
func (r *AlignedPolynomialEnvelope) Contacts() [2]PolynomialMixedEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [2]PolynomialMixedEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineAlignedPolynomialEnvelope accepts a unit identity XY base and at
// least one unit identity XY roof. Both roofs must cover the full base,
// with strict positive height above it. Axis is the physical free axis.
// The original roofs retain their own UV charts on every native coedge.
func CombineAlignedPolynomialEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		(op != parametric.Intersection && op != parametric.Union) || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	limits := o.Section.Patch
	if e := requireIdentitySplineChart(ctx, base, limits); e != nil {
		return nil, e
	}
	bp, e := geom.NewSurfacePatch(base, limits)
	if e != nil {
		return nil, e
	}
	roofs := [2]*geom.BSplineSurface{a, b}
	first := 0
	if requireIdentitySplineChart(ctx, a, limits) != nil {
		if e := requireIdentitySplineChart(ctx, b, limits); e != nil {
			return nil, e
		}
		first, roofs = 1, [2]*geom.BSplineSurface{b, a}
	}
	var patches [2]*geom.SurfacePatch
	for i, roof := range roofs {
		patches[i], e = geom.NewSurfacePatch(roof, limits)
		if e != nil {
			return nil, e
		}
		align, e := geom.AlignAffineXYCharts(ctx, bp, patches[i], limits)
		if e != nil {
			return nil, e
		}
		height, e := align.HeightRange(ctx, geom.GeneralSurfaceHeightOptions{
			MaxPoles: limits.MaxPoles, MaxOperations: o.Proof.MaxOperations,
			MaxCoefficientBits: o.Proof.MaxCoefficientBits, MaxCells: o.Proof.MaxCells,
		})
		if e != nil {
			return nil, e
		}
		if height.StrictSign != -1 {
			return nil, geom.ErrConvergence
		}
	}
	aligned, e := geom.DiscoverAlignedSplineGraphPolynomialArrangement(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	proof := aligned.Arrangement
	if !proof.Proven || proof.FreeAxis != axis || proof.Graph.ProjectionOrientation != 1 ||
		(!slices.Equal(proof.ContactOrder, []int{3, 2}) && !slices.Equal(proof.ContactOrder, []int{2, 3})) {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := polynomialNodePieces(proof.Edges, uv)
	if e != nil {
		return nil, e
	}
	loops, e := polynomialNodeRegionLoops(proof.Regions, uv, pieces)
	if e != nil {
		return nil, e
	}
	charts := [2]graphEnvelopeChart{{}, {
		transfer: func(p openSectionPiece) (openSectionPiece, error) {
			return transferAlignedPolynomialPiece(p, aligned.TargetMap)
		},
		reversed: aligned.Owners[1].ProjectionOrientation < 0,
	}}
	regions := make([]graphEnvelopeRegion, len(loops))
	for i, loop := range loops {
		regions[i] = graphEnvelopeRegion{loop: loop, first: (proof.Regions[i].HeightSide > 0) == (op == parametric.Union)}
	}
	solid, envelope, e := assembleSplineEnvelopeOnCharts(ctx, base, roofs[0], roofs[1], regions, charts, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if e != nil {
		return nil, e
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, o.Proof); e != nil {
		return nil, e
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	binding, e := bindAlignedPolynomialEnvelope(solid, base, roofs, aligned, xyz, pieces, regions, charts, envelope)
	if e != nil {
		return nil, e
	}
	out := &AlignedPolynomialEnvelope{solid: solid, sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, op: op, tolerance: tolerance}
	out.history = PolynomialCubicCrossingTangencyEnvelopeHistory{
		Base: envelope.Base, Vertices: [6]int(binding.vertices), Edges: [6][2]int(binding.chains),
		Midpoints: [2]int{binding.midpoints[1], binding.midpoints[4]},
		RoofFaces: [5]int(binding.roofFaces), RoofOwners: [5]int(binding.roofOwners),
	}
	for i := range out.history.RoofOwners {
		out.history.RoofOwners[i] ^= first
	}
	for i, n := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(solid, binding, proof.Edges, n, proof.Vertices[n.Vertex], xyz[n.Vertex])
		if e != nil {
			return nil, e
		}
		c.Order, c.DerivativeGap = proof.ContactOrder[i], proof.DerivativeGap[i]
		out.contacts[i] = c
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}

func transferAlignedPolynomialPiece(p openSectionPiece, mapping [2][3]*big.Rat) (openSectionPiece, error) {
	mapUV := func(u, v, w *big.Rat) (out [2]*big.Rat) {
		for i, row := range mapping {
			out[i] = new(big.Rat).Mul(row[0], u)
			out[i].Add(out[i], new(big.Rat).Mul(row[1], v))
			out[i].Add(out[i], new(big.Rat).Mul(row[2], w))
		}
		return out
	}
	if p.c == nil {
		one := big.NewRat(1, 1)
		a, e := exactGraphPartitionUV(mapUV(ratInset(p.a.X), ratInset(p.a.Y), one))
		if e != nil {
			return openSectionPiece{}, e
		}
		b, e := exactGraphPartitionUV(mapUV(ratInset(p.b.X), ratInset(p.b.Y), one))
		if e != nil {
			return openSectionPiece{}, e
		}
		return openSectionPiece{a: a, b: b, side: p.side}, nil
	}
	q, ok := p.c.(ExactBezierUV)
	if !ok {
		return openSectionPiece{}, geom.ErrInvalid
	}
	h := q.HomogeneousControls()
	for i, c := range h {
		uv := mapUV(c[0], c[1], c[2])
		h[i][0], h[i][1] = uv[0], uv[1]
	}
	out, e := newExactBezierUV(h, maxSourceGraphBezierControls)
	if e != nil {
		return openSectionPiece{}, e
	}
	return openSectionPiece{a: out.ends[0], b: out.ends[1], c: out, side: p.side}, nil
}

func bindAlignedPolynomialEnvelope(s Solid, base *geom.BSplineSurface, roofs [2]*geom.BSplineSurface,
	aligned geom.AlignedSplineGraphPolynomialArrangement, xyz []geom.Vec3, pieces [][]openSectionPiece,
	regions []graphEnvelopeRegion, charts [2]graphEnvelopeChart, envelope SplineGraphBooleanHistory) (polynomialEnvelopeBinding, error) {
	var result polynomialEnvelopeBinding
	t := s.Topology()
	if len(t.Shells) != 1 || t.EulerCharacteristic != 2 || t.CollapsedEdges != 0 ||
		envelope.Base != len(regions) || len(s.faces) <= envelope.Base || s.faces[envelope.Base].Surface != base {
		return result, geom.ErrConvergence
	}
	proof := aligned.Arrangement
	binding, e := bindPolynomialNodeEdgesOnChart(s, envelope.ContourEdges, proof.Edges, xyz, pieces, true)
	if e != nil {
		return result, e
	}
	result.polynomialNodeBinding = binding
	var ownerPieces [2][][]openSectionPiece
	for i, owner := range aligned.Owners {
		uv := make([]geom.Vec2, len(owner.Vertices))
		for j, v := range owner.Vertices {
			uv[j], e = exactGraphPartitionUV(v)
			if e != nil {
				return result, e
			}
		}
		edges := slices.Clone(proof.Edges)
		for j := range edges {
			edges[j].Controls = owner.Edges[j].Controls
		}
		ownerPieces[i], e = polynomialNodePieces(edges, uv)
		if e != nil {
			return result, e
		}
	}
	for i, chain := range binding.edges {
		encoded := [2]int{chain[0], -1}
		if len(chain) == 2 {
			encoded[1] = chain[1]
		}
		result.chains = append(result.chains, encoded)
		for j, id := range chain {
			edge := s.edges[id]
			model := edge.Curve.(surfaceTrimCurve)
			owner := 0
			if model.surface != roofs[0] {
				if model.surface != roofs[1] {
					return result, geom.ErrConvergence
				}
				owner = 1
			}
			expected := ownerPieces[owner][i][j]
			start := s.vertices[edge.Start]
			if (geom.Vec2{X: start.X, Y: start.Y}) != pieces[i][j].a {
				expected = openSectionPiece{a: expected.b, b: expected.a, c: reversePCurve(expected.c), side: expected.side}
			}
			if model.a != expected.a || model.b != expected.b || !matchesTangencyTrim(model.curve, expected.c) {
				return result, geom.ErrConvergence
			}
		}
	}

	wallOwners := make(map[int]int, len(envelope.WallFaces))
	for i, region := range regions {
		owner := 0
		if !region.first {
			owner = 1
		}
		result.roofFaces = append(result.roofFaces, i)
		result.roofOwners = append(result.roofOwners, owner)
		chains := make([]int, 0, len(region.loop))
		var loop []openSectionPiece
		for _, use := range proof.Regions[i].Edges {
			local, ids := ownerPieces[owner][use.Edge], binding.edges[use.Edge]
			for j, p := range local {
				if use.Reversed {
					k := len(local) - 1 - j
					p = local[k]
					p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
					chains = append(chains, ids[k])
				} else {
					chains = append(chains, ids[j])
				}
				loop = append(loop, p)
			}
		}
		for _, common := range region.loop[len(loop):] {
			p, e := charts[owner].piece(common)
			if e != nil {
				return result, e
			}
			loop = append(loop, p)
			chains = append(chains, -1)
		}
		if charts[owner].reversed {
			slices.Reverse(loop)
			slices.Reverse(chains)
			for j := range loop {
				p := &loop[j]
				p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
			}
		}
		face := s.faces[i]
		if e := bindPolynomialTangencyFace(face, roofs[owner], charts[owner].reversed, loop); e != nil {
			return result, fmt.Errorf("trimmed: aligned roof region %d: %w", i, e)
		}
		for j, id := range chains {
			use := face.Outer[j]
			if id >= 0 {
				if use.Edge != id {
					return result, geom.ErrConvergence
				}
				continue
			}
			model, ok := s.edges[use.Edge].Curve.(surfaceTrimCurve)
			a, b := use.UVStart, use.UVEnd
			if use.Reversed {
				a, b = b, a
			}
			if !ok || model.surface != roofs[owner] || model.a != a || model.b != b || model.curve != nil {
				return result, geom.ErrConvergence
			}
			if _, duplicate := wallOwners[use.Edge]; duplicate {
				return result, geom.ErrConvergence
			}
			wallOwners[use.Edge] = owner
		}
	}
	if e := bindAlignedPolynomialWallUses(s, envelope.WallFaces, wallOwners, roofs); e != nil {
		return result, e
	}
	return result, nil
}

func bindAlignedPolynomialWallUses(s Solid, wallFaces []int, owners map[int]int, roofs [2]*geom.BSplineSurface) error {
	for _, i := range wallFaces {
		face := s.faces[i]
		if len(face.Outer) != 4 {
			return geom.ErrConvergence
		}
		top := face.Outer[2]
		owner, exists := owners[top.Edge]
		reversed, ok := top.Curve.(reverseTrim)
		if !exists || !ok || !top.Reversed {
			return geom.ErrConvergence
		}
		var trim wallTrim
		switch p := reversed.differentiableTrim.(type) {
		case wallTrim:
			if owner != 0 {
				return geom.ErrConvergence
			}
			trim = p
		case affineGraphWallTrim:
			if owner != 1 {
				return geom.ErrConvergence
			}
			trim = p.wallTrim
		default:
			return geom.ErrConvergence
		}
		model := s.edges[top.Edge].Curve.(surfaceTrimCurve)
		if trim.surface != roofs[owner] || trim.a != model.a || trim.b != model.b {
			return geom.ErrConvergence
		}
		delete(owners, top.Edge)
	}
	if len(owners) != 0 {
		return geom.ErrConvergence
	}
	return nil
}

func (r *AlignedPolynomialEnvelope) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges[:], h.Vertices[:], h.Midpoints[:], h.RoofFaces[:], h.RoofOwners[:], h.Base, r.contacts[:])
}

// WriteJSON records the unmodified input supports and exact contact identities.
func (r *AlignedPolynomialEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipe(ctx, w, nativeAlignedPolynomialEnvelope, r.sources, r.axis, r.op, r.tolerance, r.recipeLedger())
}

// ReadAlignedPolynomialEnvelopeJSON re-proves the exact chart alignment,
// original owner restrictions, wall and roof agreement and native links.
func ReadAlignedPolynomialEnvelopeJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	recipe, sources, e := readPolynomialEnvelopeRecipe(ctx, r, limits, nativeAlignedPolynomialEnvelope, 2)
	if e != nil {
		return nil, e
	}
	out, e := CombineAlignedPolynomialEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
