package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ExtrudeExactCircularLoopWithThroughHole extrudes an exact line/minor-arc
// exterior and a disjoint circular hole along local Z. The bore uses four
// exact quarter-circle patches sharing the two planar cap trim cycles.
func ExtrudeExactCircularLoopWithThroughHole(ctx context.Context, input []ExactCircularLoopSegment, bore ExtrusionCircle, height float64, o ExactCircularLoopOptions) (Solid, error) {
	if ctx == nil || !uvFinite(bore.Center) || !finite(bore.Radius) || bore.Radius <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	var segments [4]ExactCircularLoopSegment
	offsets := [4]geom.Vec2{{X: bore.Radius}, {Y: bore.Radius}, {X: -bore.Radius}, {Y: -bore.Radius}}
	for i, offset := range offsets {
		x, xExact := new(big.Rat).Add(ratInset(bore.Center.X), ratInset(offset.X)).Float64()
		y, yExact := new(big.Rat).Add(ratInset(bore.Center.Y), ratInset(offset.Y)).Float64()
		if !xExact || !yExact || !finite(x) || !finite(y) {
			return Solid{}, geom.ErrUnrepresentable
		}
		segments[i] = ExactCircularLoopSegment{Start: geom.Vec2{X: x, Y: y}, Center: bore.Center, Radius: bore.Radius}
	}
	for i := range segments {
		segments[i].End = segments[(i+1)%len(segments)].Start
	}
	exterior, err := extrudeExactCircularLoop(ctx, input, height, o, false)
	if err != nil {
		return Solid{}, err
	}
	cylinder, err := extrudeExactCircularLoop(ctx, segments[:], height, o, false)
	if err != nil {
		return Solid{}, err
	}
	vertices := append(exterior.Vertices(), cylinder.vertices...)
	edges, faces := exterior.Edges(), exterior.Faces()
	vo, eo := len(exterior.vertices), len(edges)
	for _, edge := range cylinder.edges {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	for i, cap := range cylinder.faces[:2] {
		loop := make([]Use, len(cap.Outer))
		for j, use := range cap.Outer {
			curve, ok := use.Curve.(ExactQuadraticUV)
			if !ok {
				return Solid{}, geom.ErrInvalid
			}
			use.Edge += eo
			use.UVStart, use.UVEnd = use.UVEnd, use.UVStart
			use.Reversed = !use.Reversed
			use.Curve = curve.reversed()
			loop[len(loop)-1-j] = use
		}
		faces[i].Holes = [][]Use{loop}
		proof, err := ValidateFaceTrimLoops(ctx, faces[i], o.Trims)
		if err != nil {
			return Solid{}, err
		}
		switch proof.Status {
		case trimvalidate.Invalid:
			return Solid{}, fmt.Errorf("trimmed: intersecting through-hole cap trim: %w", geom.ErrDomain)
		case trimvalidate.Certified:
		default:
			return Solid{}, fmt.Errorf("trimmed: unresolved through-hole cap trim: %w", geom.ErrConvergence)
		}
	}
	for _, face := range cylinder.Faces()[2:] {
		face.Reversed = !face.Reversed
		for i := range face.Outer {
			face.Outer[i].Edge += eo
		}
		faces = append(faces, face)
	}
	out, err := NewSolidContext(ctx, vertices, edges, faces, exterior.tolerance)
	if err != nil {
		return Solid{}, err
	}
	proof, err := out.ValidateEdgeAgreement(ctx, o.Edges)
	if err != nil {
		return Solid{}, err
	}
	if !proof.Complete {
		return Solid{}, fmt.Errorf("trimmed: unresolved through-hole sweep coedges: %w", geom.ErrConvergence)
	}
	return out, nil
}
