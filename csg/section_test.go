package csg

import (
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func cuttingPlane(t *testing.T, origin, normal geom.Vec3) geom.Plane {
	t.Helper()
	p, e := geom.NewPlane(origin, normal)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSectionAndSplitDimensions(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	cases := []struct {
		name                     string
		origin, normal           geom.Vec3
		kind                     SectionKind
		points                   int
		area, negative, positive float64
	}{
		{"proper square", geom.Vec3{X: 0.25}, geom.Vec3{X: 1}, SectionArea, 4, 1, 0.25, 0.75},
		{"face tangent", geom.Vec3{}, geom.Vec3{X: 1}, SectionArea, 4, 1, 0, 1},
		{"opposite face tangent", geom.Vec3{X: 1}, geom.Vec3{X: 1}, SectionArea, 4, 1, 1, 0},
		{"edge tangent", geom.Vec3{}, geom.Vec3{X: 1, Y: 1}, SectionSegment, 2, 0, 0, 1},
		{"vertex tangent", geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1}, SectionPoint, 1, 0, 0, 1},
		{"outside positive", geom.Vec3{X: -1}, geom.Vec3{X: 1}, SectionEmpty, 0, 0, 0, 1},
		{"outside negative", geom.Vec3{X: 2}, geom.Vec3{X: 1}, SectionEmpty, 0, 0, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plane := cuttingPlane(t, tc.origin, tc.normal)
			section, e := Section(s, plane, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			if section.Kind != tc.kind || len(section.Points) != tc.points {
				t.Fatal(section)
			}
			near(t, section.Area, tc.area)
			result, e := Split(s, plane, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			if result.Section.Kind != section.Kind {
				t.Fatal(result.Section)
			}
			near(t, result.Negative.Tessellate().SignedVolume(), tc.negative)
			near(t, result.Positive.Tessellate().SignedVolume(), tc.positive)
			if e = result.Negative.Validate(); e != nil {
				t.Fatal(e)
			}
			if e = result.Positive.Validate(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestDiagonalSectionAndOrientation(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	origin := geom.Vec3{X: 0.5, Y: 0.5, Z: 0.5}
	for _, normal := range []geom.Vec3{{X: 1, Y: 1, Z: 1}, {X: -1, Y: -1, Z: -1}} {
		plane := cuttingPlane(t, origin, normal)
		section, e := Section(s, plane, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if section.Kind != SectionArea || len(section.Points) != 6 {
			t.Fatal(section)
		}
		near(t, section.Area, 3*math.Sqrt(3)/4)
		cross := geom.Vec3{}
		for i, p := range section.Points {
			q := section.Points[(i+1)%len(section.Points)]
			cross = cross.Add(p.Sub(origin).Cross(q.Sub(origin)))
			if math.Abs(plane.SignedDistance(p)) > 1e-12 {
				t.Fatal("section point off plane", p)
			}
		}
		if cross.Dot(normal) <= 0 {
			t.Fatal("section winding does not follow plane normal")
		}
		result, e := Split(s, plane, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, result.Negative.Tessellate().SignedVolume(), 0.5)
		near(t, result.Positive.Tessellate().SignedVolume(), 0.5)
	}
}
func TestSectionSubdividedFacesAndPlaneOrigin(t *testing.T) {
	cube := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	m := cube.Tessellate()
	s := solidFromTriangles(t, m.Vertices, m.Triangles)
	plane := cuttingPlane(t, geom.Vec3{X: 0.5, Y: 1e200, Z: -1e200}, geom.Vec3{X: 1})
	section, e := Section(s, plane, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if section.Kind != SectionArea || len(section.Points) != 4 {
		t.Fatal("section hull retained coplanar subdivisions", section)
	}
	near(t, section.Area, 1)
	split, e := Split(s, plane, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, split.Negative.Tessellate().SignedVolume(), 0.5)
	near(t, split.Positive.Tessellate().SignedVolume(), 0.5)
}
func TestSectionExactNearTangencyAndThinSplit(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	outside := cuttingPlane(t, geom.Vec3{X: -math.SmallestNonzeroFloat64}, geom.Vec3{X: 1})
	section, e := Section(s, outside, 1e-9)
	if e != nil || section.Kind != SectionEmpty {
		t.Fatal("near miss was snapped to tangent", section, e)
	}
	inside := cuttingPlane(t, geom.Vec3{X: math.SmallestNonzeroFloat64}, geom.Vec3{X: 1})
	section, e = Section(s, inside, 1e-9)
	if e != nil || section.Kind != SectionArea {
		t.Fatal(section, e)
	}
	if _, e = Split(s, inside, 1e-9); !errors.Is(e, ErrUnsupported) {
		t.Fatal("thin part should fail explicitly instead of becoming tangent", e)
	}
}
func TestSplitRotationAndReversedNormal(t *testing.T) {
	cube := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	rotation, e := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7)
	if e != nil {
		t.Fatal(e)
	}
	transform := geom.Translation(geom.Vec3{X: 2, Y: -3, Z: 4}).Mul(rotation)
	s, e := cube.Transformed(transform)
	if e != nil {
		t.Fatal(e)
	}
	for _, direction := range []float64{1, -1} {
		normal, e := transform.ApplyNormal(geom.Vec3{X: direction})
		if e != nil {
			t.Fatal(e)
		}
		plane := cuttingPlane(t, transform.Apply(geom.Vec3{X: 0.25}), normal)
		result, e := Split(s, plane, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		negative, positive := 0.25, 0.75
		if direction < 0 {
			negative, positive = positive, negative
		}
		near(t, result.Negative.Tessellate().SignedVolume(), negative)
		near(t, result.Positive.Tessellate().SignedVolume(), positive)
		near(t, result.Section.Area, 1)
	}
}
func TestSectionErrorsAndEmpty(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	plane := cuttingPlane(t, geom.Vec3{}, geom.Vec3{X: 1})
	if _, e := Section(s, geom.Plane{}, 1e-9); e == nil {
		t.Fatal("invalid plane")
	}
	if _, e := Split(s, geom.Plane{}, 1e-9); e == nil {
		t.Fatal("invalid plane")
	}
	if _, e := Section(s, plane, 0); e == nil {
		t.Fatal("invalid tolerance")
	}
	if _, e := Split(s.Reversed(), plane, 1e-9); e == nil {
		t.Fatal("inward operand")
	}
	section, e := Section(brep.Solid{}, plane, 1e-9)
	if e != nil || section.Kind != SectionEmpty {
		t.Fatal(section, e)
	}
	split, e := Split(brep.Solid{}, plane, 1e-9)
	if e != nil || len(split.Positive.Vertices()) != 0 || len(split.Negative.Vertices()) != 0 {
		t.Fatal(split, e)
	}
	huge, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1e150, Y: 1e150, Z: 1e150})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Section(huge, plane, 1e-9); e == nil {
		t.Fatal("nonfinite source volume must not bypass convex orientation validation")
	}
}
func FuzzAxisSplitConservation(f *testing.F) {
	f.Add(0.25)
	f.Fuzz(func(t *testing.T, x float64) {
		if math.IsNaN(x) || x < 0.01 || x > 0.99 {
			return
		}
		s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
		cut := cuttingPlane(t, geom.Vec3{X: x}, geom.Vec3{X: 1})
		result, e := Split(s, cut, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, result.Negative.Tessellate().SignedVolume(), x)
		near(t, result.Positive.Tessellate().SignedVolume(), 1-x)
		near(t, result.Section.Area, 1)
	})
}

func TestProperCutsThroughOriginalEdgesAndVertices(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	for _, tc := range []struct {
		normal         geom.Vec3
		points         int
		negative, area float64
	}{
		{geom.Vec3{X: 1, Y: 1}, 4, 0.5, math.Sqrt(2)},
		{geom.Vec3{X: 1, Y: 1, Z: 1}, 3, 1.0 / 6, math.Sqrt(3) / 2},
	} {
		plane := cuttingPlane(t, geom.Vec3{X: 1}, tc.normal)
		result, e := Split(s, plane, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if len(result.Section.Points) != tc.points {
			t.Fatal(result.Section)
		}
		near(t, result.Section.Area, tc.area)
		near(t, result.Negative.Tessellate().SignedVolume(), tc.negative)
		near(t, result.Positive.Tessellate().SignedVolume(), 1-tc.negative)
	}
}
func TestSplitPlaneBudget(t *testing.T) {
	n := MaxPlanes - 2
	polygon := make([]geom.Vec3, n)
	for i := range polygon {
		a := 2 * math.Pi * float64(i) / float64(n)
		polygon[i] = geom.Vec3{X: math.Cos(a), Y: math.Sin(a)}
	}
	s, e := brep.Prism(polygon, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	plane := cuttingPlane(t, geom.Vec3{Z: 0.5}, geom.Vec3{Z: 1})
	section, e := Section(s, plane, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(section.Points) != n {
		t.Fatal(section)
	}
	near(t, section.Area, float64(n)*math.Sin(2*math.Pi/float64(n))/2)
	if _, e = Split(s, plane, 1e-9); e == nil {
		t.Fatal("split ignored supporting-plane budget")
	}
}
