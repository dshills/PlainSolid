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

// WritePeriodicTrimmedFace writes a canonical type-144 carrier for an exact
// single-cell face. Both source replay and strict output reopen must retain
// the original cyclic basis, UV/model owners and source ranges before any
// bytes are published.
func WritePeriodicTrimmedFace(ctx context.Context, w io.Writer, face PeriodicTrimmedFace) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 4, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicTrimmedFace(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: periodic trim no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	cell := [2]geom.ExactParameterInterval{proved.KnotOwners[0].Domain, proved.KnotOwners[1].Domain}
	if err := checkPeriodicTrimTransferStations(nil, proved.Bounds, cell); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of periodic trim: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: periodic trim changed on strict reopen: %w", geom.ErrUnrepresentable)
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
