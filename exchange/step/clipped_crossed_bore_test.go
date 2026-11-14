package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestClippedCrossedBoreSTEP(t *testing.T) {
	for _, depth := range []float64{4.8, 6, 7.2} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) { testClippedCrossedBoreSTEP(t, depth) })
	}
}

func testClippedCrossedBoreSTEP(t *testing.T, depth float64) {
	t.Helper()
	for i, n := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if err != nil {
			t.Fatal(err)
		}
		p := CrossedBoreBody{Name: "split blind Hole floor", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12},
			Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2, BlindDepth: depth, ClippedFloor: true}
		unit := .001
		if i == 2 {
			unit = .0254
		}
		model := CrossedBoreModel{Body: p, LengthUnit: unit}
		var out bytes.Buffer
		if err := WriteCrossedBore(&out, model); err != nil {
			t.Fatal(n, err)
		}
		if !strings.Contains(out.String(), "ELLIPSE(") || strings.Contains(out.String(), "B_SPLINE_CURVE_WITH_KNOTS(") {
			t.Fatal("missing analytic conics")
		}
		decoded, err := ReadCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
		if err != nil {
			t.Fatal(n, err)
		}
		if math.Abs(decoded.Body.BlindDepth-model.Body.BlindDepth) > 1e-10 {
			t.Fatal(decoded)
		}
		model.Body.BlindDepth = decoded.Body.BlindDepth
		if !reflect.DeepEqual(decoded, model) {
			t.Fatal(decoded, model)
		}
		var again bytes.Buffer
		if err := WriteCrossedBore(&again, decoded); err != nil || !bytes.Equal(again.Bytes(), out.Bytes()) {
			t.Fatal(n, err)
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
			continue
		}
		dir := t.TempDir()
		src, dest := filepath.Join(dir, "clipped.step"), filepath.Join(dir, "mass.json")
		if err := os.WriteFile(src, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		stepOracle(t, dest, "step-mass-precise", src)
		raw, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		var occt struct {
			Volume, Area float64
			Centroid     [3]float64
		}
		if err := json.Unmarshal(raw, &occt); err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		var body trimmed.BoxCrossedBoresResult
		switch {
		case depth < p.Crossing.Z:
			body, err = trimmed.BoxBeforeCrossedBores(ctx, f, p.Dimensions, p.Crossing, p.Radius, depth, 1e-9)
		case depth == p.Crossing.Z:
			body, err = trimmed.BoxCenterCrossedBores(ctx, f, p.Dimensions, p.Crossing, p.Radius, 1e-9)
		default:
			body, err = trimmed.BoxClippedCrossedBores(ctx, f, p.Dimensions, p.Crossing, p.Radius, depth, 1e-9)
		}
		if err != nil {
			t.Fatal(err)
		}
		mass, err := body.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		scale := unit / .001
		if math.Abs(mass.Volume-occt.Volume/(scale*scale*scale)) > 1e-8*mass.Volume ||
			math.Abs(mass.Area-occt.Area/(scale*scale)) > 1e-8*mass.Area ||
			mass.Centroid.Distance(geom.Vec3{X: occt.Centroid[0], Y: occt.Centroid[1], Z: occt.Centroid[2]}.Scale(1/scale)) > 1e-6 {
			t.Fatal(n, mass, occt)
		}
	}
}
