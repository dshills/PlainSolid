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
	bounds := [][2]geom.ExactParameterInterval{proved.Bounds}
	for _, hole := range proved.Holes {
		bounds = append(bounds, hole.Bounds)
	}
	if err := checkPeriodicTrimTransferStations(nil, bounds...); err != nil {
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
func checkPeriodicTrimTransferStations(seamU *big.Rat, boxes ...[2]geom.ExactParameterInterval) error {
	for axis := range 2 {
		var stations []*big.Rat
		if axis == 0 && seamU != nil {
			stations = append(stations, seamU)
		}
		for _, box := range boxes {
			stations = append(stations, box[axis].Lower, box[axis].Upper)
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
