package step

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestExactControlEllipticalPrismCore(t *testing.T) {
	t.Parallel()
	b := mappedRationalBasis{Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, X: [3]*big.Rat{big.NewRat(3, 5), big.NewRat(4, 5), new(big.Rat)}, Y: [3]*big.Rat{big.NewRat(-4, 5), big.NewRat(3, 5), new(big.Rat)}, Z: [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	t.Log("construct")
	s, _, e := sewExactControlEllipticalPrism(ctx, b, 5, 2, 3, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("persist")
	var raw bytes.Buffer
	if e = trimmed.WriteJSON(&raw, s); e != nil {
		t.Fatal(e)
	}
	s, e = trimmed.ReadJSON(&raw, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	t.Log("vertices")
	v, e := s.RationalVertices(context.Background())
	if e != nil || len(v) != 8 {
		t.Fatal(v, e)
	}
	nonbinary := false
	for _, p := range v {
		for _, c := range p.Point {
			_, exact := c.Float64()
			nonbinary = nonbinary || !exact
		}
	}
	if !nonbinary {
		t.Fatal("lost rational endpoints")
	}
	t.Log("agreement")
	proof, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal(u)
		}
	}
	t.Log("mass")
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-3, MaxSamples: 200000})
	if e != nil || math.Abs(p.Volume-30*math.Pi) > .1 || p.Centroid.Distance(geom.Vec3{X: .1, Y: 2, Z: 4.5}) > .01 {
		t.Fatal(p, e)
	}
}
