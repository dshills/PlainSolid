package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestAlgebraicGraphStockPreservesIntactVoids(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8)
	// Y=5/8 crosses the original outer wall at X=√5 but stays strictly
	// below the through-void. Both generated caps must retain the hole.
	const z = .5 - 5./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, RationalGraphShellRegionOptions{})
	if e != nil || len(pair.Seams) != 1 {
		t.Fatal("expected one seam on the outer material", len(pair.Seams), e)
	}
	for _, cap := range pair.Caps {
		caps, e := cap.MaterialRegions(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || len(caps) != 1 || len(caps[0].Holes) != 1 || caps[0].Holes[0] == caps[0].Outer {
			t.Fatal("exact rational ray proof must assign intact void to the outer cap", caps, e)
		}
		if partial, e := cap.MaterialRegions(ctx, geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, geom.ErrConvergence) || partial != nil {
			t.Fatal("incomplete nesting must not publish a region", partial, e)
		}
	}
	out, e := pair.ExactIndexedBoundary(ctx, section, geom.ExactCurvePlaneEventOptions{})
	if e != nil || out.EulerCharacteristic != 0 || out.BoundaryComponents != 1 ||
		len(out.Faces[0].Loops) != 2 || len(out.Faces[1].Loops) != 2 {
		t.Fatal("both cap holes must contribute to the closed genus-one topology", out, e)
	}
}
