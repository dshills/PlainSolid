package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep"
	"plainsolid/geom"
	"reflect"
)

const occurrenceColorsNativeFormat = "plainsolid-step-flat-contextual-appearance"

type occurrenceColorNativeBody struct {
	SourceBodyID  int64
	SourceFaceIDs []int64
	Name          string
	Vertices      []geom.Vec3
	Edges         []brep.Edge
	Faces         []brep.Face
	SurfaceColor  *RGB
	FaceColors    map[int]RGB
}
type occurrenceColorNativeProduct struct {
	DefinitionID int64
	Bodies       []occurrenceColorNativeBody
}
type occurrenceColorsEnvelope struct {
	Format      string
	Version     int
	Source      string
	LengthUnit  float64
	Definitions []AssemblyProduct
	Roots       []int64
	Occurrences []AssemblyOccurrence
	Products    []occurrenceColorNativeProduct
	Overrides   []OccurrenceSurfaceColor
}

// occurrenceColorsClaims copies native boundary topology, not tessellation.
// It does not construct a solid from untrusted stored claims: the reader only
// returns the newly authenticated source model after comparing all claims.
func occurrenceColorsClaims(ctx context.Context, m OccurrenceColoredAssembly) (occurrenceColorsEnvelope, error) {
	if len(m.Overrides) == 0 {
		return occurrenceColorsEnvelope{}, geom.ErrInvalid
	}
	return occurrenceAppearanceClaims(ctx, m)
}

// The internal appearance comparison also permits a proved return to defaults;
// the public contextual persistence path above still requires an override.
func occurrenceAppearanceClaims(ctx context.Context, m OccurrenceColoredAssembly) (occurrenceColorsEnvelope, error) {
	fail := func() (occurrenceColorsEnvelope, error) { return occurrenceColorsEnvelope{}, geom.ErrInvalid }
	a := m.Assembly.Assembly
	if !independentNativeSourceRecordBound(a.Source, 4096, 8) || len(a.Products) < 1 || len(a.Products) > 16 || len(a.Roots) > 16 || len(a.Occurrences) > 4096 || len(m.Overrides) > 64 || len(m.Assembly.ProductBodies) != len(a.Products) {
		return fail()
	}
	raw, e := mappedPlanarSourceBytes(a.Source)
	if e != nil {
		return fail()
	}
	if len(raw) > 1<<20 {
		return fail()
	}
	out := occurrenceColorsEnvelope{Format: occurrenceColorsNativeFormat, Version: 1, Source: string(raw), LengthUnit: a.LengthUnit, Definitions: append([]AssemblyProduct(nil), a.Products...), Roots: append([]int64(nil), a.Roots...), Occurrences: append([]AssemblyOccurrence(nil), a.Occurrences...), Overrides: append([]OccurrenceSurfaceColor(nil), m.Overrides...)}
	totalBodies, vertices, edges, faces := 0, 0, 0, 0
	seen := map[int64]bool{}
	for i, p := range a.Products {
		if e = ctx.Err(); e != nil {
			return occurrenceColorsEnvelope{}, e
		}
		if seen[p.DefinitionID] || len(p.Representations) > 4096 || len(p.Name) > 32768 || len(p.Identifier) > 32768 || len(p.Description) > 32768 {
			return fail()
		}
		seen[p.DefinitionID] = true
		out.Definitions[i].Representations = append([]int64(nil), p.Representations...)
		bs, ok := m.Assembly.ProductBodies[p.DefinitionID]
		if !ok || len(bs) > 16 {
			return fail()
		}
		sourceIDs, _, e := a.productItems(p.DefinitionID)
		if e != nil || len(sourceIDs) != len(bs) {
			return fail()
		}
		product := occurrenceColorNativeProduct{DefinitionID: p.DefinitionID}
		for j, b := range bs {
			if len(b.Body.Name) > 32768 {
				return fail()
			}
			q := occurrenceColorNativeBody{SourceBodyID: sourceIDs[j], Name: b.Body.Name, Vertices: b.Body.Solid.Vertices(), Edges: b.Body.Solid.Edges(), Faces: b.Body.Solid.Faces()}
			totalBodies++
			vertices += len(q.Vertices)
			edges += len(q.Edges)
			faces += len(q.Faces)
			if totalBodies > 16 || vertices > 8192 || edges > 8192 || faces > 4096 {
				return fail()
			}
			q.SourceFaceIDs, e = bodyFaceIDs(a.Source, sourceIDs[j])
			if e != nil || len(q.SourceFaceIDs) != len(q.Faces) {
				return fail()
			}
			if b.SurfaceColor != nil {
				c := *b.SurfaceColor
				if !validRGB(c) {
					return fail()
				}
				q.SurfaceColor = &c
			}
			if b.FaceColors != nil {
				if len(b.FaceColors) > len(q.Faces) {
					return fail()
				}
				q.FaceColors = map[int]RGB{}
				for k, c := range b.FaceColors {
					if k < 0 || k >= len(q.Faces) || !validRGB(c) {
						return fail()
					}
					q.FaceColors[k] = c
				}
			}
			product.Bodies = append(product.Bodies, q)
		}
		out.Products = append(out.Products, product)
	}
	return out, nil
}

// WriteOccurrenceColoredAssemblyJSON stages a distinct version-one source-bound
// envelope. Original STEP is re-proved once and all supplied geometry, placement,
// product-local color and contextual ancestry claims must match that proof.
func WriteOccurrenceColoredAssemblyJSON(ctx context.Context, w io.Writer, m OccurrenceColoredAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	actual, e := occurrenceColorsClaims(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadOccurrenceColoredAssembly(bytes.NewBufferString(actual.Source), Limits{})
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	expected, e := occurrenceColorsClaims(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, expected) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(actual)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if len(raw) > 8<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// ReadOccurrenceColoredAssemblyJSON rebuilds the native planar geometry and
// appearance through the original bounded STEP reader exactly once. Stored
// native boundaries and claims are evidence to compare, never repair inputs.
// Cancellation is observed around the bounded legacy source proof, not inside it.
func ReadOccurrenceColoredAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (OccurrenceColoredAssembly, error) {
	var zero OccurrenceColoredAssembly
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
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	var q occurrenceColorsEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != occurrenceColorsNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Products) > 16 || len(q.Definitions) > 16 || len(q.Overrides) > 64 || len(q.Occurrences) > 4096 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadOccurrenceColoredAssembly(bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	expected, e := occurrenceColorsClaims(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
