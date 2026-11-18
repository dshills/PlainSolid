package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestWrappedCylinderBoreRimFilletNeutralTransfer(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{Z: 2}, 6.25, 8, o)
	if err != nil {
		t.Fatal(err)
	}
	hole, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: 1.5, Y: 2, Z: 2}, 2, 8, o)
	if err != nil {
		t.Fatal(err)
	}
	stock, _, err := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, hole, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	formats := []struct {
		ext   string
		write func(context.Context, io.Writer, trimmed.Solid, geom.SurfacePatchOptions) error
	}{
		{"step", func(ctx context.Context, w io.Writer, s trimmed.Solid, o geom.SurfacePatchOptions) error {
			return step.WriteWrappedCylinderBoreRimFillet(ctx, w, s, "eccentric through-bore rim fillets", .001, o)
		}},
		{"igs", func(ctx context.Context, w io.Writer, s trimmed.Solid, o geom.SurfacePatchOptions) error {
			return iges.WriteWrappedCylinderBoreRimFillet(ctx, w, s, .001, o)
		}},
	}
	for _, c := range []struct {
		mode         string
		lower, upper float64
	}{
		{"lower", .5, 0},
		{"upper", 0, .75},
		{"both", .5, .75},
	} {
		t.Run(c.mode, func(t *testing.T) {
			solid, _, err := trimmed.FilletWrappedCylinderBoreRims(ctx, stock, c.lower, c.upper, o)
			if err != nil {
				t.Fatal(err)
			}
			var native bytes.Buffer
			if err := trimmed.WriteJSON(&native, solid); err != nil {
				t.Fatal(err)
			}
			replay, err := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
			if err != nil {
				t.Fatal(err)
			}
			mass, err := replay.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil {
				t.Fatal(err)
			}
			for _, format := range formats {
				t.Run(format.ext, func(t *testing.T) {
					var raw, repeated bytes.Buffer
					if err := format.write(ctx, &raw, solid, o); err != nil {
						t.Fatal(err)
					}
					if err := format.write(ctx, &repeated, replay, o); err != nil || !bytes.Equal(raw.Bytes(), repeated.Bytes()) {
						t.Fatalf("output changed after exact native replay: %v", err)
					}
					if format.ext == "step" {
						imported, err := step.ReadWrappedCylinderBoreRimFillet(ctx, bytes.NewReader(raw.Bytes()), step.Limits{}, o)
						if err != nil || imported.LengthUnit != .001 || len(imported.NativeEdgeSourceIDs) != len(solid.Edges()) {
							t.Fatalf("strict STEP reopen: %v", err)
						}
						repeated.Reset()
						if err := format.write(ctx, &repeated, imported.Solid, o); err != nil || !bytes.Equal(raw.Bytes(), repeated.Bytes()) {
							t.Fatalf("STEP bytes changed after strict neutral round trip: %v", err)
						}
					}
					if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_BORE_RIM_TRANSFER"); oracle != "" {
						checkOCCTWrappedBoreFilletTransfer(t, ctx, oracle, format.ext, raw.Bytes(), replay, c.lower, c.upper, mass)
					}
				})
			}
		})
	}
	for _, format := range formats {
		t.Run(format.ext+"/reject-sharp", func(t *testing.T) {
			var raw bytes.Buffer
			if err := format.write(ctx, &raw, stock, o); !errors.Is(err, geom.ErrConvergence) || raw.Len() != 0 {
				t.Fatalf("accepted a sharp bore through the rounded-rim writer: %v", err)
			}
		})
	}
}

func checkOCCTWrappedBoreFilletTransfer(t *testing.T, ctx context.Context, oracle, ext string, data []byte,
	solid trimmed.Solid, lower, upper float64, mass parametric.Properties) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eccentric-bore."+ext)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.CommandContext(ctx, oracle, path).CombinedOutput()
	var out struct {
		Solids, Faces int
		Volume, Area  float64
		Vertices      [][3]float64
		Edges         [][3]int
		Caps          [][2]float64
		Cylinders     [][5]float64
		Toroids       [][9]float64
	}
	start := bytes.Index(raw, []byte(`{"Solids":`))
	if err != nil || start < 0 || json.Unmarshal(raw[start:], &out) != nil {
		t.Fatalf("pinned OCCT reader: %v\n%s", err, raw)
	}
	v, edges := solid.Vertices(), solid.Edges()
	if out.Solids != 1 || out.Faces != solid.Topology().Faces || len(out.Vertices) != len(v) || len(out.Edges) != len(edges) ||
		out.Volume <= 0 || math.Abs(out.Volume-mass.Volume) > 1e-7 || math.Abs(out.Area-mass.Area) > 1e-7 {
		t.Fatalf("signed material or source topology changed: %s", raw)
	}
	vertices := map[[3]float64]int{}
	for i, p := range v {
		vertices[[3]float64{p.X, p.Y, p.Z}] = i
	}
	oldIDs := make([]int, len(out.Vertices))
	for i, p := range out.Vertices {
		id, ok := vertices[p]
		if !ok {
			t.Fatalf("OCCT created or repeated a vertex %v", p)
		}
		oldIDs[i] = id
		delete(vertices, p)
	}
	incidences := map[[2]int]int{}
	for _, edge := range edges {
		a, b := v[edge.Start], v[edge.End]
		curve := 1
		if a.X == b.X && a.Y == b.Y {
			curve = 0
		}
		incidences[[2]int{min(edge.Start, edge.End), max(edge.Start, edge.End)}] = curve
	}
	for _, edge := range out.Edges {
		if edge[0] < 0 || edge[1] < 0 || edge[0] >= len(oldIDs) || edge[1] >= len(oldIDs) {
			t.Fatalf("unresolved original vertex in edge %v", edge)
		}
		a, b := oldIDs[edge[0]], oldIDs[edge[1]]
		key := [2]int{min(a, b), max(a, b)}
		if curve, ok := incidences[key]; !ok || curve != edge[2] {
			t.Fatalf("OCCT changed a circle quadrant, meridian or straight generator %v", edge)
		}
		delete(incidences, key)
	}
	if len(out.Caps) != 2 || !slices.Contains(out.Caps, [2]float64{2, 2}) || !slices.Contains(out.Caps, [2]float64{10, 2}) {
		t.Fatalf("original cap wires changed: %v", out.Caps)
	}
	if len(out.Cylinders) != 2 || !slices.Contains(out.Cylinders, [5]float64{0, 0, 6.25, 2, 10}) ||
		!slices.Contains(out.Cylinders, [5]float64{1.5, 2, 2, 2 + lower, 10 - upper}) {
		t.Fatalf("original cylindrical axes or bands changed: %v", out.Cylinders)
	}
	var want [][9]float64
	if lower != 0 {
		want = append(want, [9]float64{1.5, 2, 2 + lower, 2 + lower, lower, 2, 2 + lower, 2, 2 + lower})
	}
	if upper != 0 {
		want = append(want, [9]float64{1.5, 2, 10 - upper, 2 + upper, upper, 2, 2 + upper, 10 - upper, 10})
	}
	for _, torus := range want {
		if len(out.Toroids) != len(want) || !slices.Contains(out.Toroids, torus) {
			t.Fatalf("OCCT changed the original rolling radius or meridian interval: %v", out.Toroids)
		}
	}
	t.Logf("valid positive body, %d analytic faces, %d original edges, %d original vertices, volume %.16g, area %.16g",
		out.Faces, len(out.Edges), len(out.Vertices), out.Volume, out.Area)
}
