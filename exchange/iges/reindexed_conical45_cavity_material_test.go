package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestReindexedConical45CavityMaterial(t *testing.T) {
	ctx := context.Background()
	for _, placed := range []bool{false, true} {
		name := "conical45-cavity-material"
		count := 39
		center := geom.Vec3{Z: 366. / 145}
		if placed {
			name = "placed-conical45-cavity-material"
			count = 40
			center = geom.Vec3{X: 10 + 366./145, Y: -3, Z: 5}
		}
		raw, e := os.ReadFile("testdata/loop-start-" + name + ".igs")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(name, e)
		}
		if (m.Placed != nil) != placed {
			t.Fatal("family")
		}
		for role, id := range m.SourceForCanonical {
			if id != 2*count-role {
				t.Fatal("record role")
			}
		}
		if m.SourceVertexOrdinals[2*count-15][1] != 2 || m.SourceVertexOrdinals[2*count-15][2] != 1 || m.SourceVertexOrdinals[2*count-55][1] != 2 || m.SourceUseOrdinals[2*count-25][0] != 4 {
			t.Fatal("source use/vertex identity")
		}
		for _, id := range []int{37, 77} {
			if m.SourceFaceOrdinals[2*count-id][1] != 3 || m.SourceFaceOrdinals[2*count-id][3] != 1 {
				t.Fatal("face ordinal")
			}
		}
		for _, id := range []int{23, 63} {
			if m.SourceEdgeOrdinals[2*count-id][1] != 3 || m.SourceEdgeOrdinals[2*count-id][3] != 1 {
				t.Fatal("edge ordinal")
			}
		}
		if m.SourceUseOrdinals[2*count-65][0] != 4 {
			t.Fatal("inner cyclic source use")
		}
		mass, e := m.Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(mass.Volume-290*math.Pi/3) > 1e-7 || math.Abs(mass.Area-math.Pi*(68+48*math.Sqrt2)) > 1e-7 || mass.Centroid.Distance(center) > 1e-8 {
			t.Fatal(mass, e)
		}
		var native, source bytes.Buffer
		if e = WriteReindexedConical45CavityMaterialJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		p, e := ReadReindexedConical45CavityMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteReindexedConical45CavitySource(ctx, &source, p); e != nil || !bytes.Equal(source.Bytes(), raw) {
			t.Fatal("source changed", e)
		}
		if placed {
			_, e = ReadPlacedConical45CavityMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		} else {
			_, e = ReadConical45CavityMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		}
		if e == nil {
			t.Fatal("canonical widened")
		}
		for _, mutation := range []struct {
			prefix string
			field  int
			value  string
		}{{"194,", 4, "44"}, {"504,", 4, "9"}, {"100,", 4, "9"}, {"186,", 1, "999"}, {"186,", 5, "1"}} {
			bad := append([]byte(nil), raw...)
			changed := false
			for _, line := range bytes.Split(bad, []byte("\n")) {
				if len(line) != 80 || line[72] != 'P' || !bytes.HasPrefix(line, []byte(mutation.prefix)) {
					continue
				}
				v := strings.Split(strings.TrimSpace(string(line[:64])), ",")
				suffix := ""
				if strings.HasSuffix(v[mutation.field], ";") {
					suffix = ";"
				}
				v[mutation.field] = mutation.value + suffix
				text := strings.Join(v, ",")
				if len(text) > 64 {
					t.Fatal("mutation overflow")
				}
				copy(line[:64], text+strings.Repeat(" ", 64-len(text)))
				changed = true
				break
			}
			if !changed {
				t.Fatal("missing mutation")
			}
			if _, e = ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
				t.Fatal("bad support/incidence", mutation)
			}
		}
		for _, q := range []struct{ owner, index, target int }{{71, 1, 9}, {39, 4, 37}, {63, 3, 15}, {37, 2, 37}} {
			limits, _ := (Limits{}).defaults()
			doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), limits, true, false, false, false, false, false, true, true, true, true)
			if e != nil {
				t.Fatal(e)
			}
			for i, rec := range doc.entities {
				if rec.id == m.SourceForCanonical[q.owner] {
					doc.entities[i].values[q.index].text = fmt.Sprint(m.SourceForCanonical[q.target])
				}
			}
			bad, e := mixedRoofFile(doc)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
				t.Fatal("cross shell or cycle", q)
			}
		}
		for _, kind := range []string{"edge", "face"} {
			f := m
			if kind == "edge" {
				f.SourceEdgeOrdinals = map[int]map[int]int{m.SourceForCanonical[23]: {1: 1}}
			} else {
				f.SourceFaceOrdinals = map[int]map[int]int{m.SourceForCanonical[37]: {1: 1}}
			}
			var dst bytes.Buffer
			if e := WriteReindexedConical45CavitySource(ctx, &dst, f); e == nil || dst.Len() != 0 {
				t.Fatal("stale ordinal", kind)
			}
		}
		forged := p
		forged.SourceVertexOrdinals = map[int]map[int]int{2*count - 15: {1: 1, 2: 2}}
		var rejected bytes.Buffer
		if e = WriteReindexedConical45CavitySource(ctx, &rejected, forged); e == nil || rejected.Len() != 0 {
			t.Fatal("stale ordinal")
		}
		for _, l := range []Limits{{MaxEntities: count - 1}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
			if _, e = ReadReindexedConical45CavityMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
				t.Fatal("limit")
			}
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID"); oracle != "" {
			path := filepath.Join(t.TempDir(), "source.igs")
			if e = os.WriteFile(path, source.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			b, e := exec.Command(oracle, path).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", b, e)
			}
			var q struct {
				Faces, Solids int
				Area, Volume  float64
				Center        [3]float64
			}
			at := bytes.LastIndex(b, []byte("{"))
			if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Faces != 6 || q.Solids != 1 || math.Abs(q.Area-mass.Area) > 1e-7 || math.Abs(q.Volume-mass.Volume) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(center) > 1e-8 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzReindexedConical45CavityMaterial(f *testing.F) {
	for _, n := range []string{"conical45-cavity-material", "placed-conical45-cavity-material"} {
		b, e := os.ReadFile("testdata/loop-start-" + n + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		_, _ = ReadReindexedConical45CavityMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
