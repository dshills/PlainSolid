package geom

import (
	"context"
	"testing"
)

func TestDiscoverAffineXYGraphArcTubes(t *testing.T) {
	a, b, arc := commonXYArcFixture(t)
	c := *b
	c.degreeU, c.degreeV = b.degreeV, b.degreeU
	c.countU, c.countV = b.countV, b.countU
	c.knotsU = make([]float64, len(b.knotsV))
	c.knotsV = make([]float64, len(b.knotsU))
	for i, x := range b.knotsV {
		c.knotsU[i] = 2 + 3*x
	}
	for i, x := range b.knotsU {
		c.knotsV[len(b.knotsU)-1-i] = 3 - 6*x
	}
	c.net = make([]exactHomogeneous, len(b.net))
	for i := 0; i < c.countU; i++ {
		for j := 0; j < c.countV; j++ {
			c.net[i*c.countV+j] = b.net[(b.countU-1-j)*b.countV+i]
		}
	}
	start, end := .25, .5
	if arc.Certificate.FixedParameterAxis == 0 {
		start, end = .0625, .25
	}
	r, e := DiscoverAffineXYGraphArcTubes(context.Background(), arc, a, &c, start, end, SurfaceArcTubeDiscoveryOptions{Padding: .1, MaxOperations: 16000000})
	if e != nil || !r.Complete || len(r.Tubes) == 0 {
		t.Fatal(r, e)
	}
	for _, candidate := range r.Tubes {
		if candidate.Proof.b.Domain() != candidate.Box.B || candidate.Box.B.UMin < 2 || candidate.Box.B.UMax > 5 || candidate.Box.B.VMin < -3 || candidate.Box.B.VMax > 3 {
			t.Fatal(candidate)
		}
	}
	if r.OriginalB != c.Domain() {
		t.Fatal("lost original chart")
	}
	limited, e := DiscoverAffineXYGraphArcTubes(context.Background(), arc, a, &c, start, end, SurfaceArcTubeDiscoveryOptions{Padding: .1, MaxIntervals: 3, MaxOperations: 16000000})
	if e == nil || limited.Complete || len(limited.Pending) == 0 || limited.VisitedIntervals > 3 {
		t.Fatal("bounded pending", limited, e)
	}
	tiny, e := DiscoverAffineXYGraphArcTubes(context.Background(), arc, a, &c, start, end, SurfaceArcTubeDiscoveryOptions{MaxOperations: 1})
	if e == nil || tiny.Complete || len(tiny.Pending) == 0 {
		t.Fatal("tiny budget", tiny, e)
	}

	align, e := AlignAffineXYCharts(context.Background(), a, &c, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	mapped, exact, _, e := align.MapSourceBox(context.Background(), arc.source.Domain(), SurfacePatchOptions{})
	if e != nil || !exact {
		t.Fatal(mapped, e)
	}
	missing, e := CertifyAffineXYSurfacePairCellInTubes(context.Background(), a, &c, SurfacePairParameterBox{A: arc.source.Domain(), B: mapped}, nil, SurfaceTubeUnionCoverageOptions{MaxOperations: 100000, MaxCells: 32})
	if missing.Covered {
		t.Fatal("covered without tubes", missing, e)
	}

}
