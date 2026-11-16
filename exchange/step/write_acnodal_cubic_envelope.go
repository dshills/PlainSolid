package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// AcnodalCubicSTEPOptions controls the coedge proof and decimal bound.
type AcnodalCubicSTEPOptions = ClippedPolynomialFuseSTEPOptions

// WriteAcnodalCubicEnvelope writes the original material boundary of an
// acnodal Common or Fuse. The isolated contact stays inside its original face.
func WriteAcnodalCubicEnvelope(ctx context.Context, w io.Writer, envelope *trimmed.AcnodalCubicEnvelope, name string,
	unit float64, limits Limits, o AcnodalCubicSTEPOptions) error {
	if envelope == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEPMode(ctx, w, envelope.Boundary(), name, unit, limits, o, true)
}

// VerifyAcnodalCubicEnvelopeSTEP checks the canonical graph and units against
// the source-proved native material envelope.
func VerifyAcnodalCubicEnvelopeSTEP(ctx context.Context, input io.Reader, envelope *trimmed.AcnodalCubicEnvelope, name string,
	unit float64, limits Limits, o AcnodalCubicSTEPOptions) error {
	if envelope == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, limits, "acnodal cubic envelope", func(w io.Writer) error {
		return WriteAcnodalCubicEnvelope(ctx, w, envelope, name, unit, limits, o)
	})
}
