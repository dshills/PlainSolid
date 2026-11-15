package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const rationalAnisotropicBoreNativeFormat = "plainsolid-step-rational-anisotropic-station-bore-assembly"

type rationalAnisotropicBoreNativeEnvelope struct {
	Format        string
	Version       int
	Source        string
	Definitions   []AssemblyProduct
	Roots         []int64
	Product       AssemblyProduct
	SourceBodyID  int64
	SourceFaceIDs []int64
	Name          string
	CapNames      [2]string
	LengthUnit    float64
	Body          anisotropicAssemblySideClaim
	Inner         anisotropicAssemblySideClaim
	Native        json.RawMessage
	Occurrences   []rationalHierarchyRelationClaim
}

func rationalAnisotropicBoreNativeClaim(ctx context.Context, m RationalWrappedAnisotropicStationBoreAssembly) (rationalAnisotropicBoreNativeEnvelope, error) {
	q := rationalAnisotropicBoreNativeEnvelope{Format: rationalAnisotropicBoreNativeFormat, Version: 1, Definitions: m.Definitions, Roots: m.Roots, Product: m.Product, SourceBodyID: m.SourceBodyID, SourceFaceIDs: m.SourceFaceIDs, Name: m.Body.Name, CapNames: m.Body.CapNames, LengthUnit: m.Body.LengthUnit}
	if e := ctx.Err(); e != nil {
		return q, e
	}
	if !independentNativeSourceRecordBound(m.Source, 4096, 16) || len(m.Definitions) != 2 || len(m.Roots) != 1 || len(m.Occurrences) < 2 || len(m.Occurrences) > 16 || len(m.SourceFaceIDs) > 34 || (!anisotropicAssemblyNames(m.Body.Outer.Name, m.Body.Outer.CapNames, m.Body.Outer.Walls) || len(m.Body.Name) > 4096 || len(m.Body.CapNames[0]) > 4096 || len(m.Body.CapNames[1]) > 4096 || !anisotropicAssemblyNames(m.Body.Inner.Name, m.Body.Inner.CapNames, m.Body.Inner.Walls)) {
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
	q.Body = anisotropicAssemblySideClaim{Name: m.Body.Outer.Name, CapNames: m.Body.Outer.CapNames, LengthUnit: m.Body.Outer.LengthUnit}
	for _, w := range m.Body.Outer.Walls {
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
	q.Inner = anisotropicAssemblySideClaim{Name: m.Body.Inner.Name, CapNames: m.Body.Inner.CapNames, LengthUnit: m.Body.Inner.LengthUnit}
	for _, w := range m.Body.Inner.Walls {
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
		q.Inner.Walls = append(q.Inner.Walls, weightedAssemblyWallClaim{w.Name, w.SideEdges, w.CornerVertices, r.Surface.Data(), r.Options.WrapU, r.Options.WrapV})
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

// WriteRationalWrappedAnisotropicStationBoreAssemblyJSON authenticates the complete
// original STEP and all supplied local material, source and exact pose claims.
func WriteRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, w io.Writer, m RationalWrappedAnisotropicStationBoreAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := rationalAnisotropicBoreNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewBufferString(q.Source), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		return e
	}
	want, e := rationalAnisotropicBoreNativeClaim(ctx, proof)
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

// ReadRationalWrappedAnisotropicStationBoreAssemblyJSON re-proves source once and
// rejects forged material, occurrence ownership, rational matrices or metadata.
func ReadRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalWrappedAnisotropicStationBoreAssembly, error) {
	var zero RationalWrappedAnisotropicStationBoreAssembly
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
	var q rationalAnisotropicBoreNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != rationalAnisotropicBoreNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewBufferString(q.Source), l.Source, WrappedSurfaceOptions{})
	if e != nil {
		return zero, e
	}
	want, e := rationalAnisotropicBoreNativeClaim(ctx, m)
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
