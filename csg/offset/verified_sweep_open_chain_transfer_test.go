package offset

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/exchange"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestVerifiedSweepOpenChainPlanarBRepTransfer(t *testing.T) {
	ctx := context.Background()
	const unit = .001
	for _, control := range []struct {
		name        string
		nonparallel bool
		volume      *big.Rat
	}{
		{"affine", false, big.NewRat(3, 2)},
		{"nonparallel", true, big.NewRat(41, 32)},
	} {
		t.Run(control.name, func(t *testing.T) {
			chain, cuts := authoredSweepBRepSource(t, control.nonparallel, false, big.NewRat(1, 8))
			result, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx, chain, VerifiedSweepOpenChainBRepOptions{})
			if err != nil {
				t.Fatal(err)
			}
			solid := result.Solid()

			var native bytes.Buffer
			if err := exchange.WriteBRepJSON(&native, solid); err != nil {
				t.Fatal(err)
			}
			reopened, report, err := exchange.ReadBRepJSONValidated(ctx, bytes.NewReader(native.Bytes()),
				exchange.BRepLimits{}, collision.AssemblyOptions{})
			if err != nil || report.SignedVolume.Cmp(control.volume) != 0 || len(report.Shells) != 1 {
				t.Fatal("native planar persistence changed material", report, err)
			}
			if !reflect.DeepEqual(reopened.Vertices(), solid.Vertices()) ||
				!reflect.DeepEqual(reopened.Edges(), solid.Edges()) || !reflect.DeepEqual(reopened.Faces(), solid.Faces()) {
				t.Fatal("native byte receipt changed indexed source ancestry")
			}
			var rewritten bytes.Buffer
			if err := exchange.WriteBRepJSON(&rewritten, reopened); err != nil || !bytes.Equal(rewritten.Bytes(), native.Bytes()) {
				t.Fatal("native rewrite changed canonical bytes", err)
			}
			// The generic planar payload preserves the exact index graph
			// used by the caller's original chart and corner ledger.
			result.solid = reopened
			checkOriginalSweepBRep(t, chain, result, cuts)

			var part21 bytes.Buffer
			stepInput := step.Model{LengthUnit: unit, Bodies: []step.Body{{Name: control.name, Solid: solid}}}
			stepOptions := step.VertexContactSTEPOptions{}
			stepProof, err := step.WriteVertexContactAssembly(ctx, &part21, stepInput, stepOptions)
			if err != nil || stepProof.SignedVolume.Cmp(control.volume) != 0 || len(stepProof.Contacts) != 0 {
				t.Fatal("exact planar STEP write changed single-shell material", stepProof, err)
			}
			if bytes.Count(part21.Bytes(), []byte("=ADVANCED_FACE(")) != 22 || bytes.Count(part21.Bytes(), []byte("=EDGE_CURVE(")) != 44 {
				t.Fatal("STEP writer changed the original planar face or edge partition")
			}
			model, stepReopen, err := step.ReadVertexContactAssembly(ctx, bytes.NewReader(part21.Bytes()), stepOptions)
			if err != nil || model.LengthUnit != unit || len(model.Bodies) != 1 ||
				stepReopen.SignedVolume.Cmp(control.volume) != 0 || len(stepReopen.Contacts) != 0 {
				t.Fatal("strict STEP reopen changed unit or solid count", stepReopen, err)
			}
			checkTransferredSweepPlanarBRep(t, solid, model.Bodies[0].Solid, control.volume)

			var type186 bytes.Buffer
			if err := iges.WritePlanarSolid(ctx, &type186, solid, unit, iges.PlanarSolidOptions{}); err != nil {
				t.Fatal(err)
			}
			igesModel, err := iges.ReadPlanarSolid(ctx, bytes.NewReader(type186.Bytes()), iges.PlanarSolidOptions{})
			if err != nil || igesModel.LengthUnit != unit {
				t.Fatal("IGES reopen changed unit or material", err)
			}
			checkTransferredSweepPlanarBRep(t, solid, igesModel.Solid, control.volume)
			t.Logf("native %d bytes; STEP %d bytes; IGES %d bytes; 24 vertices, 44 edges, 22 faces; exact volume %s",
				native.Len(), part21.Len(), type186.Len(), control.volume)
		})
	}
}

func checkTransferredSweepPlanarBRep(t *testing.T, original, transferred brep.Solid, exactVolume *big.Rat) {
	t.Helper()
	if transferred.Topology() != original.Topology() {
		t.Fatal("neutral transfer changed native topology", original.Topology(), transferred.Topology())
	}
	if !reflect.DeepEqual(sweepTransferOutwardQuads(t, original), sweepTransferOutwardQuads(t, transferred)) {
		t.Fatal("neutral transfer changed exact world vertices, directed edges or outward face partition")
	}
	locator, err := collision.NewPointLocator(context.Background(), transferred, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	shells := locator.Shells()
	if len(shells) != 1 || shells[0].SignedVolume.Cmp(exactVolume) != 0 || !locator.BoundaryCollision().Complete {
		t.Fatal("neutral transfer changed exact embedded material", shells)
	}
}

// Coordinate-keyed outward rings compare the transfer independently of
// numbering or the first coedge in each complete native quadrilateral.
func sweepTransferOutwardQuads(t *testing.T, solid brep.Solid) map[[4]geom.Vec3]bool {
	t.Helper()
	vertices, edges, faces := solid.Vertices(), solid.Edges(), solid.Faces()
	out := make(map[[4]geom.Vec3]bool, len(faces))
	for _, face := range faces {
		if len(face.Boundary) != 4 || len(face.Holes) != 0 {
			t.Fatal("neutral transfer disrupted original quadrilateral ancestry", face)
		}
		var wire [4]geom.Vec3
		first := 0
		for i, use := range face.Boundary {
			edge := edges[use.Edge]
			v := edge.Start
			if use.Reversed {
				v = edge.End
			}
			wire[i] = vertices[v]
			a, b := wire[i], wire[first]
			if a.X < b.X || (a.X == b.X && (a.Y < b.Y || (a.Y == b.Y && a.Z < b.Z))) {
				first = i
			}
		}
		var canonical [4]geom.Vec3
		for i := range canonical {
			canonical[i] = wire[(first+i)%len(wire)]
		}
		if out[canonical] {
			t.Fatal("neutral transfer duplicated an outward face", canonical)
		}
		out[canonical] = true
	}
	return out
}
