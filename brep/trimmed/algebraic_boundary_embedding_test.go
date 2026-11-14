package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAlgebraicBoundaryDisplayEmbedding(t *testing.T) {
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

	opts := AlgebraicBoundaryEmbeddingOptions{Boundary: AlgebraicBoundaryMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .01}}}
	r, e := sub.TessellateEmbeddedBoundaryCertified(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Boundary.TopologyVertices) != 6 || len(r.Boundary.Coedges) != 12 || r.PairChecks == 0 {
		t.Fatal("lost root/owner identity")
	}
	opts.MaxPairs = 1
	if r, e := sub.TessellateEmbeddedBoundaryCertified(ctx, opts); !errors.Is(e, geom.ErrConvergence) || len(r.Boundary.Vertices) != 0 {
		t.Fatal("partial pair budget", e)
	}
}

func TestBoundaryPolylineExactContacts(t *testing.T) {
	tests := []struct {
		name  string
		v     []geom.Vec3
		edges [][]int
		bad   bool
	}{
		{"shared-corner", []geom.Vec3{{}, {X: 1}, {Y: 1}}, [][]int{{0, 1}, {0, 2}}, false},
		{"shared-collinear-end", []geom.Vec3{{}, {X: 1}, {X: 2}}, [][]int{{0, 1}, {1, 2}}, false},
		{"proper-cross", []geom.Vec3{{}, {X: 1, Y: 1}, {Y: 1}, {X: 1}}, [][]int{{0, 1}, {2, 3}}, true},
		{"near-skew", []geom.Vec3{{}, {X: 1, Y: 1}, {Y: 1, Z: -1e-200}, {X: 1, Z: 2e-200}}, [][]int{{0, 1}, {2, 3}}, false},
		{"unindexed-touch", []geom.Vec3{{}, {X: 1}, {X: 1}, {X: 2}}, [][]int{{0, 1}, {2, 3}}, true},
		{"shared-overlap", []geom.Vec3{{}, {X: 1}, {X: 2}}, [][]int{{0, 1}, {0, 2}}, true},
		{"interior-touch", []geom.Vec3{{}, {X: 2}, {X: 1}, {X: 1, Y: 1}}, [][]int{{0, 1}, {2, 3}}, true},
		{"near-parallel-cross", []geom.Vec3{{}, {X: 1, Y: 1e-200}, {Y: 1e-200}, {X: 1}}, [][]int{{0, 1}, {2, 3}}, true},
		{"vertical-shared", []geom.Vec3{{}, {Z: 1}, {Z: 2}}, [][]int{{0, 1}, {1, 2}}, false},
		{"duplicate-segment", []geom.Vec3{{}, {X: 1}}, [][]int{{0, 1}, {1, 0}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, e := checkBoundaryPolylineEmbedding(context.Background(), tt.v, tt.edges, 10)
			if (e != nil) != tt.bad || n != 1 {
				t.Fatal(n, e)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := checkBoundaryPolylineEmbedding(ctx, tests[0].v, tests[0].edges, 10); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := checkBoundaryPolylineEmbedding(context.Background(), tests[0].v, [][]int{{0, 1, 2, 0}}, 1); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
}
