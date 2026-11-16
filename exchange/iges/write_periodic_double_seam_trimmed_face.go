package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

	"plainsolid/geom"
)

// WritePeriodicDoubleSeamTrimmedFace emits a canonical exact carrier for
// a proved four-cell trim. Source replay and strict reopen preserve both
// original seams, all eight edge pairs and their source model intervals.
func WritePeriodicDoubleSeamTrimmedFace(ctx context.Context, w io.Writer, face PeriodicDoubleSeamTrimmedFace) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 8, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicDoubleSeamTrimmedFace(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: double-seam trim no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	var boxes [8][2]geom.ExactParameterInterval
	for u := range proved.Cells {
		for v, c := range proved.Cells[u] {
			i := 2 * (2*u + v)
			boxes[i] = c.Bounds
			boxes[i+1] = [2]geom.ExactParameterInterval{c.KnotOwners[0].Domain, c.KnotOwners[1].Domain}
		}
	}
	if err := checkPeriodicTrimTransferStations(nil, boxes[:]...); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of double-seam periodic trim: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: periodic double-seam trim changed on strict reopen: %w", geom.ErrUnrepresentable)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(carrier)
	if err == nil && n != len(carrier) {
		return io.ErrShortWrite
	}
	return err
}
