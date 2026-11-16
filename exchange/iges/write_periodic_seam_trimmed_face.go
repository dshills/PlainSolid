package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"plainsolid/geom"
)

// WritePeriodicSeamTrimmedFace writes a canonical type-144 carrier for a
// face proved by ReadPeriodicSeamTrimmedFace. It re-proves the original
// and the output before publishing any bytes. The original directory IDs,
// directed UV/model pairs, lifted stations and exact model ranges are kept.
func WritePeriodicSeamTrimmedFace(ctx context.Context, w io.Writer, face PeriodicSeamTrimmedFace) error {
	if w == nil || face.original == "" {
		return geom.ErrInvalid
	}
	source, err := readPeriodicTrimSourceForAxes(ctx, strings.NewReader(face.original), Limits{}, 6, true)
	if err != nil {
		return err
	}
	proved, err := provePeriodicSeamTrimmedFace(ctx, source)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(face, proved) {
		return fmt.Errorf("iges: seam trim no longer matches its original proof: %w", geom.ErrUnrepresentable)
	}
	carrier, err := writePeriodicTrimRecords(source)
	if err != nil {
		return err
	}
	reopened, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(carrier), Limits{})
	if err != nil {
		return fmt.Errorf("iges: strict reopen of periodic seam trim: %w", err)
	}
	// Only the carrier bytes are new. The native controls, owner links,
	// oriented boundary and projection proof must all replay identically.
	reopened.original = proved.original
	if !reflect.DeepEqual(proved, reopened) {
		return fmt.Errorf("iges: periodic seam trim changed on strict reopen: %w", geom.ErrUnrepresentable)
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

func writePeriodicTrimRecords(source periodicTrimSource) ([]byte, error) {
	support, err := encodeExactPeriodicSurface128WithBounds(source.face.Support, source.support, &source.face.Bounds)
	if err != nil {
		return nil, err
	}
	// Compute standard globals and model extents through the ordinary
	// standalone surface writer. Only the indexed record graph is replaced.
	a := topologyAuthor{geometry: Model{Entities: []Entity{{Surface: source.support}}}}
	root, contour := 0, 10500
	for i := range len(source.graph.records) {
		id := 2*i + 1
		r := source.graph.records[id]
		data := support
		if r.kind != 128 {
			data, err = periodicTrimRecord(r)
			if err != nil {
				return nil, fmt.Errorf("iges: periodic trim owner #%d: %w", id, err)
			}
		}
		a.add(r.kind, 0, data)
		emitted := &a.records[i]
		emitted.label, emitted.subscript = r.label, r.subscript
		switch r.kind {
		case 144:
			emitted.status = &root
		case 142:
			emitted.status = &contour
		}
	}
	return a.file(source.face.LengthUnit)
}

// The strict graph proof already established the finite clamped layout, so
// each of its original coefficients can be emitted directly and exactly.
func periodicTrimRecord(r topologyRecord) (string, error) {
	count, integers := len(r.values), len(r.values)
	switch r.kind {
	case 144:
		holes, _ := intValue(r.values[3])
		count, integers = 5+holes, 5+holes
	case 142:
		count, integers = 6, 6
	case 102:
		edges, _ := intValue(r.values[1])
		count, integers = 2+edges, 2+edges
	case 110:
		count, integers = 7, 1
	case 126:
		upper, _ := intValue(r.values[1])
		count, integers = 7+6*(upper+1)+5, 7
	}
	tokens := make([]string, count, count+2)
	for i, p := range r.values[:integers] {
		n, err := intValue(p)
		if err != nil {
			return "", err
		}
		tokens[i] = strconv.Itoa(n)
	}
	exact := make([]*big.Rat, count-integers)
	for i, p := range r.values[integers:count] {
		q, err := igesPlanarExact(p)
		if err != nil {
			return "", err
		}
		exact[i] = q
		tokens[i+integers], err = periodic128Decimal(q)
		if err != nil {
			return "", err
		}
	}
	switch r.kind {
	case 110:
		axis := 0
		if exact[0].Cmp(exact[3]) == 0 {
			axis = 1
		}
		if err := periodicTrimTransferInterval(exact[axis], exact[axis+3]); err != nil {
			return "", err
		}
	case 126:
		n := (len(exact) - 5) / 6
		if _, err := periodicIGESTransferWeights(exact[2*n : 3*n]); err != nil {
			return "", err
		}
		if err := periodicTrimTransferInterval(exact[6*n], exact[6*n+1]); err != nil {
			return "", err
		}
	}
	return joinTopologyValues(append(tokens, "0", "0")), nil
}

func periodicTrimTransferInterval(a, b *big.Rat) error {
	x, _ := a.Float64()
	y, _ := b.Float64()
	ulp := func(f float64) float64 {
		return math.Abs(math.Nextafter(f, math.Copysign(math.Inf(1), f)) - f)
	}
	if math.Abs(x-y) <= max(ulp(x), ulp(y)) {
		return fmt.Errorf("iges: distinct periodic trim stations collapse in binary64 transfer: %w", geom.ErrUnrepresentable)
	}
	return nil
}
