package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestMixedOriginalShellMaterial(t *testing.T) {
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	results, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}, bc)
	if e != nil {
		t.Fatal("combine", e)
	}
	for i, r := range results {
		topology, e := r.Topology()
		if e != nil || topology.EulerCharacteristic != []int{2, 2, 4}[i] {
			t.Fatal("topology", i, topology, e)
		}
		t.Log(i, topology)
		restricted := 0
		var retained [2]bool
		for edge, x := range r.Edges() {
			if x.SectionPiece < 0 {
				continue
			}
			piece, e := r.SectionPiece(edge)
			if e != nil {
				t.Fatal(e)
			}
			if piece.Restricted != nil {
				restricted++
				if piece.Index < 0 || piece.Index > 1 || retained[piece.Index] {
					t.Fatal("duplicated restricted piece")
				}
				retained[piece.Index] = true
				if piece.Segment != nil || piece.SourceOperand != 1 || piece.Faces != [2]int{5, 3} {
					t.Fatal("erased original chart")
				}
				d, e := piece.Restricted.Piece(piece.Index)
				if e != nil {
					t.Fatal(e)
				}
				if piece.Index == 1 && d.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 {
					t.Fatal("lost exact restriction")
				}
			}
		}
		if restricted != 2 {
			t.Fatal("missing rational pieces", i, restricted)
		}
		for face, owner := range r.Faces() {
			f, e := r.OriginalFace(face)
			if e != nil || f.Surface != [2]Solid{a, b}[owner.Operand].faces[owner.Original].Surface {
				t.Fatal("lost support", e)
			}
		}
	}

	for _, r := range results {
		if _, err := json.Marshal(r); !errors.Is(err, geom.ErrUnrepresentable) {
			t.Fatal("erased exact geometry", err)
		}
		faces := r.Faces()
		edges := r.Edges()
		vertices := r.Vertices()
		faces[0].Outer[0].Edge = -1
		edges[0].Start = -1
		vertices[0].Operand = -1
		if r.Faces()[0].Outer[0].Edge < 0 || r.Edges()[0].Start < 0 || r.Vertices()[0].Operand < 0 {
			t.Fatal("mutable result")
		}
	}
	small := bc
	small.Imprint.MaxPieces = 1
	if got, err := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, small); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("partial imprint", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := section.CombineAlgebraicOperations(canceled, []parametric.BooleanOp{parametric.Intersection}, bc); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled material", err)
	}
	if got, err := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Union, parametric.Union}, bc); got != nil || !errors.Is(err, geom.ErrInvalid) {
		t.Fatal("duplicate selection", err)
	}
	for i, r := range results {
		location, err := r.LocateMaterial(ctx, geom.Vec3{X: 1, Y: 1, Z: -19.0 / 128}, SplineShellBooleanOptions{})
		if err != nil || location != parametric.Boundary {
			t.Fatal("exact retained junction boundary", i, location, err)
		}
	}
	near := geom.Vec3{X: math.Nextafter(1, 0), Y: 1, Z: -19.0 / 128}
	if location, err := results[1].LocateMaterial(ctx, near, SplineShellBooleanOptions{}); err == nil && location != parametric.Inside || err != nil && !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("near vertex snapped or misclassified", location, err)
	}
	limited := SplineShellBooleanOptions{}
	limited.Embedding.Patches.MaxOperations = 1
	if _, err := results[0].LocateMaterial(ctx, geom.Vec3{X: 1, Y: 1, Z: -19.0 / 128}, limited); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("vertex witness budget", err)
	}
	for _, row := range []struct {
		p      geom.Vec3
		inside [3]bool
	}{
		{geom.Vec3{X: .5, Y: .5, Z: -.25}, [3]bool{true, true, false}},
		{geom.Vec3{X: .5, Y: .5, Z: -.75}, [3]bool{false, true, true}},
		{geom.Vec3{X: .5, Y: .5, Z: -.025}, [3]bool{false, true, true}},
	} {
		for i, r := range results {
			location, e := r.LocateMaterial(ctx, row.p, SplineShellBooleanOptions{})
			want := parametric.Outside
			if row.inside[i] {
				want = parametric.Inside
			}
			if e != nil || location != want {
				t.Fatal("material", i, row.p, location, e)
			}
		}
	}
}
