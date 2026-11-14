package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"math"
	"os"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestTorusSTEPBridge(t *testing.T) {
	for name, angle := range map[string]float64{"ring": 2 * math.Pi, "quarter": math.Pi / 2, "negativeMajor": -4 * math.Pi / 3} {
		t.Run(name, func(t *testing.T) {
			b := New()
			body := shape(t, b, map[string]any{
				"op": "revolveCircle", "origin": vector{5, -5, 0}, "direction": vector{0, 1, 0},
				"center": vector{10, 0, 0}, "normal": vector{0, 0, 1}, "radius": 2, "angle": angle,
			})
			placed := shape(t, b, map[string]any{
				"op": "transform", "handle": body.Handle,
				"matrix": geom.Affine{{-1, 0, 0, 11}, {0, 0, 1, -3}, {0, -1, 0, 9}},
			})
			for _, original := range []Shape{body, placed} {
				file, err := b.exportSTEP([]string{original.Handle})
				if err != nil {
					t.Fatal(err)
				}
				raw, err := base64.StdEncoding.DecodeString(file.Base64)
				if err != nil || !bytes.Contains(raw, []byte("TOROIDAL_SURFACE")) {
					t.Fatal(err)
				}
				got, err := b.importSTEP(string(raw))
				if err != nil {
					t.Fatal(err)
				}
				near(t, got.GeometryAssertions.Volume, 20*math.Pi*math.Abs(angle))
				near(t, got.GeometryAssertions.SurfaceArea, original.GeometryAssertions.SurfaceArea)
				for i := range got.Bounds.Min {
					near(t, got.Bounds.Min[i], original.Bounds.Min[i])
					near(t, got.Bounds.Max[i], original.Bounds.Max[i])
				}
				tess, err := b.shapes[got.Handle].tessellate(request{})
				if err != nil {
					t.Fatal(err)
				}
				if err := tess.ValidateSolid(); err != nil {
					t.Fatal(err)
				}
				again, err := b.exportSTEP([]string{got.Handle})
				if err != nil || again.Base64 != file.Base64 {
					t.Fatal("canonical re-export", err)
				}
			}
		})
	}
}

func TestTorusSTEPBridgeIndependentAndUnits(t *testing.T) {
	b := New()
	raw, err := os.ReadFile("../../exchange/step/testdata/torus-elbow.step")
	if err != nil {
		t.Fatal(err)
	}
	independent, err := b.importSTEP(strings.Replace(string(raw), "TOROIDAL_SURFACE", "toroidal_surface", 1))
	if err != nil {
		t.Fatal(err)
	}
	near(t, independent.GeometryAssertions.Volume, 1.5*math.Pi*math.Pi)
	if _, err := b.exportSTEP([]string{independent.Handle}); err != nil {
		t.Fatal(err)
	}
	f, err := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	p := step.TorusElbowBody{Frame: f, MajorRadius: 3, MinorRadius: 1, Angle: math.Pi / 2}
	if err := step.WriteTorusElbows(&source, step.TorusElbowModel{Bodies: []step.TorusElbowBody{p}, LengthUnit: .01}); err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(source.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 1500*math.Pi*math.Pi)
	want := Bounds{Min: vector{10, 20, 20}, Max: vector{50, 60, 40}}
	for i := range want.Min {
		near(t, got.Bounds.Min[i], want.Min[i])
		near(t, got.Bounds.Max[i], want.Max[i])
	}
	if _, err := b.exportSTEP([]string{got.Handle}); err != nil {
		t.Fatal(err)
	}
}
