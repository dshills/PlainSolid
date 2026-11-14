package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func diskBoundSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/disk-bound-occt-reversed.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestDiskHoleBoundEncoding(t *testing.T) {
	raw := diskBoundSource(t)
	v, e := ReadDiskHoleBoundEncoding(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Model.Bodies) != 1 || len(v.ReversedBoundIDs) != 9 || len(v.GenericOuterBoundIDs) != 6 {
		t.Fatal(len(v.ReversedBoundIDs), len(v.GenericOuterBoundIDs))
	}
	b := v.Model.Bodies[0]
	if len(b.Holes) != 1 || len(b.Blind) != 1 || b.Height != 4 || b.Outer.Radius != 5 || b.Holes[0].Center.X != -2 || b.Blind[0].Center.X != 2 {
		t.Fatal(b)
	}
	for _, id := range v.ReversedBoundIDs {
		r, _ := simple(v.Source, id, "FACE_BOUND", 3)
		sense, e := boolean(r.Args[2])
		if e != nil || sense {
			t.Fatal("source mutated")
		}
	}
	if _, e = ReadDiskHoleExtrusions(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("canonical reader widened")
	}
	var out bytes.Buffer
	if e = WriteDiskHoleExtrusions(&out, v.Model); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadDiskHoleExtrusions(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestDiskHoleBoundEncodingRejects(t *testing.T) {
	raw := diskBoundSource(t)
	base, e := ReadDiskHoleBoundEncoding(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"bound-only", "edge-only", "loop-only", "radius", "shared-use"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		bid := base.ReversedBoundIDs[0]
		b := f.Entities[bid]
		lid := b.Records[0].Args[1].Ref
		loop := f.Entities[lid]
		uses := loop.Records[0].Args[1].Items
		uid := uses[0].Ref
		switch mode {
		case "bound-only":
			b.Records[0].Args[2].Text = "T"
			f.Entities[bid] = b
		case "edge-only":
			en := f.Entities[uid]
			sense, _ := boolean(en.Records[0].Args[4])
			en.Records[0].Args[4].Text = "T"
			if sense {
				en.Records[0].Args[4].Text = "F"
			}
			f.Entities[uid] = en
		case "loop-only":
			for a, z := 0, len(uses)-1; a < z; a, z = a+1, z-1 {
				uses[a], uses[z] = uses[z], uses[a]
			}
			loop.Records[0].Args[1].Items = uses
			f.Entities[lid] = loop
		case "shared-use":
			uses[1] = uses[0]
			loop.Records[0].Args[1].Items = uses
			f.Entities[lid] = loop
		case "radius":
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "CIRCLE" {
					en.Records[0].Args[2].Text = "9."
					f.Entities[id] = en
					break
				}
			}
		}
		v, e := ReadDiskHoleBoundEncoding(bytes.NewReader(serializeWrappedProofFile(f)), Limits{})
		if e == nil || v.Source != nil {
			t.Fatal(mode)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxFaces: 2}, {MaxEdgeUses: 4}, {MaxVertices: 2}} {
		if _, e := ReadDiskHoleBoundEncoding(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	for _, path := range []string{"testdata/disk-pocket-occt.step", "testdata/occt-disk-hole-assembly.step"} {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadDiskHoleBoundEncoding(bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("vendor promoted", path)
		}
	}
}
func TestOCCTDiskHoleBoundEncoding(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	source := diskBoundSource(t)
	v, e := ReadDiskHoleBoundEncoding(bytes.NewReader(source), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteDiskHoleExtrusions(&out, v.Model); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{source, out.Bytes()} {
		path := filepath.Join(t.TempDir(), "disk.step")
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		b, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(b))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(b[at:], &p); e != nil {
			t.Fatal(e)
		}
		if p.Faces != 6 || p.Solids != 1 || math.Abs(p.Volume-94*math.Pi) > 1e-7 || math.Abs(p.Area-100*math.Pi) > 1e-7 || math.Abs(p.Center[0]-2./47) > 1e-8 || math.Abs(p.Center[2]-95./47) > 1e-8 {
			t.Fatal(p)
		}
	}
}
func FuzzDiskHoleBoundEncoding(f *testing.F) {
	f.Add(diskBoundSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadDiskHoleBoundEncoding(bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxSolids: 2, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 128})
	})
}
