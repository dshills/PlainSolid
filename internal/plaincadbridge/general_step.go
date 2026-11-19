package plaincadbridge

import (
	"bytes"
	"errors"
	"fmt"

	"plainsolid/exchange/step"
)

// writeGeneralSTEPBody writes a result of the general Boolean. It is reached
// only on a bridge whose general field is set. A result that a writer of
// writeSTEPBody already takes keeps that writer and the reader that goes
// with it. Any other is written by step.WriteTrimmedBoundary as the solid
// stores it, and general reports that.
func writeGeneralSTEPBody(s stored) (data []byte, general bool, err error) {
	if data, err := writeSTEPBody(s); err == nil {
		return data, false, nil
	}
	var out bytes.Buffer
	if err := step.WriteTrimmedBoundary(&out, s.analytic.solid, .001, "PlainSolid"); err != nil {
		if errors.Is(err, step.ErrTrimmedBoundary) {
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
// on what its file reads back as, so a file that no reader of importStep
// takes is refused, and the refusal says that the reader is what is missing.
func generalSTEPReadback(err error) error {
	return &failureError{"UNSUPPORTED", "STEP export wrote the boundary, and no STEP reader of the bridge reads it back to verify it: " + err.Error()}
}
