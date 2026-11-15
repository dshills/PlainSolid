package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const exactLoopAssemblyNativeFormat = "plainsolid-step-exact-circular-loop-assembly"

type exactLoopAssemblyNativeEnvelope struct {
	Format      string
	Version     int
	Source      string
	Assembly    Assembly
	Body        exactLoopNativeEnvelope
	Occurrences []rationalHierarchyRelationClaim
}

func exactLoopAssemblyNativeClaim(ctx context.Context, m ExactCircularLoopAssembly) (exactLoopAssemblyNativeEnvelope, error) {
	return exactLoopAssemblyNativeClaimMode(ctx, m, false)
}
func exactLoopAssemblyNativeClaimMode(ctx context.Context, m ExactCircularLoopAssembly, nested bool) (exactLoopAssemblyNativeEnvelope, error) {
	products, minUses, maxUses := 2, 2, 16
	if nested {
		products, minUses, maxUses = 3, 3, 17
	}
	q := exactLoopAssemblyNativeEnvelope{Format: exactLoopAssemblyNativeFormat, Version: 1, Assembly: m.Assembly}
	if e := ctx.Err(); e != nil {
		return q, e
	}
	if !independentNativeSourceRecordBound(m.Assembly.Source, 4096, 16) || len(m.Assembly.Products) != products || len(m.Assembly.Roots) != 1 || len(m.Assembly.Occurrences) < minUses || len(m.Assembly.Occurrences) > maxUses || len(m.Occurrences) < minUses || len(m.Occurrences) > maxUses {
		return q, geom.ErrInvalid
	}
	for _, p := range m.Assembly.Products {
		if len(p.Identifier) > 4096 || len(p.Name) > 4096 || len(p.Description) > 4096 || len(p.Representations) != 1 {
			return q, geom.ErrInvalid
		}
	}
	for _, u := range m.Assembly.Occurrences {
		if len(u.Name) > 4096 || len(u.Identifier) > 4096 {
			return q, geom.ErrInvalid
		}
	}
	raw, e := mappedPlanarSourceBytes(m.Assembly.Source)
	if e != nil {
		return q, e
	}
	if len(raw) > 1<<20 {
		return q, geom.ErrConvergence
	}
	q.Source = string(raw)
	q.Assembly.Source = nil
	q.Body, e = exactLoopNativeClaim(ctx, m.Body.ExactCircularLoopExtrusionProduct, m.Body.Product, true)
	if e != nil {
		return q, e
	}
	if q.Body.Source != q.Source {
		return q, geom.ErrInvalid
	}
	// Body is a claim within this assembly, not a standalone vendor envelope.
	q.Body.Source = ""
	q.Body.Format = ""
	q.Body.Version = 0
	for _, r := range m.Occurrences {
		if len(r.Name) > 4096 {
			return q, geom.ErrInvalid
		}
		matrix, e := rationalHierarchyMatrixClaim(r.Matrix())
		if e != nil {
			return q, e
		}
		r.pose = mappedRationalBasis{}
		q.Occurrences = append(q.Occurrences, rationalHierarchyRelationClaim{r, matrix})
	}
	return q, ctx.Err()
}

// WriteExactCircularLoopAssemblyJSON stores the original source, one local
// native solid and all exact shared occurrence claims after full source reproof.
// It is a distinct version-one format, not an extension of standalone tags.
func WriteExactCircularLoopAssemblyJSON(ctx context.Context, w io.Writer, m ExactCircularLoopAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := exactLoopAssemblyNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadExactCircularLoopAssembly(ctx, bytes.NewBufferString(q.Source), Limits{})
	if e != nil {
		return e
	}
	want, e := exactLoopAssemblyNativeClaim(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, want) {
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

// ReadExactCircularLoopAssemblyJSON rebuilds from original STEP once and compares
// native local geometry, source role IDs, product ownership and exact rigid uses.
// Failed validation returns no partial material or occurrence result.
func ReadExactCircularLoopAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (ExactCircularLoopAssembly, error) {
	var zero ExactCircularLoopAssembly
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
	var q exactLoopAssemblyNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != exactLoopAssemblyNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadExactCircularLoopAssembly(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := exactLoopAssemblyNativeClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	q.Body.Native, e = independentCompact(q.Body.Native)
	if e != nil || !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
