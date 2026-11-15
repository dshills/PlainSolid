package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestTrimmedTubeKnotSectionNative(t *testing.T) {
	t.Run("represented spline", func(t *testing.T) { testTrimmedTubeKnotSectionNative(t, false) })
	t.Run("exact tensor", func(t *testing.T) { testTrimmedTubeKnotSectionNative(t, true) })
}

func testTrimmedTubeKnotSectionNative(t *testing.T, useTensor bool) {
	ctx := context.Background()
	k, u := []float64{0, 0, .5, 1, 1}, []float64{0, 0, 2, 2}
	var points [2][]geom.Vec3
	for i, x := range []float64{0, .5, 1} {
		for _, y := range []float64{0, 2} {
			points[0] = append(points[0], geom.Vec3{X: x, Y: y})
			points[1] = append(points[1], geom.Vec3{X: x, Y: y, Z: y - []float64{0, .5, 1.5}[i]})
		}
	}
	var surfaces [2]geom.Surface
	var p [2]*geom.SurfacePatch
	for i := range surfaces {
		if i == 1 && useTensor {
			r := geom.ExactTensorSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 2, KnotsU: k, KnotsV: u,
				Controls: [][4]string{{"0", "0", "-1/4", "1"}, {"0", "2", "7/4", "1"}, {"1/2", "0", "-1/3", "1"}, {"1/2", "2", "5/3", "1"}, {"1", "0", "-4/3", "1"}, {"1", "2", "2/3", "1"}}}
			var e error
			p[i], e = geom.ImportExactTensorSurfaceRecipe(ctx, r, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = p[i].ToBSplineSurface(); !errors.Is(e, geom.ErrUnrepresentable) {
				t.Fatal("fixture original cannot be a float B-spline", e)
			}
			surfaces[i], e = geom.NewExactTensorSurface(ctx, p[i], geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			continue
		}
		s, e := geom.NewBSplineSurface(1, 1, 3, 2, points[i], k, u, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces[i] = s
		p[i], e = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	boxes := [2]geom.SurfacePairParameterBox{
		{A: geom.ParameterRectangle{UMin: .375, UMax: .5, VMax: 1}, B: geom.ParameterRectangle{UMin: .25, UMax: .5, VMax: 1}},
		{A: geom.ParameterRectangle{UMin: .5, UMax: .625, VMax: 1}, B: geom.ParameterRectangle{UMin: .5, UMax: .75, VMax: 1}},
	}
	anchor := [4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2)}
	if useTensor {
		anchor[1], anchor[3] = big.NewRat(1, 3), big.NewRat(1, 3)
	}
	var seeds intersect.SplineSeedReport
	for i, box := range boxes {
		a, e := p[0].Trim(box.A)
		if e != nil {
			t.Fatal(e)
		}
		b, e := p[1].Trim(box.B)
		if e != nil {
			t.Fatal(e)
		}
		h, e := geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, a, b, geom.SurfaceTubeBoundaryOptions{AtUpper: i == 0, Anchor: anchor})
		if e != nil || !h.ProvenBranch {
			t.Fatal(h, e)
		}
		bound, e := intersect.NewTubeBoundaryAnchor(i == 0, anchor)
		if e != nil {
			t.Fatal(e)
		}
		seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{
			Cell: geom.SurfaceSurfaceCell{DomainA: box.A, DomainB: box.B}, Tube: &h,
			BoundaryAnchor: bound})
	}
	charts := [2]intersect.Patch{
		{Surface: surfaces[0], Min: geom.Vec2{X: .375}, Max: geom.Vec2{X: .625, Y: 1}},
		{Surface: surfaces[1], Min: geom.Vec2{X: .25}, Max: geom.Vec2{X: .75, Y: 1}},
	}
	var faces [2]Face
	for i := range faces {
		v := [4]geom.Vec2{{X: .40625, Y: .25}, {X: .59375, Y: .25}, {X: .59375, Y: .875}, {X: .40625, Y: .875}}
		faces[i].Surface = surfaces[i]
		for j, x := range v {
			faces[i].Outer = append(faces[i].Outer, Use{Edge: j, UVStart: x, UVEnd: v[(j+1)%4]})
		}
	}
	o := AlgebraicTrimmedPairSectionOptions{}
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	r, e := SectionTrimmedSplinePair(ctx, charts, faces, &seeds, o)
	if e != nil || !r.Complete || len(r.Branches) != 1 || len(r.Branches[0].Runs) != 1 || len(r.Branches[0].Runs[0].Intervals) != 2 {
		t.Fatalf("complete=%v coverage=%v issues=%v err=%v", r.Complete, r.Trace.Coverage.CoversAllContacts, r.Trace.Issues, e)
	}
	check := func(r AlgebraicTrimmedPairSection) {
		t.Helper()
		p, e := r.PartitionBothFacesByTubeRun(ctx, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil || len(p.Faces[0].Cut) != 2 || p.Faces[0].Cut[0].EndPoint != p.Faces[0].Cut[1].StartPoint {
			t.Fatalf("knot identity lost: %v", e)
		}
		if useTensor {
			if _, ok := p.Faces[1].SourceSurface.(*geom.ExactTensorSurface); !ok || p.Faces[1].Support != nil {
				t.Fatalf("lost original tensor chart: %T", p.Faces[1].SourceSurface)
			}
			cuts := p.Faces[0].Cut
			heights := [3]*big.Rat{big.NewRat(61, 192), big.NewRat(1, 3), big.NewRat(25, 48)}
			mid, _, e := geom.NewExactRationalRoot(ctx, big.NewRat(1, 2), geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			direction, e := cuts[0].FreeStart.Compare(ctx, mid, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !direction.Proven || direction.Order == 0 {
				t.Fatal("knot must be interior to the directed run", direction, e)
			}
			if direction.Order > 0 {
				heights[0], heights[2] = heights[2], heights[0]
			}
			for i, knot := range []*geom.ExactComposedCurveEvent{cuts[0].StartPoint, cuts[0].EndPoint, cuts[1].EndPoint} {
				root, _, e := knot.CoordinateRoot(ctx, 1, geom.ExactCurvePlaneEventOptions{})
				if e != nil {
					t.Fatal(i, e)
				}
				want, _, e := geom.NewExactRationalRoot(ctx, heights[i], geom.ExactCurvePlaneEventOptions{})
				if e != nil {
					t.Fatal(e)
				}
				q, e := want.Compare(ctx, root, geom.ExactCurvePlaneEventOptions{})
				if e != nil || !q.Proven || q.Order != 0 {
					t.Fatal("exact piecewise original section", i, q, e)
				}
			}
		}
	}
	check(r)
	var file bytes.Buffer
	if e := WriteTrimmedSplinePairSectionJSON(&file, r); e != nil {
		t.Fatal(e)
	}
	if useTensor && !bytes.Contains(file.Bytes(), []byte(`"RationalParameters":["1/2","1/3","1/2","1/3"]`)) {
		t.Fatal("nonbinary dependent anchor must survive in original form")
	}
	back, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{}, o)
	if e != nil {
		t.Fatal(e)
	}
	check(back)
	var native nativeTrimmedPairSection
	if e := json.Unmarshal(file.Bytes(), &native); e != nil {
		t.Fatal(e)
	}
	stored := native.Tubes[0].Cells[0].BoundaryAnchor
	if useTensor {
		var e error
		anchor[1] = big.NewRat(334, 1000)
		native.Tubes[0].Cells[0].BoundaryAnchor, e = intersect.NewTubeBoundaryAnchor(stored.AtUpper, anchor)
		if e != nil {
			t.Fatal(e)
		}
	} else {
		stored.Parameters[1] += .001
	}
	data, e := json.Marshal(native)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(data), JSONLimits{}, o); e == nil || q.Complete {
		t.Fatal("changed anchor accepted", e)
	}
}
