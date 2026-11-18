package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func TestReadSideOpenDualDiskProduct(t *testing.T) {
	for _, through := range []bool{false, true} {
		d := sideDiskDescriptor(t, through)
		var b bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
			t.Fatal(e)
		}
		got, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, d.Tolerance)
		if e != nil {
			t.Fatal(through, e)
		}
		var out bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &out, got.Descriptor); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(b.Bytes(), out.Bytes()) {
			t.Fatal("roundtrip differs")
		}
	}
}

func sideDiskMutateSource(t testing.TB, raw []byte, edit func(*File)) []byte {
	t.Helper()
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	edit(f)
	identity := map[int64]int64{}
	for id := range f.Entities {
		identity[id] = id
	}
	var b bytes.Buffer
	b.Write(raw[:bytes.Index(raw, []byte("DATA;"))+5])
	b.WriteByte('\n')
	for _, id := range f.Order {
		if en, ok := f.Entities[id]; ok {
			fmt.Fprintf(&b, "#%d=%s;\n", id, assemblyEntity(en, identity))
		}
	}
	b.WriteString("ENDSEC;END-ISO-10303-21;\n")
	return b.Bytes()
}
func sideDiskEditRecord(f *File, kind string, edit func(*Record)) bool {
	for _, id := range f.Order {
		en := f.Entities[id]
		for i := range en.Records {
			if en.Records[i].Type == kind {
				edit(&en.Records[i])
				return true
			}
		}
	}
	return false
}
func TestReadSideOpenDualDiskProductRejectsTampering(t *testing.T) {
	slowtest.Skip(t)
	var b bytes.Buffer
	d := sideDiskDescriptor(t, false)
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	flip := func(v *Value) {
		if v.Text == "T" {
			v.Text = "F"
		} else {
			v.Text = "T"
		}
	}
	tests := []struct {
		name, kind string
		edit       func(*Record)
	}{
		{"body-name", "MANIFOLD_SOLID_BREP", func(r *Record) { r.Args[0].Text = "forged" }},
		{"product-name", "PRODUCT", func(r *Record) { r.Args[1].Text = "forged" }},
		{"radius", "CYLINDRICAL_SURFACE", func(r *Record) { r.Args[2].Text = "2.25" }},
		{"circle-radius", "CIRCLE", func(r *Record) { r.Args[2].Text = "2.25" }},
		{"vertex-position", "CARTESIAN_POINT", func(r *Record) { r.Args[1].Items[0].Text = "123." }},
		{"direction", "DIRECTION", func(r *Record) { r.Args[1].Items[0].Text = "0.5" }},
		{"edge-sense", "EDGE_CURVE", func(r *Record) { flip(&r.Args[4]) }},
		{"coedge-sense", "ORIENTED_EDGE", func(r *Record) { flip(&r.Args[4]) }},
		{"face-sense", "ADVANCED_FACE", func(r *Record) { flip(&r.Args[3]) }},
		{"bound-sense", "FACE_OUTER_BOUND", func(r *Record) { flip(&r.Args[2]) }},
		{"edge-sharing", "EDGE_CURVE", func(r *Record) { r.Args[2] = r.Args[1] }},
		{"protocol", "APPLICATION_PROTOCOL_DEFINITION", func(r *Record) { r.Args[2].Text = "1995" }},
		{"units", "SI_UNIT", func(r *Record) { r.Args[1].Text = "RADIAN" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := sideDiskMutateSource(t, b.Bytes(), func(f *File) {
				if !sideDiskEditRecord(f, tt.kind, tt.edit) {
					t.Fatal("mutation missing")
				}
			})
			got, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
			if e == nil || got.Body != 0 || len(got.Faces) != 0 {
				t.Fatalf("accepted body %d, error %v", got.Body, e)
			}
		})
	}
	for _, kind := range []string{"SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION"} {
		t.Run("missing-"+kind, func(t *testing.T) {
			raw := sideDiskMutateSource(t, b.Bytes(), func(f *File) {
				for id, en := range f.Entities {
					if en.Records[0].Type == kind {
						delete(f.Entities, id)
					}
				}
			})
			if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
				t.Fatal("missing root accepted")
			}
		})
	}
	extra := bytes.Replace(b.Bytes(), []byte("ENDSEC;\nEND-ISO"), []byte("#999999=CARTESIAN_POINT('',(0.,0.,0.));\nENDSEC;\nEND-ISO"), 1)
	if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(extra), Limits{}, 1e-9); e == nil {
		t.Fatal("detached geometry")
	}
	wrongSchema := bytes.Replace(b.Bytes(), []byte("CONFIG_CONTROL_DESIGN"), []byte("AUTOMOTIVE_DESIGN"), 1)
	if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(wrongSchema), Limits{}, 1e-9); e == nil {
		t.Fatal("schema")
	}
	vendor, e := os.ReadFile("testdata/overlapping-side-pockets.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(vendor), Limits{}, 1e-9); e == nil {
		t.Fatal("rounded pcurve source accepted")
	}
}
func TestReadSideOpenDualDiskProductPosesAndReordering(t *testing.T) {
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
			if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
				t.Fatal(e)
			}
			f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mapping := map[int64]int64{}
			for id := range f.Entities {
				mapping[id] = id + 1000
			}
			var renumbered bytes.Buffer
			renumbered.Write(b.Bytes()[:bytes.Index(b.Bytes(), []byte("DATA;"))+5])
			renumbered.WriteByte('\n')
			for i := len(f.Order) - 1; i >= 0; i-- {
				id := f.Order[i]
				fmt.Fprintf(&renumbered, "#%d=%s;\n", mapping[id], assemblyEntity(f.Entities[id], mapping))
			}
			renumbered.WriteString("ENDSEC;END-ISO-10303-21;\n")
			raw := bytes.ReplaceAll(renumbered.Bytes(), []byte(",2.5)"), []byte(",25.E-1)"))
			got, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
			if e != nil {
				t.Fatal(through, pose, e)
			}
			if got.Descriptor.Min != (geom.Vec2{}) || got.Descriptor.Max != (geom.Vec2{X: 10, Y: 6}) || got.Descriptor.Depth != d.Depth || got.Descriptor.Name != d.Name || got.Descriptor.LengthUnit != d.LengthUnit || got.Descriptor.Frame.XAxis() != d.Frame.XAxis() || got.Descriptor.Frame.ZAxis() != d.Frame.ZAxis() {
				t.Fatal("descriptor", got.Descriptor)
			}
			if (got.Native.FloorFace < 0) != through || len(got.Native.WallFaces[0]) != 1 || len(got.Native.WallFaces[1]) != 1 {
				t.Fatal("native roles")
			}
		}
	}
}
func TestReadSideOpenDualDiskProductLimits(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadSideOpenDualDiskProduct(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: 10}, {MaxFaces: 2}, {MaxVertices: 3}, {MaxEdgeUses: 4}} {
		if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), l, 1e-9); e == nil {
			t.Fatal("limit", l)
		}
	}
	if _, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 0); e == nil {
		t.Fatal("tolerance")
	}
	if _, e := ReadSideOpenDualDiskProduct(context.Background(), strings.NewReader(strings.Repeat(" ", 1<<20+1)), Limits{}, 1e-9); e == nil {
		t.Fatal("byte bound")
	}
}

func TestOCCTReadSideOpenDualDiskProduct(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		d := sideDiskDescriptor(t, through)
		var b bytes.Buffer
		if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
			t.Fatal(e)
		}
		got, e := ReadSideOpenDualDiskProduct(context.Background(), &b, Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		props, e := got.Native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil {
			t.Fatal(e)
		}
		b.Reset()
		if e = WriteSideOpenDualDiskProduct(context.Background(), &b, got.Descriptor); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "reconstructed.step")
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
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
		var want struct {
			Volume, Area float64
			Center       [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(out[start:]), &want); e != nil {
			t.Fatal(e)
		}
		center := geom.Vec3{X: want.Center[0], Y: want.Center[1], Z: want.Center[2]}
		if math.Abs(props.Volume-want.Volume) > 3e-7 || math.Abs(props.Area-want.Area) > 3e-7 || props.Centroid.Sub(center).Length() > 3e-8 {
			t.Fatal(through, props, want)
		}
	}
}

func TestReadSideOpenDualDiskProductPreservesSourceUnits(t *testing.T) {
	var b bytes.Buffer
	d := sideDiskDescriptor(t, true)
	if e := WriteSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	raw := sideDiskMutateSource(t, b.Bytes(), func(f *File) {
		if !sideDiskEditRecord(f, "SI_UNIT", func(r *Record) { r.Args[0].Text = "CENTI" }) {
			t.Fatal("missing unit")
		}
	})
	got, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	if got.Descriptor.LengthUnit != .01 || got.Descriptor.Height != 6 || got.Descriptor.Tolerance != 1e-8 {
		t.Fatal("source units scaled or tolerance inferred", got.Descriptor)
	}
}
