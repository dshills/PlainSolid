package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestMappedSplineChainIndependentJointSamples(t *testing.T) {
	ctx := context.Background()
	original, closing, _, _ := mappedChainClosedFixture(t)
	knots := []float64{0, 0, 0, 0, 1, 1, 1, 1}
	var chains [2]MappedSplineChain
	for owner, chain := range original.sources {
		var uses []MappedSplineUV
		for i := 0; i+1 < len(original.cuts); i++ {
			cell, err := chain.cell(original.cuts[i], original.cuts[i+1])
			if err != nil {
				t.Fatal(err)
			}
			var poles []geom.Vec3
			controls := cell.HomogeneousControls()
			for _, p := range controls {
				if p[2].Cmp(controls[0][2]) != 0 {
					t.Fatal("expected one polynomial coedge")
				}
				x, err := geom.RepresentedExactScalar(new(big.Rat).Quo(p[0], p[2]))
				if err != nil {
					t.Fatal(err)
				}
				y, err := geom.RepresentedExactScalar(new(big.Rat).Quo(p[1], p[2]))
				if err != nil {
					t.Fatal(err)
				}
				poles = append(poles, geom.Vec3{X: x, Y: y})
			}
			c, err := geom.NewBSpline(3, poles, knots, nil)
			if err != nil {
				t.Fatal(err)
			}
			use, err := NewMappedSplineUV(ctx, SplineUV{Curve: c, Range: UVInterval{Max: 1}}, big.NewRat(1, 1), geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			uses = append(uses, use)
		}
		var err error
		chains[owner], err = NewMappedSplineEdgeChain(ctx, uses, original.cuts[1:len(original.cuts)-1], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	r, err := CertifyMappedSplineChainAgreement(ctx, original.supports[0], chains[0], original.supports[1], chains[1], edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := r.WriteJSON(ctx, &data, MappedChainNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	native := data.Bytes()
	loaded, err := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(native), MappedChainNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var replay bytes.Buffer
	if err := loaded.WriteJSON(ctx, &replay, MappedChainNativeOptions{}); err != nil || !bytes.Equal(native, replay.Bytes()) {
		t.Fatal("independent original bases did not survive native reload", err)
	}
	var malformed nativeMappedChainBoundary
	if err := json.Unmarshal(native, &malformed); err != nil {
		t.Fatal(err)
	}
	if malformed.Version != 2 || len(malformed.Owners[1].Sources) != 3 || malformed.Owners[1].Source != nil {
		t.Fatal("independent source recipe shape")
	}
	malformed.Owners[1].Sources[1] = malformed.Owners[1].Sources[0]
	changed, err := json.Marshal(malformed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(changed), MappedChainNativeOptions{}); err == nil {
		t.Fatal("alien coedge accepted")
	}
	mesh, err := loaded.TessellateFaceCertified(ctx, 0, closing[0], MappedChainFaceOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .005,
			Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .04},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	joins := 0
	for i, curr := range mesh.Samples[1:] {
		prev := mesh.Samples[i]
		if curr.Cell == prev.Cell {
			continue
		}
		if curr.Vertex != prev.Vertex || curr.ModelParameter.Cmp(prev.ModelParameter) != 0 {
			t.Fatal("joint samples must share the same model identity")
		}
		for owner := range 2 {
			if prev.AuthoredParameters[owner].Cmp(big.NewRat(1, 1)) != 0 || curr.AuthoredParameters[owner].Sign() != 0 ||
				prev.AuthoredParts[owner]+1 != curr.AuthoredParts[owner] {
				t.Fatal("joint lost the incoming or outgoing raw parameter", owner)
			}
		}
		joins++
	}
	if joins != 2 {
		t.Fatal("two original coedge joins expected", joins)
	}
}
