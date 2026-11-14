package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const weightedAssemblyNativeFormat = "plainsolid-step-wrapped-rational-station-assembly"

type weightedAssemblyWallClaim struct {
	Name                      string
	SideEdges, CornerVertices [4]int64
	Surface                   geom.BSplineSurfaceData
	WrapU, WrapV              bool
}
type weightedAssemblyNativeEnvelope struct {
	Format                           string
	Version                          int
	Source                           string
	Assembly                         Assembly
	Product                          AssemblyProduct
	SourceBodyID                     int64
	SourceFaceIDs, SourceWallFaceIDs []int64
	Name                             string
	CapNames                         [2]string
	LengthUnit                       float64
	Walls                            []weightedAssemblyWallClaim
	Native                           json.RawMessage
	Occurrences                      []rationalHierarchyRelationClaim
}

func weightedAssemblyNativeClaim(ctx context.Context, m WrappedRationalStationAssembly) (weightedAssemblyNativeEnvelope, error) {
	return weightedAssemblyNativeClaimBound(ctx, m, 16)
}
func weightedAssemblyNativeClaimBound(ctx context.Context, m WrappedRationalStationAssembly, maxWalls int) (weightedAssemblyNativeEnvelope, error) {
	q := weightedAssemblyNativeEnvelope{Format: weightedAssemblyNativeFormat, Version: 1, Assembly: m.Assembly, Product: m.Product, SourceBodyID: m.SourceBodyID, SourceFaceIDs: m.SourceFaceIDs, SourceWallFaceIDs: m.SourceWallFaceIDs, Name: m.Body.Name, CapNames: m.Body.CapNames, LengthUnit: m.Body.LengthUnit}
	if e := ctx.Err(); e != nil {
		return q, e
	}
	if !independentNativeSourceRecordBound(m.Assembly.Source, 4096, 8) || len(m.Body.Walls) < 1 || len(m.Body.Walls) > maxWalls || len(m.Occurrences) < 2 || len(m.Occurrences) > 16 || len(m.Assembly.Products) != 2 || len(m.Assembly.Occurrences) > 16 || len(m.SourceFaceIDs) > maxWalls+2 || len(m.SourceWallFaceIDs) > maxWalls {
		return q, geom.ErrInvalid
	}
	source, e := mappedPlanarSourceBytes(m.Assembly.Source)
	if e != nil {
		return q, e
	}
	if len(source) > 1<<20 {
		return q, geom.ErrConvergence
	}
	q.Source = string(source)
	q.Assembly.Source = nil
	for _, wall := range m.Body.Walls {
		if e = ctx.Err(); e != nil {
			return q, e
		}
		if wall.Geometry == nil || len(wall.Name) > 32768 {
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
		q.Walls = append(q.Walls, weightedAssemblyWallClaim{wall.Name, wall.SideEdges, wall.CornerVertices, recipe.Surface.Data(), recipe.Options.WrapU, recipe.Options.WrapV})
	}
	if len(m.Body.Solid.Faces()) > maxWalls+2 || len(m.Body.Solid.Edges()) > 256 || len(m.Body.Solid.Vertices()) > 256 {
		return q, geom.ErrInvalid
	}
	native, e := prismNativeBytes(m.Body.Solid)
	if e != nil {
		return q, e
	}
	q.Native, e = independentCompact(native)
	if e != nil {
		return q, e
	}
	for _, r := range m.Occurrences {
		matrix, e := rationalHierarchyMatrixClaim(r.Matrix())
		if e != nil {
			return q, e
		}
		r.pose = mappedRationalBasis{}
		q.Occurrences = append(q.Occurrences, rationalHierarchyRelationClaim{r, matrix})
	}
	return q, ctx.Err()
}

// WriteWrappedRationalStationAssemblyJSON stores one local native asset and the
// original STEP graph. A complete source reproof binds every support and use
// before any output. This version covers filled lofts only.
func WriteWrappedRationalStationAssemblyJSON(ctx context.Context, w io.Writer, m WrappedRationalStationAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := weightedAssemblyNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewBufferString(q.Source), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		return e
	}
	expected, e := weightedAssemblyNativeClaim(ctx, proof)
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

// ReadWrappedRationalStationAssemblyJSON re-proves the original local geometry
// and each source occurrence once, comparing all stored claims atomically.
// Native geometry is regenerated from STEP, never trusted from the payload.
func ReadWrappedRationalStationAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (WrappedRationalStationAssembly, error) {
	var zero WrappedRationalStationAssembly
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
	var q weightedAssemblyNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != weightedAssemblyNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewBufferString(q.Source), l.Source, WrappedSurfaceOptions{})
	if e != nil {
		return zero, e
	}
	expected, e := weightedAssemblyNativeClaim(ctx, m)
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
