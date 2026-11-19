package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"testing"
)

func sideDiskDescriptor(t *testing.T, through bool) SideOpenDualDiskDescriptor {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 10, Z: 6}, geom.Vec3{X: -1}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	depth := 3.
	if through {
		depth = 6
	}
	return SideOpenDualDiskDescriptor{Name: "two merged side pockets", Frame: f, Min: geom.Vec2{Y: 4}, Max: geom.Vec2{X: 10, Y: 10}, Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}, Height: 6, Depth: depth, LengthUnit: .001, Tolerance: 1e-9}
}
func TestWriteSideOpenDualDiskProduct(t *testing.T) {
	t.Parallel()
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, through)); e != nil {
			t.Fatal(through, e)
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		for _, ent := range f.Entities {
			for _, r := range ent.Records {
				counts[r.Type]++
			}
		}
		if counts["CYLINDRICAL_SURFACE"] != 2 || counts["PCURVE"] != 0 || counts["SURFACE_CURVE"] != 0 || counts["MANIFOLD_SOLID_BREP"] != 1 || counts["CIRCLE"] != 4 {
			t.Fatal(through, counts)
		}
	}
}

func TestOCCTWriteSideOpenDualDiskProduct(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		for pose := 0; pose < 3; pose++ {
			d := sideDiskDescriptor(t, through)
			if pose == 1 {
				d.Frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
			}
			if pose == 2 {
				d.Frame, _ = geom.NewFrame(geom.Vec3{X: 20, Y: -4, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
				d.Discs[0], d.Discs[1] = d.Discs[1], d.Discs[0]
			}
			var b bytes.Buffer
			if err := WriteSideOpenDualDiskProduct(context.Background(), &b, d); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "native.step")
			if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(oracle, "read", path).CombinedOutput()
			if err != nil {
				t.Fatalf("through=%v: %v\n%s", through, err, out)
			}
			start := bytes.LastIndex(out, []byte("{\"Faces\""))
			if start < 0 {
				t.Fatal(string(out))
			}
			var got struct {
				Faces, Planes, Cylinders int
				Volume, Area             float64
				Center                   [3]float64
			}
			if err := json.Unmarshal(bytes.TrimSpace(out[start:]), &got); err != nil {
				t.Fatal(err)
			}
			theta := math.Acos(.6)
			B := 6.25*(math.Pi-theta) + 3
			P := 10 * (math.Pi - theta)
			volume := 360 - 3*B
			area := 288 + 1.5*P
			cy := (2464 - 12*B) / volume
			cz := (1080 - 13.5*B) / volume
			if through {
				volume = 360 - 6*B
				area = 264 + 3*P - 2*B
				cy = (2408 - 24*B) / volume
				cz = 3
			}
			localCenter := geom.Vec3{X: 5, Y: cy, Z: 6 - cz}
			expectedCenter := d.Frame.Origin().Add(d.Frame.XAxis().Scale(localCenter.X)).Add(d.Frame.YAxis().Scale(localCenter.Y)).Add(d.Frame.ZAxis().Scale(localCenter.Z))
			if got.Cylinders != 2 || math.Abs(got.Volume-volume) > 1e-8 || math.Abs(got.Area-area) > 1e-8 || math.Abs(got.Center[0]-expectedCenter.X) > 1e-9 || math.Abs(got.Center[1]-expectedCenter.Y) > 1e-9 || math.Abs(got.Center[2]-expectedCenter.Z) > 1e-9 {
				t.Fatalf("through=%v got %+v expected volume %g area %g center %g %g", through, got, volume, area, cy, cz)
			}
			t.Logf("through=%v pose=%d %+v", through, pose, got)
		}
	}
}

func TestWriteSideOpenDualDiskProductRejectsAtomically(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*SideOpenDualDiskDescriptor)
	}{
		{"unequal", func(d *SideOpenDualDiskDescriptor) { d.Discs[1].Radius = 2 }},
		{"tangent", func(d *SideOpenDualDiskDescriptor) { d.Discs[1].Center.X = 8.5 }},
		{"off-side", func(d *SideOpenDualDiskDescriptor) { d.Discs[1].Center.Y = 4.5 }},
		{"past-stock", func(d *SideOpenDualDiskDescriptor) { d.Depth = 7 }},
		{"zero-depth", func(d *SideOpenDualDiskDescriptor) { d.Depth = 0 }},
		{"non-SI", func(d *SideOpenDualDiskDescriptor) { d.LengthUnit = .0254 }},
		{"nonfinite", func(d *SideOpenDualDiskDescriptor) { d.Height = math.NaN() }},
		{"name-limit", func(d *SideOpenDualDiskDescriptor) { d.Name = strings.Repeat("a", 1025) }},
		{"invalid-name", func(d *SideOpenDualDiskDescriptor) { d.Name = string([]byte{255}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := sideDiskDescriptor(t, false)
			tt.edit(&d)
			var b bytes.Buffer
			if err := WriteSideOpenDualDiskProduct(context.Background(), &b, d); err == nil || b.Len() != 0 {
				t.Fatalf("error %v bytes %d", err, b.Len())
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var b bytes.Buffer
	if err := WriteSideOpenDualDiskProduct(ctx, &b, sideDiskDescriptor(t, false)); !errors.Is(err, context.Canceled) || b.Len() != 0 {
		t.Fatal(err, b.Len())
	}
	if err := WriteSideOpenDualDiskProduct(nil, &b, sideDiskDescriptor(t, false)); err == nil {
		t.Fatal("nil context")
	}
	if err := WriteSideOpenDualDiskProduct(context.Background(), nil, sideDiskDescriptor(t, false)); err == nil {
		t.Fatal("nil writer")
	}
	if err := WriteSideOpenDualDiskProduct(context.Background(), sideDiskShortWriter{}, sideDiskDescriptor(t, false)); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	sentinel := errors.New("sink failure")
	if err := WriteSideOpenDualDiskProduct(context.Background(), sideDiskErrorWriter{sentinel}, sideDiskDescriptor(t, false)); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

type sideDiskShortWriter struct{}

func (sideDiskShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type sideDiskErrorWriter struct{ err error }

func (w sideDiskErrorWriter) Write(p []byte) (int, error) { return 0, w.err }

func TestWriteSideOpenDualDiskProductOrderAndSource(t *testing.T) {
	t.Parallel()
	for _, through := range []bool{false, true} {
		for _, swapped := range []bool{false, true} {
			d := sideDiskDescriptor(t, through)
			d.Name = "cutter's α part"
			if swapped {
				d.Discs[0], d.Discs[1] = d.Discs[1], d.Discs[0]
			}
			var a, b bytes.Buffer
			for _, w := range []*bytes.Buffer{&a, &b} {
				if err := WriteSideOpenDualDiskProduct(context.Background(), w, d); err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Fatal("non-deterministic output")
			}
			source, err := Parse(bytes.NewReader(a.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range source.Entities {
				for _, r := range e.Records {
					if r.Type == "PRODUCT" && (len(r.Args) < 2 || r.Args[1].Text != d.Name) {
						t.Fatalf("product name %+v", r)
					}
					if r.Type == "MANIFOLD_SOLID_BREP" && r.Args[0].Text != d.Name {
						t.Fatal("body name lost")
					}
				}
			}
		}
	}
}
