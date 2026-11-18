package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestNativeNestedMixedCavityDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	outer := testWarpedSplinePrism(t, false, false)
	inner, _ := makeOpenSplineBox(t, [3]float64{.5, .5, .25}, [3]float64{1.5, 1.5, .75}, 0)
	body, h, e := CombineSeparatedSplineShells(ctx, outer, inner, parametric.Difference, SplineShellBooleanOptions{})
	if e != nil || !h.BInsideA {
		t.Fatal("original material", e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSON(bytes.NewReader(raw.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	options := NestedMixedDisplayOptions{Meshing: StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(body.faces))}, Validation: collision.LocationOptions{MaxTriangles: 100000, MaxCollisionPairs: 1000000}}
	for i := range options.Meshing.Faces {
		options.Meshing.Faces[i] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}
	}
	prepared, e := body.PrepareNestedMixedDisplay(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	nesting := prepared.Nesting()
	if len(nesting) != 2 || nesting[0].Parent != -1 || nesting[0].Depth != 0 || nesting[1].Parent != 0 || nesting[1].Depth != 1 {
		t.Fatal("source nesting", nesting)
	}
	gaps := prepared.Clearances()
	if len(gaps) != 1 || gaps[0].FacePairs != 36 || gaps[0].SquaredGapLower.Cmp(gaps[0].RequiredSquaredGap) <= 0 {
		t.Fatal("source/error clearance", gaps)
	}
	// Independent source support clearance: the inner box is at least1/4 from
	// every outer full face box, including the curved roof whose minimum is1.
	if gaps[0].SquaredGapLower.Cmp(r(1, 16)) > 0 {
		t.Fatal("nonconservative source gap")
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	world, e := prepared.Place(ctx, pose, placement.ClosedDisplayOptions{Validation: options.Validation})
	if e != nil {
		t.Fatal(e)
	}
	shells := world.Display().Shells()
	if len(shells) != 2 || shells[1].Depth != 1 || shells[1].SignedVolume.Sign() != -1 || shells[0].SignedVolume.Sign() != 1 {
		t.Fatal("cavity orientation")
	}
	if math.Abs(world.Display().Mesh().SignedVolume()-4.5) > .05 {
		t.Fatal("independent volume 5-.5")
	}
	owners := world.Display().Faces()
	allowed := map[int]bool{}
	for _, face := range nesting[1].Faces {
		allowed[face] = true
	}
	for _, tri := range shells[1].Triangles {
		if !allowed[owners[tri]] {
			t.Fatal("inward original face lost")
		}
	}
	ancestry := world.Ancestry()
	if len(ancestry.EdgeVertices) != len(body.edges) || len(ancestry.FaceVertices) != len(body.faces) {
		t.Fatal("native identity")
	}
	if world.SquaredDistance().Cmp(prepared.SquaredDistance()) < 0 {
		t.Fatal("error bound")
	}
	gaps[0].SquaredGapLower.SetInt64(-1)
	nesting[1].Faces[0] = -1
	if prepared.Clearances()[0].SquaredGapLower.Sign() <= 0 || prepared.Nesting()[1].Faces[0] < 0 {
		t.Fatal("mutable source proof")
	}
	// The represented shell still embeds after this large exact translation,
	// but its conservative rounding allowance consumes the original gap budget.
	// The native-source bridge must reject even though generic display passes.
	shift := new(big.Rat).Add(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 46)), r(1, 128))
	coarsePose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), shift}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := prepared.prepared.display.Place(ctx, coarsePose, placement.ClosedDisplayOptions{Validation: options.Validation}); e != nil {
		t.Fatal("generic rounded embedding", e)
	}
	if got, e := prepared.Place(ctx, coarsePose, placement.ClosedDisplayOptions{Validation: options.Validation}); !errors.Is(e, geom.ErrConvergence) || got != nil {
		t.Fatal("world source clearance", e)
	}
	// Add a material island inside the original void; this is a third boundary
	// component with depth2, not a collection of unrelated positive solids.
	island, _ := makeOpenSplineBox(t, [3]float64{.75, .75, .375}, [3]float64{1.25, 1.25, .625}, 0)
	material, _, e := CombineNestedSplineComponents(ctx, body, island, parametric.Union, SplineShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	islandOptions := options
	islandOptions.Meshing.Faces = make([]SolidMixedFaceMeshOptions, len(material.faces))
	for i := range islandOptions.Meshing.Faces {
		islandOptions.Meshing.Faces[i] = options.Meshing.Faces[0]
	}
	three, e := material.PrepareNestedMixedDisplay(ctx, islandOptions)
	if e != nil {
		t.Fatal("island", e)
	}
	if len(three.Nesting()) != 3 || three.Nesting()[2].Parent != 1 || three.Nesting()[2].Depth != 2 || len(three.Clearances()) != 3 {
		t.Fatal("island source proof")
	}
	image, e := three.Place(ctx, pose, placement.ClosedDisplayOptions{Validation: options.Validation})
	if e != nil {
		t.Fatal(e)
	}
	if len(image.Display().Shells()) != 3 || image.Display().Shells()[2].Depth != 2 || image.Display().Shells()[2].SignedVolume.Sign() != 1 || math.Abs(image.Display().Mesh().SignedVolume()-(4.5+1./16)) > .05 {
		t.Fatal("island display/material volume")
	}
	for _, which := range []string{"pairs", "triangles", "collision"} {
		limited := options
		switch which {
		case "pairs":
			limited.MaxFacePairs = 35
		case "triangles":
			limited.MaxTriangles = 1
		case "collision":
			limited.Validation.MaxCollisionPairs = 1
		}
		if got, e := body.PrepareNestedMixedDisplay(ctx, limited); e == nil || got != nil {
			t.Fatal("atomic limit", which)
		}
	}
	// A native, genuinely contained shell can still be too close for this
	// display's proved deviation. Do not infer clearance from disjoint meshes.
	near, _ := makeOpenSplineBox(t, [3]float64{.5, .5, 1. / 1024}, [3]float64{1.5, 1.5, .75}, 0)
	close, _, e := CombineSeparatedSplineShells(ctx, outer, near, parametric.Difference, SplineShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got, e := close.PrepareNestedMixedDisplay(ctx, options); !errors.Is(e, geom.ErrConvergence) || got != nil {
		t.Fatal("unproved source/display separation")
	}
	// Wrong native polarity is rejected before any display can reinterpret it.
	wrong, e := AssembleShells(ctx, []PlacedSolid{{outer, geom.Identity()}, {inner, geom.Identity()}}, 128)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := wrong.PrepareNestedMixedDisplay(ctx, options); e == nil || got != nil {
		t.Fatal("wrong original inward orientation")
	}
}
