package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestReadUnequalSideOpenDualDiskProduct(t *testing.T) {
	for _, through := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := unequalSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteUnequalSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(e)
				}
				got, e := ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
				if e != nil {
					t.Fatal(through, swap, posed, e)
				}
				if got.Descriptor.Discs[0].Radius != 13 || got.Descriptor.Discs[1].Radius != 15 || got.Descriptor.Name != d.Name || got.Descriptor.LengthUnit != d.LengthUnit || got.Descriptor.Frame != d.Frame || (got.SourceFloorFace == 0) != through {
					t.Fatal("normalized descriptor/lineage", got.Descriptor)
				}
				f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
				if e != nil {
					t.Fatal(e)
				}
				for owner, id := range got.SourceWallFaces {
					face, e := simple(f, id, "ADVANCED_FACE", 4)
					if e != nil {
						t.Fatal(e)
					}
					surface, e := simple(f, face.Args[2].Ref, "CYLINDRICAL_SURFACE", 3)
					if e != nil {
						t.Fatal(e)
					}
					radius, e := cylinderSupportNumber(surface.Args[2])
					if e != nil || radius != got.Descriptor.Discs[owner].Radius {
						t.Fatal("source radius owner", e)
					}
					frame, e := sideDiskReadFrame(f, surface.Args[1].Ref)
					if e != nil {
						t.Fatal(e)
					}
					local, e := sideDiskLocal(got.Descriptor.Frame, frame.Origin())
					if e != nil || local.X != got.Descriptor.Discs[owner].Center.X || local.Y != 0 || local.Z != 0 {
						t.Fatal("source center owner", e, local)
					}
					if id != got.Faces[got.Native.WallFaces[owner][0]] {
						t.Fatal("native/source face lineage")
					}
				}
				var out bytes.Buffer
				if e = WriteUnequalSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
					t.Fatal("canonical roundtrip", e)
				}
			}
		}
	}
}
func TestReadUnequalSideOpenDualDiskProductEncodingProofs(t *testing.T) {
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteUnequalSideOpenDualDiskProduct(context.Background(), &b, unequalSideDiskDescriptor(t, through, true, true)); e != nil {
			t.Fatal(e)
		}
		raw, _ := sideDiskFlipLines(t, b.Bytes(), "paired")
		raw = sideDiskReverseCircleNormals(t, raw, 1)
		raw, _ = sideDiskReversePlanes(t, raw, "paired")
		if _, e := ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("default widened")
		}
		options := SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}
		got, e := ReadUnequalSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
		if e != nil {
			t.Fatal(e)
		}
		if len(got.ReversedModelLines) == 0 || len(got.RephasedModelCircles) != 4 || len(got.OppositeNormalModelCircles) != 4 || len(got.OppositeNormalPlanes) == 0 {
			t.Fatal("encoding lineage")
		}
		var out bytes.Buffer
		if e = WriteUnequalSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
			t.Fatal("normalized ownership changed", e)
		}
	}
}
func TestReadUnequalSideOpenDualDiskProductRejects(t *testing.T) {
	var b bytes.Buffer
	if e := WriteUnequalSideOpenDualDiskProduct(context.Background(), &b, unequalSideDiskDescriptor(t, false, false, false)); e != nil {
		t.Fatal(e)
	}
	options := SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}
	mutations := []struct {
		name string
		edit func(*File)
	}{
		{"cylinder-radius", func(f *File) {
			sideDiskEditRecord(f, "CYLINDRICAL_SURFACE", func(r *Record) { r.Args[2].Text = "12." })
		}},
		{"circle-radius", func(f *File) { sideDiskEditRecord(f, "CIRCLE", func(r *Record) { r.Args[2].Text = "15." }) }},
		{"swapped-radius-owners", func(f *File) {
			for _, en := range f.Entities {
				if len(en.Records) == 1 && en.Records[0].Type == "CYLINDRICAL_SURFACE" {
					r := en.Records[0]
					x, _ := cylinderSupportNumber(r.Args[2])
					if x == 13 {
						r.Args[2].Text = "15."
					} else {
						r.Args[2].Text = "13."
					}
				}
			}
		}},
		{"circle-edge-owner", func(f *File) {
			var first Value
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "EDGE_CURVE" {
					r := en.Records[0]
					if f.Entities[r.Args[3].Ref].Records[0].Type == "CIRCLE" {
						if first.Kind == Reference {
							r.Args[3] = first
							return
						}
						first = r.Args[3]
					}
				}
			}
		}},
		{"units", func(f *File) { sideDiskEditRecord(f, "SI_UNIT", func(r *Record) { r.Args[1].Text = "RADIAN" }) }},
		{"product", func(f *File) { sideDiskEditRecord(f, "PRODUCT", func(r *Record) { r.Args[1].Text = "foreign" }) }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			raw := sideDiskMutateSource(t, b.Bytes(), tt.edit)
			got, e := ReadUnequalSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9, options)
			if e == nil || got.Body != 0 || len(got.Faces) != 0 {
				t.Fatal("invalid/partial result", e)
			}
		})
	}
	var equal bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &equal, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadUnequalSideOpenDualDiskProductWithOptions(context.Background(), &equal, Limits{}, 1e-9, options); e == nil {
		t.Fatal("equal domain accepted")
	}
	raw := sideDiskMutateSource(t, b.Bytes(), func(f *File) { sideDiskEditRecord(f, "SI_UNIT", func(r *Record) { r.Args[0].Text = "CENTI" }) })
	got, e := ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-8)
	if e != nil || got.Descriptor.LengthUnit != .01 || got.Descriptor.Height != 6 || got.Descriptor.Tolerance != 1e-8 {
		t.Fatal("source units", e)
	}
	if _, e = ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{MaxFaces: 2}, 1e-9); e == nil {
		t.Fatal("limits ignored")
	}
}
func TestOCCTReadUnequalSideOpenDualDiskProduct(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	type measure struct {
		Volume, Area float64
		Center       [3]float64
	}
	run := func(mode, path string) measure {
		out, e := exec.Command(oracle, mode, path).CombinedOutput()
		if e != nil {
			t.Fatalf("%v %s", e, out)
		}
		start := bytes.LastIndex(out, []byte("{\"Faces\""))
		if start < 0 {
			t.Fatal(string(out))
		}
		var m measure
		if e = json.Unmarshal(bytes.TrimSpace(out[start:]), &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	for _, through := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteUnequalSideOpenDualDiskProduct(context.Background(), &b, unequalSideDiskDescriptor(t, through, true, true)); e != nil {
			t.Fatal(e)
		}
		got, e := ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		props, e := got.Native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil {
			t.Fatal(e)
		}
		b.Reset()
		if e = WriteUnequalSideOpenDualDiskProduct(context.Background(), &b, got.Descriptor); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "roundtrip.step")
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		occt := run("read", path)
		center := geom.Vec3{X: occt.Center[0], Y: occt.Center[1], Z: occt.Center[2]}
		if math.Abs(props.Volume-occt.Volume) > 1e-4 || math.Abs(props.Area-occt.Area) > 1e-4 || props.Centroid.Sub(center).Length() > 1e-6 {
			t.Fatal("native/OCCT", props, occt)
		}
		mode := "unequal-side"
		if through {
			mode = "unequal-through"
		}
		vendor := filepath.Join(t.TempDir(), "vendor.step")
		run(mode, vendor)
		source, e := os.ReadFile(vendor)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadUnequalSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(source), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}); e == nil {
			t.Fatal("rounded vendor imported")
		}
	}
}
