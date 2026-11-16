package iges

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// AcnodalCubicIGESOptions controls the coedge proof and decimal bound.
type AcnodalCubicIGESOptions = ClippedPolynomialFuseIGESOptions

// WriteAcnodalCubicEnvelope writes the original material boundary of an
// acnodal Common or Fuse. The isolated contact stays inside its original face.
func WriteAcnodalCubicEnvelope(ctx context.Context, w io.Writer, envelope *trimmed.AcnodalCubicEnvelope,
	unit float64, limits Limits, o AcnodalCubicIGESOptions) error {
	if envelope == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGESMode(ctx, w, envelope.Boundary(), unit, limits, o, true)
}

// VerifyAcnodalCubicEnvelopeIGES checks the canonical topology and units
// against the source-proved native material envelope.
func VerifyAcnodalCubicEnvelopeIGES(ctx context.Context, input io.Reader, envelope *trimmed.AcnodalCubicEnvelope,
	unit float64, limits Limits, o AcnodalCubicIGESOptions) error {
	if envelope == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, limits, "acnodal cubic envelope", func(w io.Writer) error {
		return WriteAcnodalCubicEnvelope(ctx, w, envelope, unit, limits, o)
	})
}
