package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const rationalAnisotropicNativeFormat = "plainsolid-step-rational-anisotropic-station-assembly"

type rationalAnisotropicNativeEnvelope struct {
	Format        string
	Version       int
	Source        string
	Definitions   []AssemblyProduct
	Roots         []int64
	Product       AssemblyProduct
	SourceBodyID  int64
	SourceFaceIDs []int64
	Body          anisotropicAssemblySideClaim
	Native        json.RawMessage
	Occurrences   []rationalHierarchyRelationClaim
}

func rationalAnisotropicNativeClaim(ctx context.Context, m RationalWrappedAnisotropicStationAssembly, nestedMode ...bool) (rationalAnisotropicNativeEnvelope, error) {
	q := rationalAnisotropicNativeEnvelope{Format: rationalAnisotropicNativeFormat, Version: 1, Definitions: m.Definitions, Roots: m.Roots, Product: m.Product, SourceBodyID: m.SourceBodyID, SourceFaceIDs: m.SourceFaceIDs}
	if e := ctx.Err(); e != nil {
		return q, e
	}
	products, minimum, maximum := 2, 2, 16
	if len(nestedMode) == 1 && nestedMode[0] {
		products, minimum, maximum = 3, 3, 17
	}
	if !independentNativeSourceRecordBound(m.Source, 4096, 16) || len(m.Definitions) != products || len(m.Roots) != 1 || len(m.Occurrences) < minimum || len(m.Occurrences) > maximum || len(m.SourceFaceIDs) > 34 || !anisotropicAssemblyNames(m.Body.Name, m.Body.CapNames, m.Body.Walls) {
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
	q.Body = anisotropicAssemblySideClaim{Name: m.Body.Name, CapNames: m.Body.CapNames, LengthUnit: m.Body.LengthUnit}
	for _, w := range m.Body.Walls {
		if w.Geometry == nil {
			return q, geom.ErrInvalid
		}
		u, v := w.Geometry.FundamentalPatch().ControlShape()
		if u != 9 || v != 2 {
			return q, geom.ErrInvalid
		}
		r, e := w.Geometry.ExportRecipe()
		if e != nil {
			return q, e
		}
		q.Body.Walls = append(q.Body.Walls, weightedAssemblyWallClaim{w.Name, w.SideEdges, w.CornerVertices, r.Surface.Data(), r.Options.WrapU, r.Options.WrapV})
	}
	if len(m.Body.Solid.Faces()) > 34 || len(m.Body.Solid.Edges()) > 256 || len(m.Body.Solid.Vertices()) > 256 {
		return q, geom.ErrInvalid
	}
	b, e := prismNativeBytes(m.Body.Solid)
	if e != nil {
		return q, e
	}
	q.Native, e = independentCompact(b)
	if e != nil {
		return q, e
	}
	for _, u := range m.Occurrences {
		if len(u.Name) > 4096 {
			return q, geom.ErrInvalid
		}
		a, e := rationalHierarchyMatrixClaim(u.Matrix())
		if e != nil {
			return q, e
		}
		u.pose = mappedRationalBasis{}
		q.Occurrences = append(q.Occurrences, rationalHierarchyRelationClaim{u, a})
	}
	return q, ctx.Err()
}

// WriteRationalWrappedAnisotropicStationAssemblyJSON authenticates the complete
// original STEP and all supplied local material, source and exact pose claims.
func WriteRationalWrappedAnisotropicStationAssemblyJSON(ctx context.Context, w io.Writer, m RationalWrappedAnisotropicStationAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := rationalAnisotropicNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewBufferString(q.Source), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		return e
	}
	want, e := rationalAnisotropicNativeClaim(ctx, proof)
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
	if len(b) > 16<<20 {
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

// ReadRationalWrappedAnisotropicStationAssemblyJSON re-proves source once and
// rejects forged material, occurrence ownership, rational matrices or metadata.
func ReadRationalWrappedAnisotropicStationAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalWrappedAnisotropicStationAssembly, error) {
	var zero RationalWrappedAnisotropicStationAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
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
	var q rationalAnisotropicNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != rationalAnisotropicNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewBufferString(q.Source), l.Source, WrappedSurfaceOptions{})
	if e != nil {
		return zero, e
	}
	want, e := rationalAnisotropicNativeClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	q.Native, e = independentCompact(q.Native)
	if e != nil || !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
