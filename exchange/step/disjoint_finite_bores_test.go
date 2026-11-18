package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"

	"testing"
)

func TestFiniteDisjointBoresSTEP(t *testing.T) {
	slowtest.Skip(t)
	for _, normal := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		for _, op := range []bool{false, true} {
			for _, mixed := range []bool{false, true} {
				f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: -4, Z: 7}, normal)
				if e != nil {
					t.Fatal(e)
				}
				p := DisjointBoresBody{Name: "blind cardinal", Frame: f, Dimensions: geom.Vec3{X: 24, Y: 24, Z: 24}, Blind: []DisjointBlindBore{{DisjointBore{0, geom.Vec3{Y: 10, Z: 10}, 1}, 4, op}, {DisjointBore{2, geom.Vec3{X: 10, Y: 10}, 1}, 7, !op}}}
				if mixed {
					p.Bores = []DisjointBore{{1, geom.Vec3{X: 20, Z: 20}, 1}}
				}
				var buf bytes.Buffer
				if e := WriteDisjointBores(&buf, DisjointBoresModel{p, .001}); e != nil {
					t.Fatal(normal, op, mixed, e)
				}
				got, e := ReadDisjointBores(bytes.NewReader(buf.Bytes()), Limits{})
				if e != nil || !reflect.DeepEqual(got.Body, p) {
					t.Fatal(got, p, e)
				}
				s, e := got.Body.Solid(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				mass, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				v := 24.*24*24 - math.Pi*(4+7)
				a := 6.*24*24 + 2*math.Pi*(4+7)
				if mixed {
					v -= math.Pi * 24
					a += 2 * math.Pi * 23
				}
				if math.Abs(v-mass.Volume) > 1e-7 || math.Abs(a-mass.Area) > 1e-7 {
					t.Fatal(mass, v, a)
				}
				if os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
					path, report := filepath.Join(t.TempDir(), "disjoint-blind.step"), filepath.Join(t.TempDir(), "mass.json")
					if e := os.WriteFile(path, buf.Bytes(), 0600); e != nil {
						t.Fatal(e)
					}
					stepOracle(t, report, "step-mass-precise", path)
					raw, e := os.ReadFile(report)
					if e != nil {
						t.Fatal(e)
					}
					var oracle struct {
						Volume, Area float64
						Centroid     [3]float64
					}
					if e = json.Unmarshal(raw, &oracle); e != nil {
						t.Fatal(e)
					}
					if math.Abs(oracle.Volume-v) > 1e-7 || math.Abs(oracle.Area-a) > 1e-7 {
						t.Fatal("OCCT mass", oracle, v, a)
					}
					if geom.Vec3(geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}).Distance(mass.Centroid) > 1e-7 {
						t.Fatal("OCCT centroid", oracle, mass.Centroid)
					}
				}

				reverse := p
				reverse.Blind = []DisjointBlindBore{p.Blind[1], p.Blind[0]}
				var again bytes.Buffer
				if e := WriteDisjointBores(&again, DisjointBoresModel{reverse, .001}); e != nil || !bytes.Equal(buf.Bytes(), again.Bytes()) {
					t.Fatal("order", e)
				}
			}
		}
	}
}
