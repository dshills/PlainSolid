package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicGraphPlanarStockCertified(t *testing.T) {
	ctx := context.Background()
	s := makeSquareRationalGraphTube(t)
	const z = 5. / 16
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z + 1./8}, {Y: 1, Z: z}},
	}
	opts := RationalGraphShellRegionOptions{}
	pair, e := s.ClipAlgebraicGraphCapPair(ctx, planes, RationalGraphShellSectionOptions{}, opts)
	if e != nil {
		t.Fatal(e)
	}
	meshOptions := AlgebraicGraphPlanarMeshOptions{}
	meshOptions.Boundary.Display.Deflection = .004
	out, e := pair.TessellatePlanarStockCertified(ctx, meshOptions)
	if e != nil || out.SignedVolume == nil || out.SignedVolume.Cmp(ratInset(13./32)) != 0 ||
		out.MaximumFillError > meshOptions.Boundary.Display.Deflection || len(out.Mesh.Triangles) != len(out.IndexedFaces) {
		t.Fatal("closed exact-cap stock display", out.SignedVolume, out.MaximumFillError, e)
	}
	if out.Boundary.Indexed.EulerCharacteristic != 2 || len(out.Mesh.Triangles) < 20 {
		t.Fatal("original sleeve is opened into a closed notch", out.Boundary.Indexed)
	}
	meshOptions.Polygon = trimvalidate.Options{MaxEdges: 1}
	if failed, e := pair.TessellatePlanarStockCertified(ctx, meshOptions); e == nil || len(failed.Mesh.Triangles) != 0 {
		t.Fatal("polygon proof budget must fail atomically", e)
	}
	meshOptions.Polygon = trimvalidate.Options{}
	meshOptions.Boundary.MaxPairs = 1
	if failed, e := pair.TessellatePlanarStockCertified(ctx, meshOptions); !errors.Is(e, geom.ErrConvergence) || len(failed.Mesh.Triangles) != 0 {
		t.Fatal("shared boundary proof limit must fail atomically", e)
	}
	q := big.NewRat
	origin := [3]*big.Rat{q(5, 2), q(1, 2), q(0, 1)}
	direction := [3]*big.Rat{q(0, 1), q(1, 7), q(1, 1)}
	extent := [2]*big.Rat{q(0, 1), q(1, 1)}
	line, e := pair.IntersectPlanarStockRationalLine(ctx, origin, direction, extent, RationalGraphShellSectionOptions{}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(line.Contacts) != 2 || len(line.Material) != 1 {
		t.Fatal("arbitrary rational transverse line must hit both retained caps", line, e)
	}
	for i, hit := range line.Contacts {
		want := [2]*big.Rat{q(1, 2), q(5, 8)}[i]
		if hit.Parameter.Cmp(want) != 0 || hit.Entry != (i == 0) ||
			out.Boundary.Indexed.Faces[hit.IndexedFace].Cap != i {
			t.Fatal("exact line/cap ownership", i, hit)
		}
	}
	origin = [3]*big.Rat{q(3, 2), q(-1, 1), q(9, 16)}
	direction = [3]*big.Rat{q(0, 1), q(1, 1), q(0, 1)}
	line, e = pair.IntersectPlanarStockRationalLine(ctx, origin, direction, [2]*big.Rat{q(0, 1), q(5, 1)}, RationalGraphShellSectionOptions{}, geom.ExactCurvePlaneEventOptions{})
	// At x=3/2 both cap heights are 1/2; the queried z=9/16 is
	// outside everywhere along this line.
	if e != nil || len(line.Contacts)+len(line.Material) != 0 {
		t.Fatal("discarded side supports must not report material", line, e)
	}
	origin[0] = q(5, 2)
	line, e = pair.IntersectPlanarStockRationalLine(ctx, origin, direction, [2]*big.Rat{q(0, 1), q(5, 1)}, RationalGraphShellSectionOptions{}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(line.Contacts) != 2 || len(line.Material) != 1 {
		t.Fatal("two original walls must enclose one material interval", line, e)
	}
	for i, hit := range line.Contacts {
		if hit.Parameter.Cmp([2]*big.Rat{q(1, 1), q(4, 1)}[i]) != 0 ||
			out.Boundary.Indexed.Faces[hit.IndexedFace].StockFace < 0 || hit.Entry != (i == 0) {
			t.Fatal("exact original wall owner", i, hit)
		}
	}
	for _, probe := range []struct {
		p       [3]*big.Rat
		squared *big.Rat
		kind    string
	}{
		{[3]*big.Rat{q(5, 2), q(1, 2), q(7, 16)}, q(1, 256), "cap"},
		{[3]*big.Rat{q(5, 2), q(-1, 8), q(9, 16)}, q(1, 64), "wall"},
		{[3]*big.Rat{q(1, 1), q(1, 2), q(1, 2)}, q(1, 4), "seam"},
	} {
		n, e := pair.NearestPlanarStockRational(ctx, probe.p, RationalGraphShellSectionOptions{}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || n.SquaredDistance == nil {
			t.Fatal("complete stock minimum", probe.kind, e)
		}
		got, e := n.SquaredDistance.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || got.Kind != geom.ExactRootKindRational || got.Rational.Cmp(probe.squared) != 0 {
			t.Fatal("exact boundary minimum", probe.kind, got, e)
		}
		switch probe.kind {
		case "cap":
			if n.Edge != -1 || n.Face < 0 || out.Boundary.Indexed.Faces[n.Face].Cap != 0 || n.Point[2].Cmp(q(1, 2)) != 0 {
				t.Fatal("cap interior owner and exact nearest point", n)
			}
		case "wall":
			if n.Edge != -1 || n.Face < 0 || out.Boundary.Indexed.Faces[n.Face].StockFace < 0 || n.Point[1].Sign() != 0 {
				t.Fatal("original stock wall owner", n)
			}
		case "seam":
			if n.Face != -1 || n.Edge < 0 || out.Boundary.Indexed.Edges[n.Edge].CapSeam < 0 || n.CurveParameter == nil {
				t.Fatal("shared cap seam must have one canonical edge owner", n)
			}
		}
	}
	if d, e := pair.NearestPlanarStockRational(ctx, origin, RationalGraphShellSectionOptions{}, geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, geom.ErrConvergence) || d.SquaredDistance != nil {
		t.Fatal("nearest query must fail atomically on proof limits", d, e)
	}

}
