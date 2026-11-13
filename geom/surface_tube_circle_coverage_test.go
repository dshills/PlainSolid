package geom

import (
	"math"
	"testing"
)

func TestCircleTubeCoverageClosedSlabs(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	poles := make([]Vec3, 9)
	q := .375 * .375
	z := [3]float64{q, -q, q}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			poles[i*3+j] = Vec3{.125 + .375*float64(i), .125 + .375*float64(j), 25.0/256 - z[i] - z[j]}
		}
	}
	s, _ := NewBSplineSurface(2, 2, 3, 3, poles, []float64{.125, .125, .125, .875, .875, .875}, []float64{.125, .125, .125, .875, .875, .875}, nil)
	b, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	var tubes []SurfacePairTubeRegion
	var add func(int, float64, float64, float64, int)
	add = func(axis int, lo, hi, sign float64, depth int) {
		f := func(x float64) float64 { return .5 + sign*math.Sqrt(25.0/256-(x-.5)*(x-.5)) }
		y0, y1 := min(f(lo), f(hi)), max(f(lo), f(hi))
		if lo <= .5 && hi >= .5 {
			y0, y1 = min(y0, .5+sign*5.0/16), max(y1, .5+sign*5.0/16)
		}
		pad := min((hi-lo)/2, .03125)
		d := ParameterRectangle{lo, hi, y0 - pad, y1 + pad}
		g := d
		g.UMin, g.UMax = lo-pad, hi+pad
		if axis == 1 {
			d = ParameterRectangle{d.VMin, d.VMax, d.UMin, d.UMax}
			g = ParameterRectangle{g.VMin, g.VMax, g.UMin, g.UMax}
		}
		x, e := a.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		y, e := b.Trim(g)
		if e != nil {
			t.Fatal(e)
		}
		proof, e := CertifySurfacePairTube(x, y, SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(e)
		}
		if !proof.ProvenBranch {
			if depth == 8 {
				t.Fatal("no tube")
			}
			mid := (lo + hi) / 2
			add(axis, lo, mid, sign, depth+1)
			add(axis, mid, hi, sign, depth+1)
			return
		}
		tubes = append(tubes, SurfacePairTubeRegion{Box: SurfacePairParameterBox{d, g}, FreeAxis: axis})
	}
	for _, sign := range []float64{-1, 1} {
		add(0, .5-3.0/16, .5+3.0/16, sign, 0)
		add(1, .5-4.0/16, .5+4.0/16, sign, 0)
	}
	isolation, e := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{ParameterTolerance: .25, SeparatingProjections: true})
	if e != nil {
		t.Fatal(e)
	}
	failures := 0
	for i, cell := range isolation.Candidates {
		r, e := CertifySurfacePairCellInTubes(a, b, SurfacePairParameterBox{cell.DomainA, cell.DomainB}, tubes, SurfaceTubeUnionCoverageOptions{MaxOperations: 16000000})
		if e != nil || !r.Covered {
			failures++
			t.Logf("candidate %d unresolved=%+v pending=%+v ops=%d err=%v", i, r.Unresolved, r.Pending, r.ExactOperations, e)
		}
	}
	if failures > 0 {
		t.Fatalf("%d/%d unresolved", failures, len(isolation.Candidates))
	}
}
