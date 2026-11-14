package step

import (
	"bytes"
	"regexp"
	"testing"

	"plainsolid/geom"
)

func TestObliqueBoreSTEP(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		axisPoint, direction geom.Vec3
		offset               geom.Vec3
		entry, exit          int
		unit                 float64
	}{
		{"opposite", geom.Vec3{Y: 2, Z: -3}, geom.Vec3{X: 1, Z: 3}, geom.Vec3{}, 0, 1, .001},
		{"adjacent", geom.Vec3{X: -1, Y: 2, Z: -1}, geom.Vec3{X: 2, Z: 1}, geom.Vec3{}, 0, 3, .001},
		{"oblique all axes", geom.Vec3{X: 1, Y: 1, Z: -3}, geom.Vec3{X: .71, Y: .33, Z: 3.14159}, geom.Vec3{}, 0, 1, .001},
		{"translated reversed adjacent", geom.Vec3{X: 7, Y: 2, Z: 3}, geom.Vec3{X: -2, Z: -1}, geom.Vec3{X: 11, Y: -37, Z: 35}, 3, 0, .01},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := ObliqueBoreModel{LengthUnit: tc.unit, Body: ObliqueBoreBody{
				Name: "Bore", Stock: geom.Box{Min: tc.offset, Max: tc.offset.Add(geom.Vec3{X: 4, Y: 5, Z: 6})}, Entry: tc.entry, Exit: tc.exit,
				AxisPoint: tc.axisPoint.Add(tc.offset), Direction: tc.direction, Radius: .35,
			}}
			var data bytes.Buffer
			if err := WriteObliqueBore(&data, model); err != nil {
				t.Fatal(err)
			}
			got, err := ReadObliqueBore(bytes.NewReader(data.Bytes()), Limits{})
			if err != nil || got.LengthUnit != model.LengthUnit || got.Body.Stock != model.Body.Stock || got.Body.Entry != model.Body.Entry || got.Body.Exit != model.Body.Exit || got.Body.Radius != model.Body.Radius {
				t.Fatalf("read bore %v: %v", got, err)
			}
			if got := bytes.Count(data.Bytes(), []byte("=ELLIPSE(")); got != 8 {
				t.Fatalf("got %d ellipse segments", got)
			}
			if bytes.Contains(data.Bytes(), []byte("B_SPLINE")) {
				t.Fatal("ellipse emitted as an approximation")
			}
			var second bytes.Buffer
			if err := WriteObliqueBore(&second, got); err != nil || !bytes.Equal(data.Bytes(), second.Bytes()) {
				t.Fatalf("canonical re-export changed the graph: %v", err)
			}
			ellipse := regexp.MustCompile(`(ELLIPSE\('',#\d+,[^,]+,)[^)]+\)`)
			trim := ellipse.FindSubmatch(data.Bytes())
			changed := bytes.Replace(data.Bytes(), trim[0], append(append([]byte{}, trim[1]...), "0.34)"...), 1)
			if _, err := ReadObliqueBore(bytes.NewReader(changed), Limits{}); err == nil {
				t.Fatal("changed elliptical trim passed the graph proof")
			}

		})
	}
}

func TestObliqueBoreSTEPRejectsGrazingOrPerpendicular(t *testing.T) {
	stock := geom.Box{Max: geom.Vec3{X: 4, Y: 5, Z: 6}}
	for _, body := range []ObliqueBoreBody{
		{Stock: stock, Entry: 0, Exit: 3, AxisPoint: geom.Vec3{X: 1, Y: 2}, Direction: geom.Vec3{X: 1, Z: 1e-6}, Radius: .35},
		{Stock: stock, Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 1, Y: 2}, Direction: geom.Vec3{Z: 1}, Radius: .35},
		{Stock: stock, Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 1e300, Y: 2, Z: -1e300}, Direction: geom.Vec3{X: 1, Z: 1e-10}, Radius: .35},
	} {
		var out bytes.Buffer
		err := WriteObliqueBore(&out, ObliqueBoreModel{Body: body, LengthUnit: .001})
		if err == nil || out.Len() != 0 {
			t.Fatal("invalid oblique recipe wrote a partial document")
		}
	}
}
