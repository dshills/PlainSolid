package analytic

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"testing"
	"time"
)

func TestOCCTAnalyticBridge(t *testing.T) {
	ig, st := os.Getenv("PLAINSOLID_OCCT_IGES_ANALYTIC_SUPPORTS"), os.Getenv("PLAINSOLID_OCCT_ANALYTIC_SUPPORTS")
	if ig == "" || st == "" {
		t.Skip("optional independent OCCT helpers")
	}
	run := func(bin, mode, path string) []byte {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, e := exec.CommandContext(ctx, bin, mode, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		return b
	}
	type sample struct{ Point, Du, Dv [3]float64 }
	read := func(bin, path string) []sample {
		b := run(bin, "read", path)
		i := bytes.Index(b, []byte("{\"Samples\""))
		if i < 0 {
			t.Fatal(string(b))
		}
		var out struct{ Samples []sample }
		if e := json.Unmarshal(bytes.TrimSpace(b[i:]), &out); e != nil {
			t.Fatal(e)
		}
		if len(out.Samples) != 4 {
			t.Fatal(out)
		}
		return out.Samples
	}
	for _, mode := range []string{"write", "write-negative"} {
		dir := t.TempDir()
		ip, sp := filepath.Join(dir, "source.igs"), filepath.Join(dir, "source.step")
		run(ig, mode, ip)
		run(st, mode, sp)
		b, _ := os.ReadFile(ip)
		im, e := iges.ReadAnalyticSupports(bytes.NewReader(b), iges.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		sm, e := FromIGES(im)
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = step.WriteAnalyticSupports(&out, sm); e != nil {
			t.Fatal(e)
		}
		sq := filepath.Join(dir, "converted.step")
		_ = os.WriteFile(sq, out.Bytes(), 0600)
		b, _ = os.ReadFile(sp)
		sm, e = step.ReadAnalyticSupports(bytes.NewReader(b), step.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		im, e = ToIGES(sm)
		if e != nil {
			t.Fatal(e)
		}
		out.Reset()
		if e = iges.WriteAnalyticSupports(&out, im); e != nil {
			t.Fatal(e)
		}
		iq := filepath.Join(dir, "converted.igs")
		_ = os.WriteFile(iq, out.Bytes(), 0600)
		want := read(ig, ip)
		for _, pair := range [][2]string{{st, sq}, {st, sp}, {ig, iq}} {
			got := read(pair[0], pair[1])
			for i, a := range got {
				for j, v := range [][3]float64{a.Point, a.Du, a.Dv} {
					expected := [][3]float64{want[i].Point, want[i].Du, want[i].Dv}[j]
					for k := range v {
						if math.Abs(v[k]-expected[k]) > 1e-12 {
							t.Fatal(i, a, want[i])
						}
					}
				}
			}
		}
	}
}
