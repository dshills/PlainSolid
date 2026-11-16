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

// WritePeriodicConvexPolygonTrimmedFace replays the original polygon
// proof and strictly reopens the canonical carrier before any write.
func WritePeriodicConvexPolygonTrimmedFace(ctx context.Context, w io.Writer, face PeriodicConvexPolygonTrimmedFace) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 0, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicConvexPolygonTrimmedFace(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: convex polygon trim no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	wires := [][]PeriodicTrimEdge{proved.Outer.Edges}
	for _, hole := range proved.Holes {
		wires = append(wires, hole.Edges)
	}
	if err := checkPeriodicConvexWireTransfer(proved.KnotOwners, wires); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConvexPolygonTrimmedFace(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of convex polygon trim: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: convex polygon trim changed on strict reopen: %w", geom.ErrUnrepresentable)
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
