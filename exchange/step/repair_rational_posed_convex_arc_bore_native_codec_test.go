package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestRationalPosedConvexArcThroughBoreJSON(t *testing.T) {
	slowtest.Skip(t)
	raw, err := os.ReadFile(tiltedBoreSource + "through-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	m := checkRationalPosedConvexArcBore(t, raw)
	ctx := context.Background()
	var encoded bytes.Buffer
	if err := WriteRationalPosedConvexArcThroughBoreJSON(ctx, &encoded, m); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRationalPosedConvexArcThroughBoreJSON(ctx, bytes.NewReader(encoded.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || replayed.OriginalSTEP() != string(raw) || replayed.CorrectedModelSTEP() != m.CorrectedModelSTEP() {
		t.Fatal("source-only native proof lost the original multi-loop graph", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedConvexArcThroughBoreJSON(ctx, &again, replayed); err != nil || !bytes.Equal(encoded.Bytes(), again.Bytes()) {
		t.Fatal("source-only native proof changed under revalidation", err)
	}
	for _, edit := range [][2]string{
		{`"PCurveID":503`, `"PCurveID":510`}, {`"BoundID":247`, `"BoundID":432`},
	} {
		forged := bytes.Replace(encoded.Bytes(), []byte(edit[0]), []byte(edit[1]), 1)
		if bytes.Equal(encoded.Bytes(), forged) {
			t.Fatal("source receipt omitted an original through-hole claim")
		}
		if out, err := ReadRationalPosedConvexArcThroughBoreJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil || out.OriginalSTEP() != "" {
			t.Fatal("forged bore owner claim produced material", err)
		}
	}
	for _, limits := range []IndependentMaterialJSONLimits{{MaxBytes: 500}, {Source: Limits{MaxFaces: 7}}} {
		if out, err := ReadRationalPosedConvexArcThroughBoreJSON(ctx, bytes.NewReader(encoded.Bytes()), limits); !errors.Is(err, geom.ErrConvergence) || out.OriginalSTEP() != "" {
			t.Fatal("exhausted source-only proof budget produced material", err)
		}
	}
	q, err := readRationalPosedPCurveRepairEnvelope(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	q.MaxDistance = 1e-13
	underBudget, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := ReadRationalPosedConvexArcThroughBoreJSON(ctx, bytes.NewReader(underBudget), IndependentMaterialJSONLimits{}); err == nil || out.OriginalSTEP() != "" {
		t.Fatal("changed repair budget produced material", err)
	}
	m.Bore.Rims[0].BoundID = 432
	var output bytes.Buffer
	if err := WriteRationalPosedConvexArcThroughBoreJSON(ctx, &output, m); !errors.Is(err, geom.ErrInvalid) || output.Len() != 0 {
		t.Fatal("native writer accepted altered source claims", err)
	}
}
