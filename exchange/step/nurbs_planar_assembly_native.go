package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nurbsPlanarAssemblyNativeFormat = "plainsolid-step-nurbs-planar-assembly"
const (
	nurbsNativeUnit    = "unit-cardinal"
	nurbsNativeScaled  = "scaled-cardinal"
	nurbsNativeOblique = "oblique-rigid"
)

// The geometry claims contain native boundary topology and appearance, not
// tessellation. Their Source is explicitly the mode's geometry proof source.
// Native construction always comes from OriginalSource through the typed codec.
type nurbsAssemblyGeometryClaims struct {
	Source      string
	LengthUnit  float64
	Definitions []AssemblyProduct
	Roots       []int64
	Occurrences []AssemblyOccurrence
	Products    []occurrenceColorNativeProduct
}
type nurbsUnitNativeClaims struct {
	Surfaces []NURBSPlanarSurfaceEvidence
	Curves   []NURBSPlanarCurveEvidence
	PCurves  []AffinePCurveEvidence
}
type nurbsScaledNativeClaims struct {
	NormalizedSource  string
	Charts            []NURBSPlanarChartScaling
	PCurves           []NURBSPlanarPCurveScaling
	ModelCurves       []NURBSPlanarCurveEvidence
	NormalizedPCurves []AffinePCurveEvidence
}
type nurbsObliqueNativeClaims struct {
	Frames []ObliqueNURBSProductFrame
	Points []ObliqueNURBSPointEvidence
}
type nurbsAssemblyEnvelope struct {
	Format         string
	Version        int
	Mode           string
	OriginalSource string
	Geometry       nurbsAssemblyGeometryClaims
	Unit           *nurbsUnitNativeClaims    `json:",omitempty"`
	Scaled         *nurbsScaledNativeClaims  `json:",omitempty"`
	Oblique        *nurbsObliqueNativeClaims `json:",omitempty"`
}

// A private discriminated result avoids an externally forgeable kind/payload
// union. Public read/write functions retain their distinct existing model types.
type nurbsNativeModel struct {
	unit    *NURBSPlanarAssembly
	scaled  *ScaledNURBSPlanarAssembly
	oblique *ObliqueNURBSPlanarAssembly
}

// WriteNURBSPlanarAssemblyJSON stores and re-proves the unit-cardinal model
// under a new version-one native tag. It rejects stale derived geometry,
// ownership, occurrence, appearance and source chart claims before output.
func WriteNURBSPlanarAssemblyJSON(ctx context.Context, w io.Writer, m NURBSPlanarAssembly) error {
	return writeNURBSAssemblyJSON(ctx, w, nurbsNativeModel{unit: &m})
}

// WriteScaledNURBSPlanarAssemblyJSON retains original and normalized chart evidence
// under the explicit scaled-cardinal mode of the new native format.
func WriteScaledNURBSPlanarAssemblyJSON(ctx context.Context, w io.Writer, m ScaledNURBSPlanarAssembly) error {
	return writeNURBSAssemblyJSON(ctx, w, nurbsNativeModel{scaled: &m})
}

// WriteObliqueNURBSPlanarAssemblyJSON retains original-coordinate native bodies
// and exact source-derived rational product frames under oblique-rigid mode.
func WriteObliqueNURBSPlanarAssemblyJSON(ctx context.Context, w io.Writer, m ObliqueNURBSPlanarAssembly) error {
	return writeNURBSAssemblyJSON(ctx, w, nurbsNativeModel{oblique: &m})
}

// ReadNURBSPlanarAssemblyJSON reconstructs exclusively from the retained source
// through one complete typed source proof. Stored geometry is compared, never
// used as a fallback or accepted by invoking another NURBS mode.
func ReadNURBSPlanarAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NURBSPlanarAssembly, error) {
	m, e := readNURBSAssemblyJSON(ctx, r, l, nurbsNativeUnit)
	if e != nil {
		return NURBSPlanarAssembly{}, e
	}
	return *m.unit, nil
}

// ReadScaledNURBSPlanarAssemblyJSON accepts only the scaled-cardinal mode and
// re-proves original chart scaling, ownership and native geometry.
func ReadScaledNURBSPlanarAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (ScaledNURBSPlanarAssembly, error) {
	m, e := readNURBSAssemblyJSON(ctx, r, l, nurbsNativeScaled)
	if e != nil {
		return ScaledNURBSPlanarAssembly{}, e
	}
	return *m.scaled, nil
}

// ReadObliqueNURBSPlanarAssemblyJSON accepts only oblique-rigid mode and
// re-proves every original/proof point and the restored native material.
func ReadObliqueNURBSPlanarAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (ObliqueNURBSPlanarAssembly, error) {
	m, e := readNURBSAssemblyJSON(ctx, r, l, nurbsNativeOblique)
	if e != nil {
		return ObliqueNURBSPlanarAssembly{}, e
	}
	return *m.oblique, nil
}

func nurbsAssemblyNativeClaims(ctx context.Context, m nurbsNativeModel) (nurbsAssemblyEnvelope, error) {
	var zero nurbsAssemblyEnvelope
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	count := 0
	var geometry ColoredAssembly
	q := nurbsAssemblyEnvelope{Format: nurbsPlanarAssemblyNativeFormat, Version: 1}
	if m.unit != nil {
		count++
		p := m.unit
		geometry = p.Geometry
		q.Mode = nurbsNativeUnit
		q.OriginalSource = p.original
		q.Unit = &nurbsUnitNativeClaims{p.Surfaces(), p.Curves(), p.PCurves()}
	}
	if m.scaled != nil {
		count++
		p := m.scaled
		geometry = p.Geometry
		q.Mode = nurbsNativeScaled
		q.OriginalSource = p.original
		q.Scaled = &nurbsScaledNativeClaims{p.normalized, p.ChartScalings(), p.PCurveScalings(), p.ModelCurves(), p.NormalizedPCurves()}
	}
	if m.oblique != nil {
		count++
		p := m.oblique
		geometry = p.Geometry
		q.Mode = nurbsNativeOblique
		q.OriginalSource = p.original
		q.Oblique = &nurbsObliqueNativeClaims{p.ProductFrames(), p.SourcePoints()}
	}
	if count != 1 || len(q.OriginalSource) == 0 || len(q.OriginalSource) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	// Reuse the bounded geometry/appearance claim copier, not its contextual
	// format or its proof dispatcher. NURBS modes do not admit contextual styles.
	a, e := occurrenceAppearanceClaims(ctx, OccurrenceColoredAssembly{Assembly: geometry})
	if e != nil {
		return zero, e
	}
	q.Geometry = nurbsAssemblyGeometryClaims{a.Source, a.LengthUnit, a.Definitions, a.Roots, a.Occurrences, a.Products}
	if !nurbsAssemblyEnvelopeBound(q) {
		return zero, geom.ErrInvalid
	}
	return q, nil
}
func nurbsAssemblyEnvelopeBound(q nurbsAssemblyEnvelope) bool {
	if q.Format != nurbsPlanarAssemblyNativeFormat || q.Version != 1 || len(q.OriginalSource) == 0 || len(q.OriginalSource) > 1<<20 || len(q.Geometry.Source) > 1<<20 || len(q.Geometry.Definitions) > 16 || len(q.Geometry.Products) > 16 || len(q.Geometry.Roots) > 16 || len(q.Geometry.Occurrences) > 4096 {
		return false
	}
	// Native fields are claims only. Count them before source work and comparison.
	nv, ne, nf, nb, uses := 0, 0, 0, 0, 0
	for _, p := range q.Geometry.Products {
		nb += len(p.Bodies)
		if nb > 16 {
			return false
		}
		for _, b := range p.Bodies {
			nv += len(b.Vertices)
			ne += len(b.Edges)
			nf += len(b.Faces)
			for _, face := range b.Faces {
				if len(face.Holes) > 4096 {
					return false
				}
				uses += len(face.Boundary)
				for _, hole := range face.Holes {
					uses += len(hole)
				}
				if uses > 16384 {
					return false
				}
			}
			if nv > 8192 || ne > 8192 || nf > 4096 || len(b.SourceFaceIDs) > 4096 || len(b.FaceColors) > 4096 {
				return false
			}
		}
	}
	switch q.Mode {
	case nurbsNativeUnit:
		return q.Unit != nil && q.Scaled == nil && q.Oblique == nil && len(q.Unit.Surfaces) > 0 && len(q.Unit.Surfaces) <= 4096 && len(q.Unit.Curves) > 0 && len(q.Unit.Curves) <= 4096 && len(q.Unit.PCurves) > 0 && len(q.Unit.PCurves) <= 4096
	case nurbsNativeScaled:
		return q.Unit == nil && q.Scaled != nil && q.Oblique == nil && len(q.Scaled.NormalizedSource) <= 1<<20 && len(q.Scaled.Charts) > 0 && len(q.Scaled.Charts) <= 4096 && len(q.Scaled.PCurves) > 0 && len(q.Scaled.PCurves) <= 4096 && len(q.Scaled.ModelCurves) > 0 && len(q.Scaled.ModelCurves) <= 4096 && len(q.Scaled.NormalizedPCurves) > 0 && len(q.Scaled.NormalizedPCurves) <= 4096
	case nurbsNativeOblique:
		return q.Unit == nil && q.Scaled == nil && q.Oblique != nil && len(q.Oblique.Frames) > 0 && len(q.Oblique.Frames) <= 16 && len(q.Oblique.Points) > 0 && len(q.Oblique.Points) <= 4096
	default:
		return false
	}
}
func reproveNURBSNative(ctx context.Context, mode, source string, l Limits) (nurbsNativeModel, error) {
	switch mode {
	case nurbsNativeUnit:
		m, e := ReadNURBSPlanarAssembly(ctx, bytes.NewBufferString(source), l)
		if e != nil {
			return nurbsNativeModel{}, e
		}
		return nurbsNativeModel{unit: &m}, nil
	case nurbsNativeScaled:
		m, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewBufferString(source), l)
		if e != nil {
			return nurbsNativeModel{}, e
		}
		return nurbsNativeModel{scaled: &m}, nil
	case nurbsNativeOblique:
		m, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewBufferString(source), l)
		if e != nil {
			return nurbsNativeModel{}, e
		}
		return nurbsNativeModel{oblique: &m}, nil
	default:
		return nurbsNativeModel{}, geom.ErrInvalid
	}
}
func writeNURBSAssemblyJSON(ctx context.Context, w io.Writer, m nurbsNativeModel) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	actual, e := nurbsAssemblyNativeClaims(ctx, m)
	if e != nil {
		return e
	}
	proof, e := reproveNURBSNative(ctx, actual.Mode, actual.OriginalSource, Limits{})
	if e != nil {
		return e
	}
	expected, e := nurbsAssemblyNativeClaims(ctx, proof)
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
func readNURBSAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, mode string) (nurbsNativeModel, error) {
	var zero nurbsNativeModel
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
	var q nurbsAssemblyEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Mode != mode || !nurbsAssemblyEnvelopeBound(q) {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := reproveNURBSNative(ctx, q.Mode, q.OriginalSource, l.Source)
	if e != nil {
		return zero, e
	}
	expected, e := nurbsAssemblyNativeClaims(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
