package step_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// meetingBody is a prism 16 wide and deep and 40 tall with a bore of
// radius big along its height and a bore of radius small across it,
// whose axis passes the first at distance d and makes an angle of the
// given sine with it, all moved by m. The two bores meet along curves
// that are not conics.
func meetingBody(t *testing.T, m geom.Affine, big, small, d, sine float64) boolean.Result {
	t.Helper()
	var base []geom.Vec3
	for _, p := range []geom.Vec2{{X: -8, Y: -8}, {X: 8, Y: -8}, {X: 8, Y: 8}, {X: -8, Y: 8}} {
		base = append(base, m.Apply(geom.Vec3{X: p.X, Y: p.Y, Z: -20}))
	}
	prism, err := brep.Prism(base, m.ApplyVector(geom.Vec3{Z: 40}))
	if err != nil {
		t.Fatal(err)
	}
	body, err := trimmed.FromPlanar(prism, boundaryOptions.Linear)
	if err != nil {
		t.Fatal(err)
	}
	var out boolean.Result
	for _, rod := range []struct {
		centre, axis geom.Vec3
		radius       float64
	}{
		{geom.Vec3{}, geom.Vec3{Z: 1}, big},
		{geom.Vec3{Y: d}, geom.Vec3{X: sine, Z: math.Sqrt(1 - sine*sine)}, small},
	} {
		tool := boundaryCylinder(t, m.Apply(rod.centre.Sub(rod.axis.Scale(30))), m.ApplyVector(rod.axis), rod.radius, 60)
		if out, err = boolean.Combine(context.Background(), body, tool, boolean.Cut, boundaryOptions); err != nil {
			t.Fatal(err)
		}
		body = out.Solid
	}
	return out
}

// meetingTurn is a rigid motion that puts no axis of a test along a
// coordinate axis.
func meetingTurn(t *testing.T) geom.Affine {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 5}, geom.Vec3{X: 2, Y: 3, Z: -6}, geom.Vec3{X: 3, Y: -6, Z: -2})
	if err != nil {
		t.Fatal(err)
	}
	x, y, z, o := frame.XAxis(), frame.YAxis(), frame.ZAxis(), frame.Origin()
	return geom.Affine{{x.X, y.X, z.X, o.X}, {x.Y, y.Y, z.Y, o.Y}, {x.Z, y.Z, z.Z, o.Z}}
}

// TestTrimmedBoundaryMeetingCurves is A11 of specs/general-boolean-quartic:
// a body with edges where two cylinders meet is written with each as a
// spline on an INTERSECTION_CURVE of its two cylinders, and reads back as
// a solid with the same faces, edges and vertices that the general
// Boolean takes and measures to the same volume. What was read writes and
// reads again.
func TestTrimmedBoundaryMeetingCurves(t *testing.T) {
	for _, c := range []struct {
		name                string
		big, small, d, sine float64
	}{
		{"unequal radii, right angle", 5, 2, 0, 1},
		{"unequal radii, skew at a slant", 5, 2, 1, 0.6},
		{"the small bore half out of the large one", 5, 3, 4, 1},
		{"equal radii, axes that pass", 3, 3, 2, 1},
	} {
		for i, m := range []geom.Affine{geom.Identity(), meetingTurn(t)} {
			t.Run(fmt.Sprintf("%s/%d", c.name, i), func(t *testing.T) {
				want := meetingBody(t, m, c.big, c.small, c.d, c.sine)
				solid := want.Solid
				for pass := range 2 {
					var written bytes.Buffer
					if err := step.WriteTrimmedBoundary(&written, solid, .001, "a part"); err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(written.String(), "INTERSECTION_CURVE") {
						t.Fatal("no edge was written as an INTERSECTION_CURVE")
					}
					read, err := step.ReadTrimmedBoundary(bytes.NewReader(written.Bytes()), step.Limits{}, .001, boundaryOptions.Linear)
					if err != nil {
						t.Fatalf("pass %d, %d bytes: %v", pass, written.Len(), err)
					}
					got := read.Bodies[0].Solid
					if len(got.Faces()) != len(solid.Faces()) || len(got.Edges()) != len(solid.Edges()) || len(got.Vertices()) != len(solid.Vertices()) {
						t.Fatalf("pass %d: read %d faces, %d edges and %d vertices of %d, %d and %d", pass,
							len(got.Faces()), len(got.Edges()), len(got.Vertices()), len(solid.Faces()), len(solid.Edges()), len(solid.Vertices()))
					}
					r, err := boolean.Combine(context.Background(), got, trimmed.Solid{}, boolean.Union, boundaryOptions)
					if err != nil {
						t.Fatalf("pass %d: %v", pass, err)
					}
					if off := math.Abs(r.Volume - want.Volume); off > 1e-9*16*16*40 {
						t.Fatalf("pass %d: volume %.12g read back as %.12g", pass, want.Volume, r.Volume)
					}
					solid = got
				}
			})
		}
	}
}

// oddCurve is a curve of a kind the writer does not know.
type oddCurve struct{ geom.Curve }

// TestTrimmedBoundaryMeetingRefusals is A12: the reader refuses a spline
// that is not where its two cylinders meet and an intersection curve that
// does not name two surfaces, and the writer refuses an edge of an
// unknown kind that is not between two cylinders.
func TestTrimmedBoundaryMeetingRefusals(t *testing.T) {
	body := meetingBody(t, geom.Identity(), 5, 2, 0, 1).Solid
	var written bytes.Buffer
	if err := step.WriteTrimmedBoundary(&written, body, .001, "a part"); err != nil {
		t.Fatal(err)
	}
	file := written.String()
	// Every pole of one spline, moved a thousand times the tolerance.
	spline := regexp.MustCompile(`B_SPLINE_CURVE_WITH_KNOTS\('',3,\(([#\d,]+)\)`).FindStringSubmatch(file)
	if spline == nil {
		t.Fatal("no spline in the file")
	}
	moved := file
	for _, id := range strings.Split(spline[1], ",") {
		pole := regexp.MustCompile(`(?m)^(` + id + `=CARTESIAN_POINT\('',\()([^,]+),`)
		moved = pole.ReplaceAllStringFunc(moved, func(line string) string {
			parts := pole.FindStringSubmatch(line)
			var x float64
			if _, err := fmt.Sscanf(parts[2], "%g", &x); err != nil {
				t.Fatal(err)
			}
			return fmt.Sprintf("%s%.17g,", parts[1], x+1e-4)
		})
	}
	if moved == file {
		t.Fatal("no pole was moved")
	}
	if _, err := step.ReadTrimmedBoundary(strings.NewReader(moved), step.Limits{}, .001, boundaryOptions.Linear); !errors.Is(err, geom.ErrInvalid) {
		t.Errorf("a spline off the curve: %v", err)
	}
	one := regexp.MustCompile(`(INTERSECTION_CURVE\('',#\d+,\(#\d+),#\d+\)`).ReplaceAllString(file, "${1})")
	if one == file {
		t.Fatal("no intersection curve was edited")
	}
	if _, err := step.ReadTrimmedBoundary(strings.NewReader(one), step.Limits{}, .001, boundaryOptions.Linear); !errors.Is(err, step.ErrTrimmedBoundary) {
		t.Errorf("an intersection curve on one surface: %v", err)
	}

	block := boundaryBox(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	edges := append([]parametric.Edge{}, block.Edges()...)
	edges[0].Curve = oddCurve{edges[0].Curve}
	odd, err := trimmed.NewSolid(block.Vertices(), edges, block.Faces(), block.Tolerance())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := step.WriteTrimmedBoundary(&out, odd, .001, "a part"); !errors.Is(err, step.ErrTrimmedBoundary) || out.Len() != 0 {
		t.Errorf("an unknown curve between planes: %v, %d bytes", err, out.Len())
	}
}
