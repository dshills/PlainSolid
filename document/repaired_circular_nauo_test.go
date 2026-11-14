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
	"strconv"
	"testing"
	"time"
)

func repairedNAUODocument(t *testing.T) (*RepairedCircularNAUODocument, []step.RepairedCircularNAUO) {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/occt-circular-nauo-dag.step")
	if e != nil {
		t.Fatal(e)
	}
	d, reports, e := ReadRepairedCircularNAUOSTEP(bytes.NewReader(raw), step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return d, reports
}
func TestRepairedCircularNAUONative(t *testing.T) {
	d, reports := repairedNAUODocument(t)
	if len(reports) != 1 || len(reports[0].Source.FailedConditions) != 4 {
		t.Fatal("per-asset evidence")
	}
	local := d.LocalAssets()
	if len(local.Planar) != 1 || len(local.Parametric) != 1 || len(local.Nodes) != 0 {
		t.Fatal("local ownership")
	}
	instances := d.Instances()
	if len(instances) != 7 {
		t.Fatal("expanded uses")
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, u := range instances {
		if seen[u.ID] {
			t.Fatal("duplicate instance ID")
		}
		seen[u.ID] = true
		counts[u.AssetID]++
	}
	for id := range local.Parametric {
		if counts[id] != 3 {
			t.Fatal("repaired each use instead of shared product")
		}
	}
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	originalBytes := append([]byte(nil), saved.Bytes()...)
	reports[0].Bounds.Tolerance.SetInt64(42)
	instances[0].Occurrence.Path[0].NAUOID = -1
	var second bytes.Buffer
	if e := d.Write(&second); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(originalBytes, second.Bytes()) {
		t.Fatal("returned evidence/path mutated payload")
	}
	n, fresh, e := ReadRepairedCircularNAUODocument(bytes.NewReader(originalBytes), 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	if len(fresh) != 1 || fresh[0].Bounds.Tolerance.Cmp(big.NewRat(42, 1)) == 0 || !reflect.DeepEqual(d.Instances(), n.Instances()) || !reflect.DeepEqual(d.Parts(), n.Parts()) {
		t.Fatal("reload lineage")
	}
	bounds, e := n.ExactWorldBounds()
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range []int64{112, 112, 72} {
		if bounds.Max[i].Cmp(big.NewRat(x, 1)) != 0 || bounds.Min[i].Cmp(big.NewRat([]int64{0, -7, -2}[i], 1)) != 0 {
			t.Fatal(bounds)
		}
	}
	var env repairedCircularNAUOEnvelope
	if e = json.Unmarshal(originalBytes, &env); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(env.Local, []byte("SquaredRadiusResidual")) || bytes.Contains(env.Local, []byte("TurnDefect")) {
		t.Fatal("trusted evidence leaked into native local payload")
	}
	for _, kind := range []string{"local", "tolerance", "version"} {
		t.Run(kind, func(t *testing.T) {
			bad := env
			switch kind {
			case "local":
				bad.Local = json.RawMessage(`{}`)
			case "tolerance":
				bad.MaxDistance = 1e-8
			case "version":
				bad.Version = 2
			}
			b, _ := json.Marshal(bad)
			got, proof, e := ReadRepairedCircularNAUODocument(bytes.NewReader(b), 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
			if e == nil || got != nil || proof != nil {
				t.Fatal("stale mixed native accepted")
			}
		})
	}
	if _, e = ReadExactDAGPlanarNAUODocument(bytes.NewReader(originalBytes), 4<<20, step.Limits{}); e == nil {
		t.Fatal("exact format widened")
	}
}

func TestOCCTRepairedCircularNAUONative(t *testing.T) {
	massBin, boundsBin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS"), os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if massBin == "" || boundsBin == "" {
		t.Skip("optional pinned OCCT")
	}
	d, _ := repairedNAUODocument(t)
	var saved bytes.Buffer
	if e := d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, reports, e := ReadRepairedCircularNAUODocument(&saved, 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil || len(reports) != 1 {
		t.Fatal(e)
	}
	// Independently sum the actual native local geometry over exact source paths.
	volume, area := 0.0, 0.0
	center := [3]float64{}
	for _, u := range d.Instances() {
		var v, a float64
		var c geom.Vec3
		for _, p := range d.Parts() {
			if p.Product.DefinitionID == u.Occurrence.Path[len(u.Occurrence.Path)-1].ChildID {
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
		}
		world, e := u.Occurrence.ApplyExact(c)
		if e != nil {
			t.Fatal(e)
		}
		volume += v
		area += a
		for i, q := range world {
			x, _ := q.Float64()
			center[i] += x * v
		}
	}
	for i := range center {
		center[i] /= volume
	}
	// Export newly constructed material using the already qualified explicit
	// cylinder/planar assembly writer, not by relabeling retained original source.
	model := step.CylinderMaterialAssemblyModel{LengthUnit: d.LengthUnit()}
	for _, p := range append([]step.AssemblyProduct{d.Root()}, d.Groups()...) {
		model.Products = append(model.Products, step.CylinderMaterialPart{ID: strconv.FormatInt(p.DefinitionID, 10), Name: p.Name, Family: step.CylinderMaterialNode})
	}
	for _, p := range d.Parts() {
		part := step.CylinderMaterialPart{ID: strconv.FormatInt(p.Product.DefinitionID, 10), Name: p.Product.Name}
		if p.Family == step.RepairedNAUOExactPlanar {
			part.Family = step.CylinderMaterialPlanar
			part.Planar = []step.Body{p.Planar}
		} else {
			part.Family = step.CylinderMaterialCylinder
			part.Cylinders = []step.CylinderBody{p.Cylinder}
		}
		model.Products = append(model.Products, part)
	}
	for _, u := range d.Relationships() {
		m := u.Matrix()
		tr := geom.Affine{}
		for i := 0; i < 3; i++ {
			for j := 0; j < 4; j++ {
				var exact bool
				tr[i][j], exact = m[i][j].Float64()
				if !exact {
					t.Fatal("test fixture pose not represented")
				}
			}
		}
		model.Occurrences = append(model.Occurrences, step.AssemblyInstance{ID: strconv.FormatInt(u.NAUOID, 10), Name: u.Name, Parent: strconv.FormatInt(u.ParentID, 10), Child: strconv.FormatInt(u.ChildID, 10), Transform: tr})
	}
	var reconstructed, original bytes.Buffer
	if e = step.WriteCylinderMaterialAssembly(&reconstructed, model); e != nil {
		t.Fatal(e)
	}
	if e = d.WriteOriginalSTEP(&original); e != nil {
		t.Fatal(e)
	}
	for name, raw := range map[string][]byte{"original": original.Bytes(), "reconstructed": reconstructed.Bytes()} {
		path := filepath.Join(t.TempDir(), name+".step")
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		for _, bin := range []string{massBin, boundsBin} {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			data, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
			cancel()
			if e != nil {
				t.Fatal(e, string(data))
			}
			if bin == massBin {
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
				if m.Faces != 33 || m.Solids != 7 || math.Abs(m.Volume-volume) > 1e-8 || math.Abs(m.Area-area) > 1e-8 {
					t.Fatal(m, volume, area)
				}
				for i := range center {
					if math.Abs(m.Center[i]-center[i]) > 1e-8 {
						t.Fatal(m, center)
					}
				}
			} else {
				at := bytes.LastIndex(data, []byte(`{"Min"`))
				if at < 0 {
					t.Fatal(string(data))
				}
				var b struct{ Min, Max [3]float64 }
				if e = json.Unmarshal(data[at:], &b); e != nil {
					t.Fatal(e)
				}
				for i, x := range []float64{112, 112, 72} {
					if math.Abs(b.Max[i]-x) > 1e-6 || math.Abs(b.Min[i]-[]float64{0, -7, -2}[i]) > 1e-6 {
						t.Fatal(b)
					}
				}
			}
		}
	}
}

func TestRepairedCircularNAUORationalPoseBoundsGate(t *testing.T) {
	raw, e := os.ReadFile("../exchange/step/testdata/occt-circular-nauo-dag.step")
	if e != nil {
		t.Fatal(e)
	}
	for old, replacement := range map[string]string{"#17 = DIRECTION('',(0.,0.,1.));": "#17 = DIRECTION('',(2.,-2.,-1.));", "#18 = DIRECTION('',(1.,0.,-0.));": "#18 = DIRECTION('',(1.,2.,-2.));"} {
		if bytes.Count(raw, []byte(old)) != 1 {
			t.Fatal("fixture datum changed")
		}
		raw = bytes.Replace(raw, []byte(old), []byte(replacement), 1)
	}
	d, reports, e := ReadRepairedCircularNAUOSTEP(bytes.NewReader(raw), step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil || len(reports) != 1 {
		t.Fatal(e)
	}
	m := d.Relationships()
	found := false
	for _, u := range m {
		if u.ToPlacementID == 15 {
			v := u.Matrix()
			if v[0][0].Cmp(big.NewRat(1, 3)) != 0 || v[1][0].Cmp(big.NewRat(2, 3)) != 0 || v[2][0].Cmp(big.NewRat(-2, 3)) != 0 {
				t.Fatal("pose rounded")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing exact source occurrence")
	}
	if _, e = d.ExactWorldBounds(); e == nil {
		t.Fatal("irrational cylinder extent rounded")
	}
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	reloaded, _, e := ReadRepairedCircularNAUODocument(&saved, 4<<20, step.Limits{}, step.CircularNAUORepairOptions{MaxDistance: 1e-9})
	if e != nil || !reflect.DeepEqual(d.Instances(), reloaded.Instances()) {
		t.Fatal("exact pose reload", e)
	}
}
