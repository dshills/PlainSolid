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

// WritePeriodicConvexTrimmedFaceWithHole emits a canonical exact carrier
// for two source-proved affine contours. Replay and strict reopen must
// preserve all original directed pairs and their position in the source
// chart before the carrier is written.
func WritePeriodicConvexTrimmedFaceWithHole(ctx context.Context, w io.Writer, face PeriodicConvexTrimmedFaceWithHole) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 4, periodicTrimAnyAxes, []int{4})
	if err != nil {
		return err
	}
	proved, err := provePeriodicConvexTrimmedFaceWithHole(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: convex periodic trim with hole no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicConvexTransfer(proved.PeriodicConvexTrimmedFace, &proved.Hole); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of convex periodic trim with hole: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: convex periodic trim with hole changed on strict reopen: %w", geom.ErrUnrepresentable)
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
