package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const occtDirectedLoopNativeFormat = "plainsolid-step-occt-exact-directed-circular-loop-extrusion"
const directedLoopNativeLimit = 8 << 20

type occtDirectedLoopNativeEnvelope struct {
	Format               string
	Version              int
	Source               string
	Product              AssemblyProduct
	Name                 string
	Frame                geom.Affine
	Outer                []trimmed.ExactCircularLoopSegment
	Holes                [][]trimmed.ExactCircularLoopSegment
	Height, LengthUnit   float64
	Body, Shell          int64
	Caps                 [2]int64
	CapBounds            [2][]int64
	RingOffsets          []int
	Walls, VerticalEdges []int64
	Rims, Vertices       [2][]int64
	Native               json.RawMessage
}

func occtDirectedLoopNativeClaim(ctx context.Context, m OCCTExactDirectedCircularLoopExtrusion) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d := m.Descriptor
	if !independentNativeSourceRecordBound(m.Source, 4096, 16) || len(d.Name) > 4096 ||
		len(d.Outer) < 3 || len(d.Holes) > 15 || len(m.RingOffsets) != len(d.Holes)+2 ||
		len(m.Product.Identifier) > 4096 || len(m.Product.Name) > 4096 ||
		len(m.Product.Description) > 4096 || len(m.Product.Representations) != 1 {
		return nil, geom.ErrInvalid
	}
	n := len(d.Outer)
	for _, h := range d.Holes {
		if len(h) < 3 || len(h) > 64-n {
			return nil, geom.ErrInvalid
		}
		n += len(h)
	}
	if n > 64 || len(m.Walls) != n || len(m.VerticalEdges) != n ||
		len(m.Native.Faces()) != 2+n || len(m.Native.Edges()) != 3*n || len(m.Native.Vertices()) != 2*n {
		return nil, geom.ErrInvalid
	}
	for j := 0; j < 2; j++ {
		if len(m.Rims[j]) != n || len(m.Vertices[j]) != n || len(m.CapBounds[j]) != len(d.Holes)+1 {
			return nil, geom.ErrInvalid
		}
	}
	raw, err := mappedPlanarSourceBytes(m.Source)
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, geom.ErrConvergence
	}
	native, err := prismNativeBytes(m.Native)
	if err != nil {
		return nil, err
	}
	return json.Marshal(occtDirectedLoopNativeEnvelope{
		Format: occtDirectedLoopNativeFormat, Version: 1, Source: string(raw),
		Product: m.Product, Name: d.Name, Frame: frameTransform(d.Frame), Outer: d.Outer, Holes: d.Holes,
		Height: d.Height, LengthUnit: d.LengthUnit, Body: m.Body, Shell: m.Shell, Caps: m.Caps, CapBounds: m.CapBounds,
		RingOffsets: m.RingOffsets, Walls: m.Walls, VerticalEdges: m.VerticalEdges, Rims: m.Rims, Vertices: m.Vertices,
		Native: json.RawMessage(native),
	})
}

// WriteOCCTExactDirectedCircularLoopExtrusionJSON records the original STEP,
// cap-bound and edge identities, and exact native result after source reproof.
func WriteOCCTExactDirectedCircularLoopExtrusionJSON(ctx context.Context, w io.Writer, m OCCTExactDirectedCircularLoopExtrusion) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	claim, err := occtDirectedLoopNativeClaim(ctx, m)
	if err != nil {
		return err
	}
	if _, err = ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx, bytes.NewReader(claim), IndependentMaterialJSONLimits{}); err != nil {
		return err
	}
	claim = append(claim, '\n')
	if err = ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(claim)
	if err == nil && n != len(claim) {
		return io.ErrShortWrite
	}
	return err
}

// ReadOCCTExactDirectedCircularLoopExtrusionJSON rebuilds the normalized
// material and its incidence from the embedded original STEP. Other fields are
// validated claims. Insignificant whitespace may differ from writer output.
func ReadOCCTExactDirectedCircularLoopExtrusionJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (OCCTExactDirectedCircularLoopExtrusion, error) {
	var zero OCCTExactDirectedCircularLoopExtrusion
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = directedLoopNativeLimit
	}
	if l.MaxBytes < 1 || l.MaxBytes > directedLoopNativeLimit {
		return zero, geom.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var header struct {
		Format, Source string
		Version        int
	}
	if err = json.Unmarshal(raw, &header); err != nil || header.Format != occtDirectedLoopNativeFormat || header.Version != 1 || len(header.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	m, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewBufferString(header.Source), l.Source)
	if err != nil {
		return zero, err
	}
	want, err := occtDirectedLoopNativeClaim(ctx, m)
	if err != nil {
		return zero, err
	}
	actual, err := independentCompact(raw)
	if err != nil || !bytes.Equal(actual, want) {
		return zero, geom.ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
