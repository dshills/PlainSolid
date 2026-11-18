package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFiniteTripleBoreSTEP(t *testing.T) {
	slowtest.Skip(t)
	for _, mask := range []int{3, 5, 6, 7} {
		for signs := 0; signs < 8; signs++ {
			t.Run(fmt.Sprintf("%d/%d", mask, signs), func(t *testing.T) {
				normals := []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
				f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normals[signs%6])
				p := TripleCrossedBoreBody{Name: "finite triple", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2}
				if mask&1 != 0 {
					p.XBlindDepth = 16
					p.XOppositeCap = signs&1 != 0
				}
				if mask&2 != 0 {
					p.YBlindDepth = 12
					p.YOppositeCap = signs&2 != 0
				}
				if mask&4 != 0 {
					p.BlindDepth = 10
					p.OppositeCap = signs&4 != 0
				}
				m := TripleCrossedBoreModel{Body: p, LengthUnit: .001}
				var b bytes.Buffer
				if e := WriteTripleCrossedBore(&b, m); e != nil {
					t.Fatal(e)
				}
				got, e := ReadTripleCrossedBore(bytes.NewReader(b.Bytes()), Limits{})
				if e != nil || got != m {
					t.Fatal(got, e)
				}
				var again bytes.Buffer
				if e = WriteTripleCrossedBore(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
					t.Fatal(e, "unstable")
				}
				if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
					return
				}
				dir := t.TempDir()
				src, dst := filepath.Join(dir, "finite.step"), filepath.Join(dir, "mass.json")
				if e = os.WriteFile(src, b.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				stepOracle(t, dst, "step-mass-precise", src)
				raw, e := os.ReadFile(dst)
				if e != nil {
					t.Fatal(e)
				}
				var o struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if e = json.Unmarshal(raw, &o); e != nil {
					t.Fatal(e)
				}
				volume := 3840. - 192*math.Pi + 64*math.Sqrt2
				area := 1504. + 168*math.Pi - 96*math.Sqrt2
				moment := p.Crossing.Scale(volume)
				for axis, d := range []float64{p.XBlindDepth, p.YBlindDepth, p.BlindDepth} {
					if d == 0 {
						continue
					}
					size := []float64{20, 16, 12}[axis]
					length := size - d
					v := 4 * math.Pi * length
					center := []float64{10, 8, 6}
					center[axis] = (d + size) / 2
					if signs&(1<<axis) != 0 {
						center[axis] = (size - d) / 2
					}
					moment = moment.Add(geom.Vec3{X: center[0], Y: center[1], Z: center[2]}.Scale(v))
					volume += v
					area += 8*math.Pi - 4*math.Pi*length
				}
				wantCenter := f.ToWorld(moment.Scale(1 / volume))
				actual := geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}
				if math.Abs(o.Volume-volume) > 1e-7 || math.Abs(o.Area-area) > 1e-7 || actual.Distance(wantCenter) > 1e-7 {
					t.Fatal(o, volume, area, wantCenter)
				}
			})
		}
	}
}

func TestFiniteTripleBoreSTEPDecimalDepths(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16.25, Y: -32.5, Z: 48.125}, n)
		m := TripleCrossedBoreModel{LengthUnit: .001, Body: TripleCrossedBoreBody{Name: "decimal floors", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2, XBlindDepth: 16.2, YBlindDepth: 12.3, BlindDepth: 10.1, XOppositeCap: true, OppositeCap: true}}
		var b bytes.Buffer
		if e := WriteTripleCrossedBore(&b, m); e != nil {
			t.Fatal(n, e)
		}
		got, e := ReadTripleCrossedBore(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(n, e)
		}
		var again bytes.Buffer
		if e = WriteTripleCrossedBore(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal(n, e, "unstable")
		}
	}
}
