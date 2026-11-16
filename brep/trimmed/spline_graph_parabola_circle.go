package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ParabolaCircleGraphOptions bounds the original shared-base and mixed
// contact proofs.
type ParabolaCircleGraphOptions struct {
	Stock intersect.SplinePlaneOptions
	Proof geom.GraphPointContactOptions
}

// ParabolaCircleGraphHistory binds the original roof regions, the proper
// open branch and the interior loop to the native envelope.
type ParabolaCircleGraphHistory struct {
	Base         int
	RoofFaces    [3]int // below, above, interior circle
	RoofOwners   [3]int // 0 for original A, 1 for original B
	ParabolaEdge int
	CircleEdges  [4]int
	WallFaces    [6]int
}

// ParabolaCircleGraphEnvelope is one closed Common or Fuse shell over
// the original positive unit stock, with the original graph supports.
type ParabolaCircleGraphEnvelope struct {
	solid     Solid
	history   ParabolaCircleGraphHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaCircleGraphEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaCircleGraphEnvelope) History() ParabolaCircleGraphHistory {
	if r == nil {
		return ParabolaCircleGraphHistory{}
	}
	return r.history
}

// CombineParabolaCircleSplineGraphs builds Common or Fuse of two original
// positive graph stocks sharing an identity unit XY chart and a spline base.
// Their exact contact consists of one wall-to-wall parabola and a disjoint
// interior circle. Circle radius, UV endpoints and model vertices must be
// representable exactly.
func CombineParabolaCircleSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	freeAxis int, op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleGraphEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || freeAxis < 0 || freeAxis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || tolerance <= 0 || !finite(tolerance) ||
		o.Stock.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, base, a, b, o.Stock)
	if err != nil {
		return nil, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircle(ctx, patches[0], patches[1], freeAxis, o.Proof)
	if err != nil {
		return nil, err
	}
	regions, err := parabolaCircleGraphRegions(proof)
	if err != nil {
		return nil, err
	}
	s, assembly, err := combineOpenGraphRegions(ctx, base, a, b, regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	if err := validateExactGraphPartitionEdges(ctx, s, o.Proof); err != nil {
		return nil, err
	}
	history, err := bindParabolaCircleGraphEnvelope(ctx, s, assembly, proof, [3]*geom.BSplineSurface{base, a, b}, op)
	if err != nil {
		return nil, err
	}
	return &ParabolaCircleGraphEnvelope{solid: s, history: history, sources: [3]*geom.BSplineSurface{base, a, b},
		axis: freeAxis, op: op, tolerance: tolerance}, nil
}

func parabolaCircleGraphRegions(proof *geom.SplineGraphParabolaCircle) (out [3]openGraphRegion, err error) {
	open, err := parabolaOpenGraphRegions(proof.ParabolaControls, proof.FreeAxis)
	if err != nil {
		return out, err
	}
	copy(out[:2], open[:])
	circle, reversed, err := exactGraphCircleRings(proof.CircleCenter, proof.RadiusSquared)
	if err != nil {
		return out, err
	}
	enclosing := 0
	if proof.CircleAbove {
		enclosing = 1
	}
	out[enclosing].holes = [][]openSectionPiece{reversed}
	out[2].loop = circle
	for i, sign := range proof.RegionSides {
		out[i].sign = sign
	}
	return out, nil
}

func parabolaOpenGraphRegions(parabola [3][2]*big.Rat, freeAxis int) (out [2]openGraphRegion, err error) {
	var controls [3][3]*big.Rat
	for i, c := range parabola {
		controls[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
	}
	geometry, err := newExactQuadraticUV(controls, false)
	if err != nil {
		return out, err
	}
	start, err := exactGraphPartitionUV(parabola[0])
	if err != nil {
		return out, err
	}
	end, err := exactGraphPartitionUV(parabola[2])
	if err != nil {
		return out, err
	}
	forward := openSectionPiece{a: start, b: end, c: geometry, side: -1}
	backward := openSectionPiece{a: end, b: start, c: reversePCurve(geometry), side: -1}
	zero, br, tr, tl := geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1}
	if freeAxis == 0 {
		out[0].loop = []openSectionPiece{{a: zero, b: br, side: 0}, {a: br, b: end, side: 1}, backward, {a: start, b: zero, side: 3}}
		out[1].loop = []openSectionPiece{forward, {a: end, b: tr, side: 1}, {a: tr, b: tl, side: 2}, {a: tl, b: start, side: 3}}
	} else {
		out[0].loop = []openSectionPiece{{a: zero, b: start, side: 0}, forward, {a: end, b: tl, side: 2}, {a: tl, b: zero, side: 3}}
		out[1].loop = []openSectionPiece{{a: start, b: br, side: 0}, {a: br, b: tr, side: 1}, {a: tr, b: end, side: 2}, backward}
	}
	return out, nil
}

func exactGraphCircleRings(center [2]*big.Rat, radiusSquared *big.Rat) (outer, inner []openSectionPiece, err error) {
	var controls [3][3]*big.Rat
	circle, err := ellipseGraphContactLoop(geom.GraphEllipseHint{Center: center,
		RadiusSquared: [2]*big.Rat{radiusSquared, radiusSquared}})
	if err != nil {
		return nil, nil, err
	}
	reversed := make([]openSectionPiece, len(circle))
	for i, piece := range circle {
		// Keep the exact homogeneous quarter-arc controls in the native
		// trim type used by source-backed analytic exchange.
		spline, ok := piece.c.(SplineUV)
		if !ok || spline.Curve == nil || spline.Curve.Degree() != 2 || spline.Range != (UVInterval{Max: 1}) {
			return nil, nil, geom.ErrConvergence
		}
		poles, weights := spline.Curve.Poles(), spline.Curve.Weights()
		if len(poles) != 3 || len(weights) != 3 {
			return nil, nil, geom.ErrConvergence
		}
		for j, point := range poles {
			w := ratInset(weights[j])
			controls[j] = [3]*big.Rat{new(big.Rat).Mul(ratInset(point.X), w), new(big.Rat).Mul(ratInset(point.Y), w), w}
		}
		arc, err := newExactQuadraticUV(controls, false)
		if err != nil {
			return nil, nil, err
		}
		circle[i].c = arc
		reversed[len(circle)-1-i] = openSectionPiece{a: piece.b, b: piece.a, c: reversePCurve(arc), side: -1}
	}
	return circle, reversed, nil
}

func bindParabolaCircleGraphEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory, proof *geom.SplineGraphParabolaCircle,
	stock [3]*geom.BSplineSurface, op parametric.BooleanOp) (ParabolaCircleGraphHistory, error) {
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
		len(s.faces) != 10 || len(s.edges) != 23 || len(s.vertices) != 16 || assembly.Base != 3 ||
		len(assembly.ContourEdges) != 5 || len(assembly.WallFaces) != 6 {
		return ParabolaCircleGraphHistory{}, fmt.Errorf("trimmed: mixed graph envelope topology: %w", geom.ErrConvergence)
	}
	h := ParabolaCircleGraphHistory{Base: assembly.Base, RoofFaces: [3]int{0, 1, 2}, ParabolaEdge: assembly.ContourEdges[0]}
	copy(h.CircleEdges[:], assembly.ContourEdges[1:])
	copy(h.WallFaces[:], assembly.WallFaces)
	if s.faces[h.Base].Surface != stock[0] || !s.faces[h.Base].Reversed {
		return ParabolaCircleGraphHistory{}, geom.ErrConvergence
	}
	for i, face := range h.RoofFaces {
		if (proof.RegionSides[i] > 0) != (op == parametric.Union) {
			h.RoofOwners[i] = 1
		}
		if s.faces[face].Surface != stock[1+h.RoofOwners[i]] {
			return ParabolaCircleGraphHistory{}, geom.ErrConvergence
		}
	}
	enclosing := 0
	if proof.CircleAbove {
		enclosing = 1
	}
	for i, face := range h.RoofFaces {
		wantHoles := 0
		if i == enclosing {
			wantHoles = 1
		}
		if len(s.faces[face].Holes) != wantHoles {
			return ParabolaCircleGraphHistory{}, geom.ErrConvergence
		}
	}
	if !opposedGraphContourUses(s.faces[0].Outer, s.faces[1].Outer, h.ParabolaEdge) {
		return ParabolaCircleGraphHistory{}, geom.ErrConvergence
	}
	for _, edge := range h.CircleEdges {
		if !opposedGraphContourUses(s.faces[enclosing].Holes[0], s.faces[2].Outer, edge) {
			return ParabolaCircleGraphHistory{}, geom.ErrConvergence
		}
	}
	if err := validateGraphEnvelopeSplineTrims(ctx, s.faces[:h.Base+1]); err != nil {
		return ParabolaCircleGraphHistory{}, err
	}
	return h, nil
}

func opposedGraphContourUses(a, b []Use, edge int) bool {
	find := func(uses []Use) (Use, bool) {
		i := slices.IndexFunc(uses, func(u Use) bool { return u.Edge == edge })
		if i < 0 {
			return Use{}, false
		}
		return uses[i], true
	}
	first, okA := find(a)
	second, okB := find(b)
	return okA && okB && first.Reversed != second.Reversed &&
		first.UVStart == second.UVEnd && first.UVEnd == second.UVStart
}

func validateGraphEnvelopeSplineTrims(ctx context.Context, faces []Face) error {
	for _, face := range faces {
		proved, err := ValidateFaceTrimLoops(ctx, face, trimvalidate.Options{})
		if err != nil {
			return err
		}
		if proved.Status != trimvalidate.Certified {
			return geom.ErrConvergence
		}
	}
	return nil
}
