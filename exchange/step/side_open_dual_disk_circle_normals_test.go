package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"
)

func sideDiskReverseCircleNormals(t *testing.T, raw []byte, phase int) []byte {
	t.Helper()
	reversed, _ := sideDiskRephaseCircles(t, raw, phase, "normal")
	return sideDiskMutateSource(t, reversed, func(f *File) {
		for _, en := range f.Entities {
			if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
				continue
			}
			edge := en.Records[0]
			model := f.Entities[edge.Args[3].Ref]
			if len(model.Records) != 1 || model.Records[0].Type != "CIRCLE" {
				continue
			}
			if edge.Args[4].Text == "T" {
				edge.Args[4].Text = "F"
			} else {
				edge.Args[4].Text = "T"
			}
		}
	})
}
func TestReadSideOpenDualDiskProductOppositeCircleNormals(t *testing.T) {
	slowtest.Skip(t)
	for _, through := range []bool{false, true} {
		for phase := 0; phase < 4; phase++ {
			var b bytes.Buffer
			if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
				t.Fatal(e)
			}
			raw := sideDiskReverseCircleNormals(t, b.Bytes(), phase)
			if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
				t.Fatal("default widened")
			}
			options := SideOpenDualDiskReadOptions{AllowOppositeModelCircleNormals: true, AllowRephasedModelCircles: phase != 0}
			got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
			if e != nil {
				t.Fatal(through, phase, e)
			}
			wantPhase := 0
			if phase != 0 {
				wantPhase = 4
			}
			if len(got.OppositeNormalModelCircles) != 4 || len(got.RephasedModelCircles) != wantPhase {
				t.Fatal("provenance")
			}
			var out bytes.Buffer
			if e = WriteSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
				t.Fatal("canonical shape changed", e)
			}
			if _, e = ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowRephasedModelCircles: true}); e == nil {
				t.Fatal("opposite permission inferred")
			}
			if phase != 0 {
				if _, e = ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowOppositeModelCircleNormals: true}); e == nil {
					t.Fatal("phase permission inferred")
				}
			}
		}
	}
}
func TestReadSideOpenDualDiskProductOppositeCircleRejects(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	options := SideOpenDualDiskReadOptions{AllowOppositeModelCircleNormals: true, AllowRephasedModelCircles: true}
	for _, mutation := range []string{"normal", "complement", "center", "radius"} {
		t.Run(mutation, func(t *testing.T) {
			raw, _ := sideDiskRephaseCircles(t, b.Bytes(), 1, mutation)
			got, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
			if e == nil || got.Body != 0 || len(got.OppositeNormalModelCircles) != 0 {
				t.Fatal("unpaired/complement/support change accepted", e)
			}
		})
	}
	raw := sideDiskReverseCircleNormals(t, b.Bytes(), 0)
	raw = sideDiskMutateSource(t, raw, func(f *File) {
		var first Value
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "CIRCLE" {
				placement := f.Entities[en.Records[0].Args[1].Ref].Records[0]
				if first.Kind == Reference {
					placement.Args[2] = first
					return
				}
				first = placement.Args[2]
			}
		}
	})
	if _, e := ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options); e == nil {
		t.Fatal("shared normal accepted")
	}
}
func TestOCCTSideOpenDualDiskOppositeCircleNormals(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		for phase := 0; phase < 4; phase++ {
			var b bytes.Buffer
			if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
				t.Fatal(e)
			}
			raw := sideDiskReverseCircleNormals(t, b.Bytes(), phase)
			path := filepath.Join(t.TempDir(), "opposite.step")
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			out, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatalf("through=%v phase=%d: %v %s", through, phase, e, out)
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
				t.Fatal(through, phase, got, volume, area)
			}
		}
	}
}
