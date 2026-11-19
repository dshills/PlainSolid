package step_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/internal/stepmeasure"
)

var boundaryOptions = boolean.Options{
	Linear: 1e-7, Angular: 1e-9, Volume: 1e-7,
	Integration: parametric.IntegrationOptions{RelativeTolerance: 1e-9, MaxSamples: 64000000},
}

func boundaryBox(t *testing.T, min, max geom.Vec3) trimmed.Solid {
	t.Helper()
	b, err := brep.Box(min, max)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromPlanar(b, boundaryOptions.Linear)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// boundaryCylinder is a cylinder of the given radius from a point, along a
// direction, for a height.
func boundaryCylinder(t *testing.T, from, along geom.Vec3, radius, height float64) trimmed.Solid {
	t.Helper()
	frame, err := geom.FrameFromNormal(from, along)
	if err != nil {
		t.Fatal(err)
	}
	c, err := parametric.Cylinder(frame, radius, height)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromParametric(c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// boundaryRotation turns by an angle about an axis through the origin and
// then moves by an offset.
func boundaryRotation(axis geom.Vec3, angle float64, offset geom.Vec3) geom.Affine {
	n, _ := axis.Unit()
	s, c := math.Sincos(angle)
	k := 1 - c
	return geom.Affine{
		{c + n.X*n.X*k, n.X*n.Y*k - n.Z*s, n.X*n.Z*k + n.Y*s, offset.X},
		{n.Y*n.X*k + n.Z*s, c + n.Y*n.Y*k, n.Y*n.Z*k - n.X*s, offset.Y},
		{n.Z*n.X*k - n.Y*s, n.Z*n.Y*k + n.X*s, c + n.Z*n.Z*k, offset.Z},
	}
}

// boundaryResult is a result of the general Boolean, placed or not, with
// the measures the Boolean reports for it in closed form.
type boundaryResult struct {
	name  string
	solid trimmed.Solid
	want  parametric.Properties
}

// boundaryResults are the solids the writer and the reader are tested on.
// They differ in what they put in a boundary, not in what the writer or
// the reader does with it: a seam, loops of arcs on planes and on
// cylinders, ellipses, a wall with its normal towards the axis and one
// with it away, and surfaces and curves under a placement that turns or
// mirrors.
func boundaryResults(t *testing.T) []boundaryResult {
	t.Helper()
	up := geom.Vec3{Z: 1}
	turn := boundaryRotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7, geom.Vec3{X: 5, Y: -3, Z: 2})
	mirror := geom.Affine{{-1, 0, 0, 10}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	oblique := geom.Vec3{X: 0.3, Z: 1}
	var out []boundaryResult
	for _, c := range []struct {
		name  string
		tool  trimmed.Solid
		op    boolean.Operation
		place *geom.Affine
	}{
		{"planes only", boundaryBox(t, geom.Vec3{X: 2, Y: 1, Z: 1}, geom.Vec3{X: 6, Y: 3, Z: 3}), boolean.Cut, nil},
		{"through bore", boundaryCylinder(t, geom.Vec3{X: 2, Y: 2, Z: -1}, up, 1, 6), boolean.Cut, nil},
		{"blind bore", boundaryCylinder(t, geom.Vec3{X: 2, Y: 2, Z: 2}, up, 1, 6), boolean.Cut, nil},
		{"boss", boundaryCylinder(t, geom.Vec3{X: 2, Y: 2, Z: 2}, up, 1, 6), boolean.Union, nil},
		{"cylinder cut by planes", boundaryCylinder(t, geom.Vec3{X: 2, Y: 2, Z: -1}, up, 1, 6), boolean.Common, nil},
		{"bore that leaves through a side", boundaryCylinder(t, geom.Vec3{X: 3.5, Y: 2, Z: -1}, up, 1, 6), boolean.Cut, nil},
		{"oblique bore", boundaryCylinder(t, geom.Vec3{X: 1, Y: 2, Z: -3}, oblique, 0.7, 12), boolean.Cut, nil},
		{"oblique boss", boundaryCylinder(t, geom.Vec3{X: 1, Y: 2, Z: 1}, oblique, 0.7, 6), boolean.Union, nil},
		{"through bore, turned", boundaryCylinder(t, geom.Vec3{X: 2, Y: 2, Z: -1}, up, 1, 6), boolean.Cut, &turn},
		{"oblique boss, turned", boundaryCylinder(t, geom.Vec3{X: 1, Y: 2, Z: 1}, oblique, 0.7, 6), boolean.Union, &turn},
		{"blind bore, mirrored", boundaryCylinder(t, geom.Vec3{X: 1.5, Y: 2, Z: 2}, up, 1, 6), boolean.Cut, &mirror},
		{"oblique boss, mirrored", boundaryCylinder(t, geom.Vec3{X: 1, Y: 2, Z: 1}, oblique, 0.7, 6), boolean.Union, &mirror},
	} {
		r, err := boolean.Combine(context.Background(), boundaryBox(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4}), c.tool, c.op, boundaryOptions)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		solid := r.Solid
		if c.place != nil {
			if solid, err = solid.Transformed(*c.place); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
		}
		out = append(out, boundaryResult{c.name, solid, r.Properties})
	}
	return out
}

// boundaryMeasured requires a file to measure, from its entities alone, as
// the solid does: the volume and the area the Boolean reports in closed
// form, and the box of the solid's edges.
func boundaryMeasured(t *testing.T, file []byte, c boundaryResult) stepmeasure.Measures {
	t.Helper()
	got, err := stepmeasure.Boundary(file)
	if err != nil {
		t.Fatalf("%v\n%s", err, file)
	}
	if math.Abs(got.Volume-c.want.Volume) > 1e-9*c.want.Volume || math.Abs(got.Area-c.want.Area) > 1e-9*c.want.Area {
		t.Errorf("file measures volume %.12g and area %.12g, the solid has %.12g and %.12g", got.Volume, got.Area, c.want.Volume, c.want.Area)
	}
	// The box of the edges, from samples of the solid's own curves.
	var samples []geom.Vec3
	for _, e := range c.solid.Edges() {
		for k := 0; k <= 256; k++ {
			p, err := e.Curve.Eval(e.Range.Min + (e.Range.Max-e.Range.Min)*float64(k)/256)
			if err != nil {
				t.Fatal(err)
			}
			samples = append(samples, p)
		}
	}
	box, err := geom.Bounds(samples)
	if err != nil {
		t.Fatal(err)
	}
	if got.Box.Min.Distance(box.Min) > 1e-4 || got.Box.Max.Distance(box.Max) > 1e-4 {
		t.Errorf("file has box %v, the edges of the solid have %v", got.Box, box)
	}
	return got
}

// TestWriteTrimmedBoundary writes results of the general Boolean and
// measures each file from its entities alone. The file must also have the
// faces and edges of the solid.
func TestWriteTrimmedBoundary(t *testing.T) {
	for _, c := range boundaryResults(t) {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := step.WriteTrimmedBoundary(&out, c.solid, .001, "PlainSolid"); err != nil {
				t.Fatal(err)
			}
			got := boundaryMeasured(t, out.Bytes(), c)
			if got.Faces != len(c.solid.Faces()) || got.Edges != len(c.solid.Edges()) {
				t.Errorf("file has %d faces and %d edges, solid has %d and %d", got.Faces, got.Edges, len(c.solid.Faces()), len(c.solid.Edges()))
			}
		})
	}
}

// TestWriteTrimmedBoundaryRefusals holds the writer to an explicit error,
// with nothing written, for a solid outside its class.
func TestWriteTrimmedBoundaryRefusals(t *testing.T) {
	block := boundaryBox(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	sphere := func(t *testing.T) trimmed.Solid {
		p, err := parametric.Sphere(geom.Vec3{}, 1)
		if err != nil {
			t.Fatal(err)
		}
		s, err := trimmed.FromParametric(p)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, c := range []struct {
		name  string
		solid func(t *testing.T) trimmed.Solid
		unit  float64
		label string
		want  error
	}{
		{"a sphere", sphere, .001, "PlainSolid", step.ErrTrimmedBoundary},
		{"a cylinder with caps in polar charts", func(t *testing.T) trimmed.Solid {
			return boundaryCylinder(t, geom.Vec3{}, geom.Vec3{Z: 1}, 1, 2)
		}, .001, "PlainSolid", step.ErrTrimmedBoundary},
		{"two shells", func(t *testing.T) trimmed.Solid {
			r, err := boolean.Combine(context.Background(), block, boundaryBox(t, geom.Vec3{X: 7}, geom.Vec3{X: 8, Y: 1, Z: 1}), boolean.Union, boundaryOptions)
			if err != nil {
				t.Fatal(err)
			}
			return r.Solid
		}, .001, "PlainSolid", step.ErrTrimmedBoundary},
		{"a placement that scales", func(t *testing.T) trimmed.Solid {
			s, err := block.Transformed(geom.Affine{{2, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, .001, "PlainSolid", step.ErrTrimmedBoundary},
		{"no solid", func(*testing.T) trimmed.Solid { return trimmed.Solid{} }, .001, "PlainSolid", step.ErrTrimmedBoundary},
		{"a name that is not text", func(*testing.T) trimmed.Solid { return block }, .001, "\xff", geom.ErrInvalid},
		{"no length unit", func(*testing.T) trimmed.Solid { return block }, 0, "PlainSolid", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			err := step.WriteTrimmedBoundary(&out, c.solid(t), c.unit, c.label)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
			if out.Len() != 0 {
				t.Fatalf("%d bytes written with the error %v", out.Len(), err)
			}
		})
	}
	if err := step.WriteTrimmedBoundary(nil, block, .001, "PlainSolid"); !errors.Is(err, geom.ErrInvalid) {
		t.Fatalf("nil writer: %v", err)
	}
}
