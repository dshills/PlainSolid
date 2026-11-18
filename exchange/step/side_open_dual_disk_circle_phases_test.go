package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func sideDiskRephaseCircles(t *testing.T, raw []byte, quarter int, mutation string) ([]byte, int) {
	t.Helper()
	n := 0
	out := sideDiskMutateSource(t, raw, func(f *File) {
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
				continue
			}
			edge := en.Records[0]
			model := f.Entities[edge.Args[3].Ref]
			if len(model.Records) != 1 || model.Records[0].Type != "CIRCLE" {
				continue
			}
			circle := model.Records[0]
			pid := circle.Args[1].Ref
			frame, e := sideDiskReadFrame(f, pid)
			if e != nil {
				t.Fatal(e)
			}
			placement := f.Entities[pid].Records[0]
			ref := f.Entities[placement.Args[3].Ref].Records[0]
			axis := []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.XAxis().Scale(-1), frame.YAxis().Scale(-1)}[quarter]
			ref.Args[1].Items = []Value{{Kind: Number, Text: real(axis.X)}, {Kind: Number, Text: real(axis.Y)}, {Kind: Number, Text: real(axis.Z)}}
			switch mutation {
			case "complement":
				if edge.Args[4].Text == "T" {
					edge.Args[4].Text = "F"
				} else {
					edge.Args[4].Text = "T"
				}
			case "normal":
				normal := f.Entities[placement.Args[2].Ref].Records[0]
				z := frame.ZAxis().Scale(-1)
				normal.Args[1].Items = []Value{{Kind: Number, Text: real(z.X)}, {Kind: Number, Text: real(z.Y)}, {Kind: Number, Text: real(z.Z)}}
			case "center":
				p := f.Entities[placement.Args[1].Ref].Records[0]
				p.Args[1].Items[0].Text = "123."
			case "radius":
				circle.Args[2].Text = "3."
			case "non-cardinal":
				ref.Args[1].Items[0].Text = "0.6"
			}
			n++
		}
	})
	return out, n
}
func TestReadSideOpenDualDiskProductCircleRephase(t *testing.T) {
	slowtest.Skip(t)
	for _, through := range []bool{false, true} {
		for quarter := 1; quarter <= 3; quarter++ {
			d := sideDiskDescriptor(t, through)
			if quarter == 2 {
				d.Frame, _ = geom.NewFrame(geom.Vec3{X: 20, Y: -4, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
			}
			if quarter == 3 {
				d.Frame, _ = geom.NewFrame(geom.Vec3{X: 3, Y: 20, Z: -4}, geom.Vec3{Z: 1}, geom.Vec3{Y: -1})
			}
			var b bytes.Buffer
			if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
				t.Fatal(e)
			}
			raw, n := sideDiskRephaseCircles(t, b.Bytes(), quarter, "")
			if n != 4 {
				t.Fatal(n)
			}
			if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
				t.Fatal("default widened")
			}
			got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowRephasedModelCircles: true})
			if e != nil {
				t.Fatal(through, quarter, e)
			}
			if len(got.RephasedModelCircles) != 4 || len(got.ReversedModelLines) != 0 {
				t.Fatal("provenance")
			}
			var out bytes.Buffer
			if e = WriteSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(b.Bytes(), out.Bytes()) {
				t.Fatal("canonical shape changed")
			}
			combined, lines := sideDiskFlipLines(t, raw, "paired")
			both, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(combined), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowRephasedModelCircles: true, AllowReversedModelLines: true})
			if e != nil || len(both.RephasedModelCircles) != 4 || len(both.ReversedModelLines) != lines {
				t.Fatal("combined independent proofs", e)
			}
		}
	}
}
func TestReadSideOpenDualDiskProductCircleRephaseRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []string{"complement", "normal", "center", "radius", "non-cardinal"} {
		t.Run(mutation, func(t *testing.T) {
			raw, _ := sideDiskRephaseCircles(t, b.Bytes(), 1, mutation)
			got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowRephasedModelCircles: true})
			if e == nil || got.Body != 0 || len(got.RephasedModelCircles) != 0 {
				t.Fatal("invalid or partial proof", e)
			}
		})
	}
	raw, _ := sideDiskRephaseCircles(t, b.Bytes(), 1, "")
	shared := sideDiskMutateSource(t, raw, func(f *File) {
		var first Value
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "CIRCLE" {
				p := f.Entities[en.Records[0].Args[1].Ref].Records[0]
				if first.Kind == Reference {
					p.Args[3] = first
					return
				}
				first = p.Args[3]
			}
		}
	})
	if _, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(shared), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowRephasedModelCircles: true}); e == nil {
		t.Fatal("shared reference accepted")
	}
	// Circle-only authorization must not silently enable paired line changes.
	lines, _ := sideDiskFlipLines(t, raw, "paired")
	if _, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(lines), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowRephasedModelCircles: true}); e == nil {
		t.Fatal("line opt-in inferred")
	}
}
func TestOCCTSideOpenDualDiskCircleRephase(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		for quarter := 1; quarter <= 3; quarter++ {
			var b bytes.Buffer
			if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
				t.Fatal(e)
			}
			raw, _ := sideDiskRephaseCircles(t, b.Bytes(), quarter, "")
			path := filepath.Join(t.TempDir(), "rephased.step")
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
			area := 288 + 15*angle
			volume := 360 - 3*(6.25*angle+3)
			if through {
				area = 258 + 17.5*angle
				volume = 360 - 6*(6.25*angle+3)
			}
			if math.Abs(got.Volume-volume) > 1e-8 || math.Abs(got.Area-area) > 1e-8 {
				t.Fatal(through, quarter, got, volume, area)
			}
		}
	}
}
