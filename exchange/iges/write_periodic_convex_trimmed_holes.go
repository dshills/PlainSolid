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

// WritePeriodicConvexTrimmedFaceWithHoles emits the canonical carrier for
// two source-proved disjoint affine holes. Original contour order and all
// directed source pairs must survive exact replay and strict reopen.
func WritePeriodicConvexTrimmedFaceWithHoles(ctx context.Context, w io.Writer, face PeriodicConvexTrimmedFaceWithHoles) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 4, periodicTrimAnyAxes, []int{4, 4})
	if err != nil {
		return err
	}
	proved, err := provePeriodicConvexTrimmedFaceWithHoles(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: convex periodic trim with holes no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicConvexTransfer(proved.PeriodicConvexTrimmedFace, proved.Holes[:]...); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConvexTrimmedFaceWithHoles(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of convex periodic trim with holes: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: convex periodic trim with holes changed on strict reopen: %w", geom.ErrUnrepresentable)
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
