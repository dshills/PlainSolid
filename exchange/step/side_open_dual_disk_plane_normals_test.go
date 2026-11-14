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

func sideDiskReversePlanes(t *testing.T, raw []byte, mode string) ([]byte, int) {
	t.Helper()
	n := 0
	out := sideDiskMutateSource(t, raw, func(f *File) {
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != "ADVANCED_FACE" {
				continue
			}
			face := en.Records[0]
			surface := f.Entities[face.Args[2].Ref]
			if len(surface.Records) != 1 || surface.Records[0].Type != "PLANE" {
				continue
			}
			placement := f.Entities[surface.Records[0].Args[1].Ref].Records[0]
			if mode != "normal-only" {
				if face.Args[3].Text == "T" {
					face.Args[3].Text = "F"
				} else {
					face.Args[3].Text = "T"
				}
			}
			if mode != "sense-only" {
				normal := f.Entities[placement.Args[2].Ref].Records[0]
				for i, v := range normal.Args[1].Items {
					x, e := cylinderSupportNumber(v)
					if e != nil {
						t.Fatal(e)
					}
					normal.Args[1].Items[i].Text = real(-x)
				}
			}
			switch mode {
			case "origin":
				p := f.Entities[placement.Args[1].Ref].Records[0]
				p.Args[1].Items[0].Text = "123."
			case "reference":
				r := f.Entities[placement.Args[3].Ref].Records[0]
				for i, v := range r.Args[1].Items {
					x, e := cylinderSupportNumber(v)
					if e != nil {
						t.Fatal(e)
					}
					r.Args[1].Items[i].Text = real(-x)
				}
			case "bound":
				bound := f.Entities[face.Args[1].Items[0].Ref].Records[0]
				if bound.Args[2].Text == "T" {
					bound.Args[2].Text = "F"
				} else {
					bound.Args[2].Text = "T"
				}
			}
			n++
		}
	})
	return out, n
}
func TestReadSideOpenDualDiskProductOppositePlanes(t *testing.T) {
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
			t.Fatal(e)
		}
		raw, n := sideDiskReversePlanes(t, b.Bytes(), "paired")
		if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("default widened")
		}
		options := SideOpenDualDiskReadOptions{AllowOppositePlaneNormals: true}
		got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
		if e != nil {
			t.Fatal(through, e)
		}
		if len(got.OppositeNormalPlanes) != n {
			t.Fatal("source roles")
		}
		want := 14
		if through {
			want = 7
		}
		if n != want {
			t.Fatal(n, want)
		}
		var out bytes.Buffer
		if e = WriteSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
			t.Fatal("canonical geometry changed", e)
		}
		// All independent exact encoding opt-ins compose without changing adjacency.
		combined, _ := sideDiskFlipLines(t, raw, "paired")
		combined = sideDiskReverseCircleNormals(t, combined, 1)
		options.AllowReversedModelLines = true
		options.AllowRephasedModelCircles = true
		options.AllowOppositeModelCircleNormals = true
		got, e = ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(combined), Limits{}, 1e-9, options)
		if e != nil || len(got.OppositeNormalPlanes) != n || len(got.OppositeNormalModelCircles) != 4 {
			t.Fatal("composed proof", e)
		}
	}
}
func TestReadSideOpenDualDiskProductOppositePlanesRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	options := SideOpenDualDiskReadOptions{AllowOppositePlaneNormals: true}
	for _, mode := range []string{"normal-only", "sense-only", "origin", "reference", "bound"} {
		t.Run(mode, func(t *testing.T) {
			raw, _ := sideDiskReversePlanes(t, b.Bytes(), mode)
			got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
			if e == nil || got.Body != 0 || len(got.OppositeNormalPlanes) != 0 {
				t.Fatal("invalid/partial proof", e)
			}
		})
	}
	raw, _ := sideDiskReversePlanes(t, b.Bytes(), "paired")
	raw = sideDiskMutateSource(t, raw, func(f *File) {
		var first Value
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "PLANE" {
				p := f.Entities[en.Records[0].Args[1].Ref].Records[0]
				if first.Kind == Reference {
					p.Args[2] = first
					return
				}
				first = p.Args[2]
			}
		}
	})
	if _, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options); e == nil {
		t.Fatal("shared normal")
	}
}
func TestOCCTSideOpenDualDiskOppositePlanes(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		for _, combined := range []bool{false, true} {
			var b bytes.Buffer
			if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
				t.Fatal(e)
			}
			raw, _ := sideDiskReversePlanes(t, b.Bytes(), "paired")
			if combined {
				raw, _ = sideDiskFlipLines(t, raw, "paired")
				raw = sideDiskReverseCircleNormals(t, raw, 1)
			}
			path := filepath.Join(t.TempDir(), "opposite-planes.step")
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			out, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatalf("through=%v combined=%v: %v %s", through, combined, e, out)
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
			area := 288 + 15*angle
			volume := 360 - 3*(6.25*angle+3)
			if through {
				area = 258 + 17.5*angle
				volume = 360 - 6*(6.25*angle+3)
			}
			if math.Abs(got.Volume-volume) > 1e-8 || math.Abs(got.Area-area) > 1e-8 {
				t.Fatal(through, combined, got, volume, area)
			}
		}
	}
}
