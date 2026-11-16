package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestNativeNodalCubicCut(t *testing.T) {
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

func TestNativeNodalCubicCutReplay(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	options := NodalCubicCutOptions{}
	r, err := CutNodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, options)
	if err != nil {
		t.Fatal(err)
	}
	c := r.Contact()
	if c.UV[0][0].RatString() != "3" || c.UV[0][1].RatString() != "-9/4" ||
		c.UV[1][0].RatString() != "13/16" || c.UV[1][1].RatString() != "1/2" {
		t.Fatal("original owner coordinates of the Cut node changed", c.UV)
	}
	c.UV[0][0].SetInt64(99)
	if r.Contact().UV[0][0].RatString() != "3" {
		t.Fatal("contact exposed mutable source data")
	}
	var raw, reencoded bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadNodalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.WriteJSON(ctx, &reencoded); err != nil || !bytes.Equal(raw.Bytes(), reencoded.Bytes()) {
		t.Fatal("native Cut replay changes original source, volumes or node incidence", err)
	}
	for _, update := range []func(*nodalCubicCutLedger){
		func(l *nodalCubicCutLedger) { l.Components[0].Volume = "1" },
		func(l *nodalCubicCutLedger) { l.Vertices[1]++ },
		func(l *nodalCubicCutLedger) { l.Rays[0][0] = l.Rays[0][1] },
		func(l *nodalCubicCutLedger) { l.Components[1].Walls = l.Components[1].Walls[:1] },
	} {
		var change nodalCubicCutRecipe
		if err := json.Unmarshal(raw.Bytes(), &change); err != nil {
			t.Fatal(err)
		}
		update(change.Ledger)
		reencoded.Reset()
		if err := writePolynomialGraphRecipe(ctx, &reencoded, change); err != nil {
			t.Fatal(err)
		}
		if out, err := ReadNodalCubicCutJSON(ctx, &reencoded, JSONLimits{}, options); !errors.Is(err, geom.ErrConvergence) || out != nil {
			t.Fatal("altered original layer ledger was accepted", err)
		}
	}
	for _, l := range []JSONLimits{{MaxBytes: int64(raw.Len() - 1)}, {MaxControlPoints: 4}, {MaxEntities: 7}} {
		if out, err := ReadNodalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), l, options); err == nil || out != nil {
			t.Fatal("limited source reproof published Cut components", l, err)
		}
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

func TestNativeNodalCubicCutDisplay(t *testing.T) {
	ctx := context.Background()
	src := nodalCubicCommonFixture(t)
	r, err := CutNodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, NodalCubicCutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	meshes, err := r.Tessellate(ctx, o)
	if err != nil || len(meshes) != 2 {
		t.Fatal("closed Cut component display", err)
	}
	for i, part := range r.Components() {
		m := meshes[i]
		if len(m.Vertices) < len(part.vertices) || !slices.Equal(m.Vertices[:len(part.vertices)], part.vertices) || m.SignedVolume() <= 0 {
			t.Fatal("Cut display changes an original vertex or positive component", i, m.SignedVolume())
		}
		t.Logf("component %d display %d triangles volume %.15g", i, len(m.Triangles), m.SignedVolume())
	}
	c := r.Contact()
	declared := []collision.AssemblyVertexContact{{A: collision.AssemblyVertex{Solid: 0, Vertex: c.Vertices[0]}, B: collision.AssemblyVertex{Solid: 1, Vertex: c.Vertices[1]}}}
	if report, err := collision.ValidateMeshVertexContactAssembly(ctx, meshes, declared, collision.VertexContactAssemblyOptions{}); err != nil ||
		!slices.Equal(report.Contacts, declared) {
		t.Fatal("display contact differs from the isolated original point", report, err)
	}
	o.MaxTriangles = 1
	if out, err := r.Tessellate(ctx, o); !errors.Is(err, parametric.ErrBudget) || out != nil {
		t.Fatal("Cut display publishes a partial assembly", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := r.Tessellate(cancelled, o); !errors.Is(err, context.Canceled) || out != nil {
		t.Fatal("cancelled Cut display publishes components", err)
	}
}
