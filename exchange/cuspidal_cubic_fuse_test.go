package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicFuseExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Fuse", .001
	// The original first stock contributes 6345/512. On the pointed
	// region the second stock adds the independent signed gap 243/22528.
	const volume = 6345.0/512 + 243.0/22528
	so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
	iopts := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_fuse_order_%d.json", order))
			if err != nil {
				t.Fatal(err)
			}
			fuse, err := trimmed.ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicFuseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicFuse(ctx, &st, fuse, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicFuse(ctx, &ig, fuse, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicFuseSTEP(ctx, bytes.NewReader(st.Bytes()), fuse, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicFuseIGES(ctx, bytes.NewReader(ig.Bytes()), fuse, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			changed := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S1."), 1)
			if bytes.Equal(changed, st.Bytes()) ||
				!errors.Is(step.VerifyCuspidalCubicFuseSTEP(ctx, bytes.NewReader(changed), fuse, name, units, step.Limits{}, so), geom.ErrConvergence) {
				t.Fatal("changed Fuse STEP curve ownership passed verification")
			}
			if err := iges.VerifyCuspidalCubicFuseIGES(ctx, bytes.NewReader(ig.Bytes()), fuse, 1, iges.Limits{}, iopts); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed Fuse IGES physical units accepted: %v", err)
			}
			checkCuspidalMaterialReadback(t, ctx, fuse.Boundary(), fuse.History().Vertices, st.Bytes(), ig.Bytes(), volume)
			var refused bytes.Buffer
			limitedStep, limitedIGES := so, iopts
			limitedStep.Proof.MaxWork, limitedIGES.Proof.MaxWork = 1, 1
			if err := step.WriteCuspidalCubicFuse(ctx, &refused, fuse, name, units, step.Limits{}, limitedStep); err == nil || refused.Len() != 0 {
				t.Fatal("limited STEP proof published Fuse geometry", err)
			}
			if err := iges.WriteCuspidalCubicFuse(ctx, &refused, fuse, units, iges.Limits{}, limitedIGES); err == nil || refused.Len() != 0 {
				t.Fatal("limited IGES proof published Fuse geometry", err)
			}
		})
	}
}

func checkCuspidalMaterialReadback(t *testing.T, ctx context.Context, solid trimmed.Solid, section [3]int, stepData, igesData []byte, volume float64) {
	t.Helper()
	vertices := solid.Vertices()
	cusp := vertices[section[0]]
	xyz := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	for _, f := range []struct {
		name, extension, reader string
		data                    []byte
	}{
		{"step", "step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), stepData},
		{"iges", "igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), igesData},
	} {
		if f.reader != "" {
			report := readPinnedExactComponents(t, ctx, f.extension, f.data, f.reader)
			if report.Solids != 1 || len(report.Components) != 1 || report.IndexedVertices != len(vertices) {
				t.Fatalf("independent %s lost the original cusp shell: %+v", f.name, report)
			}
			part := report.Components[0]
			if part.Shells != 1 || part.Faces != len(solid.Faces()) || part.Edges != len(solid.Edges()) ||
				!(math.Abs(part.Volume-volume) <= 1e-8) {
				t.Fatalf("independent %s changed signed material or topology: %+v, want volume %.16g", f.name, part, volume)
			}
			original := make(map[[3]float64]bool, len(vertices))
			for _, p := range vertices {
				original[xyz(p)] = true
			}
			for _, p := range part.Vertices {
				if !original[p] {
					t.Fatalf("independent %s changed an exact vertex %v", f.name, p)
				}
				delete(original, p)
			}
			if len(original) != 0 {
				t.Fatalf("independent %s omitted %d original vertices", f.name, len(original))
			}
			exits := map[[3]float64]bool{xyz(vertices[section[1]]): true, xyz(vertices[section[2]]): true}
			for _, ends := range part.EdgeVertices {
				if ends[0] < 0 || ends[0] >= len(vertices) || ends[1] < 0 || ends[1] >= len(vertices) {
					t.Fatalf("independent %s lost a model endpoint", f.name)
				}
				a, b := part.Vertices[ends[0]], part.Vertices[ends[1]]
				if b == xyz(cusp) {
					a, b = b, a
				}
				if a == xyz(cusp) {
					if !exits[b] {
						t.Fatalf("independent %s changed the directed cusp incidence to %v", f.name, b)
					}
					delete(exits, b)
				}
			}
			if len(exits) != 0 {
				t.Fatalf("independent %s omitted an original cusp germ", f.name)
			}
			t.Logf("%s reopened one %d-face solid with %d edges, %d vertices and signed volume %.15g",
				f.name, part.Faces, part.Edges, report.IndexedVertices, part.Volume)
		}
		checkCuspidalCubicReadbackJets(t, ctx, f.name, f.extension, f.data, cusp)
	}
}
