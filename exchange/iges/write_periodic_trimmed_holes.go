package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"slices"
	"strings"

	"plainsolid/geom"
)

// WritePeriodicTrimmedFaceWithHoles writes an exact canonical type-144
// carrier preserving the source directory IDs, clipped support bounds and
// all directed outer and inner edges. The original source and the complete
// output must both replay the strict proof before any bytes are published.
func WritePeriodicTrimmedFaceWithHoles(ctx context.Context, w io.Writer, face PeriodicTrimmedFaceWithHoles) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceWithHoles(ctx, strings.NewReader(face.original), Limits{}, 4, 4, 2)
	if err != nil {
		return err
	}
	proved, err := provePeriodicTrimmedFaceWithHoles(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: trim with holes no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicHoleTransferStations(proved); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of periodic trim with holes: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: periodic trim with holes changed on strict reopen: %w", geom.ErrUnrepresentable)
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

// A distinct station on another wire can collapse even when every individual
// edge interval survives transfer. Check all contour bounds in the shared chart.
func checkPeriodicHoleTransferStations(f PeriodicTrimmedFaceWithHoles) error {
	for i, outer := range f.Bounds {
		stations := []*big.Rat{outer.Lower, outer.Upper}
		for _, hole := range f.Holes {
			stations = append(stations, hole.Bounds[i].Lower, hole.Bounds[i].Upper)
		}
		slices.SortFunc(stations, (*big.Rat).Cmp)
		for j, q := range stations[1:] {
			if q.Cmp(stations[j]) > 0 {
				if err := periodicTrimTransferInterval(stations[j], q); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
