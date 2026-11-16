package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ClippedPolynomialCommonIGESOptions controls exact native reproof and
// decimal serialization of derived model and parameter-space curve poles.
type ClippedPolynomialCommonIGESOptions = ClippedPolynomialFuseIGESOptions

// WriteClippedPolynomialCommon writes a source-proved transverse clipped
// Common as one type 186 solid. It retains both roof charts, the base and
// planar wall charts, and the two original parameter-space uses of each edge.
// A higher-order contact is outside the supported wire family.
func WriteClippedPolynomialCommon(ctx context.Context, w io.Writer, common *trimmed.AlignedPolynomialEnvelope, unit float64, l Limits, o ClippedPolynomialCommonIGESOptions) error {
	if err := requireTransverseClippedCommonIGES(common); err != nil {
		return err
	}
	return writeExactBezierSingleIGES(ctx, w, common.Boundary(), unit, l, o)
}

// VerifyClippedPolynomialCommonIGES matches the original type 186 geometry,
// ownership, physical units and transfer settings against the native
// proof. Start-section annotations may vary.
func VerifyClippedPolynomialCommonIGES(ctx context.Context, input io.Reader, common *trimmed.AlignedPolynomialEnvelope, unit float64, l Limits, o ClippedPolynomialCommonIGESOptions) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if err := requireTransverseClippedCommonIGES(common); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := readTopologySource(input, l, true)
	if err != nil {
		return err
	}
	var canonical bytes.Buffer
	if err := WriteClippedPolynomialCommon(ctx, &canonical, common, unit, l, o); err != nil {
		return err
	}
	expected, err := readTopologySource(&canonical, l, true)
	if err != nil {
		return err
	}
	if actual.unit != expected.unit || !reflect.DeepEqual(actual.entities, expected.entities) ||
		!bytes.Equal(exactBezierIGESHeaderRecords(actual.bytes), exactBezierIGESHeaderRecords(expected.bytes)) {
		return fmt.Errorf("iges: clipped Common topology, global records or directory differ from source proof: %w", geom.ErrConvergence)
	}
	return ctx.Err()
}

func requireTransverseClippedCommonIGES(common *trimmed.AlignedPolynomialEnvelope) error {
	if _, ok := common.ClippedFootprint(); !ok {
		return geom.ErrInvalid
	}
	for _, contact := range common.Contacts() {
		if contact.Order != 1 {
			return fmt.Errorf("iges: clipped Common requires transverse wire contacts: %w", geom.ErrDomain)
		}
	}
	return nil
}
