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

// WriteClippedPolynomialCommon writes a source-proved clipped Common as one
// type 186 solid. It preserves the original supports and paired coedges.
// If the original contacts are cubic and quadratic, it splits the cusp
// lens on its original roof so that each wire retains just one tip.
func WriteClippedPolynomialCommon(ctx context.Context, w io.Writer, common *trimmed.AlignedPolynomialEnvelope, unit float64, l Limits, o ClippedPolynomialCommonIGESOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	boundary, err := clippedCommonIGESBoundary(ctx, common, o)
	if err != nil {
		return err
	}
	return writeExactBezierSingleIGES(ctx, w, boundary, unit, l, o)
}

// VerifyClippedPolynomialCommonIGES matches the original type 186 geometry,
// ownership, physical units and transfer settings against the native
// proof. Start-section annotations may vary.
func VerifyClippedPolynomialCommonIGES(ctx context.Context, input io.Reader, common *trimmed.AlignedPolynomialEnvelope, unit float64, l Limits, o ClippedPolynomialCommonIGESOptions) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if _, ok := common.ClippedFootprint(); !ok {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "clipped Common", func(w io.Writer) error {
		return WriteClippedPolynomialCommon(ctx, w, common, unit, l, o)
	})
}

// WriteClippedPolynomialThreeTransverseCommon writes one source-proved
// intersection solid with its three exact four-ray junction vertices. Both
// roof charts, the base and wall charts and all original edge uses remain.
func WriteClippedPolynomialThreeTransverseCommon(ctx context.Context, w io.Writer, common *trimmed.ClippedPolynomialThreeTransverseCommon, unit float64, l Limits, o ClippedPolynomialCommonIGESOptions) error {
	if err := requireThreeTransverseClippedCommonIGES(common); err != nil {
		return err
	}
	return writeExactBezierSingleIGES(ctx, w, common.Boundary(), unit, l, o)
}

// VerifyClippedPolynomialThreeTransverseCommonIGES authenticates the native
// three-contact solid, all original edge owners and its declared units.
func VerifyClippedPolynomialThreeTransverseCommonIGES(ctx context.Context, input io.Reader, common *trimmed.ClippedPolynomialThreeTransverseCommon, unit float64, l Limits, o ClippedPolynomialCommonIGESOptions) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if err := requireThreeTransverseClippedCommonIGES(common); err != nil {
		return err
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "clipped Common", func(w io.Writer) error {
		return WriteClippedPolynomialThreeTransverseCommon(ctx, w, common, unit, l, o)
	})
}

func verifyExactBezierSingleIGES(ctx context.Context, input io.Reader, l Limits, family string, write func(io.Writer) error) error {
	if ctx == nil || input == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actual, err := readTopologySource(input, l, true)
	if err != nil {
		return err
	}
	var canonical bytes.Buffer
	if err := write(&canonical); err != nil {
		return err
	}
	expected, err := readTopologySource(&canonical, l, true)
	if err != nil {
		return err
	}
	if actual.unit != expected.unit || !reflect.DeepEqual(actual.entities, expected.entities) ||
		!bytes.Equal(exactBezierIGESHeaderRecords(actual.bytes), exactBezierIGESHeaderRecords(expected.bytes)) {
		return fmt.Errorf("iges: %s topology, global records or directory differ from source proof: %w", family, geom.ErrConvergence)
	}
	return ctx.Err()
}

func clippedCommonIGESBoundary(ctx context.Context, common *trimmed.AlignedPolynomialEnvelope, o ClippedPolynomialCommonIGESOptions) (trimmed.Solid, error) {
	if _, ok := common.ClippedFootprint(); !ok {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	contacts := common.Contacts()
	switch [2]int{contacts[0].Order, contacts[1].Order} {
	case [2]int{1, 1}:
		return common.Boundary(), nil
	case [2]int{2, 3}, [2]int{3, 2}:
		return common.SubdivideClippedCommonLens(ctx, geom.GraphPointContactOptions{
			MaxOperations: o.Proof.MaxWork, MaxCoefficientBits: o.Proof.MaxBits,
		})
	default:
		return trimmed.Solid{}, fmt.Errorf("iges: unsupported clipped Common contact orders: %w", geom.ErrDomain)
	}
}

func requireThreeTransverseClippedCommonIGES(common *trimmed.ClippedPolynomialThreeTransverseCommon) error {
	if _, ok := common.ClippedFootprint(); !ok {
		return geom.ErrInvalid
	}
	contacts := common.Contacts()
	if len(contacts) != 3 {
		return geom.ErrInvalid
	}
	for _, contact := range contacts {
		if contact.Order != 1 {
			return fmt.Errorf("iges: clipped Common requires transverse wire contacts: %w", geom.ErrDomain)
		}
	}
	return nil
}
