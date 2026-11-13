package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestComposedCollectionPolylineSharedJunction(t *testing.T) {
	ctx := context.Background()
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(ctx, uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lo := importTestRoot(t, big.NewRat(-1, 2), false, "1/2", "1/2")
	hi := importTestRoot(t, big.NewRat(-3, 4), false, "3/4", "3/4")
	cells, e = cells.RestrictAlgebraic(ctx, lo, hi, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := cells.TessellateCertified(ctx, ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: ComposedCurvePolylineOptions{Deflection: 1e-3}})
	if e != nil || !r.Complete || len(r.Junctions) != 3 || len(r.Vertices) != len(r.Segments)+1 || r.MaximumError > 1e-3 {
		t.Fatal(r, e)
	}
	j := r.Junctions[1].Vertex
	if j <= 0 || j >= len(r.Vertices)-1 {
		t.Fatal("missing shared junction")
	}
	uses := 0
	for _, s := range r.Segments {
		if s.Vertices[0] == j || s.Vertices[1] == j {
			uses++
		}
	}
	if uses != 2 {
		t.Fatal("junction not shared", uses)
	}
	limited, e := cells.TessellateCertified(ctx, ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: ComposedCurvePolylineOptions{Deflection: 1e-3, MaxSegments: 1}})
	if !errors.Is(e, ErrConvergence) || limited.Complete || len(limited.Segments) > 1 {
		t.Fatal(limited, e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = cells.TessellateCertified(cancelCtx, ComposedCollectionPolylineOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestComposedCollectionPolylineDistinctCollapsedEvents(t *testing.T) {
	ctx := context.Background()
	_, uv, _, _ := algebraicSeamFixture(t) // (t²,t,0)
	cell, e := NewExactComposedBezierCell(ctx, uv, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	root := func(bits uint) *ExactAlgebraicRoot {
		tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), bits))
		return importTestRoot(t, new(big.Rat).Neg(new(big.Rat).Add(big.NewRat(1, 4), tiny)), true, "1/2", "3/4")
	}
	lo, hi := root(110), root(109)
	restriction, e := cell.RestrictAlgebraic(ctx, lo, hi, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells := SurfaceCurveCompositionCells{Cells: []*ExactComposedCurveCell{restriction.Cell}}
	options := ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: ComposedCurvePolylineOptions{Deflection: 1e-20}}
	r, e := cells.TessellateCertified(ctx, options)
	if e != nil || !r.Complete || !r.DisplayDegenerate || len(r.Vertices) != 2 || len(r.CollapsedSegments) != 1 || len(r.CoincidentJunctionGroups) != 1 {
		t.Fatal(r, e)
	}
	if r.Junctions[0].Vertex == r.Junctions[1].Vertex {
		t.Fatal("distinct events welded")
	}
	order, e := r.Junctions[0].Root.Compare(ctx, r.Junctions[1].Root, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != -1 {
		t.Fatal(order, e)
	}
	options.JunctionPoints = []Vec3{{.25, .5, 0}, {.25, math.Nextafter(.5, 1), 0}}
	rejected, e := cells.TessellateCertified(ctx, options)
	if e == nil || rejected.Complete {
		t.Fatal("uncertified display displacement accepted", rejected)
	}
	options.Deflection = 1e-12
	separated, e := cells.TessellateCertified(ctx, options)
	if e != nil || !separated.Complete || separated.DisplayDegenerate || separated.MaximumError > options.Deflection {
		t.Fatal(separated, e)
	}
	options.JunctionPoints = options.JunctionPoints[:1]
	if _, e = cells.TessellateCertified(ctx, options); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
