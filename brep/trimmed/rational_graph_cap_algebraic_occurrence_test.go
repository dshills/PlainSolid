//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicGraphStockRationalOccurrence(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := makeWarpedSquareRationalGraphTube(t, 1./8)
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	selection := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := s.ClipAlgebraicGraphCapPair(ctx, planes, selection, RationalGraphShellRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q := big.NewRat
	matrix := [3][4]*big.Rat{
		{q(3, 5), q(0, 1), q(4, 5), q(1, 7)},
		{q(0, 1), q(1, 1), q(0, 1), q(-1, 3)},
		{q(-4, 5), q(0, 1), q(3, 5), q(1, 11)},
	}
	pose, e := placement.NewRationalRigid(matrix, 4096)
	if e != nil {
		t.Fatal(e)
	}
	root := geom.ExactCurvePlaneEventOptions{}
	w, e := pair.PlaceVerticalMaterialRigid(ctx, pose, selection, root)
	if e != nil {
		t.Fatal(e)
	}
	if p, e := w.Pose(); e != nil || p[2][3].Cmp(q(1, 11)) != 0 {
		t.Fatal("exact placed translation", p, e)
	}
	var payload bytes.Buffer
	if e := WriteAlgebraicGraphStockOccurrenceJSON(&payload, w); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), payload.Bytes()...)
	w, e = ReadAlgebraicGraphStockOccurrenceJSON(ctx, bytes.NewReader(raw), JSONLimits{}, selection,
		RationalGraphShellRegionOptions{}, root)
	if e != nil {
		t.Fatal("reload original geometry and exact pose", e)
	}
	var corrupt nativeAlgebraicGraphStockOccurrence
	if e := json.Unmarshal(raw, &corrupt); e != nil {
		t.Fatal(e)
	}
	corrupt.Pose[0][0] = "4/5"
	damaged, e := json.Marshal(corrupt)
	if e != nil {
		t.Fatal(e)
	}
	if partial, e := ReadAlgebraicGraphStockOccurrenceJSON(ctx, bytes.NewReader(damaged), JSONLimits{}, selection,
		RationalGraphShellRegionOptions{}, root); e == nil || partial.pair.caps[0].recipe != nil {
		t.Fatal("non-rigid native pose must fail atomically", e)
	}
	for _, factor := range []int64{-1, 0, 1} {
		eps := new(big.Rat).SetFrac(big.NewInt(factor), new(big.Int).Lsh(big.NewInt(1), 120))
		local := [3]*big.Rat{q(1, 2), q(7, 4), new(big.Rat).Add(q(35, 64), eps)}
		p, e := pose.TransformPoint(ctx, local, false, 4096)
		if e != nil {
			t.Fatal(e)
		}
		got, e := w.ClassifyRational(ctx, p)
		want := map[int64]parametric.Location{-1: parametric.Inside, 0: parametric.Boundary, 1: parametric.Outside}[factor]
		if e != nil || got != want {
			t.Fatal("world sub-ULP classification", factor, got, want, e)
		}
	}
	origin, e := pose.TransformPoint(ctx, [3]*big.Rat{q(0, 1), q(89, 64), q(1, 2)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	direction, e := pose.TransformVector(ctx, [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	hits, e := w.IntersectCapLineRational(ctx, 0, origin, direction, [2]*big.Rat{q(-1, 1), q(4, 1)}, root)
	if e != nil || len(hits.Contacts) != 4 || len(hits.Material) != 2 {
		t.Fatal("world exact cap line keeps its parameter and original owners", hits.Contacts, e)
	}
	proof, e := hits.Contacts[2].Parameter.SignPolynomial(ctx, []*big.Rat{q(-25, 8), q(0, 1), q(1, 1)}, root)
	if e != nil || proof.Sign != 0 {
		t.Fatal("world transport changed an irrational line parameter", proof, e)
	}
	throughWall, e := pose.TransformPoint(ctx, [3]*big.Rat{q(0, 1), q(89, 64), q(513, 1024)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	solidHits, e := w.IntersectLineRational(ctx, throughWall, direction, [2]*big.Rat{q(-1, 1), q(4, 1)}, root)
	if e != nil || len(solidHits.Contacts) != 4 || len(solidHits.Material) != 2 ||
		solidHits.Contacts[2].StockFace != w.pair.caps[0].wires[hits.Contacts[2].Wire][hits.Contacts[2].Use].Source.Face {
		t.Fatal("world line retains native stock-wall owner and both intervals", solidHits, e)
	}
	same, e := solidHits.Contacts[2].Parameter.Compare(ctx, hits.Contacts[2].Parameter, root)
	if e != nil || !same.Proven || same.Order != 0 {
		t.Fatal("world line changed its exact algebraic parameter", same, e)
	}
	touchOrigin, e := pose.TransformPoint(ctx, [3]*big.Rat{q(0, 1), q(79, 128), q(1025, 2048)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	touchDirection, e := pose.TransformVector(ctx, [3]*big.Rat{q(1, 1), q(7, 16), q(0, 1)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	touch, e := w.IntersectLineRationalWithTouches(ctx, touchOrigin, touchDirection, [2]*big.Rat{q(-1, 1), q(4, 1)}, root)
	if e != nil || len(touch.Contacts) != 3 || len(touch.Material) != 1 || !touch.Contacts[1].Touch ||
		touch.Contacts[1].StockFace < 0 {
		t.Fatal("world tangent and native wall owner", touch, e)
	}
	at, e := touch.Contacts[1].Parameter.SignPolynomial(ctx, []*big.Rat{q(-7, 4), q(1, 1)}, root)
	if e != nil || at.Sign != 0 {
		t.Fatal("proper pose changed exact tangency station", at, e)
	}
	options := AlgebraicGraphStockFillOptions{Deflection: .12}
	options.Boundary.Section = selection
	options.Boundary.Display.Deflection = .003
	options.Boundary.EndpointTolerance = 1e-12
	mesh, e := w.TessellateCertified(ctx, options)
	if e != nil || mesh.SignedVolume == nil || mesh.MaximumFillError > options.Deflection || len(mesh.Mesh.Triangles) != len(mesh.Source.IndexedFaces) {
		t.Fatal("closed world display and original ancestry", mesh.MaximumFillError, e)
	}
	// Bounding the exact volume is invariant under the world pose.
	width := q(1, 1<<20)
	a, e := pair.BoundVerticalPolynomialVolume(ctx, width, selection, root)
	if e != nil {
		t.Fatal(e)
	}
	b, e := w.BoundVolume(ctx, width, selection, root)
	if e != nil || a.Lower.Cmp(b.Lower) != 0 || a.Upper.Cmp(b.Upper) != 0 {
		t.Fatal("proper rational rigid placement must preserve volume bounds", a, b, e)
	}

	localWall := [3]*big.Rat{q(7, 4), q(177, 128), q(1025, 2048)}
	localWall[0].Sub(q(7, 4), q(7, 8192))
	localWall[1].Add(localWall[1], q(1, 512))
	worldWall, e := pose.TransformPoint(ctx, localWall, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	near, e := w.NearestBoundaryRational(ctx, worldWall, root)
	if e != nil || near.Wall < 0 || near.HeightCap >= 0 || near.ModelParameter == nil {
		t.Fatal("world nearest source wall and height", near, e)
	}
	dProof, e := near.SquaredDistance.SignPolynomial(ctx, []*big.Rat{q(-305, 67108864), q(1, 1)}, root)
	if e != nil || dProof.Sign != 0 {
		t.Fatal("proper pose changed exact distance", dProof, e)
	}
	worldCap, e := pose.TransformPoint(ctx, [3]*big.Rat{q(1, 2), q(7, 4), q(127, 256)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	near, e = w.NearestBoundaryRational(ctx, worldCap, root)
	if e != nil || near.Cap != 0 || near.Point[0] == nil {
		t.Fatal("exact world plane witness", near, e)
	}
	foot, e := pose.TransformPoint(ctx, near.Point, true, 4096)
	if e != nil || foot[0].Cmp(q(1, 2)) != 0 || foot[1].Cmp(q(7, 4)) != 0 || foot[2].Cmp(q(1, 2)) != 0 {
		t.Fatal("placed witness did not invert exactly", foot, e)
	}
	mass, e := w.BoundCentroid(ctx, q(1, 1<<18), selection, root)
	if e != nil || mass.Volume.Lower.Sign() <= 0 {
		t.Fatal("posed centroid proof", e)
	}
	// The exact (3/5,4/5) rotation and rational offset map the
	// independently evaluated original centroid into world space.
	local := [3]float64{1.7998525281375826, 2.843033752390164, .5917521095243853}
	expected := [3]float64{3*local[0]/5 + 4*local[2]/5 + 1./7, local[1] - 1./3,
		(-4*local[0]+3*local[2])/5 + 1./11}
	for i, want := range expected {
		lo, hi := mass.Coordinates[i].Lower, mass.Coordinates[i].Upper
		if lo.Cmp(ratInset(want)) > 0 || hi.Cmp(ratInset(want)) < 0 {
			t.Fatal("world centroid and signed interval transform", i, lo, hi, want)
		}
		v := ratInset(.7179226785061876 * want)
		if mass.FirstMoments[i].Lower.Cmp(v) > 0 || mass.FirstMoments[i].Upper.Cmp(v) < 0 {
			t.Fatal("world moment must include translation times volume", i, mass.FirstMoments[i], v)
		}
	}

	areaRoots := root
	areaRoots.MaxOperations = 100000000
	areaRoots.MaxSubdivisions = 30000
	area, e := w.BoundSurfaceArea(ctx, q(1, 20), areaRoots)
	if e != nil || area.Interval.Lower.Sign() <= 0 || area.Interval.Lower.Cmp(ratInset(11.940081106438639)) > 0 ||
		area.Interval.Upper.Cmp(ratInset(11.940081106438639)) < 0 {
		t.Fatal("rigid occurrence must preserve source area enclosure", area, e)
	}
	full, e := w.BoundMass(ctx, q(1, 1<<17), selection, root)
	if e != nil {
		t.Fatal("world second moments and inertia", e)
	}
	rot := [3][3]float64{{.6, 0, .8}, {0, 1, 0}, {-.8, 0, .6}}
	localInertia := [3][3]float64{{.2056399733393955, -.14196249019767038, -.008872655637354399},
		{-.14196249019767038, .5669076886572255, -.01266064505970843},
		{-.008872655637354399, -.01266064505970843, .7664083572284945}}
	localSecond := [3][3]float64{{2.8895263860264615, 3.8156026204427085, .7735080718994141},
		{3.8156026204427085, 6.005425122192287, 1.2204730978696003},
		{.7735080718994141, 1.2204730978696003, .2544650381262126}}
	offset := [3]float64{1. / 7, -1. / 3, 1. / 11}
	volume := .7179226785061876
	for k, ij := range [6][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}} {
		i, j := ij[0], ij[1]
		var expectedInertia, expectedSecond float64
		for a := 0; a < 3; a++ {
			for b := 0; b < 3; b++ {
				expectedInertia += rot[i][a] * localInertia[a][b] * rot[j][b]
				expectedSecond += rot[i][a] * localSecond[a][b] * rot[j][b]
			}
			expectedSecond += (rot[i][a]*offset[j] + rot[j][a]*offset[i]) * local[a] * volume
		}
		expectedSecond += offset[i] * offset[j] * volume
		for _, z := range []struct {
			b    geom.ExactParameterInterval
			want float64
		}{{full.CentroidInertia[i][j], expectedInertia}, {full.SecondMoments[k], expectedSecond}} {
			if z.b.Lower.Cmp(ratInset(z.want)) > 0 || z.b.Upper.Cmp(ratInset(z.want)) < 0 {
				t.Fatal("world second moment or rotated inertia", ij, z.b, z.want)
			}
		}
	}

}
