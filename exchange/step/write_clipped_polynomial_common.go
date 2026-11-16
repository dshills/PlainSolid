package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ClippedPolynomialCommonSTEPOptions configures the common native source
// proof and the optional bound on nondecimal model and pcurve poles.
type ClippedPolynomialCommonSTEPOptions = ClippedPolynomialFuseSTEPOptions

// WriteClippedPolynomialCommon transfers a source-proved intersection of
// two unequal original polynomial stocks with two transverse contacts. It
// preserves both roof charts, the original base, the affine wall clocks,
// and both uses of each edge. Higher-order cusps remain outside this transfer.
func WriteClippedPolynomialCommon(ctx context.Context, w io.Writer, common *trimmed.AlignedPolynomialEnvelope, name string, lengthUnit float64, l Limits, o ClippedPolynomialCommonSTEPOptions) error {
	if err := requireTransverseClippedCommonSTEP(common); err != nil {
		return err
	}
	return writeExactBezierSingleSTEP(ctx, w, common.Boundary(), name, lengthUnit, l, o)
}

// VerifyClippedPolynomialCommonSTEP authenticates the canonical DATA graph
// against the original native clipped Common result and its declared units.
func VerifyClippedPolynomialCommonSTEP(ctx context.Context, input io.Reader, common *trimmed.AlignedPolynomialEnvelope, name string, lengthUnit float64, l Limits, o ClippedPolynomialCommonSTEPOptions) error {
	if err := requireTransverseClippedCommonSTEP(common); err != nil {
		return err
	}
	return verifyClippedCommonSTEP(ctx, input, l, func(w io.Writer) error {
		return WriteClippedPolynomialCommon(ctx, w, common, name, lengthUnit, l, o)
	})
}

// WriteClippedPolynomialThreeTransverseCommonSTEP transfers one closed Common
// shell and all three original transverse contact vertices. Both owner
// pcurves and the original base, roof and wall charts are carried in STEP.
func WriteClippedPolynomialThreeTransverseCommonSTEP(ctx context.Context, w io.Writer, common *trimmed.ClippedPolynomialThreeTransverseCommon, name string, lengthUnit float64, l Limits, o ClippedPolynomialCommonSTEPOptions) error {
	if err := requireThreeTransverseClippedCommonSTEP(common); err != nil {
		return err
	}
	return writeExactBezierSingleSTEP(ctx, w, common.Boundary(), name, lengthUnit, l, o)
}

// VerifyClippedPolynomialThreeTransverseCommonSTEP authenticates the three
// original contacts and all incident owner uses in a canonical STEP graph.
func VerifyClippedPolynomialThreeTransverseCommonSTEP(ctx context.Context, input io.Reader, common *trimmed.ClippedPolynomialThreeTransverseCommon, name string, lengthUnit float64, l Limits, o ClippedPolynomialCommonSTEPOptions) error {
	if err := requireThreeTransverseClippedCommonSTEP(common); err != nil {
		return err
	}
	return verifyClippedCommonSTEP(ctx, input, l, func(w io.Writer) error {
		return WriteClippedPolynomialThreeTransverseCommonSTEP(ctx, w, common, name, lengthUnit, l, o)
	})
}

func verifyClippedCommonSTEP(ctx context.Context, input io.Reader, l Limits, writeExpected func(io.Writer) error) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := Parse(input, l)
	if err != nil {
		return err
	}
	var expected bytes.Buffer
	if err := writeExpected(&expected); err != nil {
		return err
	}
	want, err := Parse(&expected, l)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual.Entities, want.Entities) {
		return fmt.Errorf("step: clipped Common DATA differs from its source proof: %w", geom.ErrConvergence)
	}
	return ctx.Err()
}

func requireThreeTransverseClippedCommonSTEP(common *trimmed.ClippedPolynomialThreeTransverseCommon) error {
	if _, ok := common.ClippedFootprint(); !ok {
		return geom.ErrInvalid
	}
	contacts := common.Contacts()
	if len(contacts) != 3 {
		return geom.ErrInvalid
	}
	for _, contact := range contacts {
		if contact.Order != 1 {
			return geom.ErrDomain
		}
	}
	return nil
}

func requireTransverseClippedCommonSTEP(common *trimmed.AlignedPolynomialEnvelope) error {
	if _, ok := common.ClippedFootprint(); !ok {
		return geom.ErrInvalid
	}
	for _, contact := range common.Contacts() {
		if contact.Order != 1 {
			return fmt.Errorf("step: clipped Common requires transverse contacts for portable wire transfer: %w", geom.ErrDomain)
		}
	}
	return nil
}
