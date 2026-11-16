package iges

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCorrectedConicalRimFilletBoreIGES(t *testing.T) {
	ctx := context.Background()
	original := conicalRimFilletBoreIGESFixture(t, "rounded")
	if _, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(original), Limits{}, geom.SurfacePatchOptions{}); err == nil {
		t.Fatal("strict reader accepted the uncorrected source")
	}
	p, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(original), Limits{}, ConicalRimFilletBoreRepairOptions{3e-11})
	if err != nil {
		t.Fatal(err)
	}
	corrected, ids, err := p.CorrectedModelIGES(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 58 {
		t.Fatalf("expected only three obsolete cone datum records to disappear, got %d entities", len(ids))
	}
	for id := 1; id < 116; id += 2 {
		want := id
		if id > 9 {
			want += 6
		}
		if ids[id] != want {
			t.Fatalf("corrected directory #%d has original #%d, want #%d", id, ids[id], want)
		}
	}
	prefix := func(src []byte) []byte {
		for i := 0; i+80 < len(src); i += 81 {
			if src[i+72] == 'D' {
				return src[:i]
			}
		}
		return nil
	}
	if !bytes.Equal(prefix(original), prefix(corrected)) || len(prefix(corrected)) == 0 {
		t.Fatal("corrected file changed the source Start or Global records")
	}
	doc, err := readTopologyRecordPolicyWithDirectory(bytes.NewReader(corrected), Limits{}, true, false, false, false, false, false, true, false, false, conicalRimBoreDirectory)
	if err != nil || len(doc.entities) != 58 || doc.entities[4].kind != 128 || doc.entities[4].id != 9 {
		t.Fatalf("corrected file did not reuse the original cone support slot: %v", err)
	}
	reopened, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(corrected), Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.SourceGroupID != 1 || reopened.SourceSolidID != 3 || reopened.SourceShellID != 5 ||
		reopened.SourceEdgeListID != 13 || reopened.SourceVertexListID != 17 ||
		!reflect.DeepEqual(reopened.NativeFaceSourceIDs, []int{7, 53, 77, 65, 91, 105}) {
		t.Fatalf("strict reopen lost the original owned entities: %v", reopened.NativeFaceSourceIDs)
	}
	originalNative, err := igesPrismBytes(p.Material().Solid)
	if err != nil {
		t.Fatal(err)
	}
	freshNative, err := igesPrismBytes(reopened.Solid)
	if err != nil || !bytes.Equal(freshNative, originalNative) {
		t.Fatalf("corrected reopen changed native bytes: %v", err)
	}

	var receipt bytes.Buffer
	if err := WriteRepairedConicalRimFilletBoreJSON(ctx, &receipt, p); err != nil {
		t.Fatal(err)
	}
	const priorReceiptSHA256 = "c5dccba2ac26b52d9e7216bf14fd8826027f9208b9f9a1e387f547c9775e6263"
	if got := fmt.Sprintf("%x", sha256.Sum256(receipt.Bytes())); got != priorReceiptSHA256 {
		t.Fatalf("corrected carrier changed the preexisting source-only receipt: %s", got)
	}
	restored, err := ReadRepairedConicalRimFilletBoreJSON(ctx, bytes.NewReader(receipt.Bytes()), ConicalRimFilletBoreRepairJSONLimits{})
	if err != nil || !bytes.Equal(restored.OriginalIGES(), original) {
		t.Fatalf("receipt failed to replay the original source: %v", err)
	}
	again, repeated, err := restored.CorrectedModelIGES(ctx)
	if err != nil || !bytes.Equal(corrected, again) || !reflect.DeepEqual(ids, repeated) {
		t.Fatalf("corrected carrier and directory mapping do not replay byte-stably: %v", err)
	}

	if oracle := os.Getenv("PLAINSOLID_OCCT_CONICAL_RIM_BORE_IGES_READER"); oracle != "" {
		var points [6]geom.Vec3
		for i, index := range reopened.NativeVertexSourceIndices {
			if index != 0 {
				points[index-1] = reopened.Solid.Vertices()[i]
			}
		}
		mass, err := reopened.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil {
			t.Fatal(err)
		}
		verifyConicalRimBoreIGESComparator(t, oracle, corrected, points, mass)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if raw, ownership, err := p.CorrectedModelIGES(canceled); !errors.Is(err, context.Canceled) || raw != nil || ownership != nil {
		t.Fatalf("canceled corrected carrier returned partial output: %v", err)
	}
}
