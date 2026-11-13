package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAlgebraicBoundaryNetwork(t *testing.T) {
	ctx := context.Background()
	s := solidEllipticalLensFixture(t)
	makeRecipe := func(edge int, offset *big.Rat) AlgebraicSeamRecipe {
		m := s.edges[edge].Curve.(surfaceTrimCurve)
		h, e := quadraticTrimSourceControls(m.curve.(SplineUV))
		if e != nil {
			t.Fatal(e)
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		uv, _ := q.Endpoints()
		axis := 0
		if uv[0][0].Cmp(uv[1][0]) == 0 {
			axis = 1
		}
		value := new(big.Rat).Quo(new(big.Rat).Add(uv[0][axis], uv[1][axis]), big.NewRat(2, 1))
		if offset != nil {
			value.Add(value, offset)
		}
		plane := [4]*big.Rat{}
		plane[axis] = big.NewRat(1, 1)
		plane[3] = new(big.Rat).Neg(value)
		p, e := exactFaceSupportPatch(Face{Surface: m.surface}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		hits, e := q.IntersectModelRawPlane(ctx, p, plane, geom.SurfaceCurveAgreementOptions{}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || len(hits.Hits) != 1 {
			t.Fatal("root", hits, e)
		}
		recipe, _, e := s.PrepareAlgebraicSeamRecipe(ctx, edge, plane, hits.Hits[0].Event, AlgebraicSeamOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return recipe
	}
	first := makeRecipe(0, nil)
	second := makeRecipe(2, nil)
	sub, e := s.SubdivideAlgebraicSeams(ctx, []AlgebraicSeamRecipe{first, second}, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	opts := AlgebraicBoundaryMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .01}}
	got, e := sub.TessellateBoundaryCertified(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Edges) != 6 || len(got.TopologyVertices) != 6 || len(got.Coedges) != 12 {
		t.Fatal("ownership counts")
	}
	for _, edge := range got.Edges {
		if len(edge) < 2 {
			t.Fatal("empty edge")
		}
	}
	for _, coedge := range got.Coedges {
		want := got.Edges[coedge.Edge]
		for i, id := range coedge.Vertices {
			j := i
			if coedge.Reverse {
				j = len(want) - 1 - i
			}
			if id != want[j] {
				t.Fatal("shared samples")
			}
		}
	}
	for i, cert := range got.Certificates {
		if !cert.Complete || cert.MaximumError > .01 {
			t.Fatal("certificate", i)
		}
	}
	low := opts
	low.Display.MaxSegments = 1
	if partial, err := sub.TessellateBoundaryCertified(ctx, low); err == nil || len(partial.Vertices) != 0 {
		t.Fatal("partial budget result")
	}
	low = opts
	low.MaxCharts = 1
	if partial, err := sub.TessellateBoundaryCertified(ctx, low); err == nil || len(partial.Vertices) != 0 {
		t.Fatal("chart budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sub.TessellateBoundaryCertified(cancelled, opts); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	close := makeRecipe(0, epsilon)
	nearby, e := s.SubdivideAlgebraicSeams(ctx, []AlgebraicSeamRecipe{first, close}, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(nearby.vertices) != 6 {
		t.Fatal("distinct roots lost")
	}
	if partial, err := nearby.TessellateBoundaryCertified(ctx, opts); !errors.Is(err, geom.ErrDegenerate) || len(partial.Vertices) != 0 {
		t.Fatal("display collision must reject without welding", err)
	}
	t.Log("segments", got.Segments, "vertices", len(got.Vertices), "operations", got.ExactOperations)
}
