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

// WritePeriodicConcavePolygonTrimmedFace replays the original simple
// concave loop proof and strictly reopens the canonical carrier before
// writing. Winding, reflex turns and separation must survive binary64.
func WritePeriodicConcavePolygonTrimmedFace(ctx context.Context, w io.Writer, face PeriodicConcavePolygonTrimmedFace) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 0, periodicTrimAnyAxes)
	if err != nil {
		return err
	}
	proved, err := provePeriodicConcavePolygonTrimmedFace(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: concave trim no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	if err := checkPeriodicConcaveTransfer(proved); err != nil {
		return err
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of concave trim: %w", err)
	}
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: concave trim changed on strict reopen: %w", geom.ErrUnrepresentable)
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

func checkPeriodicConcaveTransfer(face PeriodicConcavePolygonTrimmedFace) error {
	wires, err := periodicRoundedAffineWires(face.KnotOwners, [][]PeriodicTrimEdge{face.Edges})
	if err != nil {
		return err
	}
	return checkPeriodicConcaveRoundedOuter(face.Edges, wires[0])
}

func checkPeriodicConcaveRoundedOuter(edges []PeriodicTrimEdge, rounded [][2]*big.Rat) error {
	if err := provePeriodicUVConcavity(rounded); err != nil {
		return fmt.Errorf("iges: concave trim loses its original simple winding in binary64 transfer: %w", err)
	}
	n := len(rounded)
	original := periodicAffineWireVertices(edges)
	for i, a := range original {
		j, k := (i+1)%n, (i+2)%n
		if periodicUVTurn(a, original[j], original[k]) != periodicUVTurn(rounded[i], rounded[j], rounded[k]) {
			return fmt.Errorf("iges: concave trim changes an original turn in binary64 transfer: %w", geom.ErrUnrepresentable)
		}
	}
	return nil
}
