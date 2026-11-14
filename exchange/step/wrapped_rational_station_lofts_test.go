package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func rationalStationSTEPFixture(t *testing.T, n int) WrappedRationalStationLoftBody {
	t.Helper()
	xy := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var poles []geom.Vec3
	for _, p := range xy {
		poles = append(poles, geom.Vec3{X: .25 + 5*p[0], Y: -.5 + 5*p[1]})
	}
	profile, e := geom.NewBSpline(2, poles, []float64{-2, -2, -2, -1, -1, 1, 1, 4, 4, 8, 8, 8}, []float64{1, .5, 1, .75, 1, .75, 1, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	stations := []trimmed.WrappedRationalProfileStation{{Height: r(0, 1), Scale: r(3, 5)}, {Height: r(1, 1), Scale: r(1, 5)}, {Height: r(3, 1), Scale: r(2, 5)}}
	solid, _, e := trimmed.LoftWrappedRationalProfileStations(context.Background(), profile, stations[:n+1], 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := WrappedRationalStationLoftFromSolid(context.Background(), solid, "Weighted waist", .001, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.CapNames = [2]string{"Floor", "Roof"}
	for i := range m.Walls {
		m.Walls[i].Name = "Original weighted wall"
	}
	return m
}

func TestWrappedRationalStationSTEP(t *testing.T) {
	for _, n := range []int{1, 2} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			m := rationalStationSTEPFixture(t, n)
			var raw bytes.Buffer
			if e := WriteWrappedRationalStationLoft(&raw, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadWrappedRationalStationLoft(bytes.NewReader(raw.Bytes()), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if got.Name != m.Name || got.CapNames != m.CapNames || len(got.Walls) != n {
				t.Fatal(got)
			}
			for i, w := range m.Walls {
				a, _ := w.Geometry.ExportRecipe()
				b, _ := got.Walls[i].Geometry.ExportRecipe()
				if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
					t.Fatal("original support changed")
				}
				if i > 0 && got.Walls[i-1].SideEdges[2] != got.Walls[i].SideEdges[0] {
					t.Fatal("unshared rim")
				}
			}
			var again bytes.Buffer
			if e = WriteWrappedRationalStationLoft(&again, got); e != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
				t.Fatal("nondeterministic roundtrip", e)
			}
			if _, e = ReadWrappedStationLoft(bytes.NewReader(raw.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("circular gate widened")
			}
			if _, e = ReadWrappedRationalStationLoft(bytes.NewReader(raw.Bytes()), Limits{MaxFaces: 2}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("limit ignored")
			}
			for _, mutation := range []string{"orientation", "pcurve", "station", "interior", "detached"} {
				t.Run(mutation, func(t *testing.T) {
					f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
					if e != nil {
						t.Fatal(e)
					}
					changed := false
					for _, id := range f.Order {
						if mutation == "detached" {
							if _, err := simple(f, id, "ADVANCED_FACE", 4); err == nil {
								next := int64(1)
								for existing := range f.Entities {
									if existing >= next {
										next = existing + 1
									}
								}
								clone := f.Entities[id]
								clone.ID = next
								f.Entities[next] = clone
								f.Order = append(f.Order, next)
								changed = true
								break
							}
						}
						if mutation == "interior" {
							if base, ok := component(f.Entities[id], "B_SPLINE_CURVE"); ok && len(base.Args) > 1 && len(base.Args[1].Items) > 2 {
								pid, _ := ref(base.Args[1].Items[1])
								rec, err := simple(f, pid, "CARTESIAN_POINT", 2)
								if err == nil && len(rec.Args[1].Items) == 2 {
									rec.Args[1].Items[0] = Value{Kind: Number, Text: "19."}
									f.Entities[pid] = Entity{ID: pid, Records: []Record{rec}}
									changed = true
									break
								}
							}
						}
						if mutation == "orientation" {
							if rec, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
								rec.Args[3] = Value{Kind: Enumeration, Text: "F"}
								f.Entities[id] = Entity{ID: id, Records: []Record{rec}}
								changed = true
								break
							}
						}
						if mutation == "pcurve" {
							if rec, e := simple(f, id, "PCURVE", 3); e == nil {
								rec.Args[1] = Value{Kind: Reference, Ref: got.Walls[0].SideEdges[0]}
								f.Entities[id] = Entity{ID: id, Records: []Record{rec}}
								changed = true
								break
							}
						}
						if mutation == "station" {
							if rec, e := simple(f, id, "CARTESIAN_POINT", 2); e == nil && len(rec.Args[1].Items) == 3 {
								rec.Args[1].Items[2] = Value{Kind: Number, Text: "17."}
								f.Entities[id] = Entity{ID: id, Records: []Record{rec}}
								changed = true
								break
							}
						}
					}
					if !changed {
						t.Fatal("mutation absent")
					}
					if _, e = ReadWrappedRationalStationLoft(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
						t.Fatal("accepted mutation")
					}
				})
			}
		})
	}
}

// Independent Green integration of four rational quadratic arcs. This uses
// neither the native source evaluator nor its certified mass implementation.
func rationalStationSTEPArea() float64 {
	total := 0.
	for _, w := range []float64{.5, .75, .75, .75} {
		f := func(t float64) float64 {
			a := 1 - t
			d := a*a + 2*w*a*t + t*t
			dd := -2*a + 2*w*(1-2*t) + 2*t
			x := a*a + 2*w*a*t
			y := 2*w*a*t + t*t
			dx := -2*a + 2*w*(1-2*t)
			dy := 2*w*(1-2*t) + 2*t
			return (x*(dy*d-y*dd) - y*(dx*d-x*dd)) / (2 * d * d * d)
		}
		const n = 8192
		sum := f(0) + f(1)
		for i := 1; i < n; i++ {
			factor := 2.
			if i%2 == 1 {
				factor = 4
			}
			sum += factor * f(float64(i)/n)
		}
		total += sum / (3 * n)
	}
	return total
}

func TestWrappedRationalStationSTEPMass(t *testing.T) {
	m := rationalStationSTEPFixture(t, 2)
	var raw bytes.Buffer
	if e := WriteWrappedRationalStationLoft(&raw, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedRationalStationLoft(&raw, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := got.Solid.BoundWrappedRationalStationMass(context.Background(), trimmed.WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 1000)})
	if e != nil {
		t.Fatal(e)
	}
	truth := new(big.Rat).SetFloat64(9 * rationalStationSTEPArea())
	if p.Volume.Lower.Cmp(truth) > 0 || p.Volume.Upper.Cmp(truth) < 0 {
		t.Fatal(p.Volume, truth)
	}
}

func TestWrappedRationalStationSTEPOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_ORACLE") == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE and PLAINSOLID_OCCT_STEP=1")
	}
	m := rationalStationSTEPFixture(t, 2)
	var raw bytes.Buffer
	if e := WriteWrappedRationalStationLoft(&raw, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/weighted.step"
	if e := os.WriteFile(path, raw.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Volume   float64
		Centroid [3]float64
	}
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-9*rationalStationSTEPArea()) > 1e-6 || math.Abs(p.Centroid[2]-71./54) > 1e-7 {
		t.Fatal(p)
	}
	t.Logf("independent OCCT volume %.12g centroid %v", p.Volume, p.Centroid)
}

func TestWrappedRationalStationSTEPAtomic(t *testing.T) {
	m := rationalStationSTEPFixture(t, 2)
	var raw bytes.Buffer
	m.Walls[1].Geometry = m.Walls[0].Geometry // overlapping, unequal shared stations
	if e := WriteWrappedRationalStationLoft(&raw, m); e == nil || raw.Len() != 0 {
		t.Fatal("non-atomic mismatch", e)
	}
	m.Walls = nil
	if e := WriteWrappedRationalStationLoft(&raw, m); e == nil || raw.Len() != 0 {
		t.Fatal("empty writer", e)
	}
}
