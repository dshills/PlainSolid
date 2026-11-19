package step

import (
	"bytes"
	"errors"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedFaceRejectsSourceWeightRounding(t *testing.T) {
	t.Parallel()
	// Both support and edge curves would otherwise normalize 1/3 identically,
	// making comparison of decoded curves conceal a source alteration.
	g, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 2}, {X: 2}, {X: 2, Y: 2}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 3, 3})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSplineFaces(&b, SplineFaceModel{LengthUnit: 1, Faces: []RectangularSplineFace{{Name: "weights", Geometry: g}}}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var fid int64
	changed := 0
	for id, entity := range f.Entities {
		if _, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
			fid = id
		}
		for i, record := range entity.Records {
			if record.Type != "RATIONAL_B_SPLINE_SURFACE" && record.Type != "RATIONAL_B_SPLINE_CURVE" {
				continue
			}
			var replace func(*Value)
			replace = func(v *Value) {
				if v.Kind == Aggregate {
					for j := range v.Items {
						replace(&v.Items[j])
					}
					return
				}
				x, e := numeric(*v)
				if e != nil {
					t.Fatal(e)
				}
				if x == 1 {
					v.Text = "3."
				} else {
					v.Text = "1."
					changed++
				}
			}
			replace(&record.Args[0])
			entity.Records[i] = record
		}
		f.Entities[id] = entity
	}
	if changed == 0 || fid == 0 {
		t.Fatal("no rational graph mutated")
	}
	if _, e = decodeBoundedSplineFace(f, fid, 100); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatalf("want unrepresentable, got %v", e)
	}
	// Generic spline import remains available with its ordinary float semantics.
	for id := range f.Entities {
		if _, ok := component(f.Entities[id], "B_SPLINE_SURFACE"); ok {
			if _, e := DecodeBSplineSurface(f, id, 100); e != nil {
				t.Fatal(e)
			}
		}
	}

	// Normalizing only the support leaves the model-edge source mismatch.
	for _, kind := range []string{"RATIONAL_B_SPLINE_SURFACE", "RATIONAL_B_SPLINE_CURVE"} {
		for id, entity := range f.Entities {
			for i, record := range entity.Records {
				if record.Type != kind {
					continue
				}
				var normalize func(*Value)
				normalize = func(v *Value) {
					if v.Kind == Aggregate {
						for j := range v.Items {
							normalize(&v.Items[j])
						}
						return
					}
					x, _ := numeric(*v)
					if x == 3 {
						v.Text = "1."
					} else {
						v.Text = "0.3333333333333333"
					}
				}
				normalize(&record.Args[0])
				entity.Records[i] = record
			}
			f.Entities[id] = entity
		}
		_, err := decodeBoundedSplineFace(f, fid, 100)
		if kind == "RATIONAL_B_SPLINE_SURFACE" && !errors.Is(err, geom.ErrUnrepresentable) {
			t.Fatal(err)
		}
		if kind == "RATIONAL_B_SPLINE_CURVE" && err != nil {
			t.Fatalf("normalized graph should certify: %v", err)
		}
	}

}
