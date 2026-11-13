package iges

import (
	"errors"
	"plainsolid/geom"
	"testing"
)

func TestSourceWeightRoundingCannotCertifyTrim(t *testing.T) {
	raw := "128,1,1,1,1,0,0,0,0,0,0,0,1,1,0,0,1,1,1,3,1,3,0,0,0,2,0,0,0,3,0,2,3,0,0,1,0,1;"
	budget := 1000
	p, e := parseParameters(raw, ',', ';', &budget)
	if e != nil {
		t.Fatal(e)
	}
	source, e := decode(128, p, 4)
	if e != nil {
		t.Fatal(e)
	}
	if !source.sourceWeightsRounded {
		t.Fatal("lost source normalization provenance")
	}
	// Ordinary geometry decode succeeds. A newly constructed exact contour of
	// that rounded geometry is internally consistent, but cannot certify source.
	m, e := NewRectangularSurface(source.Surface.Geometry, geom.ParameterRectangle{UMax: 1, VMax: 1}, 1)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e != nil {
		t.Fatal(e)
	}
	m.Entities[m.CurvesOnSurfaces[0].Surface] = source
	if e = m.ValidateCurveOnSurface(m.CurvesOnSurfaces[0]); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal(e)
	}
	if e = m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e == nil {
		t.Fatal("rounded source certified")
	}
	// Direct composite leaves must carry the same rejection, not just support.
	m.Entities[m.CurvesOnSurfaces[0].Surface].sourceWeightsRounded = false
	root := m.Entities[m.CurvesOnSurfaces[0].ModelCurve].Composite
	m.Entities[root.Curves[0]].sourceWeightsRounded = true
	if e = m.ValidateCurveOnSurface(m.CurvesOnSurfaces[0]); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal(e)
	}
}

func TestCurveSourceNormalizationProvenance(t *testing.T) {
	for _, tc := range []struct {
		weights string
		rounded bool
	}{{"1,3", true}, {"1,2", false}} {
		budget := 1000
		p, e := parseParameters("126,1,1,0,0,0,0,0,0,1,1,"+tc.weights+",0,0,0,2,0,0,0,1,0,0,0;", ',', ';', &budget)
		if e != nil {
			t.Fatal(e)
		}
		ent, e := decode(126, p, 2)
		if e != nil {
			t.Fatal(e)
		}
		if ent.sourceWeightsRounded != tc.rounded {
			t.Fatal(tc, ent.sourceWeightsRounded)
		}
	}
}
