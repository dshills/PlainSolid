package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// IndependentMaterialSIUnit is the explicit target of heterogeneous normalization.
type IndependentMaterialSIUnit string

const (
	IndependentMetre      IndependentMaterialSIUnit = "metre"
	IndependentMillimetre IndependentMaterialSIUnit = "millimetre"
)

type NormalizedIndependentMaterials struct {
	tolerance                        float64
	OriginalSource, NormalizedSource *File
	Members                          []IndependentExactMaterial
	OriginalUnits                    []IndependentMaterialSIUnit
	TargetUnit                       IndependentMaterialSIUnit
}

// ReadNormalizedIndependentMaterials accepts only strict planar products, exact
// model-only split cylinders and natural spheres. Each root's dimensional roles
// are validated independently under one complete shared ownership partition.
// Planar rational LINE speeds do not relax round-product scalar requirements.
func ReadNormalizedIndependentMaterials(ctx context.Context, r io.Reader, target IndependentMaterialSIUnit, l Limits, tol float64) (NormalizedIndependentMaterials, error) {
	m, e := readNormalizedIndependentUnits(ctx, r, RoundMaterialSIUnit(target), l, tol, false, true)
	if e != nil {
		return NormalizedIndependentMaterials{}, e
	}
	q := NormalizedIndependentMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: target, tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, IndependentMaterialSIUnit(u))
	}
	return q, nil
}

func readHeterogeneousUnitMaterial(ctx context.Context, f *File, l Limits, tol float64, torusMode ...bool) (IndependentExactMaterial, error) {
	bad := func() (IndependentExactMaterial, error) { return IndependentExactMaterial{}, geom.ErrInvalid }
	planar, faces := true, 0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "ADVANCED_FACE" {
			continue
		}
		faces++
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil || face.Args[2].Kind != Reference {
			return bad()
		}
		support := f.Entities[face.Args[2].Ref]
		if len(support.Records) != 1 || support.Records[0].Type != "PLANE" {
			planar = false
		}
	}
	if faces == 0 {
		return bad()
	}
	if !planar {
		m, e := readIndependentExactMaterial(ctx, f, l, tol, true, true, true, len(torusMode) == 1 && torusMode[0])
		if e != nil {
			return IndependentExactMaterial{}, e
		}
		if m.Cylinder == nil && m.Sphere == nil && m.Torus == nil {
			return bad()
		}
		return m, nil
	}
	// Only the exclusively owned planar leaf receives a speed-one proof view.
	// Directions/points/topology and every original source ID remain unchanged.
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return bad()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	for id, en := range view.Entities {
		if e = ctx.Err(); e != nil {
			return IndependentExactMaterial{}, e
		}
		for j, rec := range en.Records {
			if rec.Type != "VECTOR" {
				continue
			}
			if len(rec.Args) != 3 {
				return bad()
			}
			speed, e := liftSourceRat(rec.Args[2])
			if e != nil || speed.Sign() <= 0 || speed.Num().BitLen() > 256 || speed.Denom().BitLen() > 256 {
				return bad()
			}
			en.Records[j].Args[2] = Value{Kind: Number, Text: "1."}
		}
		view.Entities[id] = en
	}
	m, e := readIndependentExactMaterial(ctx, view, l, tol, true, false, false)
	if e != nil {
		return IndependentExactMaterial{}, e
	}
	if m.Planar == nil {
		return bad()
	}
	m.Source = f
	return m, nil
}
