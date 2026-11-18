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

func TestReadUnequalCenterSideOpenDualDiskProduct(t *testing.T) {
	slowtest.Skip(t)
	for _, through := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				for _, unit := range []float64{1, .001} {
					d := unequalCenterSideDiskDescriptor(t, through, swap, posed)
					d.LengthUnit = unit
					var b bytes.Buffer
					if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
						t.Fatal(e)
					}
					r, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, d.Tolerance)
					if e != nil {
						t.Fatal(e)
					}
					if r.Descriptor.Frame.Origin() != d.Frame.Origin() || r.Descriptor.Discs[0].Center.Y != -3 || r.Descriptor.Discs[1].Center.Y != -20 || r.Descriptor.Discs[0].Radius != 5 || r.Descriptor.Discs[1].Radius != 29 || r.Descriptor.LengthUnit != unit {
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
					if e = WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &again, r.Descriptor); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
						t.Fatal("canonical roundtrip", e)
					}
				}
			}
		}
	}
}
func TestReadUnequalCenterSideOpenDualDiskProductEncodingProofs(t *testing.T) {
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, unequalCenterSideDiskDescriptor(t, through, true, true)); e != nil {
			t.Fatal(e)
		}
		raw, _ := sideDiskFlipLines(t, b.Bytes(), "paired")
		raw = sideDiskReverseCircleNormals(t, raw, 1)
		raw, _ = sideDiskReversePlanes(t, raw, "paired")
		if _, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("default widened")
		}
		options := SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}
		got, e := ReadUnequalCenterSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
		if e != nil {
			t.Fatal(e)
		}
		if len(got.ReversedModelLines) == 0 || len(got.RephasedModelCircles) != 4 || len(got.OppositeNormalModelCircles) != 4 || len(got.OppositeNormalPlanes) == 0 {
			t.Fatal("encoding lineage")
		}
		var out bytes.Buffer
		if e = WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
			t.Fatal("normalized ownership changed", e)
		}
	}
}
func TestOCCTReadUnequalCenterSideOpenDualDiskProduct(t *testing.T) {
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
		mode := "unequal-center-side"
		if through {
			mode = "unequal-center-through"
		}
		reference := run(mode, filepath.Join(t.TempDir(), "independent.step"))
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := unequalCenterSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(e)
				}
				parsed, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, d.Tolerance)
				if e != nil {
					t.Fatal(e)
				}
				b.Reset()
				if e = WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, parsed.Descriptor); e != nil {
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
func TestReadUnequalCenterSideOpenDualDiskJoinAndOwnerRejects(t *testing.T) {
	d := unequalCenterSideDiskDescriptor(t, false, false, false)
	var b bytes.Buffer
	if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	edits := []func(*File){
		func(f *File) {
			sideDiskEditRecord(f, "PRODUCT", func(r *Record) { r.Args[0].Text = offsetSideDiskProductID })
		},
		func(f *File) { sideDiskEditRecord(f, "SI_UNIT", func(r *Record) { r.Args[1].Text = "RADIAN" }) },
		func(f *File) { sideDiskEditRecord(f, "CIRCLE", func(r *Record) { r.Args[2].Text = "5.000000000001" }) },
		func(f *File) {
			for _, id := range f.Order {
				r := f.Entities[id].Records[0]
				if r.Type == "CARTESIAN_POINT" {
					p, e := splitTriple(f, id, "CARTESIAN_POINT")
					if e == nil && p.X == 10 && p.Y == 1 && p.Z == 0 {
						r.Args[1].Items[0].Text = "10.000000000001"
						return
					}
				}
			}
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
		func(f *File) {
			var first Value
			for _, id := range f.Order {
				r := f.Entities[id].Records[0]
				if r.Type == "ORIENTED_EDGE" {
					if first.Kind == Reference {
						r.Args[3] = first
						return
					}
					first = r.Args[3]
				}
			}
		},
	}
	for i, edit := range edits {
		raw := sideDiskMutateSource(t, b.Bytes(), edit)
		out, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
		if e == nil || out.Body != 0 || out.Descriptor.Join[0] != nil || out.RadicalJoin[0] != nil {
			t.Fatal("invalid/partial", i, e)
		}
	}
	var old bytes.Buffer
	if e := WriteOffsetSideOpenDualDiskProduct(context.Background(), &old, offsetSideDiskDescriptor(t, false, false, false)); e != nil {
		t.Fatal(e)
	}
	forged := bytes.ReplaceAll(old.Bytes(), []byte(offsetSideDiskProductID), []byte(unequalCenterSideDiskProductID))
	if _, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(forged), Limits{}, 1e-9); e == nil {
		t.Fatal("marker replaces material proof")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadUnequalCenterSideOpenDualDiskProduct(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{MaxEntities: 16}, 1e-9); e == nil {
		t.Fatal("limit")
	}
}
func TestReadUnequalCenterSideOpenDualDiskStockOrigin(t *testing.T) {
	d := unequalCenterSideDiskDescriptor(t, false, true, true)
	d.Min = geom.Vec2{X: 2, Y: 3}
	d.Max = geom.Vec2{X: 62, Y: 13}
	d.Join = [2]*big.Rat{big.NewRat(12, 1), big.NewRat(4, 1)}
	for i := range d.Discs {
		d.Discs[i].Center.X += 2
		d.Discs[i].Center.Y += 3
	}
	var b bytes.Buffer
	if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	out, e := ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	want, e := sideDiskWorld(d.Frame, geom.Vec3{X: 2, Y: 3})
	if e != nil || out.Descriptor.Frame.Origin() != want || out.Descriptor.Discs[0].Center.Y != -3 || out.Descriptor.Discs[1].Center.Y != -20 || out.Descriptor.Join[0].Cmp(big.NewRat(10, 1)) != 0 || out.Descriptor.Join[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("lost local source frame", out.Descriptor, e)
	}
	out.RadicalJoin[0].SetInt64(999)
	if out.Descriptor.Join[0].Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatal("proof aliases descriptor")
	}
}
func TestReadUnequalCenterSideOpenDualDiskVendorRejects(t *testing.T) {
	for _, mode := range []string{"side", "through"} {
		raw, e := os.ReadFile("testdata/occt_unequal_center_dual_disk_" + mode + ".step")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadUnequalCenterSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("vendor rounded trims accepted")
		}
	}
}
