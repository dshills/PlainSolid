package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestOCCTMappedChainRectangularAsymmetricJoinedIndependentRoofs(t *testing.T) {
	slowtest.Skip(t)
	verifyThreeJoinedRoofSource(t, context.Background(), readInferredSharedMappedChainFixture(t, asymmetricJoinedRoofPath), threeJoinedAsymmetric)
}

func TestOCCTMappedChainRectangularAsymmetricJoinedIndependentRoofsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	verifyThreeJoinedRoofDisplay(t, readInferredSharedMappedChainFixture(t, asymmetricJoinedRoofPath), 3, .14)
}

func TestOCCTMappedChainRectangularAsymmetricJoinedIndependentRoofsRejectsMissingJoint(t *testing.T) {
	t.Parallel()
	raw := readInferredSharedMappedChainFixture(t, asymmetricJoinedRoofPath)
	bad := readInferredSharedMappedChainFixture(t, asymmetricJoinedMissingJointPath)
	rejectAsymmetricJoinedRoofMissingJoint(t, context.Background(), raw, bad)
}

func rejectAsymmetricJoinedRoofMissingJoint(t *testing.T, ctx context.Context, raw, bad []byte) {
	t.Helper()
	o := threeJoinedRoofOptions()
	got, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(bad), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || len(got.Native.ChainEdges) != 0 || len(got.SupportJoints) != 0 || len(got.Crossings) != 0 {
		t.Fatal("the independently moved south support knot must reject atomically", err)
	}
	rejectChangedJoinedRationalNativeSupportWithOptions(t, ctx, raw, bad, o, false, "support knot must belong")
}

func TestOCCTMappedChainRectangularAsymmetricJoinedIndependentRoofsRejectsFalseMasks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := readInferredSharedMappedChainFixture(t, asymmetricJoinedRoofPath)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, threeJoinedRoofOptions())
	if err != nil {
		t.Fatal(err)
	}
	o := trimmed.MappedChainShellNativeOptions{}
	var native bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &native, o); err != nil {
		t.Fatal(err)
	}
	field := func(changes any) []byte {
		t.Helper()
		raw, err := json.Marshal(changes)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte(`,"JointSupportChanges":`), raw...)
	}
	receipt := field(m.Shell.OriginalJointSupportChanges())
	if bytes.Count(native.Bytes(), receipt) != 1 {
		t.Fatal("native v12 must record both exact joint owner masks")
	}
	for _, tc := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"false shared joint", bytes.Replace(native.Bytes(), receipt, field([][2]bool{{true, true}, {true, true}}), 1), geom.ErrConvergence},
		{"missing masks", bytes.Replace(native.Bytes(), receipt, nil, 1), geom.ErrInvalid},
		{"legacy version",
			bytes.Replace(bytes.Replace(native.Bytes(), receipt, nil, 1), []byte(`"Version":12`), []byte(`"Version":11`), 1), geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(tc.raw), o)
			if !errors.Is(err, tc.want) || got.Topology().Faces != 0 {
				t.Fatal("native reload must reject the false source declaration", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularAsymmetricJoinedIndependentRoofsLive(t *testing.T) {
	t.Parallel()
	verifyPinnedThreeJoinedRoofs(t, threeJoinedAsymmetric)
}
