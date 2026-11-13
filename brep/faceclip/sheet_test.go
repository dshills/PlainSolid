package faceclip

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestSheetHoleAndCollinearVertices(t *testing.T) {
	b := boundaryFor(t)
	sheet, e := TriangulateSheet(context.Background(), b, SheetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for i, l := range b.Loops {
		count += len(l.Vertices)
		for j, p := range l.Vertices {
			if !same(p, sheet.ExactVertices[sheet.LoopVertices[i][j]]) {
				t.Fatal("exact vertex lost")
			}
		}
	}
	if len(sheet.Mesh.Vertices) != count || sheet.MaxSquaredDisplacement.Sign() != 0 {
		t.Fatal("vertices")
	}
	if e = sheet.Mesh.Validate(); e != nil {
		t.Fatal(e)
	}
}
func TestSheetRationalWindowAndTolerance(t *testing.T) {
	r, e := ClipRegion(context.Background(), holed(t), 1, []Halfspace{{geom.Vec3{X: 3}, 1}, {geom.Vec3{Y: 3}, 2}}, RegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := MergeLoops(context.Background(), r, LoopOptions{})
	if e != nil {
		t.Fatal(e)
	}
	sheet, e := TriangulateSheet(context.Background(), b, SheetOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if sheet.MaxSquaredDisplacement.Sign() <= 0 {
		t.Fatal("expected rational rounding")
	}
	if _, e = TriangulateSheet(context.Background(), b, SheetOptions{MaxDisplacement: 1e-30}); !errors.Is(e, ErrConversion) {
		t.Fatal(e)
	}
}
func TestSheetRejectNonplanarRounding(t *testing.T) {
	b := Boundary{Axis: 2, ProjectedArea: big.NewRat(1, 1), Loops: []Loop{{Parent: -1, Vertices: []Point{{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 3)}, {big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 3)}, {big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 3)}, {big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(1, 3)}}}}}
	if _, e := TriangulateSheet(context.Background(), b, SheetOptions{}); !errors.Is(e, ErrConversion) {
		t.Fatal(e)
	}
	if _, e := TriangulateSheet(context.Background(), boundaryFor(t), SheetOptions{MaxTriangles: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
