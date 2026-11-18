//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestTrimmedSplinePairCoincidentSourceSpan(t *testing.T) {
	ctx := context.Background()
	var patches [2]intersect.Patch
	var exact [2]*geom.SurfacePatch
	for i, bounds := range [2][2]float64{{0, 1}, {-.25, 1.25}} {
		lo, hi := bounds[0], bounds[1]
		var poles []geom.Vec3
		for _, u := range bounds {
			for _, v := range []float64{0, 1} {
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: float64(i) * (v - .5)})
			}
		}
		s, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{lo, lo, hi, hi}, []float64{0, 0, 1, 1}, nil)
		if err != nil {
			t.Fatal(err)
		}
		patches[i] = intersect.Patch{Surface: s, Min: geom.Vec2{X: lo}, Max: geom.Vec2{X: hi, Y: 1}}
		exact[i], err = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	tube, err := geom.CertifySurfacePairTube(exact[0], exact[1], geom.SurfacePairTubeOptions{FreeAxis: 0})
	if err != nil || !tube.ProvenBranch {
		t.Fatal(tube, err)
	}
	seeds := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: exact[0].Domain(), DomainB: exact[1].Domain()}, Tube: &tube}}}
	wire := func(id int, p [4]geom.Vec2) []Use {
		out := make([]Use, len(p))
		for i := range p {
			out[i] = Use{Edge: id + i, UVStart: p[i], UVEnd: p[(i+1)%len(p)]}
		}
		return out
	}
	a := wire(77, [4]geom.Vec2{{X: .25, Y: .5}, {X: .75, Y: .5}, {X: .75, Y: 1}, {X: .25, Y: 1}})
	spline, err := geom.NewBSpline(3, []geom.Vec3{{X: .75, Y: .5}, {X: .625, Y: .5}, {X: .375, Y: .5}, {X: .25, Y: .5}},
		[]float64{2, 2, 2, 2, 6, 6, 6, 6}, []float64{1, .5, .75, 1})
	if err != nil {
		t.Fatal(err)
	}
	a[0].Curve = SplineUV{Curve: spline, Range: UVInterval{Min: 2, Max: 6}, Reversed: true}
	a[0].Reversed = true
	b := wire(81, [4]geom.Vec2{{X: -.25}, {X: 1.25}, {X: 1.25, Y: 1}, {X: -.25, Y: 1}})
	h := wire(85, [4]geom.Vec2{{X: .375, Y: .25}, {X: .375, Y: .75}, {X: .625, Y: .75}, {X: .625, Y: .25}})
	faces := [2]Face{{Surface: patches[0].Surface, Outer: a}, {Surface: patches[1].Surface, Outer: b, Holes: [][]Use{h}}}
	if r, err := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, AlgebraicTrimmedPairSectionOptions{}); !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("legacy isolated-contact mode accepted a coincidence", r.Complete, err)
	}
	o := AlgebraicTrimmedPairSectionOptions{}
	o.Clip.CoincidentSpans = true
	r, err := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, o)
	if err != nil || !r.Complete {
		t.Fatal("exact section on an original span", r.Complete, err)
	}
	check := func(r AlgebraicTrimmedPairSection) {
		t.Helper()
		if len(r.Branches) != 1 || len(r.Branches[0].Runs) != 2 || len(r.Branches[0].Clips) != 1 {
			t.Fatal("hole must split the retained boundary into two runs")
		}
		clip := r.Branches[0].Clips[0]
		if len(clip.Intervals) != 5 || len(clip.FaceTrims[0].Coincidences) != 1 || len(clip.FaceTrims[1].Coincidences) != 0 {
			t.Fatalf("wrong interval/ownership counts: %d %d %d", len(clip.Intervals), len(clip.FaceTrims[0].Coincidences), len(clip.FaceTrims[1].Coincidences))
		}
		for i, v := range clip.Intervals {
			if v.Retained != (i == 1 || i == 3) || (v.Locations[0] == trimvalidate.Boundary) != (i > 0 && i < 4) || (v.CoincidentTrim[0] == 0) != (i > 0 && i < 4) || v.CoincidentTrim[1] != -1 {
				t.Fatal("unexpected boundary/closed-face interval", i, v.Locations, v.CoincidentTrim, v.Retained)
			}
		}
		span := clip.FaceTrims[0].Coincidences[0]
		for i, value := range []int64{6, 2} {
			end := span.Ends[i]
			wanted, _, err := geom.NewExactRationalRoot(ctx, big.NewRat(value, 1), geom.ExactCurvePlaneEventOptions{})
			if err != nil {
				t.Fatal(err)
			}
			q, err := end.Parameter.Compare(ctx, wanted, geom.ExactCurvePlaneEventOptions{})
			if end.Source != (SplineTubeTrimSource{Use: 0, Edge: 77, Reversed: true, ParameterStart: 6, ParameterEnd: 2}) || q.Order != 0 || !q.Proven || err != nil {
				t.Fatal("original directed spline span provenance", end.Source, q, err)
			}
		}
		if r.runSource() != nil {
			t.Fatal("section on existing coedge admitted to new-edge partitioner")
		}
	}
	check(r)
	var data bytes.Buffer
	if err := WriteTrimmedSplinePairSectionJSON(&data, r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data.Bytes(), []byte(`"version":3`)) {
		t.Fatal("native recipe lost the coincidence mode")
	}
	back, err := ReadTrimmedSplinePairSectionJSON(ctx, &data, JSONLimits{}, AlgebraicTrimmedPairSectionOptions{})
	if err != nil || !back.Complete {
		t.Fatal("coincident native recipe independent reload", err)
	}
	check(back)

	// Now the opposite face is crossed from one original side to the other.
	// Both endpoints are in the interior of the original weighted coedge.
	faces[1].Outer = wire(91, [4]geom.Vec2{{X: .375, Y: .25}, {X: .625, Y: .25}, {X: .625, Y: .75}, {X: .375, Y: .75}})
	faces[1].Holes = nil
	section, err := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, o)
	if err != nil || !section.Complete || len(section.Branches[0].Runs) != 1 {
		t.Fatal("original coedge crosses the whole opposite face", section.Complete, err)
	}
	if p, err := section.ImprintCoincidentTrimRun(ctx, 1, 0, 0, AlgebraicTubeFacePartitionOptions{}); err == nil || len(p.BoundaryRanges) != 0 {
		t.Fatal("interior face attributed as owner of original coincident trim", err)
	}
	if p, err := section.ImprintCoincidentTrimRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}); err == nil || len(p.BoundaryRanges) != 0 {
		t.Fatal("bounded exact imprint must fail atomically", err)
	}
	section.Branches[0].Clips[0].FaceTrims[0].Coincidences[0].Ends[0].Source.Edge = -1
	section.Branches[0].Runs[0].Intervals = nil
	imprint, err := section.ImprintCoincidentTrimRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
	if err != nil || imprint.Partition.Chart != 1 || imprint.Partition.SourceSurface != patches[1].Surface || len(imprint.Partition.Cut) != 1 || len(imprint.BoundaryRanges) != 1 {
		t.Fatal("certified opposite-face partition and original ownership", err)
	}
	if p := imprint.Partition; len(p.Fragments[0].Outer) != 3 || len(p.Fragments[1].Outer) != 3 || p.Fragments[0].CutReversed == p.Fragments[1].CutReversed {
		t.Fatal("opposite original face partition incidence", p.Fragments)
	}
	owned := imprint.BoundaryRanges[0]
	if owned.Source != (SplineTubeTrimSource{Use: 0, Edge: 77, Reversed: true, ParameterStart: 6, ParameterEnd: 2}) {
		t.Fatal("mutable report changed source ownership", owned.Source)
	}
	// Independently subtract qW from the original ascending weighted cubic.
	// In normalized original units the crossing equations are these cubics.
	for _, recipe := range []geom.ExactRootRecipe{
		{Version: 1, Coefficients: []string{"3", "-6", "3", "-1"}, Lower: "1/2", Upper: "1"},
		{Version: 1, Coefficients: []string{"2", "-6", "-3", "1"}, Lower: "0", Upper: "1/2"},
	} {
		root, err := geom.ImportExactRoot(ctx, recipe, geom.ExactRootCodecOptions{})
		if err != nil {
			t.Fatal(err)
		}
		root, _, err = root.AffineMap(ctx, big.NewRat(2, 1), big.NewRat(4, 1), geom.ExactCurvePlaneEventOptions{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, bound := range []*geom.ExactAlgebraicRoot{owned.Start, owned.End} {
			q, err := root.Compare(ctx, bound, geom.ExactCurvePlaneEventOptions{})
			if err != nil || !q.Proven {
				t.Fatal(q, err)
			}
			found = found || q.Order == 0
		}
		if !found {
			t.Fatal("wrong original weighted coedge split", recipe)
		}
	}
}
