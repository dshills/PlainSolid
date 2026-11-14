package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicGraphCapPairIrrationalOwners(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8)
	// Both individually rational sections cross only vertical stock
	// walls. Their seam Y=11/8, Z=1/2 meets the original sheared
	// through-void curve Y=1+X²/8 at the shared irrational X=sqrt(3).
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	r, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, RationalGraphShellRegionOptions{})
	if e != nil || len(r.Seams) != 2 || len(r.Caps[0].Wires) != 1 || len(r.Caps[1].Wires) != 1 {
		t.Fatal("expected two original material intervals shared by both caps", len(r.Seams), len(r.Caps[0].Wires), len(r.Caps[1].Wires), e)
	}
	opts := geom.ExactCurvePlaneEventOptions{}
	count := 0
	for _, seam := range r.Seams {
		for j := 0; j < 2; j++ {
			var pair [2]*geom.ExactComposedCurveEvent
			for i, owner := range seam.Owners {
				us := r.Caps[i].Wires[owner.Wire][owner.Use]
				k := j ^ i
				if us.Reversed {
					k ^= 1
				}
				root, e := us.Cell.EndpointRoot(k)
				if e != nil {
					t.Fatal(e)
				}
				pair[i], e = us.Cell.BindRoot(ctx, root, opts)
				if e != nil {
					t.Fatal(e)
				}
			}
			equal, _, e := pair[0].CompareModelPoint(ctx, pair[1], opts)
			if e != nil || !equal {
				t.Fatal("opposing coedges must agree at both exact endpoints", equal, e)
			}
			x, _, e := pair[0].CoordinateRoot(ctx, 0, opts)
			if e != nil {
				t.Fatal(e)
			}
			if x, e := x.Classify(ctx, opts); e != nil {
				t.Fatal(e)
			} else if x.Kind == geom.ExactRootKindIrrational {
				count++
			}
		}
	}
	attachments := 0
	for _, cap := range r.Caps {
		for wi, wire := range cap.Wires {
			for ui, edge := range wire {
				att, e := cap.OriginalStockAttachment(ctx, wi, ui, geom.SurfaceCurveAgreementOptions{}, opts)
				if edge.Seam >= 0 {
					if !errors.Is(e, geom.ErrInvalid) || att.Model != nil {
						t.Fatal("a generated seam has no source stock attachment", e)
					}
					continue
				}
				if e != nil || att.Source != edge.Source || att.StockReversed == edge.Reversed || att.ExactOperations <= 0 {
					t.Fatal("retained cap edge must bind to its exact original stock chart", wi, ui, att, e)
				}
				// Model and UV restrictions must keep the actual cap root,
				// which is irrational on the inner through-void terminal.
				for j := 0; j < 2; j++ {
					capRoot, _ := edge.Cell.EndpointRoot(j)
					uvRoot, _ := att.UV.EndpointRoot(j)
					order, e := capRoot.Compare(ctx, uvRoot, opts)
					if e != nil || !order.Proven || order.Order != 0 {
						t.Fatal("original stock parameter restriction lost its root", e)
					}
				}
				attachments++
			}
		}
	}
	if attachments < 12 {
		t.Fatal("missing retained stock owners", attachments)
	}
	stockLoops, e := r.OriginalStockLoops(ctx, section, opts)
	if e != nil || len(stockLoops) < 12 {
		t.Fatal("retained original chart loops must close without rounding", len(stockLoops), e)
	}
	owners := map[[3]int]bool{}
	for _, w := range stockLoops {
		if len(w.Uses) < 3 || w.OriginalFace < 0 || w.OriginalFace >= len(stock.faces) {
			t.Fatal("invalid original face loop", w)
		}
		for _, use := range w.Uses {
			if use.Cap >= 0 {
				id := [3]int{use.Cap, use.Owner.Wire, use.Owner.Use}
				if owners[id] || r.Caps[use.Cap].Wires[use.Owner.Wire][use.Owner.Use].Source.Face != w.OriginalFace {
					t.Fatal("duplicated or transferred original attachment", w.OriginalFace, id)
				}
				owners[id] = true
			}
		}
	}
	indexed, e := r.ExactIndexedBoundary(ctx, section, opts)
	if e != nil || indexed.BoundaryComponents != 1 || indexed.EulerCharacteristic != 2 ||
		indexed.Vertices != 24 || len(indexed.Edges) != 36 || len(indexed.Faces) != 14 || len(indexed.Faces) != len(stockLoops)+2 {
		t.Fatal("the cut opens the through-hole into an exterior notch, leaving one genus-zero shell", indexed.Vertices, len(indexed.Edges), len(indexed.Faces), indexed.EulerCharacteristic, e)
	}
	// The representation must ignore abandoned public-facing diagnostics.
	diag := r
	diag.Caps = [2]AlgebraicGraphCapClip{}
	diag.Seams = nil
	rechecked, e := diag.ExactIndexedBoundary(ctx, section, opts)
	if e != nil || rechecked.Vertices != indexed.Vertices || rechecked.EulerCharacteristic != indexed.EulerCharacteristic {
		t.Fatal("diagnostic edits changed the exact shell", e)
	}
	if len(owners) != attachments {
		t.Fatal("each retained cap side must appear once on a closed stock loop", len(owners), attachments)
	}
	if count != 1 {
		t.Fatal("expected exactly one irrational shared vertex", count)
	}
	cap := RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: r.ExactOperations}}
	if bad, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, section, cap); !errors.Is(e, geom.ErrConvergence) || len(bad.Seams)+len(bad.Caps[0].Wires) != 0 {
		t.Fatal("exhausting common pair work must fail atomically", e)
	}
	// The gap over the original through-void must stay outside both caps
	// wherever their planes overlap.
	for _, q := range r.Caps {
		p := [3]*big.Rat{big.NewRat(3, 2), big.NewRat(11, 8), big.NewRat(1, 2)}
		if loc, e := q.LocateRational(ctx, p); e != nil || loc != trimvalidate.Outside {
			t.Fatal("void gap must not become a common seam", loc, e)
		}
	}
}
