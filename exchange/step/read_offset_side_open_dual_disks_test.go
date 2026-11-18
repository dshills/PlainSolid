package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestReadOffsetSideOpenDualDiskProduct(t *testing.T) {
	slowtest.Skip(t)
	for _, through := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				for _, unit := range []float64{1, .001} {
					d := offsetSideDiskDescriptor(t, through, swap, posed)
					d.LengthUnit = unit
					var b bytes.Buffer
					if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
						t.Fatal(e)
					}
					r, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, d.Tolerance)
					if e != nil {
						t.Fatal(e)
					}
					if r.Descriptor.Frame.Origin() != d.Frame.Origin() || r.Descriptor.Discs[0].Center.Y != -7.5 || r.Descriptor.Discs[1].Center.Y != -7.5 || r.Descriptor.Discs[0].Radius != 65./8 || r.Descriptor.Discs[1].Radius != 87./8 || r.Descriptor.LengthUnit != unit {
						t.Fatal(r.Descriptor)
					}
					for i, ids := range r.Native.WallFaces {
						if len(ids) != 1 || r.SourceWallFaces[i] != r.Faces[ids[0]] {
							t.Fatal("wall lineage")
						}
						if _, ok := r.Native.Solid.Faces()[ids[0]].Surface.(*geom.ExactBezierSurface); !ok {
							t.Fatal("wall class")
						}
					}
					if through {
						if r.SourceFloorFace != 0 {
							t.Fatal("through floor")
						}
					} else if r.SourceFloorFace != r.Faces[r.Native.FloorFace] {
						t.Fatal("floor lineage")
					}
					var again bytes.Buffer
					if e = WriteOffsetSideOpenDualDiskProduct(context.Background(), &again, r.Descriptor); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
						t.Fatal("canonical roundtrip", e)
					}
				}
			}
		}
	}
}
func TestReadOffsetSideOpenDualDiskRejects(t *testing.T) {
	d := offsetSideDiskDescriptor(t, false, false, false)
	var b bytes.Buffer
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"PRODUCT", "CYLINDRICAL_SURFACE", "CIRCLE"} {
		raw := sideDiskMutateSource(t, b.Bytes(), func(f *File) {
			for _, id := range f.Order {
				en := f.Entities[id]
				for j, r := range en.Records {
					if r.Type == kind {
						if kind == "PRODUCT" {
							r.Args[0].Text = exactWallSideDiskProductID
						} else {
							r.Args[2].Text = "8.125000000001"
						}
						en.Records[j] = r
						f.Entities[id] = en
						return
					}
				}
			}
		})
		r, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
		if e == nil || r.Body != 0 || len(r.Faces) != 0 {
			t.Fatal(kind, e)
		}
	}
	var forged bytes.Buffer
	old := exactWallSideDiskDescriptor(t, false, false, false)
	if e := WriteExactWallSideOpenDualDiskProduct(context.Background(), &forged, old); e != nil {
		t.Fatal(e)
	}
	raw := bytes.ReplaceAll(forged.Bytes(), []byte(exactWallSideDiskProductID), []byte(offsetSideDiskProductID))
	if _, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("forged class")
	}
}

func TestReadOffsetSideOpenDualDiskProductEncodingProofs(t *testing.T) {
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, offsetSideDiskDescriptor(t, through, true, true)); e != nil {
			t.Fatal(e)
		}
		raw, _ := sideDiskFlipLines(t, b.Bytes(), "paired")
		raw = sideDiskReverseCircleNormals(t, raw, 1)
		raw, _ = sideDiskReversePlanes(t, raw, "paired")
		if _, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("default widened")
		}
		options := SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}
		got, e := ReadOffsetSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
		if e != nil {
			t.Fatal(e)
		}
		if len(got.ReversedModelLines) == 0 || len(got.RephasedModelCircles) != 4 || len(got.OppositeNormalModelCircles) != 4 || len(got.OppositeNormalPlanes) == 0 {
			t.Fatal("encoding lineage")
		}
		var out bytes.Buffer
		if e = WriteOffsetSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
			t.Fatal("normalized ownership changed", e)
		}
	}
}
func TestReadOffsetSideOpenDualDiskGraphOwnership(t *testing.T) {
	d := offsetSideDiskDescriptor(t, false, false, true)
	var b bytes.Buffer
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	edits := []func(*File){
		func(f *File) { sideDiskEditRecord(f, "SI_UNIT", func(r *Record) { r.Args[1].Text = "RADIAN" }) },
		func(f *File) { sideDiskEditRecord(f, "PRODUCT", func(r *Record) { r.Args[1].Text = "foreign owner" }) },
		func(f *File) {
			sideDiskEditRecord(f, "CIRCLE", func(r *Record) {
				id := r.Args[1].Ref
				p := f.Entities[id].Records[0].Args[1].Ref
				f.Entities[p].Records[0].Args[1].Items[1].Text = "7.500000000001"
			})
		},
		func(f *File) {
			var first Value
			for _, id := range f.Order {
				r := f.Entities[id].Records[0]
				if r.Type == "CYLINDRICAL_SURFACE" {
					if first.Kind == Reference {
						r.Args[1] = first
						return
					}
					first = r.Args[1]
				}
			}
		},
	}
	for i, edit := range edits {
		raw := sideDiskMutateSource(t, b.Bytes(), edit)
		out, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
		if e == nil || out.Body != 0 {
			t.Fatal(i, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 64}, {MaxEntities: 16}} {
		if out, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), l, 1e-9); e == nil || out.Body != 0 {
			t.Fatal("limits", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadOffsetSideOpenDualDiskProduct(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}

func TestReadOffsetSideOpenDualDiskExactSIRescale(t *testing.T) {
	d := offsetSideDiskDescriptor(t, false, false, true)
	d.LengthUnit = 1
	scaled := d
	scaled.LengthUnit = .001
	scaled.Max.X *= 1000
	scaled.Max.Y *= 1000
	scaled.Height *= 1000
	scaled.Depth *= 1000
	var e error
	scaled.Frame, e = geom.NewFrame(d.Frame.Origin().Scale(1000), d.Frame.XAxis(), d.Frame.ZAxis())
	if e != nil {
		t.Fatal(e)
	}
	for i := range scaled.Discs {
		scaled.Discs[i].Center.X *= 1000
		scaled.Discs[i].Center.Y *= 1000
		scaled.Discs[i].Radius *= 1000
	}
	var b bytes.Buffer
	if e = WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, scaled); e != nil {
		t.Fatal(e)
	}
	got, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if got.Descriptor != scaled {
		t.Fatal("authored model scalars or unit changed", got.Descriptor)
	}
	for i := range d.Discs {
		if new(big.Rat).Quo(sideRat(got.Descriptor.Discs[i].Radius), big.NewRat(1000, 1)).Cmp(sideRat(d.Discs[i].Radius)) != 0 {
			t.Fatal("physical dimension")
		}
	}
}

func TestOCCTReadOffsetSideOpenDualDiskProduct(t *testing.T) {
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
				parsed, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, d.Tolerance)
				if e != nil {
					t.Fatal(e)
				}
				b.Reset()
				if e = WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, parsed.Descriptor); e != nil {
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
func TestReadOffsetSideOpenDualDiskStockOrigin(t *testing.T) {
	d := offsetSideDiskDescriptor(t, false, false, true)
	d.Min = geom.Vec2{X: 2, Y: 3}
	d.Max = geom.Vec2{X: 42, Y: 13}
	for i := range d.Discs {
		d.Discs[i].Center.X += 2
		d.Discs[i].Center.Y += 3
	}
	var b bytes.Buffer
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	out, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	want, e := sideDiskWorld(d.Frame, geom.Vec3{X: 2, Y: 3})
	if e != nil || out.Descriptor.Frame.Origin() != want || out.Descriptor.Discs[0].Center.Y != -7.5 {
		t.Fatal("lost external cylinder origin", out.Descriptor, e)
	}
	for i := range d.Discs {
		a, e := sideDiskWorld(d.Frame, geom.Vec3{X: d.Discs[i].Center.X, Y: d.Discs[i].Center.Y})
		if e != nil {
			t.Fatal(e)
		}
		b, e := sideDiskWorld(out.Descriptor.Frame, geom.Vec3{X: out.Descriptor.Discs[i].Center.X, Y: out.Descriptor.Discs[i].Center.Y})
		if e != nil || a != b {
			t.Fatal("cutter origin changed", e)
		}
	}
}
func TestReadOffsetSideOpenDualDiskVendorRejection(t *testing.T) {
	for _, name := range []string{"side", "through"} {
		raw, e := os.ReadFile("testdata/occt_offset_dual_disk_" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		r, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
		if e == nil || r.Body != 0 {
			t.Fatal("rounded vendor graph admitted", e)
		}
	}
}
