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

// WritePeriodicSeamTrimmedFaceWithMixedHoles writes a canonical carrier
// with the original outer lift and both clockwise holes. Original contour
// order, directory owners, seam splits and every model parameter interval
// must survive exact source replay and strict reopen before emission.
func WritePeriodicSeamTrimmedFaceWithMixedHoles(ctx context.Context, w io.Writer, face PeriodicSeamTrimmedFaceWithMixedHoles) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSource(ctx, strings.NewReader(face.original), Limits{}, 6, []int{4, 6})
	if err != nil {
		return err
	}
	proved, err := provePeriodicSeamTrimmedFaceWithMixedHoles(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: seam trim with mixed holes no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicTrimTransferStations(proved.SeamU, proved.Bounds, proved.SeamHole.Bounds, proved.CellHole.Bounds); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of periodic seam trim with mixed holes: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: periodic seam trim with mixed holes changed on strict reopen: %w", geom.ErrUnrepresentable)
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
