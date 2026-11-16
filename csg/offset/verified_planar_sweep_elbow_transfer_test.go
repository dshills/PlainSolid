package offset

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/collision"
	"plainsolid/exchange"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestVerifiedPlanarSweepElbowTransfer(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	exactVolume := big.NewRat(9, 2)
	for _, control := range []struct {
		name  string
		shear bool
	}{
		{"perpendicular", false},
		{"oblique", true},
	} {
		t.Run(control.name, func(t *testing.T) {
			elbow := verifiedPlanarSweepElbowFixture(t, control.shear)
			boundary, err := PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow(ctx, elbow, VerifiedSweepOpenChainBRepOptions{})
			if err != nil {
				t.Fatal(err)
			}
			solid := boundary.Solid()

			var native bytes.Buffer
			if err := exchange.WriteBRepJSON(&native, solid); err != nil {
				t.Fatal(err)
			}
			reopened, report, err := exchange.ReadBRepJSONValidated(ctx, bytes.NewReader(native.Bytes()),
				exchange.BRepLimits{}, collision.AssemblyOptions{})
			if err != nil || len(report.Shells) != 1 || report.SignedVolume.Cmp(exactVolume) != 0 {
				t.Fatal("native reload changed the original elbow material", report, err)
			}
			if !reflect.DeepEqual(solid.Vertices(), reopened.Vertices()) ||
				!reflect.DeepEqual(solid.Edges(), reopened.Edges()) || !reflect.DeepEqual(solid.Faces(), reopened.Faces()) {
				t.Fatal("native reload changed original boundary indices")
			}
			var rewritten bytes.Buffer
			if err := exchange.WriteBRepJSON(&rewritten, reopened); err != nil || !bytes.Equal(rewritten.Bytes(), native.Bytes()) {
				t.Fatal("native rewrite differs from the original planar graph", err)
			}
			boundary.solid = reopened
			checkPlanarSweepElbowBRep(t, elbow, boundary, control.shear)

			var part21 bytes.Buffer
			stepInput := step.Model{LengthUnit: unit, Bodies: []step.Body{{Name: control.name, Solid: solid}}}
			stepOptions := step.VertexContactSTEPOptions{}
			stepProof, err := step.WriteVertexContactAssembly(ctx, &part21, stepInput, stepOptions)
			if err != nil || stepProof.SignedVolume.Cmp(exactVolume) != 0 || len(stepProof.Contacts) != 0 {
				t.Fatal("STEP write could not retain an exact single-shell elbow", stepProof, err)
			}
			if bytes.Count(part21.Bytes(), []byte("=ADVANCED_FACE(")) != 14 || bytes.Count(part21.Bytes(), []byte("=EDGE_CURVE(")) != 28 {
				t.Fatal("STEP writer changed the original face or edge partition")
			}
			model, proof, err := step.ReadVertexContactAssembly(ctx, bytes.NewReader(part21.Bytes()), stepOptions)
			if err != nil || model.LengthUnit != unit || len(model.Bodies) != 1 ||
				proof.SignedVolume.Cmp(exactVolume) != 0 || len(proof.Contacts) != 0 {
				t.Fatal("strict STEP reopen changed the original unit or material", proof, err)
			}
			checkTransferredSweepPlanarBRep(t, solid, model.Bodies[0].Solid, exactVolume)

			var type186 bytes.Buffer
			if err := iges.WritePlanarSolid(ctx, &type186, solid, unit, iges.PlanarSolidOptions{}); err != nil {
				t.Fatal(err)
			}
			igesModel, err := iges.ReadPlanarSolid(ctx, bytes.NewReader(type186.Bytes()), iges.PlanarSolidOptions{})
			if err != nil || igesModel.LengthUnit != unit {
				t.Fatal("strict IGES reopen changed the original unit or material", err)
			}
			checkTransferredSweepPlanarBRep(t, solid, igesModel.Solid, exactVolume)
			t.Logf("native %d bytes; STEP %d bytes; IGES %d bytes; 16V/28E/14F, exact volume %s",
				native.Len(), part21.Len(), type186.Len(), exactVolume)
		})
	}
}

func verifiedPlanarSweepElbowFixture(t *testing.T, shear bool) *geom.VerifiedPlanarNormalOffsetSweepElbow {
	t.Helper()
	pairs := authoredPlanarSweepElbow(t, shear, 1, 0)
	elbow, proof, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(context.Background(), pairs[0], pairs[1], geom.SurfacePatchOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("authored elbow did not verify", proof, err)
	}
	return elbow
}
