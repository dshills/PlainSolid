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

// WritePeriodicConcavePolygonTrimmedFaceWithHole replays the complete
// original outer and inner proof and verifies its canonical carrier
// before writing. Strict containment survives binary64 conversion.
func WritePeriodicConcavePolygonTrimmedFaceWithHole(ctx context.Context, w io.Writer, face PeriodicConcavePolygonTrimmedFaceWithHole) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 0, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicConcavePolygonTrimmedFaceWithHole(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: concave trim with hole no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicConcaveHoleTransfer(proved); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of concave trim with hole: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: concave trim with hole changed on strict reopen: %w", geom.ErrUnrepresentable)
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

func checkPeriodicConcaveHoleTransfer(face PeriodicConcavePolygonTrimmedFaceWithHole) error {
	wires, err := periodicRoundedAffineWires(face.KnotOwners, [][]PeriodicTrimEdge{face.Edges, face.Hole.Edges})
	if err != nil {
		return err
	}
	if err := checkPeriodicConcaveRoundedOuter(face.Edges, wires[0]); err != nil {
		return err
	}
	if !periodicUVStrictlyConvex(wires[1], -1) {
		return fmt.Errorf("iges: concave trim hole loses its original winding or convexity in binary64 transfer: %w", geom.ErrUnrepresentable)
	}
	if err := provePeriodicConcaveHoleContainment(wires[0], wires[1]); err != nil {
		return fmt.Errorf("iges: concave trim hole loses strict containment in binary64 transfer: %w", err)
	}
	return nil
}
