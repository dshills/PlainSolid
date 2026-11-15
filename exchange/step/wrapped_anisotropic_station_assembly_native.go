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

const anisotropicAssemblyNativeFormat = "plainsolid-step-anisotropic-station-assembly"
const anisotropicBoreAssemblyNativeFormat = "plainsolid-step-anisotropic-station-bore-assembly"

type anisotropicAssemblySideClaim struct {
	Name       string
	CapNames   [2]string
	LengthUnit float64
	Walls      []weightedAssemblyWallClaim
}
type anisotropicAssemblyNativeEnvelope struct {
	Format        string
	Version       int
	Source        string
	Assembly      Assembly
	Product       AssemblyProduct
	SourceBodyID  int64
	SourceFaceIDs []int64
	Name          string
	CapNames      [2]string
	LengthUnit    float64
	Sides         []anisotropicAssemblySideClaim
	Native        json.RawMessage
	Occurrences   []rationalHierarchyRelationClaim
}

func anisotropicAssemblyNativeClaim(ctx context.Context, source WrappedAnisotropicAssemblySource, name string, caps [2]string, unit float64, sides []WrappedAnisotropicStationLoftBody, solid trimmed.Solid, bore bool) (anisotropicAssemblyNativeEnvelope, error) {
	q := anisotropicAssemblyNativeEnvelope{Format: anisotropicAssemblyNativeFormat, Version: 1, Assembly: source.Assembly, Product: source.Product, SourceBodyID: source.SourceBodyID, SourceFaceIDs: source.SourceFaceIDs, Name: name, CapNames: caps, LengthUnit: unit}
	count := 1
	if bore {
		q.Format = anisotropicBoreAssemblyNativeFormat
		count = 2
	}
	if e := ctx.Err(); e != nil {
		return q, e
	}
	if !independentNativeSourceRecordBound(source.Assembly.Source, 4096, 16) || len(source.Assembly.Products) != 2 || len(source.Assembly.Roots) != 1 || len(source.Assembly.Occurrences) < 2 || len(source.Assembly.Occurrences) > 16 || len(source.Occurrences) < 2 || len(source.Occurrences) > 16 || len(source.SourceFaceIDs) > 34 || len(sides) != count || len(name) > 4096 || len(caps[0]) > 4096 || len(caps[1]) > 4096 {
		return q, geom.ErrInvalid
	}
	for _, p := range source.Assembly.Products {
		if len(p.Identifier) > 4096 || len(p.Name) > 4096 || len(p.Description) > 4096 || len(p.Representations) != 1 {
			return q, geom.ErrInvalid
		}
	}
	for _, u := range source.Assembly.Occurrences {
		if len(u.Identifier) > 4096 || len(u.Name) > 4096 {
			return q, geom.ErrInvalid
		}
	}
	raw, e := mappedPlanarSourceBytes(source.Assembly.Source)
	if e != nil {
		return q, e
	}
	if len(raw) > 1<<20 {
		return q, geom.ErrConvergence
	}
	q.Source = string(raw)
	q.Assembly.Source = nil
	for _, side := range sides {
		if !anisotropicAssemblyNames(side.Name, side.CapNames, side.Walls) {
			return q, geom.ErrInvalid
		}
		c := anisotropicAssemblySideClaim{Name: side.Name, CapNames: side.CapNames, LengthUnit: side.LengthUnit}
		for _, wall := range side.Walls {
			if wall.Geometry == nil {
				return q, geom.ErrInvalid
			}
			u, v := wall.Geometry.FundamentalPatch().ControlShape()
			if u != 9 || v != 2 {
				return q, geom.ErrInvalid
			}
			recipe, e := wall.Geometry.ExportRecipe()
			if e != nil {
				return q, e
			}
			c.Walls = append(c.Walls, weightedAssemblyWallClaim{wall.Name, wall.SideEdges, wall.CornerVertices, recipe.Surface.Data(), recipe.Options.WrapU, recipe.Options.WrapV})
		}
		q.Sides = append(q.Sides, c)
	}
	if len(solid.Faces()) > 34 || len(solid.Edges()) > 256 || len(solid.Vertices()) > 256 {
		return q, geom.ErrInvalid
	}
	native, e := prismNativeBytes(solid)
	if e != nil {
		return q, e
	}
	q.Native, e = independentCompact(native)
	if e != nil {
		return q, e
	}
	for _, r := range source.Occurrences {
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
func anisotropicFilledClaim(ctx context.Context, m WrappedAnisotropicStationAssembly) (anisotropicAssemblyNativeEnvelope, error) {
	return anisotropicAssemblyNativeClaim(ctx, m.WrappedAnisotropicAssemblySource, m.Body.Name, m.Body.CapNames, m.Body.LengthUnit, []WrappedAnisotropicStationLoftBody{m.Body}, m.Body.Solid, false)
}
func anisotropicBoreClaim(ctx context.Context, m WrappedAnisotropicStationBoreAssembly) (anisotropicAssemblyNativeEnvelope, error) {
	return anisotropicAssemblyNativeClaim(ctx, m.WrappedAnisotropicAssemblySource, m.Body.Name, m.Body.CapNames, m.Body.LengthUnit, []WrappedAnisotropicStationLoftBody{m.Body.Outer, m.Body.Inner}, m.Body.Solid, true)
}

// WriteWrappedAnisotropicStationAssemblyJSON binds the complete original STEP,
// one local native material and all original support/occurrence claims by reproof.
func WriteWrappedAnisotropicStationAssemblyJSON(ctx context.Context, w io.Writer, m WrappedAnisotropicStationAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := anisotropicFilledClaim(ctx, m)
	if e != nil {
		return e
	}
	return writeAnisotropicAssemblyNative(ctx, w, q, false)
}

// WriteWrappedAnisotropicStationBoreAssemblyJSON uses a separate version-one tag
// retaining both original wall families and actual annular material claims.
func WriteWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, w io.Writer, m WrappedAnisotropicStationBoreAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := anisotropicBoreClaim(ctx, m)
	if e != nil {
		return e
	}
	return writeAnisotropicAssemblyNative(ctx, w, q, true)
}
func reproveAnisotropicAssemblyNative(ctx context.Context, source string, l Limits, bore bool) (WrappedAnisotropicStationAssembly, WrappedAnisotropicStationBoreAssembly, anisotropicAssemblyNativeEnvelope, error) {
	var filled WrappedAnisotropicStationAssembly
	var hole WrappedAnisotropicStationBoreAssembly
	var q anisotropicAssemblyNativeEnvelope
	var e error
	if bore {
		hole, e = ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewBufferString(source), l, WrappedSurfaceOptions{})
		if e == nil {
			q, e = anisotropicBoreClaim(ctx, hole)
		}
	} else {
		filled, e = ReadWrappedAnisotropicStationAssembly(ctx, bytes.NewBufferString(source), l, WrappedSurfaceOptions{})
		if e == nil {
			q, e = anisotropicFilledClaim(ctx, filled)
		}
	}
	return filled, hole, q, e
}
func writeAnisotropicAssemblyNative(ctx context.Context, w io.Writer, q anisotropicAssemblyNativeEnvelope, bore bool) error {
	_, _, want, e := reproveAnisotropicAssemblyNative(ctx, q.Source, Limits{}, bore)
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

// ReadWrappedAnisotropicStationAssemblyJSON regenerates from original source
// once; stored native geometry and graph/pose claims are never authorities.
func ReadWrappedAnisotropicStationAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (WrappedAnisotropicStationAssembly, error) {
	a, _, e := readAnisotropicAssemblyNative(ctx, r, l, false)
	return a, e
}

// ReadWrappedAnisotropicStationBoreAssemblyJSON re-proves complete original bore
// geometry, source ownership and every rigid use with atomic failure.
func ReadWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (WrappedAnisotropicStationBoreAssembly, error) {
	_, a, e := readAnisotropicAssemblyNative(ctx, r, l, true)
	return a, e
}
func readAnisotropicAssemblyNative(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, bore bool) (WrappedAnisotropicStationAssembly, WrappedAnisotropicStationBoreAssembly, error) {
	var a WrappedAnisotropicStationAssembly
	var h WrappedAnisotropicStationBoreAssembly
	if ctx == nil || r == nil {
		return a, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return a, h, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return a, h, geom.ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return a, h, e
	}
	if e = ctx.Err(); e != nil {
		return a, h, e
	}
	if int64(len(b)) > l.MaxBytes {
		return a, h, geom.ErrConvergence
	}
	var q anisotropicAssemblyNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	tag := anisotropicAssemblyNativeFormat
	if bore {
		tag = anisotropicBoreAssemblyNativeFormat
	}
	if d.Decode(&q) != nil || q.Format != tag || q.Version != 1 || len(q.Source) > 1<<20 {
		return a, h, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return a, h, geom.ErrInvalid
	}
	filled, hole, want, e := reproveAnisotropicAssemblyNative(ctx, q.Source, l.Source, bore)
	if e != nil {
		return a, h, e
	}
	q.Native, e = independentCompact(q.Native)
	if e != nil || !reflect.DeepEqual(q, want) {
		return a, h, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return a, h, e
	}
	return filled, hole, nil
}
