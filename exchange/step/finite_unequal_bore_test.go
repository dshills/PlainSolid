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

func TestFiniteUnequalBoreSTEP(t *testing.T) {
	for i, n := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		for _, q := range [][4]float64{{3, 2, 0, 11.2}, {2, 3, 14.4, 0}, {3, 2, 14.4, 11.2}} {
			t.Run(fmt.Sprintf("%d/%v", i, q), func(t *testing.T) {
				f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
				if err != nil {
					t.Fatal(err)
				}
				p := UnequalBoreBody{Name: "finite unequal crossed Holes", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 14},
					Crossing: geom.Vec3{X: 10, Y: 8, Z: 7}, XRadius: q[0], ZRadius: q[1], XBlindDepth: q[2], ZBlindDepth: q[3],
					XOppositeCap: q[2] != 0 && i == 1, ZOppositeCap: q[3] != 0 && i != 1}
				if i == 1 {
					p.Offset = .5
					if p.XRadius < p.ZRadius {
						p.ZRadius = 4
					} else {
						p.XRadius = 4
					}
				}
				m := UnequalBoreModel{Body: p, LengthUnit: .001}
				var out bytes.Buffer
				if err := WriteUnequalBore(&out, m); err != nil {
					t.Fatal(err)
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
				dir := t.TempDir()
				source, dest := filepath.Join(dir, "finite.step"), filepath.Join(dir, "mass.json")
				if err := os.WriteFile(source, out.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				stepOracle(t, dest, "step-mass-precise", source)
				data, err := os.ReadFile(dest)
				if err != nil {
					t.Fatal(err)
				}
				var mass struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if err := json.Unmarshal(data, &mass); err != nil {
					t.Fatal(err)
				}
				c := context.Background()
				s, err := trimmed.BoxFiniteSkewCrossedBores(c, f, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset,
					p.XBlindDepth, p.ZBlindDepth, p.XOppositeCap, p.ZOppositeCap, 1e-9)
				if err != nil {
					t.Fatal(err)
				}
				native, err := s.Solid.MassProperties(c, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
				if err != nil {
					t.Fatal(err)
				}
				if math.Abs(native.Volume-mass.Volume) > 1e-8*native.Volume ||
					math.Abs(native.Area-mass.Area) > 1e-8*native.Area ||
					native.Centroid.Distance(geom.Vec3{X: mass.Centroid[0], Y: mass.Centroid[1], Z: mass.Centroid[2]}) > 1e-6 {
					t.Fatal(native, mass)
				}
			})
		}
	}
}
