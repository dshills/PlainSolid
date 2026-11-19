//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// PrismToRoundFaceProof retains independently recognized target geometry,
// exact finite footprint coverage, and the represented construction offset.
// The planar constructor's numerical validation remains independent of these
// exact predicates. No first-contact or Boolean subtraction claim is made.
type PrismToRoundFaceProof struct {
	Target          CircularFaceRecognition
	Coverage        geom.PolygonCircularCoverage
	Height          float64
	Offset          geom.Vec3
	ExactOperations int
}

// PrismToRoundFace extrudes a source polygon with optional polygon holes along
// a coordinate axis to an actual recognized circular cap with circular holes.
// Source and target planes must be parallel. Direction magnitude is ignored;
// every generator must have the same strictly positive represented height.
// Input topology is passed to brep.PrismWithHoles without flattening holes.
func PrismToRoundFace(ctx context.Context, outer []geom.Vec3, holes [][]geom.Vec3, direction geom.Vec3, target Solid, faceIndex int, o geom.DiskPolygonOptions) (brep.Solid, PrismToRoundFaceProof, error) {
	var proof PrismToRoundFaceProof
	if ctx == nil || len(outer) < 3 || !direction.IsFinite() {
		return brep.Solid{}, proof, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return brep.Solid{}, proof, e
	}
	axis := -1
	sign := 1.
	for i, q := range []float64{direction.X, direction.Y, direction.Z} {
		if q != 0 {
			if axis >= 0 {
				return brep.Solid{}, proof, geom.ErrDomain
			}
			axis = i
			if q < 0 {
				sign = -1
			}
		}
	}
	if axis < 0 {
		return brep.Solid{}, proof, geom.ErrInvalid
	}
	max := o.MaxOperations
	if max == 0 {
		max = 1000000
	}
	if max < 1 {
		return brep.Solid{}, proof, geom.ErrInvalid
	}
	recognition, e := RecognizeCircularPlanarFace(ctx, target, faceIndex, geom.SurfacePatchEvaluationOptions{MaxOperations: max, MaxCoefficientBits: o.MaxCoefficientBits})
	proof.Target = recognition
	proof.ExactOperations = recognition.ExactOperations
	if e != nil {
		return brep.Solid{}, proof, e
	}
	cap, normal, _ := recognition.Outer.Parameters()
	components := [3]float64{normal.X, normal.Y, normal.Z}
	if components[axis] == 0 {
		return brep.Solid{}, proof, geom.ErrDomain
	}
	for i, q := range components {
		if i != axis && q != 0 {
			return brep.Solid{}, proof, geom.ErrDomain
		}
	}
	coordinate := func(p geom.Vec3) float64 { return [3]float64{p.X, p.Y, p.Z}[axis] }
	start, end := coordinate(outer[0]), coordinate(cap)
	if !outer[0].IsFinite() {
		return brep.Solid{}, proof, geom.ErrInvalid
	}
	delta := new(big.Rat).Sub(new(big.Rat).SetFloat64(end), new(big.Rat).SetFloat64(start))
	height := new(big.Rat).Set(delta)
	if sign < 0 {
		height.Neg(height)
	}
	if height.Sign() <= 0 {
		return brep.Solid{}, proof, geom.ErrDomain
	}
	h, e := geom.RepresentedExactScalar(height)
	if e != nil {
		return brep.Solid{}, proof, e
	}
	offsetValue, e := geom.RepresentedExactScalar(delta)
	if e != nil {
		return brep.Solid{}, proof, e
	}
	offset := geom.Vec3{}
	switch axis {
	case 0:
		offset.X = offsetValue
	case 1:
		offset.Y = offsetValue
	case 2:
		offset.Z = offsetValue
	}
	source := append([][]geom.Vec3{outer}, holes...)
	projected := make([][]geom.Vec3, len(source))
	maxEdges := o.MaxEdges
	if maxEdges == 0 {
		maxEdges = 256
	}
	total := 0
	for i, wire := range source {
		if len(wire) < 3 || len(wire) > maxEdges-total {
			return brep.Solid{}, proof, geom.ErrInvalid
		}
		total += len(wire)
		for _, p := range wire {
			if !p.IsFinite() {
				return brep.Solid{}, proof, geom.ErrInvalid
			}
			if coordinate(p) != start {
				return brep.Solid{}, proof, geom.ErrDomain
			}
			endpoint := p
			switch axis {
			case 0:
				endpoint.X = end
			case 1:
				endpoint.Y = end
			case 2:
				endpoint.Z = end
			}
			if p.Add(offset) != endpoint {
				return brep.Solid{}, proof, geom.ErrUnrepresentable
			}
			projected[i] = append(projected[i], endpoint)
		}
	}
	remaining := max - proof.ExactOperations
	if remaining < 1 {
		return brep.Solid{}, proof, geom.ErrConvergence
	}
	options := o
	options.MaxOperations = remaining
	proof.Coverage, e = geom.ClassifyPolygonCircularRegion(ctx, projected[0], projected[1:], recognition.Outer, recognition.Holes, options)
	proof.ExactOperations += proof.Coverage.ExactOperations
	if e != nil {
		return brep.Solid{}, proof, e
	}
	if !proof.Coverage.Covered {
		return brep.Solid{}, proof, brep.ErrTerminationUncovered
	}
	if e := ctx.Err(); e != nil {
		return brep.Solid{}, proof, e
	}
	result, e := brep.PrismWithHoles(outer, holes, offset)
	if e != nil {
		return brep.Solid{}, proof, e
	}
	proof.Height = h
	proof.Offset = offset
	return result, proof, nil
}
