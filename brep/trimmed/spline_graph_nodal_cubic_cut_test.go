package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeNodalCubicCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	o := NodalCubicCutOptions{}
	if out, err := CutNodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o); !errors.Is(err, geom.ErrDegenerate) || out != nil {
		t.Fatal("the two exterior sectors form a same-component pinch", err)
	}
	r, err := CutNodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	volumes, total := r.ExactComponentVolumes()
	if volumes[0].RatString() != "1/100925440" || volumes[1].RatString() != "4619/100925440" || total.RatString() != "3/65536" {
		t.Fatal("original source integrals", volumes, total)
	}
	c := r.Contact()
	for i, part := range r.Components() {
		h := r.boundaries[i]
		if part.faces[h.SplineFace].Surface != src[2] || part.faces[h.PlaneFace].Surface != src[1] ||
			len(part.Topology().Shells) != 1 || part.Topology().EulerCharacteristic != 2 {
			t.Fatal("positive component loses original cap ownership", i)
		}
		if len(h.ContourEdges) != ([2]int{4, 2})[i] || len(h.WallFaces) != 2*i {
			t.Fatal("original section or stock walls lost", i, h)
		}
		if part.vertices[c.Vertices[i]] != (geom.Vec3{X: 39.0 / 16, Y: 1, Z: 2243.0 / 1024}) {
			t.Fatal("the contact vertex changes between components", i, c)
		}
		mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		want, _ := volumes[i].Float64()
		if err != nil || math.Abs(mass.Volume-want) > 1e-5*want {
			t.Fatal("native Cut differs from its exact source integral", i, mass.Volume, want, err)
		}
		t.Logf("native Cut component %d volume %.15g", i, mass.Volume)
	}
}

func TestNativeNodalCubicCutChangedGap(t *testing.T) {
	ctx := context.Background()
	// Every height-gap coefficient is doubled. The node and exact
	// original XY limits are the same, so both source volumes double.
	src := nodalCubicStockFixture(t, [4]float64{-1521.0 / 8192, 403.0 / 8192, -105.0 / 8192, 27.0 / 8192}, 3.0/32)
	r, err := CutNodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, NodalCubicCutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	volumes, total := r.ExactComponentVolumes()
	if volumes[0].Cmp(big.NewRat(1, 50462720)) != 0 || volumes[1].Cmp(big.NewRat(4619, 50462720)) != 0 ||
		total.Cmp(big.NewRat(3, 32768)) != 0 {
		t.Fatal("exact volume used a fixture-specific height amplitude", volumes, total)
	}
	volumes[0].SetInt64(0)
	original, _ := r.ExactComponentVolumes()
	if original[0].Cmp(big.NewRat(1, 50462720)) != 0 {
		t.Fatal("exact volume exposed mutable source data")
	}
	nonbinary := nodalCubicStockFixture(t, [4]float64{-20.0 / 256, 12.0 / 256, -7.0 / 256, 4.0 / 256}, 81.0/1024)
	if out, err := CutNodalCubicSplineGraphs(ctx, nonbinary[0], nonbinary[2], nonbinary[1], 1, 1e-9, NodalCubicCutOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("loss of exact native vertex was hidden by the volume path", err)
	}
}
