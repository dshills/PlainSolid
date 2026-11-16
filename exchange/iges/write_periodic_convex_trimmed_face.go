package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"

	"plainsolid/geom"
)

// WritePeriodicConvexTrimmedFace emits a canonical carrier for a
// source-proved convex periodic trim. Both original replay and strict
// reopen prove the full affine edge compositions before any write.
func WritePeriodicConvexTrimmedFace(ctx context.Context, w io.Writer, face PeriodicConvexTrimmedFace) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 4, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicConvexTrimmedFace(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: convex periodic trim no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicConvexTransfer(proved, nil); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of convex periodic trim: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: convex periodic trim changed on strict reopen: %w", geom.ErrUnrepresentable)
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

func checkPeriodicConvexTransfer(f PeriodicConvexTrimmedFace, hole *PeriodicConvexTrimHole) error {
	boxes := [][2]geom.ExactParameterInterval{{f.KnotOwners[0].Domain, f.KnotOwners[1].Domain}}
	wires := [][4]PeriodicTrimEdge{f.Edges}
	if hole != nil {
		wires = append(wires, hole.Edges)
	}
	var points [2][4][2]*big.Rat
	for index, wire := range wires {
		for i, edge := range wire {
			var station [2]geom.ExactParameterInterval
			for axis, q := range edge.UVStart {
				x, _ := q.Float64()
				points[index][i][axis] = new(big.Rat).SetFloat64(x)
				if points[index][i][axis] == nil {
					return geom.ErrUnrepresentable
				}
				station[axis] = geom.ExactParameterInterval{Lower: q, Upper: q}
			}
			boxes = append(boxes, station)
		}
	}
	if err := checkPeriodicTrimTransferStations(nil, boxes...); err != nil {
		return err
	}
	if !periodicUVStrictlyConvex(points[0], 1) || hole != nil && !periodicUVStrictlyConvex(points[1], -1) {
		return fmt.Errorf("iges: convex periodic trim loses its original winding or convexity in binary64 transfer: %w", geom.ErrUnrepresentable)
	}
	if hole != nil && !periodicUVStrictlyContains(points[0], points[1]) {
		return fmt.Errorf("iges: convex trim hole loses strict containment in binary64 transfer: %w", geom.ErrUnrepresentable)
	}
	return nil
}
