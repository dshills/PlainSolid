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

// WritePeriodicSeamTrimmedFaceWithHole writes a canonical exact carrier
// for a strictly proved outer and inner lift. It retains both contour
// graphs, directed edges, the original seam and every model interval.
// The original proof and the complete output must both verify before any
// bytes are published.
func WritePeriodicSeamTrimmedFaceWithHole(ctx context.Context, w io.Writer, face PeriodicSeamTrimmedFaceWithHole) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSource(ctx, strings.NewReader(face.original), Limits{}, 6, []int{6})
	if err != nil {
		return err
	}
	proved, err := provePeriodicSeamTrimmedFaceWithHole(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: seam trim with hole no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicTrimTransferStations(proved.SeamU, proved.Bounds, proved.Hole.Bounds); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of periodic seam trim with hole: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: periodic seam trim with hole changed on strict reopen: %w", geom.ErrUnrepresentable)
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
