package step

import (
	"bytes"
	"context"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func independentMixedRootedSource(t *testing.T) []byte {
	t.Helper()
	var side, planar, cylinder, joined bytes.Buffer
	if e := WriteSideOpenDualDiskProduct(context.Background(), &side, sideDiskDescriptor(t, false)); e != nil {
		t.Fatal(e)
	}
	box, e := brep.Box(geom.Vec3{X: 20}, geom.Vec3{X: 22, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteAdvanced(&planar, Model{LengthUnit: .001, Bodies: []Body{{Name: "planar", Solid: box}}}); e != nil {
		t.Fatal(e)
	}
	frame, _ := geom.NewFrame(geom.Vec3{X: 30}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e = WriteCylinders(&cylinder, CylinderModel{LengthUnit: .001, Bodies: []CylinderBody{{Name: "cylinder", Frame: frame, Radius: 2, Height: 3}}}); e != nil {
		t.Fatal(e)
	}
	if e = JoinIndependentModels(&joined, [][]byte{side.Bytes(), planar.Bytes(), cylinder.Bytes()}, Limits{}); e != nil {
		t.Fatal(e)
	}
	return joined.Bytes()
}
func TestIndependentModelsRetainRootedMixedProducts(t *testing.T) {
	parts, e := SplitIndependentModels(bytes.NewReader(independentMixedRootedSource(t)), Limits{})
	if e != nil || len(parts) != 3 {
		t.Fatal(e, len(parts))
	}
	disk, e := ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(parts[0]), Limits{}, 1e-9)
	if e != nil || disk.Descriptor.Name != "two merged side pockets" {
		t.Fatal(e, disk.Descriptor)
	}
	planar, e := Read(bytes.NewReader(parts[1]), Limits{})
	if e != nil || len(planar.Bodies) != 1 || planar.Bodies[0].Name != "planar" {
		t.Fatal(e)
	}
	cylinder, e := ReadAnalyticBodies(bytes.NewReader(parts[2]), Limits{})
	if e != nil || len(cylinder.Bodies) != 1 || cylinder.Bodies[0].Name != "cylinder" {
		t.Fatal(e)
	}
	for _, p := range parts {
		f, e := Parse(bytes.NewReader(p), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, en := range f.Entities {
			for _, r := range en.Records {
				if r.Type == "PRODUCT" {
					n++
				}
			}
		}
		if n != 1 {
			t.Fatal("product count", n)
		}
	}
}
func TestIndependentModelsMetadataLessAndSharedContexts(t *testing.T) {
	box, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	var b bytes.Buffer
	if e := WriteAdvanced(&b, Model{LengthUnit: .001, Bodies: []Body{{Name: "a", Solid: box}, {Name: "b", Solid: box}}}); e != nil {
		t.Fatal(e)
	}
	parts, e := SplitIndependentModels(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || len(parts) != 2 {
		t.Fatal(e, len(parts))
	}
	for i, p := range parts {
		m, e := Read(bytes.NewReader(p), Limits{})
		if e != nil || len(m.Bodies) != 1 || m.Bodies[0].Name != []string{"a", "b"}[i] {
			t.Fatal(e)
		}
	}
	bare := sideDiskMutateSource(t, b.Bytes(), func(f *File) {
		for id, en := range f.Entities {
			for _, r := range en.Records {
				for _, k := range splitProductKinds {
					if r.Type == k {
						delete(f.Entities, id)
					}
				}
			}
		}
	})
	parts, e = SplitIndependentModels(bytes.NewReader(bare), Limits{})
	if e != nil || len(parts) != 2 {
		t.Fatal(e, len(parts))
	}
	for _, p := range parts {
		if bytes.Contains(p, []byte("PRODUCT(")) {
			t.Fatal("invented metadata")
		}
		if _, e := Read(bytes.NewReader(p), Limits{}); e != nil {
			t.Fatal(e)
		}
	}
}
func TestIndependentModelsRejectCrossRootsAndOrphans(t *testing.T) {
	raw := independentMixedRootedSource(t)
	tests := []struct {
		name string
		edit func(*File)
	}{
		{"two-products-one-representation", func(f *File) {
			var first Value
			for _, id := range f.Order {
				en := f.Entities[id]
				if en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
					r := &en.Records[0]
					if first.Kind == Reference {
						r.Args[1] = first
						return
					}
					first = r.Args[1]
				}
			}
		}},
		{"foreign-shape", func(f *File) {
			var first Value
			for _, id := range f.Order {
				en := f.Entities[id]
				if en.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
					r := &en.Records[0]
					if first.Kind == Aggregate {
						r.Args[1] = first
						return
					}
					first = r.Args[1]
				}
			}
		}},
		{"foreign-unit-geometry", func(f *File) {
			var point int64
			for _, id := range f.Order {
				if f.Entities[id].Records[0].Type == "CARTESIAN_POINT" {
					point = id
					break
				}
			}
			for _, id := range f.Order {
				en := f.Entities[id]
				if en.Records[0].Type == "APPLICATION_CONTEXT" {
					en.Records[0].Args[0] = Value{Kind: Reference, Ref: point}
					return
				}
			}
		}},
		{"incomplete-root", func(f *File) {
			for id, en := range f.Entities {
				if en.Records[0].Type == "APPLICATION_PROTOCOL_DEFINITION" {
					delete(f.Entities, id)
					return
				}
			}
		}},
		{"wrong-protocol", func(f *File) {
			sideDiskEditRecord(f, "APPLICATION_PROTOCOL_DEFINITION", func(r *Record) { r.Args[1].Text = "automotive_design" })
		}},
		{"orphan-product", func(f *File) {
			for id, en := range f.Entities {
				if en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
					delete(f.Entities, id)
					return
				}
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutated := sideDiskMutateSource(t, raw, tt.edit)
			parts, e := SplitIndependentModels(bytes.NewReader(mutated), Limits{})
			if e == nil || parts != nil {
				t.Fatal("accepted or returned partial models", e, len(parts))
			}
		})
	}
}

func TestIndependentModelsRetainSplitCylinderRoots(t *testing.T) {
	frame, _ := geom.NewFrame(geom.Vec3{X: 20}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	d := SplitCylinderDescriptor{Name: "split product", Frame: frame, Radius: 2, Height: 4, LengthUnit: .001}
	var a, b, joined bytes.Buffer
	if e := WriteSplitCylinderProduct(&a, d); e != nil {
		t.Fatal(e)
	}
	if e := WriteQuarterCylinderProduct(&b, d); e != nil {
		t.Fatal(e)
	}
	if e := JoinIndependentModels(&joined, [][]byte{a.Bytes(), b.Bytes()}, Limits{}); e != nil {
		t.Fatal(e)
	}
	parts, e := SplitIndependentModels(&joined, Limits{})
	if e != nil || len(parts) != 2 {
		t.Fatal(e)
	}
	if _, e = ReadSplitCylinderProduct(bytes.NewReader(parts[0]), Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadQuarterCylinderProduct(bytes.NewReader(parts[1]), Limits{}); e != nil {
		t.Fatal(e)
	}
}
