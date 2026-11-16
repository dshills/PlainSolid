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

const mixedConicalAP242DAGRepairFormat = "plainsolid-step-mixed-conical-ap242-dag-repair-source"

// MixedConicalAP242DAGRepairJSONLimits bounds the encoded receipt, the
// contained original STEP input and re-expansion of its occurrence paths.
type MixedConicalAP242DAGRepairJSONLimits struct {
	MaxBytes int64
	Source   MixedConicalAP242DAGLimits
}

type mixedConicalAP242DAGRepairClaim struct {
	SourceSHA256, CorrectedSHA256 string
	LengthUnit, MaxDistance       float64
	Roots                         []int64
	Products                      []AssemblyProduct
	Parts                         [2]NestedMixedConicalAP242PartRepair
	Occurrences                   []nestedMixedConicalAP242OccurrenceClaim
	Instances                     []mixedConicalAP242DAGInstanceClaim
}

type mixedConicalAP242DAGInstanceClaim struct {
	ProductID int64
	Path      []int64
	Pose      [3][4]*big.Rat
}

func mixedConicalAP242DAGRepairClaims(m RepairedMixedConicalAP242DAG) ([]byte, error) {
	a := m.assembly
	c := mixedConicalAP242DAGRepairClaim{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.original)), CorrectedSHA256: rationalPosedRepairDigest([]byte(m.corrected)),
		LengthUnit: a.Assembly.LengthUnit, MaxDistance: m.maxDistance, Roots: a.Assembly.Roots, Products: a.Assembly.Products, Parts: m.parts,
	}
	for _, u := range a.Occurrences {
		c.Occurrences = append(c.Occurrences, nestedMixedConicalAP242OccurrenceClaim{u, u.Matrix()})
	}
	for _, x := range a.Instances {
		c.Instances = append(c.Instances, mixedConicalAP242DAGInstanceClaim{x.ProductID, x.Path, x.Matrix()})
	}
	return json.Marshal(c)
}

// WriteRepairedMixedConicalAP242DAGJSON writes the original source and
// replayed repair and occurrence claims.
func WriteRepairedMixedConicalAP242DAGJSON(ctx context.Context, w io.Writer, m RepairedMixedConicalAP242DAG) error {
	if ctx == nil || w == nil || m.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairMixedConicalAP242DAG(ctx, strings.NewReader(m.original), MixedConicalAP242DAGLimits{}, MixedConicalAP242DAGRepairOptions{m.maxDistance})
	if err != nil {
		return err
	}
	claims, err := mixedConicalAP242DAGRepairClaims(m)
	if err != nil {
		return err
	}
	want, err := mixedConicalAP242DAGRepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claims, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: mixedConicalAP242DAGRepairFormat, Version: 1, Source: []byte(m.original), MaxDistance: m.maxDistance, Claims: claims,
	})
}

// ReadRepairedMixedConicalAP242DAGJSON reconstructs the original assembly
// repair and verifies all stored owner, pose and carrier claims.
func ReadRepairedMixedConicalAP242DAGJSON(ctx context.Context, r io.Reader, l MixedConicalAP242DAGRepairJSONLimits) (RepairedMixedConicalAP242DAG, error) {
	var zero RepairedMixedConicalAP242DAG
	q, err := readRationalPosedRepairEnvelope(ctx, r, IndependentMaterialJSONLimits{MaxBytes: l.MaxBytes, Source: l.Source.Source}, mixedConicalAP242DAGRepairFormat)
	if err != nil {
		return zero, err
	}
	m, err := RepairMixedConicalAP242DAG(ctx, bytes.NewReader(q.Source), l.Source, MixedConicalAP242DAGRepairOptions{q.MaxDistance})
	if err != nil {
		return zero, err
	}
	claims, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := mixedConicalAP242DAGRepairClaims(m)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(claims, want) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
