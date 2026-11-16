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
	if err := checkPeriodicConvexTransfer(proved); err != nil {
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

func checkPeriodicConvexTransfer(f PeriodicConvexTrimmedFace) error {
	boxes := [5][2]geom.ExactParameterInterval{
		{f.KnotOwners[0].Domain, f.KnotOwners[1].Domain},
	}
	var points [4][2]*big.Rat
	for i, edge := range f.Edges {
		for axis, q := range edge.UVStart {
			x, _ := q.Float64()
			points[i][axis] = new(big.Rat).SetFloat64(x)
			if points[i][axis] == nil {
				return geom.ErrUnrepresentable
			}
			boxes[i+1][axis] = geom.ExactParameterInterval{Lower: q, Upper: q}
		}
	}
	if err := checkPeriodicTrimTransferStations(nil, boxes[:]...); err != nil {
		return err
	}
	if !periodicUVStrictlyConvex(points) {
		return fmt.Errorf("iges: convex periodic trim loses its original winding or convexity in binary64 transfer: %w", geom.ErrUnrepresentable)
	}
	return nil
}
