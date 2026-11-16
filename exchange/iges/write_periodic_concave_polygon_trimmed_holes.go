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

// WritePeriodicConcavePolygonTrimmedFaceWithHoles replays the three
// original wires and their containment and separation proofs. It checks
// that the proofs survive binary64 conversion before emitting a carrier.
func WritePeriodicConcavePolygonTrimmedFaceWithHoles(ctx context.Context, w io.Writer, face PeriodicConcavePolygonTrimmedFaceWithHoles) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 0, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: concave trim with two holes no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicConcaveHolesTransfer(proved); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of concave trim with two holes: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: concave trim with two holes changed on strict reopen: %w", geom.ErrUnrepresentable)
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

func checkPeriodicConcaveHolesTransfer(face PeriodicConcavePolygonTrimmedFaceWithHoles) error {
	holes, err := provePeriodicRoundedConcaveHoles(face.PeriodicConcavePolygonTrimmedFace, face.Holes[:])
	if err != nil {
		return err
	}
	if !periodicConvexWiresStrictlyDisjoint(holes[0], holes[1]) {
		return fmt.Errorf("iges: concave trim holes lose strict separation in binary64 transfer: %w", geom.ErrUnrepresentable)
	}
	return nil
}
