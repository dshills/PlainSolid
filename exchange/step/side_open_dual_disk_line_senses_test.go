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
)

func sideDiskFlipLines(t *testing.T, raw []byte, mode string) ([]byte, int) {
	t.Helper()
	count := 0
	out := sideDiskMutateSource(t, raw, func(f *File) {
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
				continue
			}
			edge := en.Records[0]
			model := edge.Args[3].Ref
			me := f.Entities[model]
			if len(me.Records) != 1 || me.Records[0].Type != "LINE" {
				continue
			}
			line := me.Records[0]
			vector := f.Entities[line.Args[2].Ref].Records[0]
			direction := f.Entities[vector.Args[1].Ref].Records[0]
			if mode != "direction-only" {
				edge.Args[4].Text = "F"
			}
			if mode != "sense-only" {
				for i, v := range direction.Args[1].Items {
					x, e := cylinderSupportNumber(v)
					if e != nil {
						t.Fatal(e)
					}
					direction.Args[1].Items[i].Text = real(-x)
				}
			}
			switch mode {
			case "magnitude":
				vector.Args[2].Text = "2."
			case "origin":
				p := f.Entities[line.Args[1].Ref].Records[0]
				p.Args[1].Items[0].Text = "123."
			case "non-cardinal":
				direction.Args[1].Items[0].Text = "0.5"
			}
			count++
		}
	})
	return out, count
}
func TestReadSideOpenDualDiskProductReversedLines(t *testing.T) {
	t.Parallel()
	for _, through := range []bool{false, true} {
		d := sideDiskDescriptor(t, through)
		var b bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
			t.Fatal(e)
		}
		raw, n := sideDiskFlipLines(t, b.Bytes(), "paired")
		if n < 10 {
			t.Fatal("missing source lines", n)
		}
		if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("canonical reader widened")
		}
		got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true})
		if e != nil {
			t.Fatal(e)
		}
		if len(got.ReversedModelLines) != n {
			t.Fatal("source provenance count")
		}
		var rewritten bytes.Buffer
		if e = WriteSideOpenDualDiskProduct(context.Background(), &rewritten, got.Descriptor); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(rewritten.Bytes(), b.Bytes()) {
			t.Fatal("canonical geometry changed")
		}
		normal, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true})
		if e != nil || len(normal.ReversedModelLines) != 0 {
			t.Fatal("canonical regression", e)
		}
	}
}
func TestReadSideOpenDualDiskProductReversedLineRejects(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"direction-only", "sense-only", "magnitude", "origin", "non-cardinal"} {
		t.Run(mode, func(t *testing.T) {
			raw, _ := sideDiskFlipLines(t, b.Bytes(), mode)
			got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true})
			if e == nil || got.Body != 0 || len(got.ReversedModelLines) != 0 {
				t.Fatal("invalid/partial proof", e)
			}
		})
	}
	raw, _ := sideDiskFlipLines(t, b.Bytes(), "paired")
	// Sharing even an equal direction across model lines is outside this encoding
	// proof; canonical source dependency ownership remains part of the contract.
	raw = sideDiskMutateSource(t, raw, func(f *File) {
		var first Value
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "LINE" {
				v := f.Entities[en.Records[0].Args[2].Ref].Records[0]
				if first.Kind == Reference {
					v.Args[1] = first
					return
				}
				first = v.Args[1]
			}
		}
	})
	if _, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true}); e == nil {
		t.Fatal("shared direction accepted")
	}
	vendor, e := os.ReadFile("testdata/overlapping-side-pockets.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(vendor), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true}); e == nil {
		t.Fatal("rounded vendor accepted")
	}
}
func TestOCCTSideOpenDualDiskReversedLines(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
			t.Fatal(e)
		}
		raw, _ := sideDiskFlipLines(t, b.Bytes(), "paired")
		path := filepath.Join(t.TempDir(), "reversed.step")
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		out, e := exec.Command(oracle, "read", path).CombinedOutput()
		if e != nil {
			t.Fatalf("%v %s", e, out)
		}
		start := bytes.LastIndex(out, []byte("{\"Faces\""))
		if start < 0 {
			t.Fatal(string(out))
		}
		var got struct{ Volume, Area float64 }
		if e = json.Unmarshal(bytes.TrimSpace(out[start:]), &got); e != nil {
			t.Fatal(e)
		}
		angle := math.Pi - math.Acos(.6)
		area := 312 - 24 + 15*angle
		volume := 360 - 3*(6.25*angle+3)
		if through {
			area = 258 + 17.5*angle
			volume = 360 - 6*(6.25*angle+3)
		}
		if math.Abs(got.Volume-volume) > 1e-8 || math.Abs(got.Area-area) > 1e-8 {
			t.Fatal(through, got, volume, area)
		}
	}
}
