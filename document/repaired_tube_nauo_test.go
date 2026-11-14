package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func repairedTubeDocument(t *testing.T) (*RepairedCircularNAUODocument, *RepairedNAUOEvidence, []byte) {
	t.Helper()
	b, e := os.ReadFile("../exchange/step/testdata/occt-tube-nauo-dag.step")
	if e != nil {
		t.Fatal(e)
	}
	d, p, e := ReadRepairedCircularNAUOSTEPWithEvidence(bytes.NewReader(b), step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return d, p, b
}
func TestRepairedTubeNAUONative(t *testing.T) {
	d, p, raw := repairedTubeDocument(t)
	if len(p.Cylinders) != 1 || len(p.Tubes) != 1 || len(d.LocalAssets().Planar) != 1 || len(d.LocalAssets().Parametric) != 2 || len(d.Instances()) != 8 {
		t.Fatal("shared assets/evidence")
	}
	seen := map[string]bool{}
	for _, i := range d.Instances() {
		if seen[i.ID] {
			t.Fatal("duplicate path")
		}
		seen[i.ID] = true
	}
	var b bytes.Buffer
	if e := d.Write(&b); e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), b.Bytes()...)
	p.Tubes[0].Walls[0].Bounds.Tolerance.SetInt64(100)
	p.Tubes[0].Walls[1].ModelSeamMagnitude.SetInt64(999)
	var after bytes.Buffer
	_ = d.Write(&after)
	if !bytes.Equal(original, after.Bytes()) {
		t.Fatal("report aliases native")
	}
	fresh, evidence, e := ReadRepairedCircularNAUODocumentWithEvidence(bytes.NewReader(original), 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil || len(evidence.Tubes) != 1 || len(evidence.Cylinders) != 1 || !reflect.DeepEqual(d.Instances(), fresh.Instances()) {
		t.Fatal("reload", e)
	}
	after.Reset()
	_ = fresh.Write(&after)
	if !bytes.Equal(original, after.Bytes()) {
		t.Fatal("unstable native")
	}
	bounds, e := fresh.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i := range bounds.Min {
		if bounds.Min[i].Cmp(newRatInt([]int64{0, -8, -3}[i])) != 0 || bounds.Max[i].Cmp(newRatInt([]int64{202, 112, 72}[i])) != 0 {
			t.Fatal("world bounds")
		}
	}
	if _, _, e = ReadRepairedCircularNAUOSTEP(bytes.NewReader(raw), step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9}); e == nil {
		t.Fatal("old API discarded tube evidence")
	}
	if _, _, e = ReadRepairedCircularNAUODocument(bytes.NewReader(original), 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9}); e == nil {
		t.Fatal("old native API accepted v2")
	}
	for _, kind := range []string{"version", "geometry", "tolerance", "evidence"} {
		t.Run(kind, func(t *testing.T) {
			var env map[string]json.RawMessage
			if e := json.Unmarshal(original, &env); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "version":
				env["version"] = []byte("1")
			case "geometry":
				env["local"] = []byte("{}")
			case "tolerance":
				env["maxDistance"] = []byte("1e-8")
			case "evidence":
				env["tubeRepairs"] = []byte("[]")
			}
			b, _ := json.Marshal(env)
			got, proof, e := ReadRepairedCircularNAUODocumentWithEvidence(bytes.NewReader(b), 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
			if e == nil || got != nil || proof != nil {
				t.Fatal("accepted forged native")
			}
		})
	}
}
func newRatInt(v int64) *big.Rat { return big.NewRat(v, 1) }
func TestOCCTRepairedTubeNAUONative(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d, _, _ := repairedTubeDocument(t)
	var saved bytes.Buffer
	_ = d.Write(&saved)
	d, _, e := ReadRepairedCircularNAUODocumentWithEvidence(&saved, 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	volume, area := 0., 0.
	center := [3]float64{}
	for _, u := range d.Instances() {
		var v, a float64
		var c geom.Vec3
		for _, p := range d.Parts() {
			if p.Product.DefinitionID != u.Occurrence.Path[len(u.Occurrence.Path)-1].ChildID {
				continue
			}
			if p.Family == step.RepairedNAUOExactPlanar {
				m, e := p.Planar.Solid.MassProperties()
				if e != nil {
					t.Fatal(e)
				}
				v, a, c = m.Volume, m.SurfaceArea, m.Centroid
			} else {
				m, e := p.Circular.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				v, a, c = m.Volume, m.Area, m.Centroid
			}
		}
		world, e := u.Occurrence.ApplyExact(c)
		if e != nil {
			t.Fatal(e)
		}
		volume += v
		area += a
		for i, q := range world {
			x, _ := q.Float64()
			center[i] += v * x
		}
	}
	var original bytes.Buffer
	_ = d.WriteOriginalSTEP(&original)
	path := filepath.Join(t.TempDir(), "retained-source.step")
	_ = os.WriteFile(path, original.Bytes(), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(data))
	}
	at := bytes.LastIndex(data, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(data))
	}
	var m struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	if e = json.Unmarshal(data[at:], &m); e != nil {
		t.Fatal(e)
	}
	if m.Faces != 39 || m.Solids != 8 || math.Abs(volume-(96+140*math.Pi)) > 1e-8 || math.Abs(area-(208+196*math.Pi)) > 1e-8 || math.Abs(m.Volume-volume) > 1e-8 || math.Abs(m.Area-area) > 1e-8 {
		t.Fatal(m, volume, area)
	}
	for i := range center {
		if math.Abs(m.Center[i]-center[i]/volume) > 1e-8 {
			t.Fatal("world centroid", m.Center, center)
		}
	}
}
