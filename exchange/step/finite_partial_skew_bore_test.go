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

func TestFinitePartialSkewBoreSTEP(t *testing.T) {
	for i, q := range [][3]float64{{5, 4, 5}, {4, 5, -5}, {3, 3, 3}, {5, 2, 5}, {5, 4, 8}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: -32, Z: 48}, []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}, {Z: -1}, {Z: 1}}[i])
			if err != nil {
				t.Fatal(err)
			}
			p := UnequalBoreBody{Name: "finite partial-skew crossed bores", Frame: f, Dimensions: geom.Vec3{X: 40, Y: 40, Z: 40},
				Crossing: geom.Vec3{X: 20, Y: 16, Z: 20}, XRadius: q[0], ZRadius: q[1], Offset: q[2], ZBlindDepth: 29.5, ZOppositeCap: i != 0}
			switch i {
			case 1, 2:
				p.XBlindDepth, p.XOppositeCap = 28.75, i == 1
			case 3:
				p.ZBlindDepth = 24.5
			case 4:
				p.ZBlindDepth = 23.75
			}
			m := UnequalBoreModel{Body: p, LengthUnit: .001}
			var out bytes.Buffer
			if err := WriteUnequalBore(&out, m); err != nil {
				t.Fatal(err)
			}
			if out.Len() >= 512*1024 {
				t.Fatal("exceeds PlainCAD source budget", out.Len())
			}
			got, err := ReadUnequalBore(bytes.NewReader(out.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var again bytes.Buffer
			if err := WriteUnequalBore(&again, got); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
				t.Fatal(err)
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				return
			}
			c := context.Background()
			s, err := trimmed.BoxFinitePartialSkewBores(c, f, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset,
				p.XBlindDepth, p.ZBlindDepth, p.XOppositeCap, p.ZOppositeCap, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			mass, err := s.Solid.MassProperties(c, parametric.IntegrationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			src, dest := filepath.Join(dir, "partial-finite.step"), filepath.Join(dir, "mass.json")
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
