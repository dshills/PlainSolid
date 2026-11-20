package plaincadbridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"plainsolid/brep/boolean"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
)

// writeGeneralSTEPBody writes a body that generalResult reports: a result
// of the general Boolean on either bridge. A result that a writer of
// writeSTEPBody already takes keeps that writer and the reader that goes
// with it. Any other is written by step.WriteTrimmedBoundary as the solid
// stores it, and general reports that; one it refuses is written from its
// bore history where it has one.
func writeGeneralSTEPBody(s stored) (data []byte, general bool, err error) {
	if data, err := writeSTEPBody(s); err == nil {
		return data, false, nil
	}
	var out bytes.Buffer
	if err := step.WriteTrimmedBoundary(&out, s.analytic.solid, .001, "PlainSolid"); err != nil {
		if errors.Is(err, step.ErrTrimmedBoundary) {
			// The boundary writer has no entity for the curve in which two
			// cylinders of unequal radius or with skew axes meet (OD-Q5 of
			// specs/general-boolean-quartic). A drilled body that has one
			// is written from its history, by the writer of the legacy
			// bore body, and reads back as that body.
			if old := legacyStock(s); old.bore != nil {
				if data, err := writeSTEPBody(old); err == nil {
					return data, false, nil
				}
			}
			return nil, true, &failureError{"UNSUPPORTED", "STEP export: " + err.Error()}
		}
		return nil, true, fmt.Errorf("STEP export: %w", err)
	}
	if out.Len() > maxSTEPBytes {
		return nil, true, &failureError{"LIMIT", "STEP export exceeds 1 MiB"}
	}
	return out.Bytes(), true, nil
}

// generalSTEPReadback names the failure of an export whose file was written
// by step.WriteTrimmedBoundary and then not read back. An export is accepted
// on what its file reads back as, so a file that importStep does not take is
// refused, and the refusal says that the read-back is what failed.
func generalSTEPReadback(err error) error {
	return &failureError{"UNSUPPORTED", "STEP export wrote the boundary, and it does not read back to verify it: " + err.Error()}
}

// importGeneralSTEP is the last reader of importStep; importSTEPInUnits
// says for which sources each bridge reaches it. A source that no other
// reader takes is read
// as a boundary of planes and cylinders by step.ReadTrimmedBoundary. The
// solid it builds is then united with the empty solid by the general
// Boolean, which holds it to the class and the validation of any operand,
// measures it in closed form, and gives it the stored form of a result, so
// the imported body is published as a result of the general Boolean is.
func (b *Bridge) importGeneralSTEP(source string) (Shape, error) {
	refuse := func(why string) (Shape, error) {
		return Shape{}, &failureError{"UNSUPPORTED", "STEP source is not a supported complete planar or analytic solid: " + why}
	}
	m, err := step.ReadTrimmedBoundary(strings.NewReader(source), stepLimits(), generalOptions.Linear)
	if err != nil {
		return refuse(err.Error())
	}
	if m.LengthUnit != .001 {
		return refuse("a general boundary is read in millimetres only")
	}
	r, err := boolean.Combine(context.Background(), m.Bodies[0].Solid, trimmed.Solid{}, boolean.Union, generalOptions)
	if err != nil {
		return Shape{}, generalFailure(err)
	}
	return b.putGeneral(r)
}
