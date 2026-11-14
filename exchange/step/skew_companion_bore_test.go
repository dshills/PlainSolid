package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestSkewCompanionBoreSTEP(t *testing.T) {
	for i, n := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			if err != nil {
				t.Fatal(err)
			}
			p := UnequalBoreBody{Name: "crossed unequal holes and independent Y bore", Frame: f, Dimensions: geom.Vec3{X: 30, Y: 30, Z: 30},
				Crossing: geom.Vec3{X: 15, Y: 15, Z: 15}, XRadius: 4, ZRadius: 2, Offset: .5,
				Additional: []trimmed.CardinalThroughBore{{Axis: 1, Center: geom.Vec3{X: 24, Z: 6}, Radius: 1.5}}}
			if i > 0 {
				p.XBlindDepth, p.ZBlindDepth, p.ZOppositeCap = 22, 22, true
			}
			if i == 2 {
				p.XRadius, p.ZRadius = 2, 4
				p.Additional = append(p.Additional, trimmed.CardinalThroughBore{Axis: 1, Center: geom.Vec3{X: 6, Z: 24}, Radius: 1.5})
			}
			var out bytes.Buffer
			if err := WriteUnequalBore(&out, UnequalBoreModel{Body: p, LengthUnit: .001}); err != nil {
				t.Fatal(err)
			}
			if out.Len() >= 512*1024 {
				t.Fatal(out.Len())
			}
			got, err := ReadUnequalBore(bytes.NewReader(out.Bytes()), Limits{})
			if err != nil || len(got.Body.Additional) != len(p.Additional) {
				t.Fatal(got, err)
			}
			var again bytes.Buffer
			if err := WriteUnequalBore(&again, got); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
				t.Fatal(err)
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				return
			}
			ctx := context.Background()
			native, err := trimmed.BoxFiniteSkewCrossedBoresWithDisjoint(ctx, p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset,
				p.XBlindDepth, p.ZBlindDepth, p.XOppositeCap, p.ZOppositeCap, p.Additional, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			mass, err := native.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			src, dest := filepath.Join(dir, "compound.step"), filepath.Join(dir, "mass.json")
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
			if math.Abs(mass.Volume-occt.Volume) > 1e-8*mass.Volume || math.Abs(mass.Area-occt.Area) > 1e-8*mass.Area ||
				mass.Centroid.Distance(geom.Vec3{X: occt.Centroid[0], Y: occt.Centroid[1], Z: occt.Centroid[2]}) > 1e-6 {
				t.Fatal(mass, occt)
			}
		})
	}
}
