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
	"testing"
)

func generalSideDiskDescriptor(t *testing.T, through, swap, posed bool) GeneralSideOpenDualDiskDescriptor {
	t.Helper()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if posed {
		frame, _ = geom.NewFrame(geom.Vec3{X: 50, Z: 6}, geom.Vec3{X: -1}, geom.Vec3{Z: -1})
	}
	depth := 3.
	if through {
		depth = 6
	}
	d := GeneralSideOpenDualDiskDescriptor{Name: "external radical 5 / 8.5", Frame: frame, Max: geom.Vec2{X: 50, Y: 12}, Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 5}, {Center: geom.Vec2{X: 24.5}, Radius: 8.5}}, Height: 6, Depth: depth, LengthUnit: .001, Tolerance: 1e-9}
	if swap {
		d.Discs[0], d.Discs[1] = d.Discs[1], d.Discs[0]
	}
	return d
}
func TestWriteGeneralSideOpenDualDiskProduct(t *testing.T) {
	for _, through := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := generalSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(through, swap, posed, e)
				}
				f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
				if e != nil {
					t.Fatal(e)
				}
				counts := map[string]int{}
				radii := map[float64]int{}
				for _, en := range f.Entities {
					for _, r := range en.Records {
						counts[r.Type]++
						if r.Type == "CYLINDRICAL_SURFACE" {
							radius, e := cylinderSupportNumber(r.Args[2])
							if e != nil {
								t.Fatal(e)
							}
							radii[radius]++
						}
					}
				}
				wantFaces := 16
				if through {
					wantFaces = 9
				}
				if counts["ADVANCED_FACE"] != wantFaces || counts["CIRCLE"] != 4 || counts["PCURVE"] != 0 || counts["SURFACE_CURVE"] != 0 || radii[5] != 1 || radii[8.5] != 1 {
					t.Fatal(counts, radii)
				}
				if _, e = ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
					t.Fatal("between-centers reader widened")
				}
				if _, e = ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
					t.Fatal("equal reader widened")
				}
				if _, e = ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}); e == nil {
					t.Fatal("equal reader options widened")
				}
				var repeated bytes.Buffer
				if e = WriteGeneralSideOpenDualDiskProduct(context.Background(), &repeated, d); e != nil || !bytes.Equal(repeated.Bytes(), b.Bytes()) {
					t.Fatal("nondeterministic", e)
				}
			}
		}
	}
}
func TestWriteGeneralSideOpenDualDiskProductRejects(t *testing.T) {
	edits := []struct {
		name string
		edit func(*GeneralSideOpenDualDiskDescriptor)
	}{
		{"equal", func(d *GeneralSideOpenDualDiskDescriptor) { d.Discs[1].Radius = 5 }},
		{"tangent", func(d *GeneralSideOpenDualDiskDescriptor) { d.Discs[1].Center.X = 33.5 }},
		{"contained", func(d *GeneralSideOpenDualDiskDescriptor) { d.Discs[1].Center.X = 23.5 }},
		{"irrational-crossing", func(d *GeneralSideOpenDualDiskDescriptor) { d.Discs[1].Center.X = 25 }},
		{"off-side", func(d *GeneralSideOpenDualDiskDescriptor) { d.Discs[1].Center.Y = 1 }},
		{"invalid-depth", func(d *GeneralSideOpenDualDiskDescriptor) { d.Depth = 7 }},
		{"unit", func(d *GeneralSideOpenDualDiskDescriptor) { d.LengthUnit = .0254 }},
	}
	for _, tt := range edits {
		t.Run(tt.name, func(t *testing.T) {
			d := generalSideDiskDescriptor(t, false, false, false)
			tt.edit(&d)
			var b bytes.Buffer
			if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), &b, d); e == nil || b.Len() != 0 {
				t.Fatal("invalid/partial output", e)
			}
		})
	}
	d := generalSideDiskDescriptor(t, false, false, false)
	var b bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, SideOpenDualDiskDescriptor(d)); e == nil || b.Len() != 0 {
		t.Fatal("equal writer widened", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := WriteGeneralSideOpenDualDiskProduct(ctx, &b, d); !errors.Is(e, context.Canceled) || b.Len() != 0 {
		t.Fatal(e)
	}
	if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), sideDiskShortWriter{}, d); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
}
func TestOCCTWriteGeneralSideOpenDualDiskProduct(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	type measures struct {
		Volume, Area float64
		Center       [3]float64
		Cylinders    int
	}
	run := func(mode, path string) measures {
		out, e := exec.Command(oracle, mode, path).CombinedOutput()
		if e != nil {
			t.Fatalf("%s: %v %s", mode, e, out)
		}
		start := bytes.LastIndex(out, []byte("{\"Faces\""))
		if start < 0 {
			t.Fatal(string(out))
		}
		var m measures
		if e = json.Unmarshal(bytes.TrimSpace(out[start:]), &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	for _, through := range []bool{false, true} {
		mode := "general-side"
		if through {
			mode = "general-through"
		}
		reference := run(mode, filepath.Join(t.TempDir(), "independent.step"))
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := generalSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(t.TempDir(), "native.step")
				if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				got := run("read", path)
				expected := d.Frame.Origin().Add(d.Frame.XAxis().Scale(reference.Center[0])).Add(d.Frame.YAxis().Scale(reference.Center[1])).Add(d.Frame.ZAxis().Scale(6 - reference.Center[2]))
				actual := geom.Vec3{X: got.Center[0], Y: got.Center[1], Z: got.Center[2]}
				a := math.Pi/2 - math.Asin(.6)
				c := math.Pi/2 + math.Asin(15./17)
				removed := 9 + (25*a+72.25*c)/2
				arc := 5*a + 8.5*c
				wantV := 3600 - d.Depth*removed
				wantA := 1944 + (arc-18)*d.Depth
				if through {
					wantA -= 2 * removed
				}
				if got.Cylinders != 2 || math.Abs(got.Volume-reference.Volume) > 1e-7 || math.Abs(got.Area-reference.Area) > 1e-7 || actual.Sub(expected).Length() > 1e-8 || math.Abs(got.Volume-wantV) > 1e-7 || math.Abs(got.Area-wantA) > 1e-7 {
					t.Fatal(through, swap, posed, got, reference, expected, wantV, wantA)
				}
				t.Logf("through=%v swap=%v posed=%v volume=%.12g area=%.12g", through, swap, posed, got.Volume, got.Area)
			}
		}
	}
}

func TestWriteGeneralSideOpenDualDiskCategoryAndRightJoin(t *testing.T) {
	between := GeneralSideOpenDualDiskDescriptor(unequalSideDiskDescriptor(t, false, false, false))
	var b bytes.Buffer
	if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), &b, between); e == nil || b.Len() != 0 {
		t.Fatal("between-center category admitted", e)
	}
	external := generalSideDiskDescriptor(t, false, false, false)
	if e := WriteUnequalSideOpenDualDiskProduct(context.Background(), &b, UnequalSideOpenDualDiskDescriptor(external)); e == nil || b.Len() != 0 {
		t.Fatal("old unequal writer broadened", e)
	}
	// Reflect local geometry, rather than its frame: the radical is now x=33,
	// strictly to the right of both centers. This remains a separate minor-arc case.
	external.Discs[0].Center.X = 30
	external.Discs[1].Center.X = 25.5
	if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), &b, external); e != nil {
		t.Fatal("right external radical", e)
	}
}
