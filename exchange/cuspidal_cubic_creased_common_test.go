package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicCreasedCommonExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Common", .001
	// The original shared hinge adds 729/4096 on the full footprint.
	const volume = 277485.0/22528 + 729.0/4096
	so := step.CuspidalCubicSTEPOptions{PoleTolerance: 1e-14}
	iopts := iges.CuspidalCubicIGESOptions{PoleTolerance: 1e-14}
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_creased_common_order_%d.json", order))
			if err != nil {
				t.Fatal(err)
			}
			common, err := trimmed.ReadCuspidalCubicCommonJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCommonOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicCommon(ctx, &st, common, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicCommon(ctx, &ig, common, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicCommonSTEP(ctx, bytes.NewReader(st.Bytes()), common, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicCommonIGES(ctx, bytes.NewReader(ig.Bytes()), common, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			// The exact exported shell has four more vertices and edges
			// than the native one. Bound the neutral topology after splitting.
			var refused bytes.Buffer
			for _, limits := range []step.Limits{{MaxVertices: 16}, {MaxEdgeUses: 47}} {
				if err := step.WriteCuspidalCubicCommon(ctx, &refused, common, name, units, limits, so); !errors.Is(err, geom.ErrConvergence) || refused.Len() != 0 {
					t.Fatal("limited STEP writer published a partial knotted shell", err)
				}
			}
			ioptsLimited := iopts
			ioptsLimited.Proof.MaxWork = 1
			if err := iges.WriteCuspidalCubicCommon(ctx, &refused, common, units, iges.Limits{}, ioptsLimited); err == nil || refused.Len() != 0 {
				t.Fatal("limited IGES proof published a partial knotted shell", err)
			}
			checkCuspidalCreasedReadback(t, ctx, common.Boundary(), common.History().Vertices[0],
				[3]int{9, 24, 17}, st.Bytes(), ig.Bytes(), volume)
		})
	}
}

func checkCuspidalCreasedReadback(t *testing.T, ctx context.Context, source trimmed.Solid, cusp int, counts [3]int,
	stepData, igesData []byte, volume float64) {
	t.Helper()
	b, err := source.ExtractExactBezierBoundaryAtRationalKnots(ctx, edgecheck.Options{})
	if err != nil || ([3]int{len(b.Faces), len(b.Edges), len(b.Vertices)}) != counts {
		t.Fatalf("native neutral proof changed source topology: %v", err)
	}
	point := source.Vertices()[cusp]
	for _, f := range []struct {
		name, ext, reader string
		data              []byte
	}{
		{"step", "step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), stepData},
		{"iges", "igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), igesData},
	} {
		if f.reader != "" {
			out := readPinnedExactComponents(t, ctx, f.ext, f.data, f.reader)
			if out.Solids != 1 || len(out.Components) != 1 || out.IndexedVertices != counts[2] {
				t.Fatalf("independent %s lost the exact neutral shell: %+v", f.name, out)
			}
			part := out.Components[0]
			if part.Shells != 1 || part.Faces != counts[0] || part.Edges != counts[1] ||
				!(math.Abs(part.Volume-volume) <= 1e-8) {
				t.Fatalf("independent %s changed creased source topology or signed volume: %+v", f.name, part)
			}
			vertices := make(map[[3]float64]bool, len(b.Vertices))
			for _, v := range b.Vertices {
				vertices[[3]float64{v.X, v.Y, v.Z}] = true
			}
			for _, v := range part.Vertices {
				if !vertices[v] {
					t.Fatalf("independent %s displaced an exact neutral joint: %v", f.name, v)
				}
				delete(vertices, v)
			}
			if len(vertices) != 0 {
				t.Fatalf("independent %s omitted exact neutral vertices: %v", f.name, vertices)
			}
			t.Logf("%s retained %d faces, %d edges, %d vertices; volume %.15g area %.15g",
				f.name, part.Faces, part.Edges, out.IndexedVertices, part.Volume, part.Area)
		}
		// The crease at x=21/16 is halfway along the original
		// normalized outward germ clock, whose x law is quadratic.
		checkCuspidalCubicJetsOnSpan(t, ctx, f.name, f.ext, f.data, point, .5)
	}
}
