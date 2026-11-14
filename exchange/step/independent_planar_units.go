package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// PlanarMaterialSIUnit selects the supported exact plane-length convention.
type PlanarMaterialSIUnit string

const (
	PlanarMetre      PlanarMaterialSIUnit = "metre"
	PlanarMillimetre PlanarMaterialSIUnit = "millimetre"
)

// NormalizedIndependentPlanarMaterials retains the original geometry and units
// separately from the losslessly normalized source graph and native bodies.
type NormalizedIndependentPlanarMaterials struct {
	tolerance                        float64
	OriginalSource, NormalizedSource *File
	Members                          []IndependentExactMaterial
	OriginalUnits                    []PlanarMaterialSIUnit
	TargetUnit                       PlanarMaterialSIUnit
}

// ReadNormalizedIndependentPlanarMaterials proves original strict cardinal-plane
// products, scales 3D and plane-UV points plus LINE vector magnitudes exactly,
// and re-proves complete model/pcurve ownership after normalization. Each curve
// retains its own parameter t; this does not assert identical source parameters
// between a model line and its independently parameterized pcurves.
// Curved/trimmed curve products and nonrepresentable scalars fail atomically.
func ReadNormalizedIndependentPlanarMaterials(ctx context.Context, r io.Reader, target PlanarMaterialSIUnit, l Limits, tol float64) (NormalizedIndependentPlanarMaterials, error) {
	m, e := readNormalizedIndependentUnits(ctx, r, RoundMaterialSIUnit(target), l, tol, true)
	if e != nil {
		return NormalizedIndependentPlanarMaterials{}, e
	}
	q := NormalizedIndependentPlanarMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: target, tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, PlanarMaterialSIUnit(u))
	}
	return q, nil
}

// A positive rational speed changes only the LINE parameter, never its infinite
// support or the unique finite segment between its two authenticated vertices.
// The proof view changes no point, direction, incidence, unit or source ID.
func planarUnitSourceProof(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentExactMaterialCollection, error) {
	bad := func() (IndependentExactMaterialCollection, error) {
		return IndependentExactMaterialCollection{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 16*131072)
	l.MaxEntities = min(l.MaxEntities, 16*1024)
	l.MaxValues = min(l.MaxValues, 16*16384)
	l.MaxDepth = min(l.MaxDepth, 16)
	source, e := Parse(r, l)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return bad()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	for id, en := range view.Entities {
		if e = ctx.Err(); e != nil {
			return IndependentExactMaterialCollection{}, e
		}
		for j, rec := range en.Records {
			if rec.Type != "VECTOR" {
				continue
			}
			if len(rec.Args) != 3 {
				return bad()
			}
			q, e := liftSourceRat(rec.Args[2])
			if e != nil || q.Sign() <= 0 || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
				return bad()
			}
			en.Records[j].Args[2] = Value{Kind: Number, Text: "1."}
		}
		view.Entities[id] = en
	}
	raw, e = mappedPlanarSourceBytes(view)
	if e != nil {
		return bad()
	}
	proof, e := readIndependentMaterialSelection(ctx, bytes.NewReader(raw), l, tol, 1, 16, true, nil, true, true, true)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	for i, m := range proof.Members {
		if m.Planar == nil {
			return bad()
		}
		part := &File{Header: source.Header, Order: append([]int64(nil), m.Source.Order...), Entities: map[int64]Entity{}}
		for _, id := range part.Order {
			part.Entities[id] = source.Entities[id]
		}
		proof.Members[i].Source = part
	}
	proof.Source = source
	return proof, nil
}
