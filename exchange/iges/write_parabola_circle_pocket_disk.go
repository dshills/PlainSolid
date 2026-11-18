package iges

import (
	"context"
	"fmt"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCirclePocketDiskEnvelope transfers the original Common or
// Fuse with four transverse junctions and the original pocket and disk
// wires. The large circle uses exact active windows where its cap trims
// require them. The original small circle already has full exact quarters.
func WriteParabolaCirclePocketDiskEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCirclePocketDiskEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGESWithBoundary(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof}, false, nil, emitParabolaCircleIGESBody(ctx, [2][]int{body.History().CircleEdges}, true))
}

// VerifyParabolaCirclePocketDiskEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCirclePocketDiskEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCirclePocketDiskEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "parabola circle pocket disk envelope", func(w io.Writer) error {
		return WriteParabolaCirclePocketDiskEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCirclePocketDiskCut transfers the four-component original
// Cut with a holed pocket and four authenticated contacts. A Cut
// containing the separate disk returns geom.ErrUnrepresentable before
// writing because outward signed material on that shell is unqualified.
func WriteParabolaCirclePocketDiskCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCirclePocketDiskCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if ctx == nil || w == nil || body == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	volume := body.DiskSignedVolumePiCoefficient()
	if volume == nil {
		return geom.ErrInvalid
	}
	if volume.Sign() > 0 {
		return fmt.Errorf("IGES: isolated pocket disk orientation: %w", geom.ErrUnrepresentable)
	}
	parts, exact := body.Components(), body.Contacts()
	var contacts [4]trimmed.PolynomialMixedCutContact
	for i, node := range exact {
		contacts[i].TwoPolynomialNodeComponentContact = trimmed.TwoPolynomialNodeComponentContact{
			Parameters: node.Parameters, Point: node.Point, Components: node.Components, Vertices: node.Vertices,
		}
	}
	emitters := make([]exactBezierIGESBodyEmitter, len(parts))
	for i, component := range body.History() {
		emitters[i] = emitParabolaCircleIGESBody(ctx, [2][]int{component.CircleEdges}, true)
	}
	return writeClippedCutIGESBodies(ctx, w, parts, contacts[:], unit, l, ClippedPolynomialCutIGESOptions{Proof: proof}, emitters...)
}

// VerifyParabolaCirclePocketDiskCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCirclePocketDiskCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCirclePocketDiskCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCirclePocketDiskCut(ctx, w, body, unit, l, proof)
	})
}
