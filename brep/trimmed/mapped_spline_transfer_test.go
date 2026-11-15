package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestMappedSplineTransfer(t *testing.T) {
	ctx := context.Background()
	proof, closing, planar, seams := mappedChainClosedFixture(t)
	shell, err := CertifyMappedSplineChainShell(ctx, proof, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rect, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	faces := rect.Solid.Faces()
	for i := range rect.ChainEdges {
		f := faces[0]
		model, err := ExactTrimModelBezier(ctx, f.Surface, f.Outer[i].Curve, false, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		model, err = model.Reparameterize(2.5, 4.25, false, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for owner := range 2 {
			f := faces[owner]
			original := f.Outer[i].Curve.(MappedSplineUV)
			got, err := ProveMappedSplineTransfer(ctx, f.Surface, original.Source(), model, geom.SurfacePatchOptions{})
			if err != nil || got.ParameterWeight().Cmp(original.ParameterWeight()) != 0 {
				t.Fatal("original spline speed differs", owner, i, err)
			}
		}
		net, _, err := model.BezierControlNet(ctx, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// Endpoint positions and the proposing start tangent are unchanged,
		// so the complete lift must reject the different interior model curve.
		if len(net.Control) != 6 {
			t.Fatal("degree-five source model trace expected", len(net.Control))
		}
		net.Control[3][2].Add(net.Control[3][2], big.NewRat(1, 1024))
		perturbed, err := geom.NewExactHomogeneousBezier(ctx, net.Control, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		original := faces[1].Outer[i].Curve.(MappedSplineUV)
		if q, err := ProveMappedSplineTransfer(ctx, faces[1].Surface, original.Source(), perturbed, geom.SurfacePatchOptions{}); !errors.Is(err, geom.ErrConvergence) || q.ParameterWeight() != nil {
			t.Fatal("changed interior transfer accepted", i, err)
		}
	}
}
