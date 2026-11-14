package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

const exactLoopNativeFormat = "plainsolid-step-exact-circular-loop-extrusion"
const occtLoopNativeFormat = "plainsolid-step-occt-exact-circular-loop-extrusion"

type exactLoopNativeRecipe struct {
	Name               string
	Frame              geom.Affine
	Segments           []trimmed.ExactCircularLoopSegment
	Height, LengthUnit float64
}
type exactLoopNativeEnvelope struct {
	Format               string
	Version              int
	Source               string
	Recipe               exactLoopNativeRecipe
	Product              AssemblyProduct
	Body, Shell          int64
	Caps                 [2]int64
	Walls, VerticalEdges []int64
	Rims, Vertices       [2][]int64
	Native               json.RawMessage
}

func exactLoopNativeClaim(ctx context.Context, m ExactCircularLoopExtrusionProduct, product AssemblyProduct, vendor bool) (exactLoopNativeEnvelope, error) {
	q := exactLoopNativeEnvelope{Format: exactLoopNativeFormat, Version: 1, Product: product, Body: m.Body, Shell: m.Shell, Caps: m.Caps, Walls: m.Walls, VerticalEdges: m.VerticalEdges, Rims: m.Rims, Vertices: m.Vertices, Recipe: exactLoopNativeRecipe{m.Descriptor.Name, frameTransform(m.Descriptor.Frame), m.Descriptor.Segments, m.Descriptor.Height, m.Descriptor.LengthUnit}}
	if vendor {
		q.Format = occtLoopNativeFormat
	}
	if e := ctx.Err(); e != nil {
		return q, e
	}
	if !independentNativeSourceRecordBound(m.Source, 4096, 16) || len(m.Descriptor.Name) > 4096 || len(m.Descriptor.Segments) < 3 || len(m.Descriptor.Segments) > 64 || !planarSplineRigid(q.Recipe.Frame) || len(m.Walls) > 64 || len(m.VerticalEdges) > 64 || len(product.Representations) > 1 {
		return q, geom.ErrInvalid
	}
	for j := 0; j < 2; j++ {
		if len(m.Rims[j]) > 64 || len(m.Vertices[j]) > 64 {
			return q, geom.ErrInvalid
		}
	}
	if len(m.Native.Faces()) > 66 || len(m.Native.Edges()) > 192 || len(m.Native.Vertices()) > 128 {
		return q, geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(m.Source)
	if e != nil {
		return q, e
	}
	if len(raw) > 1<<20 {
		return q, geom.ErrConvergence
	}
	q.Source = string(raw)
	native, e := prismNativeBytes(m.Native)
	if e != nil {
		return q, e
	}
	q.Native, e = independentCompact(native)
	if e != nil {
		return q, e
	}
	return q, ctx.Err()
}
func exactLoopNativeReproof(ctx context.Context, source string, l Limits, vendor bool) (OCCTExactCircularLoopExtrusion, error) {
	if vendor {
		return ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewBufferString(source), l)
	}
	m, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewBufferString(source), l)
	return OCCTExactCircularLoopExtrusion{ExactCircularLoopExtrusionProduct: m}, e
}

// WriteExactCircularLoopExtrusionJSON records the canonical signed recipe,
// exact native solid and original source roles after a complete source reproof.
func WriteExactCircularLoopExtrusionJSON(ctx context.Context, w io.Writer, m ExactCircularLoopExtrusionProduct) error {
	return writeExactLoopNative(ctx, w, m, AssemblyProduct{}, false)
}

// WriteOCCTExactCircularLoopExtrusionJSON preserves the distinct vendor proof,
// including original product metadata and positive-height normalized recipe.
func WriteOCCTExactCircularLoopExtrusionJSON(ctx context.Context, w io.Writer, m OCCTExactCircularLoopExtrusion) error {
	return writeExactLoopNative(ctx, w, m.ExactCircularLoopExtrusionProduct, m.Product, true)
}
func writeExactLoopNative(ctx context.Context, w io.Writer, m ExactCircularLoopExtrusionProduct, p AssemblyProduct, vendor bool) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := exactLoopNativeClaim(ctx, m, p, vendor)
	if e != nil {
		return e
	}
	proof, e := exactLoopNativeReproof(ctx, q.Source, Limits{}, vendor)
	if e != nil {
		return e
	}
	expected, e := exactLoopNativeClaim(ctx, proof.ExactCircularLoopExtrusionProduct, proof.Product, vendor)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, expected) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 8<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadExactCircularLoopExtrusionJSON rebuilds only from the original canonical
// STEP. Stored geometry, signed frame/recipe and role identities are claims.
func ReadExactCircularLoopExtrusionJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (ExactCircularLoopExtrusionProduct, error) {
	m, e := readExactLoopNative(ctx, r, l, false)
	return m.ExactCircularLoopExtrusionProduct, e
}

// ReadOCCTExactCircularLoopExtrusionJSON re-proves the genuine source through
// its exact vendor topology gate, never the canonical grammar or repair paths.
func ReadOCCTExactCircularLoopExtrusionJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (OCCTExactCircularLoopExtrusion, error) {
	return readExactLoopNative(ctx, r, l, true)
}
func readExactLoopNative(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, vendor bool) (OCCTExactCircularLoopExtrusion, error) {
	var zero OCCTExactCircularLoopExtrusion
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 8 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 8<<20 {
		return zero, geom.ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	if int64(len(b)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var q exactLoopNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	format := exactLoopNativeFormat
	if vendor {
		format = occtLoopNativeFormat
	}
	if d.Decode(&q) != nil || q.Format != format || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := exactLoopNativeReproof(ctx, q.Source, l.Source, vendor)
	if e != nil {
		return zero, e
	}
	expected, e := exactLoopNativeClaim(ctx, m.ExactCircularLoopExtrusionProduct, m.Product, vendor)
	if e != nil {
		return zero, e
	}
	q.Native, e = independentCompact(q.Native)
	if e != nil || !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}

// WriteExactCircularLoopExtrusionSource re-proves the original canonical graph
// before preserving its metadata, role IDs and exact source numeric tokens.
func WriteExactCircularLoopExtrusionSource(ctx context.Context, w io.Writer, f *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return geom.ErrConvergence
	}
	if _, e = ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(b), l); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
