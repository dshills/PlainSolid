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
	"regexp"
	"strings"
	"testing"
)

func TestDisjointBlindBoresSTEP(t *testing.T) {
	slowtest.Skip(t)
	for _, normal := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		for _, op := range []bool{false, true} {
			for _, mixed := range []bool{false, true} {
				f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: -4, Z: 7}, normal)
				if e != nil {
					t.Fatal(e)
				}
				p := DisjointBoresBody{Name: "blind cardinal", Frame: f, Dimensions: geom.Vec3{X: 24, Y: 24, Z: 24}, Blind: []DisjointBlindBore{{DisjointBore{0, geom.Vec3{Y: 4, Z: 4}, 1.5}, 7, op}, {DisjointBore{1, geom.Vec3{X: 12, Z: 12}, 2}, 9, !op}}}
				if mixed {
					p.Bores = []DisjointBore{{2, geom.Vec3{X: 20, Y: 20}, 1}}
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
				v := 24.*24*24 - math.Pi*(2.25*7+4*9)
				a := 6.*24*24 + 2*math.Pi*(1.5*7+2*9)
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
func TestDisjointBlindBoresReject(t *testing.T) {
	t.Parallel()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := DisjointBoresBody{Frame: f, Dimensions: geom.Vec3{X: 24, Y: 24, Z: 24}, Bores: []DisjointBore{{0, geom.Vec3{Y: 4, Z: 4}, 1}}, Blind: []DisjointBlindBore{{DisjointBore{1, geom.Vec3{X: 12, Z: 12}, 2}, 5, false}}}
	for _, depth := range []float64{-1, 0, 24, 25, math.NaN(), math.Inf(1)} {
		p.Blind[0].Depth = depth
		var out bytes.Buffer
		if e := WriteDisjointBores(&out, DisjointBoresModel{p, .001}); e == nil || out.Len() != 0 {
			t.Fatal(depth, e)
		}
	}
}

func TestDisjointBlindBoresCorruptFloor(t *testing.T) {
	t.Parallel()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := DisjointBoresBody{Frame: f, Dimensions: geom.Vec3{X: 24, Y: 24, Z: 24}, Bores: []DisjointBore{{0, geom.Vec3{Y: 4, Z: 4}, 1}}, Blind: []DisjointBlindBore{{DisjointBore{1, geom.Vec3{X: 12, Z: 12}, 2}, 5, true}}}
	var out bytes.Buffer
	if e := WriteDisjointBores(&out, DisjointBoresModel{p, .001}); e != nil {
		t.Fatal(e)
	}
	raw := out.String()
	faces := regexp.MustCompile(`ADVANCED_FACE\('',\([^;]+`).FindAllStringIndex(raw, -1)
	last := faces[len(faces)-1]
	floor := raw[last[0]:last[1]]
	if !strings.Contains(floor, ".F.") {
		t.Fatal("floor sense")
	}
	changed := raw[:last[0]] + strings.Replace(floor, ".F.", ".T.", 1) + raw[last[1]:]
	if _, e := ReadDisjointBores(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("accepted reversed floor")
	}
	if _, e := ReadDisjointBores(strings.NewReader(raw), Limits{MaxFaces: 14}); e == nil {
		t.Fatal("ignored face budget")
	}
}
