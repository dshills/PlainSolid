package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// RoundMaterialSIUnit is deliberately limited to metre and millimetre.
type RoundMaterialSIUnit string

const (
	RoundMetre      RoundMaterialSIUnit = "metre"
	RoundMillimetre RoundMaterialSIUnit = "millimetre"
)

// NormalizedIndependentRoundMaterials keeps original dimensional evidence apart
// from the normalized graph. IDs and product ownership remain unchanged.
type NormalizedIndependentRoundMaterials struct {
	OriginalSource, NormalizedSource *File
	Members                          []IndependentExactMaterial
	OriginalUnits                    []RoundMaterialSIUnit
	TargetUnit                       RoundMaterialSIUnit
}

// ReadNormalizedIndependentRoundMaterials accepts only already-proved model-only
// cylinder/natural-sphere products with SI metre or millimetre contexts. Original
// per-root dimensions are validated before exact scaling and complete reproof.
// Every new geometric scalar must remain binary64-exact. Uncertainty metadata
// remains exact decimal. Numeric work is capped at 256 bits/64 decimal places.
func ReadNormalizedIndependentRoundMaterials(ctx context.Context, r io.Reader, target RoundMaterialSIUnit, l Limits, tol float64) (NormalizedIndependentRoundMaterials, error) {
	bad := func() (NormalizedIndependentRoundMaterials, error) {
		return NormalizedIndependentRoundMaterials{}, geom.ErrInvalid
	}
	if target != RoundMetre && target != RoundMillimetre {
		return bad()
	}
	original, e := readIndependentMaterialSelection(ctx, r, l, tol, 1, 16, true, nil, true, true, true)
	if e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	raw, e := mappedPlanarSourceBytes(original.Source)
	if e != nil {
		return bad()
	}
	normalized, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	result := NormalizedIndependentRoundMaterials{OriginalSource: original.Source, TargetUnit: target}
	changed := map[int64]bool{}
	for _, member := range original.Members {
		if member.Cylinder == nil && member.Sphere == nil {
			return bad()
		}
		unitID := int64(0)
		from := RoundMaterialSIUnit("")
		for id, en := range member.Source.Entities {
			length := false
			for _, rec := range en.Records {
				if rec.Type == "LENGTH_UNIT" {
					length = true
				}
			}
			if !length {
				continue
			}
			if unitID != 0 {
				return bad()
			}
			unitID = id
			for _, rec := range en.Records {
				if rec.Type == "SI_UNIT" && len(rec.Args) == 2 && rec.Args[1].Kind == Enumeration && rec.Args[1].Text == "METRE" {
					if rec.Args[0].Kind == Omitted {
						from = RoundMetre
					} else if rec.Args[0].Kind == Enumeration && rec.Args[0].Text == "MILLI" {
						from = RoundMillimetre
					}
				}
			}
		}
		if unitID == 0 || from == "" {
			return bad()
		}
		result.OriginalUnits = append(result.OriginalUnits, from)
		ratio := big.NewRat(1, 1)
		if from != target {
			if from == RoundMetre {
				ratio.SetInt64(1000)
			} else {
				ratio.SetFrac64(1, 1000)
			}
		}
		scale := func(v *Value, geometry bool) error {
			if v.Kind != Number || len(v.Text) > 128 {
				return geom.ErrInvalid
			}
			q, ok := new(big.Rat).SetString(v.Text)
			if !ok || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
				return geom.ErrInvalid
			}
			q.Mul(q, ratio)
			if geometry {
				f, exact := q.Float64()
				if !exact || !finiteNumber(f) {
					return geom.ErrUnrepresentable
				}
			}
			text, e := extrusionDirectionDecimal(q)
			if e != nil {
				return e
			}
			v.Text = text
			return nil
		}
		for _, id := range member.Source.Order {
			if e = ctx.Err(); e != nil {
				return NormalizedIndependentRoundMaterials{}, e
			}
			en := normalized.Entities[id]
			if changed[id] { // Only previously proved shared neutral identity records can overlap.
				continue
			}
			changed[id] = true
			for j := range en.Records {
				rec := &en.Records[j]
				switch rec.Type {
				case "CARTESIAN_POINT":
					if len(rec.Args) != 2 || rec.Args[1].Kind != Aggregate || len(rec.Args[1].Items) != 3 {
						return bad()
					}
					for k := range rec.Args[1].Items {
						if e = scale(&rec.Args[1].Items[k], true); e != nil {
							return NormalizedIndependentRoundMaterials{}, e
						}
					}
				case "CIRCLE", "CYLINDRICAL_SURFACE", "SPHERICAL_SURFACE", "VECTOR":
					if len(rec.Args) != 3 {
						return bad()
					}
					if e = scale(&rec.Args[2], true); e != nil {
						return NormalizedIndependentRoundMaterials{}, e
					}
				case "UNCERTAINTY_MEASURE_WITH_UNIT":
					if len(rec.Args) != 4 || rec.Args[0].Kind != Typed || rec.Args[0].Text != "LENGTH_MEASURE" || len(rec.Args[0].Items) != 1 || rec.Args[1].Kind != Reference || rec.Args[1].Ref != unitID {
						return bad()
					}
					if e = scale(&rec.Args[0].Items[0], false); e != nil {
						return NormalizedIndependentRoundMaterials{}, e
					}
				case "SI_UNIT":
					if id == unitID {
						if target == RoundMetre {
							rec.Args[0] = Value{Kind: Omitted}
						} else {
							rec.Args[0] = Value{Kind: Enumeration, Text: "MILLI"}
						}
					}
				}
			}
			normalized.Entities[id] = en
		}
	}
	raw, e = mappedPlanarSourceBytes(normalized)
	if e != nil {
		return bad()
	}
	proof, e := readIndependentExactMaterialCollection(ctx, bytes.NewReader(raw), l, tol, 1, 16, true, true, true)
	if e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	result.NormalizedSource = proof.Source
	result.Members = proof.Members
	return result, nil
}
