package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"strings"

	"plainsolid/geom"
)

const nestedMixedConicalAP242RepairFormat = "plainsolid-step-nested-mixed-conical-ap242-repair-source"

type nestedMixedConicalAP242RepairClaim struct {
	SourceSHA256, CorrectedSHA256 string
	LengthUnit, MaxDistance       float64
	Roots                         []int64
	Group                         AssemblyProduct
	Products                      []AssemblyProduct
	Parts                         [2]NestedMixedConicalAP242PartRepair
	Occurrences                   []nestedMixedConicalAP242OccurrenceClaim
	Instances                     []nestedMixedConicalAP242InstanceClaim
}

type nestedMixedConicalAP242OccurrenceClaim struct {
	Source ExactNAUOOccurrence
	Pose   [3][4]*big.Rat
}

type nestedMixedConicalAP242InstanceClaim struct {
	ProductID int64
	Path      [2]int64
	Pose      [3][4]*big.Rat
}

func nestedMixedConicalAP242RepairClaims(m RepairedNestedMixedConicalAP242Assembly) ([]byte, error) {
	a := m.assembly
	c := nestedMixedConicalAP242RepairClaim{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.original)), CorrectedSHA256: rationalPosedRepairDigest([]byte(m.corrected)),
		LengthUnit: a.Assembly.LengthUnit, MaxDistance: m.maxDistance, Roots: a.Assembly.Roots, Group: a.Group, Products: a.Assembly.Products,
		Parts: m.parts,
	}
	for _, u := range a.Occurrences {
		c.Occurrences = append(c.Occurrences, nestedMixedConicalAP242OccurrenceClaim{u, u.Matrix()})
	}
	for _, x := range a.Instances {
		c.Instances = append(c.Instances, nestedMixedConicalAP242InstanceClaim{x.ProductID, x.Path, x.Matrix()})
	}
	return json.Marshal(c)
}

// WriteRepairedNestedMixedConicalAP242JSON replays the original source
// and writes its immutable bytes and independently reconstructed claims.
func WriteRepairedNestedMixedConicalAP242JSON(ctx context.Context, w io.Writer, m RepairedNestedMixedConicalAP242Assembly) error {
	if ctx == nil || w == nil || m.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairNestedMixedConicalAP242Assembly(ctx, strings.NewReader(m.original), Limits{}, NestedMixedConicalAP242RepairOptions{m.maxDistance})
	if err != nil {
		return err
	}
	claim, err := nestedMixedConicalAP242RepairClaims(m)
	if err != nil {
		return err
	}
	want, err := nestedMixedConicalAP242RepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claim, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: nestedMixedConicalAP242RepairFormat, Version: 1, Source: []byte(m.original), MaxDistance: m.maxDistance, Claims: claim,
	})
}

// ReadRepairedNestedMixedConicalAP242JSON reauthenticates the hierarchy,
// replays both local repairs and requires every source-bound claim to match.
func ReadRepairedNestedMixedConicalAP242JSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RepairedNestedMixedConicalAP242Assembly, error) {
	var zero RepairedNestedMixedConicalAP242Assembly
	q, err := readRationalPosedRepairEnvelope(ctx, r, l, nestedMixedConicalAP242RepairFormat)
	if err != nil {
		return zero, err
	}
	m, err := RepairNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(q.Source), l.Source, NestedMixedConicalAP242RepairOptions{q.MaxDistance})
	if err != nil {
		return zero, err
	}
	claim, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := nestedMixedConicalAP242RepairClaims(m)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(claim, want) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
