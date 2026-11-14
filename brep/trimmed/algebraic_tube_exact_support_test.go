package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicSectionRetainsNonbinarySupports(t *testing.T) {
	ctx := context.Background()
	var h [][4]*big.Rat
	for x := int64(1); x <= 2; x++ {
		for y := int64(0); y <= 1; y++ {
			h = append(h, [4]*big.Rat{big.NewRat(x, 3), big.NewRat(y, 1), big.NewRat((x-1)*y, 3), big.NewRat(1, 1)})
		}
	}
	p, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, h, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := geom.NewExactBezierSurface(p)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch := func(s geom.Surface) intersect.Patch { return intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}} }
	trace := intersect.SplinePairTraceOptions{}
	options := geom.TubeAlgebraicGraphEndpointOptions{}
	r, e := SectionSplinePairAlgebraic(ctx, patch(a), patch(b), nil, trace, options)
	if e != nil || !r.Complete || len(r.Open) != 1 {
		t.Fatal("the original curved chart did not qualify", len(r.Open), e)
	}
	var out bytes.Buffer
	if e = WriteAlgebraicSplineSectionJSON(&out, r); e != nil {
		t.Fatal(e)
	}
	reload, e := ReadAlgebraicSplineSectionJSON(ctx, bytes.NewReader(out.Bytes()), JSONLimits{}, trace, options)
	if e != nil || !reload.Complete || len(reload.Open) != 1 {
		t.Fatal("native reload did not independently reprove the section", len(reload.Open), e)
	}
	for _, s := range []AlgebraicSplinePairSection{r, reload} {
		var x [2]string
		for i, end := range s.Open[0].Ends {
			v, e := end.Certificate.Parameters[2].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || v.Kind != geom.ExactRootKindRational || end.Operand != 0 {
				t.Fatal("exact source terminal", v, end.Operand, e)
			}
			x[i] = v.Rational.RatString()
		}
		if x != [2]string{"1/3", "2/3"} && x != [2]string{"2/3", "1/3"} {
			t.Fatal("nonrepresentable parameter was lost", x)
		}
	}
	if _, e = ReadAlgebraicSplineSectionJSON(ctx, bytes.NewReader(out.Bytes()), JSONLimits{MaxControlPoints: 3}, trace, options); e == nil {
		t.Fatal("nonbinary support bypassed the serialized control budget")
	}
	// Two XY height graphs cross along original A coordinate v=1/2.
	// Each face trims that branch to a different U interval.
	var net [][4]*big.Rat
	for x := int64(1); x <= 2; x++ {
		for y := int64(0); y <= 1; y++ {
			net = append(net, [4]*big.Rat{big.NewRat(x, 3), big.NewRat(y, 1), big.NewRat(2*y-1, 2), big.NewRat(1, 1)})
		}
	}
	q, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, net, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other, e := geom.NewExactBezierSurface(q)
	if e != nil {
		t.Fatal(e)
	}
	flat, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, makeFlatExactNet(net), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	base, e := geom.NewExactBezierSurface(flat)
	if e != nil {
		t.Fatal(e)
	}
	faces := [2]Face{{Surface: base, Outer: sectionRectangle(.25, .875, .125, .875)}, {Surface: other, Outer: sectionRectangle(.125, .75, .25, .75)}}
	ps := [2]intersect.Patch{patch(base), patch(other)}
	o := AlgebraicTrimmedPairSectionOptions{}
	da, db := flat.Domain(), q.Domain()
	da.UMin, da.UMax = .0625, .9375
	pa, e := flat.Trim(da)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := geom.CertifySurfacePairTube(pa, q, geom.SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !proof.ProvenBranch {
		t.Fatal("exact shared-XY fixture needs an independently proved tube", proof, e)
	}
	// A narrower tracing patch still covers the authored first face. It
	// keeps the verified tube away from the second chart's U boundaries.
	ps[0].Min.X, ps[0].Max.X = da.UMin, da.UMax
	seeds := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &proof}}}
	x, e := SectionTrimmedSplinePair(ctx, ps, faces, &seeds, o)
	if e != nil || !x.Complete || len(x.Branches) != 1 {
		t.Fatal("exact trimmed branch clipping", x.Trace.Issues, e)
	}
	out.Reset()
	if e = WriteTrimmedSplinePairSectionJSON(&out, x); e != nil {
		t.Fatal(e)
	}
	y, e := ReadTrimmedSplinePairSectionJSON(ctx, &out, JSONLimits{}, o)
	if e != nil || !y.Complete || len(y.Branches) != 1 {
		t.Fatal("exact trimmed branch reload", e)
	}
	for _, result := range []AlgebraicTrimmedPairSection{x, y} {
		branch := result.Branches[0]
		if len(branch.Runs) != 1 || len(branch.Runs[0].Intervals) != 1 {
			t.Fatal("one common interior branch expected", branch.Runs)
		}
		inside := 0
		for _, clip := range branch.Clips {
			for _, iv := range clip.Intervals {
				if iv.Retained && iv.Locations == [2]trimvalidate.Location{trimvalidate.Inside, trimvalidate.Inside} {
					inside++
					if iv.Witness.Cmp(big.NewRat(1, 4)) <= 0 || iv.Witness.Cmp(big.NewRat(3, 4)) >= 0 {
						t.Fatal("retained witness outside the original clip intersection", iv.Witness)
					}
				}
			}
		}
		if inside != 1 {
			t.Fatal("wrong retained interval count", inside)
		}
	}
}

func makeFlatExactNet(in [][4]*big.Rat) [][4]*big.Rat {
	out := append([][4]*big.Rat(nil), in...)
	for i := range out {
		out[i][2] = new(big.Rat)
	}
	return out
}

func sectionRectangle(u0, u1, v0, v1 float64) []Use {
	points := []geom.Vec2{{X: u0, Y: v0}, {X: u1, Y: v0}, {X: u1, Y: v1}, {X: u0, Y: v1}}
	var uses []Use
	for i, p := range points {
		uses = append(uses, Use{Edge: i, UVStart: p, UVEnd: points[(i+1)%len(points)]})
	}
	return uses
}
