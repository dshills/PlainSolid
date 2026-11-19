package blend

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// The tolerances the bridge passes.
var (
	testTolerance = Tolerance{Point: 1e-7, Angular: 1e-9}
	testOptions   = boolean.Options{Linear: 1e-7, Angular: 1e-9, Volume: 1e-7}
)

func vec(x, y, z float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: z} }

func lifted(t *testing.T, s brep.Solid, err error) trimmed.Solid {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	out, err := trimmed.FromPlanar(s, testOptions.Linear)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func boxSolid(t *testing.T, lo, hi geom.Vec3) trimmed.Solid {
	t.Helper()
	s, err := brep.Box(lo, hi)
	return lifted(t, s, err)
}

// prismSolid extrudes a polygon given in the plane z = 0 by height.
func prismSolid(t *testing.T, height float64, profile ...[2]float64) trimmed.Solid {
	t.Helper()
	points := make([]geom.Vec3, len(profile))
	for i, p := range profile {
		points[i] = vec(p[0], p[1], 0)
	}
	s, err := brep.Prism(points, vec(0, 0, height))
	return lifted(t, s, err)
}

// cylinderSolid is a cylinder whose axis starts at origin.
func cylinderSolid(t *testing.T, origin, axis geom.Vec3, radius, height float64) trimmed.Solid {
	t.Helper()
	frame, err := geom.FrameFromNormal(origin, axis)
	if err != nil {
		t.Fatal(err)
	}
	p, err := parametric.Cylinder(frame, radius, height)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromParametric(p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func combined(t *testing.T, a, b trimmed.Solid, op boolean.Operation) boolean.Result {
	t.Helper()
	r, err := boolean.Combine(context.Background(), a, b, op, testOptions)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The solids several tests share.

// slottedBox is the body of Q1: a box with a slot across its top.
func slottedBox(t *testing.T) trimmed.Solid {
	t.Helper()
	return combined(t, boxSolid(t, vec(0, 0, 0), vec(20, 10, 10)), boxSolid(t, vec(12, -1, 6), vec(16, 11, 11)), boolean.Cut).Solid
}

// touchedBox is a box with a cylinder standing on its top face that
// touches the top front edge at (10,0,10). The Boolean divides the top
// face there.
func touchedBox(t *testing.T) trimmed.Solid {
	t.Helper()
	return combined(t, boxSolid(t, vec(0, 0, 0), vec(20, 10, 10)), cylinderSolid(t, vec(10, 2, 10), vec(0, 0, 1), 2, 3), boolean.Union).Solid
}

func lPrism(t *testing.T) trimmed.Solid {
	t.Helper()
	return prismSolid(t, 10, [2]float64{0, 0}, [2]float64{20, 0}, [2]float64{20, 10}, [2]float64{10, 10}, [2]float64{10, 20}, [2]float64{0, 20})
}

func hexPrism(t *testing.T) trimmed.Solid {
	t.Helper()
	var profile [][2]float64
	for k := range 6 {
		a := float64(k) * math.Pi / 3
		profile = append(profile, [2]float64{5 * math.Cos(a), 5 * math.Sin(a)})
	}
	return prismSolid(t, 8, profile...)
}

func trianglePrism(t *testing.T) trimmed.Solid {
	t.Helper()
	return prismSolid(t, 5, [2]float64{0, 0}, [2]float64{4, 0}, [2]float64{0, 3})
}
