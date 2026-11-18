package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func offsetSideDiskDescriptor(t *testing.T, through, swap, posed bool) OffsetSideOpenDualDiskDescriptor {
	t.Helper()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if posed {
		frame, _ = geom.NewFrame(geom.Vec3{X: 40, Z: 6}, geom.Vec3{X: -1}, geom.Vec3{Z: -1})
	}
	depth := 3.
	if through {
		depth = 6
	}
	d := OffsetSideOpenDualDiskDescriptor{Name: "offset external-center walls", Frame: frame, Max: geom.Vec2{X: 40, Y: 10}, Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}, Height: 6, Depth: depth, LengthUnit: .001, Tolerance: 1e-9}
	if swap {
		d.Discs[0], d.Discs[1] = d.Discs[1], d.Discs[0]
	}
	return d
}
func TestWriteOffsetSideOpenDualDiskProduct(t *testing.T) {
	slowtest.Skip(t)
	for _, through := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := offsetSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
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
						if r.Type == "PRODUCT" && r.Args[0].Text != offsetSideDiskProductID {
							t.Fatal("missing registered offset class")
						}
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
				if counts["ADVANCED_FACE"] != wantFaces || counts["CIRCLE"] != 4 || counts["PCURVE"] != 0 || counts["SURFACE_CURVE"] != 0 || radii[65./8] != 1 || radii[87./8] != 1 {
					t.Fatal(counts, radii)
				}
				if _, e = ReadGeneralSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
					t.Fatal("ordinary general reader widened")
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
				if e = WriteOffsetSideOpenDualDiskProduct(context.Background(), &repeated, d); e != nil || !bytes.Equal(repeated.Bytes(), b.Bytes()) {
					t.Fatal("nondeterministic", e)
				}
			}
		}
	}
}
func TestOCCTWriteOffsetSideOpenDualDiskProduct(t *testing.T) {
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
		mode := "offset-side"
		if through {
			mode = "offset-through"
		}
		reference := run(mode, filepath.Join(t.TempDir(), "independent.step"))
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := offsetSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(t.TempDir(), "native.step")
				if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				got := run("read", path)
				expected := d.Frame.Origin().Add(d.Frame.XAxis().Scale(reference.Center[0])).Add(d.Frame.YAxis().Scale(reference.Center[1])).Add(d.Frame.ZAxis().Scale(6 - reference.Center[2]))
				actual := geom.Vec3{X: got.Center[0], Y: got.Center[1], Z: got.Center[2]}
				if got.Cylinders != 2 || math.Abs(got.Volume-reference.Volume) > 1e-7 || math.Abs(got.Area-reference.Area) > 1e-7 || actual.Sub(expected).Length() > 1e-8 {
					t.Fatal(through, swap, posed, got, reference, expected)
				}
				t.Logf("through=%v swap=%v posed=%v volume=%.12g area=%.12g", through, swap, posed, got.Volume, got.Area)
			}
		}
	}
}

func TestWriteOffsetSideOpenDualDiskClassProof(t *testing.T) {
	var b bytes.Buffer
	binary := OffsetSideOpenDualDiskDescriptor(generalSideDiskDescriptor(t, false, false, false))
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, binary); e == nil || b.Len() != 0 {
		t.Fatal("binary-wall category admitted", e)
	}
	d := offsetSideDiskDescriptor(t, false, false, false)
	if e := WriteGeneralSideOpenDualDiskProduct(context.Background(), &b, GeneralSideOpenDualDiskDescriptor(d)); e == nil || b.Len() != 0 {
		t.Fatal("ordinary external writer broadened", e)
	}
	cut, e := trimmed.CutOffsetSideOpenDualDisks(context.Background(), d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, d.Tolerance, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, ids := range cut.WallFaces {
		if _, ok := cut.Solid.Faces()[ids[0]].Surface.(*geom.ExactBezierSurface); !ok {
			t.Fatal("wall class changed")
		}
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, cut.Solid); e != nil {
		t.Fatal("exact original coefficient not retained", e)
	}
	if e = WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(b.Bytes(), []byte("B_SPLINE")) {
		t.Fatal("unrepresentable control exported as floating spline")
	}
}

func TestWriteOffsetSideOpenDualDiskRejects(t *testing.T) {
	for _, edit := range []func(*OffsetSideOpenDualDiskDescriptor){
		func(d *OffsetSideOpenDualDiskDescriptor) { d.Discs[0].Center.Y = 0 },
		func(d *OffsetSideOpenDualDiskDescriptor) { d.Discs[1].Center.Y = -7 },
		func(d *OffsetSideOpenDualDiskDescriptor) { d.Discs[1].Radius = 10 },
		func(d *OffsetSideOpenDualDiskDescriptor) { d.Discs[1].Center.X = 21 },
		func(d *OffsetSideOpenDualDiskDescriptor) { d.Depth = 7 },
		func(d *OffsetSideOpenDualDiskDescriptor) { d.LengthUnit = .0254 },
	} {
		d := offsetSideDiskDescriptor(t, false, false, false)
		edit(&d)
		var b bytes.Buffer
		if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e == nil || b.Len() != 0 {
			t.Fatal("invalid descriptor or partial output", e)
		}
	}
	d := offsetSideDiskDescriptor(t, false, false, false)
	var b bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := WriteOffsetSideOpenDualDiskProduct(ctx, &b, d); e != context.Canceled || b.Len() != 0 {
		t.Fatal(e)
	}
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), sideDiskShortWriter{}, d); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadExactWallSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
		t.Fatal("old exact-wall reader widened")
	}
}

func TestOffsetSideDiskVendorFixtureRemainsUnsupported(t *testing.T) {
	for _, name := range []string{"side", "through"} {
		raw, e := os.ReadFile("testdata/occt_offset_dual_disk_" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil || len(f.Entities) != 582 {
			t.Fatal(e)
		}
		if _, e = ReadExactWallSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("vendor source admitted")
		}
		if _, e = ReadGeneralSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("general source admitted")
		}
	}
}
