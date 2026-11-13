package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestCertifyAlgebraicSplineBranchTerminals(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	var poles []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	a, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	k = []float64{0, 0, 1, 1}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}, {X: 1, Y: 1, Z: .5}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	d := geom.ParameterRectangle{UMin: 7.0 / 32, UMax: .25, VMin: 5.0 / 8, VMax: 11.0 / 16}
	other := geom.ParameterRectangle{UMin: 3.0 / 16, UMax: 9.0 / 32, VMin: 19.0 / 32, VMax: 23.0 / 32}
	chart := func(s *geom.BSplineSurface, r geom.ParameterRectangle) intersect.Patch {
		return intersect.Patch{Surface: s, Min: geom.Vec2{X: r.UMin, Y: r.VMin}, Max: geom.Vec2{X: r.UMax, Y: r.VMax}}
	}
	pa, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pa, e = pa.Trim(d)
	if e != nil {
		t.Fatal(e)
	}
	pb, e = pb.Trim(other)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := geom.CertifySurfacePairTube(pa, pb, geom.SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !proof.ProvenBranch {
		t.Fatal(proof, e)
	}
	seeds := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: other}, Tube: &proof}}}
	x, y := chart(a, d), chart(b, other)
	segments, e := intersect.SplineBranchCurves(ctx, x, y, seeds, intersect.SplineTubeBranch{Cells: []intersect.OrientedTubeCell{{Cell: 0, FreeAxis: 0}}}, intersect.TubeCurveOptions{})
	if e != nil || len(segments) != 1 {
		t.Fatal(segments, e)
	}
	t.Run("two face trims and exact original spline parameter", func(t *testing.T) {
		rect := func(minU, minV, maxU, maxV float64, clockwise bool) []Use {
			p := [4]geom.Vec2{{X: minU, Y: minV}, {X: maxU, Y: minV}, {X: maxU, Y: maxV}, {X: minU, Y: maxV}}
			if clockwise {
				p[1], p[3] = p[3], p[1]
			}
			l := make([]Use, 4)
			for i := range p {
				l[i] = Use{Edge: 20 + i, UVStart: p[i], UVEnd: p[(i+1)%4]}
			}
			return l
		}
		cut := 15.0 / 64
		outer := rect(cut, 0, .5, 1, false)
		line, e := geom.NewBSpline(1, []geom.Vec3{{X: cut}, {X: cut, Y: .5}, {X: cut, Y: 1}}, []float64{2, 2, 3, 6, 6}, nil)
		if e != nil {
			t.Fatal(e)
		}
		outer[3].Curve = SplineUV{Curve: line, Range: UVInterval{2, 6}, Reversed: true}
		outer[3].Reversed = true
		fa := Face{Surface: a, Outer: outer}
		fb := Face{Surface: b, Outer: rect(0, 0, 31.0/128, 1, false)}
		q, e := ClipSplineTubeByFaceTrims(ctx, segments[0], [2]Face{fa, fb}, SplineTubeClipOptions{})
		if e != nil || !q.Complete || len(q.Events) != 4 || len(q.Intervals) != 3 ||
			q.Intervals[0].Retained || !q.Intervals[1].Retained || q.Intervals[2].Retained {
			t.Fatalf("complete=%v, events=%d, intervals=%+v, err=%v", q.Complete, len(q.Events), q.Intervals, e)
		}
		if q.Intervals[0].Locations != [2]trimvalidate.Location{trimvalidate.Outside, trimvalidate.Inside} ||
			q.Intervals[2].Locations != [2]trimvalidate.Location{trimvalidate.Inside, trimvalidate.Outside} {
			t.Fatal(q.Intervals)
		}
		contacts := q.Events[1].Contacts[0]
		if len(contacts) != 1 || len(q.Events[2].Contacts[1]) != 1 {
			t.Fatal(q.Events)
		}
		h := contacts[0]
		if h.Source != (SplineTubeTrimSource{Loop: 0, Use: 3, Edge: 23, Span: 1, Reversed: true, ParameterStart: 6, ParameterEnd: 3}) || !h.Transverse {
			t.Fatal(h.Source, h.Transverse)
		}
		// At u=15/64, v²=1823/4096. On the second source knot span
		// v=(t-3)/6+1/2=t/6, so t²=16407/1024.
		want, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1,
			Coefficients: []string{"-16407", "0", "1024"}, Lower: "3", Upper: "5"}, geom.ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		comparison, e := h.Parameter.Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !comparison.Proven || comparison.Order != 0 {
			t.Fatal(comparison, e)
		}
		// The same original branch crosses a clockwise hole twice.
		fa.Outer = rect(0, 0, 1, 1, false)
		fb = Face{Surface: b, Outer: rect(0, 0, 1, 1, false), Holes: [][]Use{
			rect(cut, 5.0/8, 31.0/128, 11.0/16, true),
		}}
		q, e = ClipSplineTubeByFaceTrims(ctx, segments[0], [2]Face{fa, fb}, SplineTubeClipOptions{})
		if e != nil || !q.Complete || len(q.Intervals) != 3 || !q.Intervals[0].Retained ||
			q.Intervals[1].Retained || !q.Intervals[2].Retained || q.Events[1].Contacts[1][0].Source.Loop != 1 {
			t.Fatalf("complete=%v intervals=%+v err=%v", q.Complete, q.Intervals, e)
		}
		// Exact equal free roots group contacts from both charts, while
		// an insufficient proof budget cannot return a complete clip.
		fa.Outer = rect(cut, 0, 1, 1, false)
		fb = Face{Surface: b, Outer: rect(cut, 0, 1, 1, false)}
		q, e = ClipSplineTubeByFaceTrims(ctx, segments[0], [2]Face{fa, fb}, SplineTubeClipOptions{})
		if e != nil || !q.Complete || len(q.Events) != 3 || len(q.Events[1].Contacts[0]) != 1 || len(q.Events[1].Contacts[1]) != 1 {
			t.Fatalf("complete=%v events=%+v err=%v", q.Complete, q.Events, e)
		}
		tangent := [4]geom.Vec2{{X: 1880.0 / 8192, Y: 5480.0 / 8192}, {X: 1993.0 / 8192, Y: 5439.0 / 8192},
			{X: 1993.0 / 8192, Y: 11.0 / 16}, {X: 1880.0 / 8192, Y: 11.0 / 16}}
		loop := make([]Use, 4)
		for i := range tangent {
			loop[i] = Use{Edge: i, UVStart: tangent[i], UVEnd: tangent[(i+1)%4]}
		}
		small := Face{Surface: a, Outer: loop}
		bounded := Face{Surface: b, Outer: rect(other.UMin, other.VMin, other.UMax, other.VMax, false)}
		// u²+v²=1/2 touches 41u+113v=85 at (41/170,113/170),
		// which lies strictly within this exact rational line segment.
		// At every other point on the branch the linear form is <85.
		s, e := SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{small, bounded}, &seeds, AlgebraicTrimmedPairSectionOptions{})
		if e != nil || !s.Complete || len(s.Branches) != 1 || len(s.Branches[0].Runs) != 0 ||
			len(s.Branches[0].Nodes) != 3 || !s.Branches[0].Nodes[1].Isolated ||
			!s.Branches[0].Clips[0].Events[1].Contacts[0][0].Tangent {
			t.Fatalf("complete=%v branches=%+v trace=%v err=%v", s.Complete, s.Branches, s.Trace.Issues, e)
		}
		small.Outer = rect(cut, d.VMin, 63.0/256, d.VMax, false)
		s, e = SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{small, bounded}, &seeds, AlgebraicTrimmedPairSectionOptions{})
		if e != nil || !s.Complete || len(s.Branches) != 1 || len(s.Branches[0].Runs) != 1 ||
			len(s.Branches[0].Runs[0].Intervals) != 1 || s.Branches[0].Runs[0].Closed {
			t.Fatalf("complete=%v branches=%+v err=%v", s.Complete, s.Branches, e)
		}
		s.Branches[0].Runs = nil
		small.Outer[1].Edge = -1
		p, e := s.PartitionFaceByTubeRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil || p.Support != a || len(p.Cut) != 1 || !p.Fragments[0].CutReversed || p.Fragments[1].CutReversed ||
			len(p.Fragments[0].Outer) != 3 || len(p.Fragments[1].Outer) != 3 {
			t.Fatalf("cut=%d fragments=%+v err=%v", len(p.Cut), p.Fragments, e)
		}
		roots := map[int][][2]*geom.ExactAlgebraicRoot{}
		for _, f := range p.Fragments {
			for _, use := range f.Outer {
				if use.Edge != use.Use+20 {
					t.Fatal("source edge identity changed", use)
				}
				roots[use.Use] = append(roots[use.Use], [2]*geom.ExactAlgebraicRoot{use.Start, use.End})
			}
		}
		for _, i := range []int{1, 3} {
			if len(roots[i]) != 2 {
				t.Fatal("terminal edge split count", i)
			}
			a, z := roots[i][0], roots[i][1]
			c, e := a[1].Compare(ctx, z[0], geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if c.Order != 0 {
				c, e = z[1].Compare(ctx, a[0], geom.ExactCurvePlaneEventOptions{})
			}
			if e != nil || !c.Proven || c.Order != 0 {
				t.Fatal("two edge pieces do not share exact split", i, c, e)
			}
		}
		if p, e := s.PartitionFaceByTubeRun(ctx, 1, 0, 0, AlgebraicTubeFacePartitionOptions{}); !errors.Is(e, geom.ErrConvergence) || len(p.Cut) != 0 {
			t.Fatal("cut that ends in the opposite face interior accepted", e)
		}
		fb = Face{Surface: b, Outer: rect(cut, other.VMin, 63.0/256, other.VMax, false)}
		s, e = SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{small, fb}, &seeds, AlgebraicTrimmedPairSectionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		pair, e := s.PartitionBothFacesByTubeRun(ctx, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil || len(pair.Faces[0].Cut) != 1 || len(pair.Faces[1].Cut) != 1 ||
			pair.Faces[0].Cut[0].FreeStart != pair.Faces[1].Cut[0].FreeStart ||
			pair.Faces[0].Cut[0].FreeEnd != pair.Faces[1].Cut[0].FreeEnd {
			t.Fatalf("paired cut does not share exact section identity: %v", e)
		}
		small.Holes = [][]Use{
			rect(61.0/256, 21.0/32, 62.0/256, 169.0/256, true),
			rect(61.0/256, 173.0/256, 62.0/256, 174.0/256, true),
		}
		s, e = SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{small, fb}, &seeds, AlgebraicTrimmedPairSectionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = s.PartitionFaceByTubeRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil || len(p.Fragments[0].Holes) != 1 || len(p.Fragments[1].Holes) != 1 {
			t.Fatalf("open cut hole assignment: %+v err=%v", p.Fragments, e)
		}
		// The lower hole has u²+v²<1/2 everywhere, so ZA−ZB<0.
		for i, f := range p.Fragments {
			want := 1
			if f.Holes[0][0].Loop == 1 {
				want = -1
			}
			if p.FragmentHeightSigns[i] != want {
				t.Fatalf("analytic height sign for hole %d is %d, got %d", f.Holes[0][0].Loop, want, p.FragmentHeightSigns[i])
			}
		}
		small.Holes = nil
		// The branch enters through the outer loop, then reaches a
		// hole whose right edge extends beyond the other operand trim.
		// Cutting this run merges the outer and hole loops without
		// disconnecting material.
		small.Outer = rect(cut, d.VMin, .25, d.VMax, false)
		small.Holes = [][]Use{rect(61.0/256, 21.0/32, 127.0/512, 43.0/64, true)}
		fb = Face{Surface: b, Outer: rect(cut, other.VMin, 31.0/128, other.VMax, false)}
		s, e = SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{small, fb}, &seeds, AlgebraicTrimmedPairSectionOptions{})
		if e != nil || !s.Complete || len(s.Branches[0].Runs) != 1 {
			t.Fatalf("outer-hole section: %v", e)
		}
		p, e = s.PartitionFaceByTubeRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil || len(p.ConnectedBoundary) != 12 || len(p.RemainingHoles) != 0 ||
			len(p.Fragments[0].Outer) != 0 || len(p.Fragments[1].Outer) != 0 {
			t.Fatalf("merged boundary uses=%d err=%v", len(p.ConnectedBoundary), e)
		}
		cuts := map[bool]int{}
		for _, u := range p.ConnectedBoundary {
			if u.Original == nil {
				cuts[u.CutReversed]++
			}
		}
		if cuts[true] != 1 || cuts[false] != 1 {
			t.Fatal(cuts)
		}
		small.Holes = nil
		// A convex arc can also enter and exit a face through the same
		// original edge. Both roots remain distinct portions of that use.
		tangent[0].Y -= 1.0 / (1 << 24)
		tangent[1].Y -= 1.0 / (1 << 24)
		for i := range tangent {
			loop[i].UVStart, loop[i].UVEnd = tangent[i], tangent[(i+1)%4]
		}
		small.Outer = loop
		s, e = SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{small, bounded}, &seeds, AlgebraicTrimmedPairSectionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = s.PartitionFaceByTubeRun(ctx, 0, 0, 0, AlgebraicTubeFacePartitionOptions{})
		if e != nil || p.Terminals[0].Source.Use != 0 || p.Terminals[1].Source.Use != 0 ||
			len(p.Fragments[0].Outer)+len(p.Fragments[1].Outer) != 6 ||
			min(len(p.Fragments[0].Outer), len(p.Fragments[1].Outer)) != 1 {
			t.Fatalf("same use cut fragments=%+v err=%v", p.Fragments, e)
		}
		if bad, e := SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, [2]Face{fa, fb}, &seeds, AlgebraicTrimmedPairSectionOptions{}); !errors.Is(e, geom.ErrDomain) || bad.Complete {
			t.Fatal("partial chart covered full trims", bad.Complete, e)
		}
		if q, e = ClipSplineTubeByFaceTrims(ctx, segments[0], [2]Face{fa, fb}, SplineTubeClipOptions{MaxIntervals: 1}); !errors.Is(e, geom.ErrConvergence) || q.Complete {
			t.Fatal(q.Complete, e)
		}
		axis, _, _, e := segments[0].ExactFreeDomain()
		if e != nil || axis != 0 {
			t.Fatal(axis, e)
		}
		if w := q.Intervals; len(w) != 0 {
			t.Fatal(w)
		}
		boxLo := [2]*big.Rat{big.NewRat(1, 4), big.NewRat(2, 3)}
		boxHi := [2]*big.Rat{big.NewRat(1, 4), big.NewRat(7, 10)}
		loc, cert, e := LocateRationalBoxInFaceTrim(ctx, fa, boxLo, boxHi, trimvalidate.Options{})
		if e != nil || cert.Status != trimvalidate.Certified || loc != trimvalidate.Inside {
			t.Fatal(loc, cert, e)
		}
	})
	root := func(poly []string) *geom.ExactAlgebraicRoot {
		r, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: poly, Lower: "5/8", Upper: "11/16"}, geom.ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	ends := [2]AlgebraicSplineBranchTerminal{{0, geom.SurfaceEdgeUMin, root([]string{"-463", "0", "1024"})}, {0, geom.SurfaceEdgeUMax, root([]string{"-7", "0", "16"})}}
	r, e := CertifyAlgebraicSplineBranchTerminals(ctx, segments, x, y, ends, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || r.Segments != 1 {
		t.Fatal(r, e)
	}
	for i, q := range r.Ends {
		auto, e := DiscoverAlgebraicSplineTerminal(ctx, segments[0], i, x, y, geom.TubeAlgebraicGraphEndpointOptions{})
		if e != nil || auto.Side != q.Side || auto.Operand != q.Operand {
			t.Fatal(auto, e)
		}
		for j, a := range auto.Certificate.Parameters {
			p, e := a.Compare(ctx, q.Certificate.Parameters[j], geom.ExactCurvePlaneEventOptions{})
			if e != nil || !p.Proven || p.Order != 0 {
				t.Fatal(p, e)
			}
		}
		if !q.Certificate.Proven || q.Certificate.SourceIsoMultiplicity != 1 || q.Certificate.ModelEvent == nil {
			t.Fatal(q)
		}
		b, e := q.Certificate.ModelEvent.Enclose(ctx, geom.ComposedCurveEndpointOptions{})
		if e != nil || math.Abs(b.Point.X-[]float64{7.0 / 32, .25}[i]) > 1e-9 || math.Abs(b.Point.Z-.5) > 1e-9 {
			t.Fatal(b, e)
		}
	}
	q, e := CertifyAlgebraicSplineBranch(ctx, segments, x, y, ends, nil, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !q.Complete || len(q.Junctions) != 0 {
		t.Fatal(q, e)
	}
	autoBranch, e := DiscoverAlgebraicSplineBranch(ctx, segments, x, y, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !autoBranch.Complete || autoBranch.Ends[0].Side != geom.SurfaceEdgeUMin || autoBranch.Ends[1].Side != geom.SurfaceEdgeUMax {
		t.Fatal(autoBranch, e)
	}
	section, e := SectionSplinePairAlgebraic(ctx, x, y, &seeds, intersect.SplinePairTraceOptions{}, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !section.Complete || !section.Trace.Coverage.CoversAllContacts || len(section.Open) != 1 || section.Closed != 0 {
		t.Fatal(section, e)
	}
	var file bytes.Buffer
	if e := WriteAlgebraicSplineSectionJSON(&file, section); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadAlgebraicSplineSectionJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{}, intersect.SplinePairTraceOptions{}, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !restored.Complete || len(restored.Open) != 1 {
		t.Fatal(restored, e)
	}
	var malformed nativeAlgebraicTubeSection
	if e := json.Unmarshal(file.Bytes(), &malformed); e != nil {
		t.Fatal(e)
	}
	malformed.Tubes[0].Cells[0].DomainA.UMax = 15.0 / 64
	changed, e := json.Marshal(malformed)
	if e != nil {
		t.Fatal(e)
	}
	if bad, e := ReadAlgebraicSplineSectionJSON(ctx, bytes.NewReader(changed), JSONLimits{}, intersect.SplinePairTraceOptions{}, geom.TubeAlgebraicGraphEndpointOptions{}); e == nil || bad.Complete {
		t.Fatal("incomplete coverage accepted", e)
	}
	ends[1].Side = geom.SurfaceEdgeUMin
	if _, e := CertifyAlgebraicSplineBranchTerminals(ctx, segments, x, y, ends, geom.TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal(e)
	}
}
