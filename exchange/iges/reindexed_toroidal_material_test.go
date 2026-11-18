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
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func TestReindexedToroidalMaterial(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, placed := range []bool{false, true} {
		name := "toroidal-material"
		count := 13
		center := geom.Vec3{}
		if placed {
			name = "placed-toroidal-material"
			count = 14
			center = geom.Vec3{X: 10, Y: -3, Z: 5}
		}
		raw, e := os.ReadFile("testdata/loop-start-" + name + ".igs")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadReindexedToroidalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
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
		if m.SourceVertexOrdinals[1] != 1 || m.SourceEdgeOrdinals[1] != 2 || m.SourceEdgeOrdinals[2] != 1 || m.SourceUseOrdinals[2*count-19][0] != 4 {
			t.Fatal("source use/vertex identity")
		}
		mass, e := m.Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(mass.Volume-8*math.Pi*math.Pi) > 1e-7 || math.Abs(mass.Area-16*math.Pi*math.Pi) > 1e-7 || mass.Centroid.Distance(center) > 1e-8 {
			t.Fatal(mass, e)
		}
		var native, source bytes.Buffer
		if e = WriteReindexedToroidalMaterialJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		p, e := ReadReindexedToroidalMaterialJSON(ctx, bytes.NewReader(native.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = WriteReindexedToroidalSource(ctx, &source, p); e != nil || !bytes.Equal(source.Bytes(), raw) {
			t.Fatal("source changed", e)
		}
		if placed {
			_, e = ReadPlacedToroidalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		} else {
			_, e = ReadToroidalMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		}
		if e == nil {
			t.Fatal("canonical widened")
		}
		for _, mutation := range []struct {
			prefix string
			field  int
			value  string
		}{{"198,", 4, "44"}, {"504,", 4, "9"}, {"100,", 4, "9"}, {"186,", 1, "999"}, {"508,", 4, "9"}} {
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
			if _, e = ReadReindexedToroidalMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
				t.Fatal("bad support/incidence", mutation)
			}
		}
		forged := p
		forged.SourceVertexOrdinals = map[int]int{1: 1, 2: 2}
		var rejected bytes.Buffer
		if e = WriteReindexedToroidalSource(ctx, &rejected, forged); e == nil || rejected.Len() != 0 {
			t.Fatal("stale ordinal")
		}
		for _, which := range []string{"edge", "use", "transform"} {
			f := p
			switch which {
			case "edge":
				f.SourceEdgeOrdinals = map[int]int{1: 1, 2: 2}
			case "use":
				f.SourceUseOrdinals = map[int][]int{p.SourceForCanonical[19]: {1, 2, 3, 4}}
			case "transform":
				f.SourceForCanonical = map[int]int{}
				for k, v := range p.SourceForCanonical {
					f.SourceForCanonical[k] = v
				}
				f.SourceForCanonical[15] = f.SourceForCanonical[11]
			}
			var dst bytes.Buffer
			if e := WriteReindexedToroidalSource(ctx, &dst, f); e == nil || dst.Len() != 0 {
				t.Fatal("forged ancestry", which)
			}
		}
		for _, pair := range [][2]int{{m.SourceForCanonical[13], m.SourceForCanonical[15]}, {m.SourceForCanonical[15], m.SourceForCanonical[15]}} {
			bad := append([]byte(nil), raw...)
			for _, line := range bytes.Split(bad, []byte("\n")) {
				if len(line) == 80 && line[72] == 'D' {
					id, _ := field(string(line[73:]))
					if id == pair[0] {
						target := pair[1]
						if id == m.SourceForCanonical[13] {
							target = 0
							if placed {
								target = m.SourceForCanonical[27]
							}
						}
						copy(line[48:56], fmt.Sprintf("%8d", target))
					}
				}
			}
			if _, e := ReadReindexedToroidalMaterial(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
				t.Fatal("shared transform or recursive transform accepted")
			}
		}
		for _, l := range []Limits{{MaxEntities: count - 1}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 8}} {
			if _, e = ReadReindexedToroidalMaterial(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
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
			if at < 0 || json.Unmarshal(b[at:], &q) != nil || q.Faces != 1 || q.Solids != 1 || math.Abs(q.Area-mass.Area) > 1e-7 || math.Abs(q.Volume-mass.Volume) > 1e-7 || (geom.Vec3{X: q.Center[0], Y: q.Center[1], Z: q.Center[2]}).Distance(center) > 1e-8 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzReindexedToroidalMaterial(f *testing.F) {
	for _, n := range []string{"toroidal-material", "placed-toroidal-material"} {
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
		_, _ = ReadReindexedToroidalMaterial(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	})
}
