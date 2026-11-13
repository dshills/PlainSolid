package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestLineCylinderContacts(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewCylinderSurface(frame, 2)
	for _, tt := range []struct {
		o, d    Vec3
		count   int
		tangent bool
	}{{Vec3{-3, 0, 4}, Vec3{1, 0, 0}, 2, false}, {Vec3{-3, 2, 4}, Vec3{1, 0, 0}, 1, true}, {Vec3{-3, 3, 4}, Vec3{1, 0, 0}, 0, false}, {Vec3{0, 0, 4}, Vec3{0, 0, 1}, 0, false}} {
		line, _ := NewLine(tt.o, tt.d)
		result, e := IntersectLineCylinder(line, surface, LineSurfaceOptions{})
		if e != nil || len(result.Hits) != tt.count {
			t.Fatal(tt, result, e)
		}
		for _, hit := range result.Hits {
			if hit.Tangent != tt.tangent || hit.Residual > 1e-9 || math.Abs(math.Hypot(hit.Point.X, hit.Point.Y)-2) > 1e-12 {
				t.Fatal(hit)
			}
		}
	}
	line, _ := NewLine(Vec3{2, 0, 4}, Vec3{0, 0, 1})
	result, e := IntersectLineCylinder(line, surface, LineSurfaceOptions{})
	if e != nil || len(result.Coincident) != 1 || !math.IsInf(result.Coincident[0].Min, -1) || !math.IsInf(result.Coincident[0].Max, 1) {
		t.Fatal(result, e)
	}
	line, _ = NewLine(Vec3{-3, 0, 0}, Vec3{1, 0, 0})
	result, e = IntersectLineCylinder(line, surface, LineSurfaceOptions{UseParameterRange: true, MinParameter: 1, MaxParameter: 1})
	if e != nil || len(result.Hits) != 1 || result.Hits[0].Parameter != 1 {
		t.Fatal(result, e)
	}
	result, e = IntersectLineCylinder(line, surface, LineSurfaceOptions{UseParameterRange: true, MinParameter: 1.1, MaxParameter: 4.9})
	if e != nil || len(result.Hits) != 0 {
		t.Fatal(result, e)
	}
}
func TestLineConeNappeAndGenerators(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewConeSurfaceSlope(frame, .5)
	if surface.slope != .5 {
		t.Fatal("test needs exactly represented cone slope", surface.slope)
	}
	for _, tt := range []struct {
		o, d     Vec3
		count    int
		singular bool
	}{{Vec3{-3, 0, 4}, Vec3{1, 0, 0}, 2, false}, {Vec3{-3, 2, 4}, Vec3{1, 0, 0}, 1, false}, {Vec3{-3, 0, -4}, Vec3{1, 0, 0}, 0, false}, {Vec3{0, 0, -3}, Vec3{0, 0, 1}, 1, true}} {
		line, _ := NewLine(tt.o, tt.d)
		result, e := IntersectLineCone(line, surface, LineSurfaceOptions{})
		if e != nil || len(result.Hits) != tt.count {
			t.Fatal(tt, result, e)
		}
		for _, hit := range result.Hits {
			if hit.Singular != tt.singular || hit.Point.Z < -1e-12 || math.Abs(math.Hypot(hit.Point.X, hit.Point.Y)-.5*hit.Point.Z) > 1e-12 {
				t.Fatal(hit)
			}
		}
	}
	line, _ := NewLine(Vec3{}, Vec3{.5, 0, 1})
	result, e := IntersectLineCone(line, surface, LineSurfaceOptions{})
	if e != nil || len(result.Coincident) != 1 || result.Coincident[0].Min != 0 || !math.IsInf(result.Coincident[0].Max, 1) {
		t.Fatal(result, e)
	}
	line, _ = NewLine(Vec3{}, Vec3{-.5, 0, -1})
	result, e = IntersectLineCone(line, surface, LineSurfaceOptions{UseParameterRange: true, MinParameter: -2, MaxParameter: 3})
	if e != nil || len(result.Coincident) != 1 || result.Coincident[0].Min != -2 || result.Coincident[0].Max != 0 {
		t.Fatal(result, e)
	}
	line, _ = NewLine(Vec3{0, 0, 2}, Vec3{.5, 0, 1})
	result, e = IntersectLineCone(line, surface, LineSurfaceOptions{})
	if e != nil || len(result.Hits) != 1 || result.Hits[0].Point.Z <= 0 {
		t.Fatal(result, e)
	}
}
func TestLineQuadricNearTangency(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewCylinderSurface(frame, 1)
	for _, tt := range []struct {
		y     float64
		count int
	}{{math.Nextafter(1, 0), 2}, {1, 1}, {math.Nextafter(1, 2), 0}} {
		line, _ := NewLine(Vec3{0, tt.y, 0}, Vec3{1, 0, 0})
		result, e := IntersectLineCylinder(line, surface, LineSurfaceOptions{})
		if e != nil || len(result.Hits) != tt.count {
			t.Fatal(tt, result, e)
		}
		if tt.count == 2 && result.Hits[0].Tangent {
			t.Fatal(result)
		}
	}
}
func TestRandomLineCylinderOracle(t *testing.T) {
	random := rand.New(rand.NewSource(70727))
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewCylinderSurface(frame, 3)
	for trial := 0; trial < 200; trial++ {
		angle := random.Float64() * 2 * math.Pi
		offset := random.Float64()*5 - 2.5
		direction := Vec3{math.Cos(angle), math.Sin(angle), 0}
		normal := Vec3{-direction.Y, direction.X, 0}
		origin := normal.Scale(offset).Add(direction.Scale(-5))
		origin.Z = random.Float64()*10 - 5
		line, _ := NewLine(origin, direction)
		result, e := IntersectLineCylinder(line, surface, LineSurfaceOptions{})
		if e != nil || len(result.Hits) != 2 {
			t.Fatal(result, e)
		}
		half := math.Sqrt(9 - offset*offset)
		for i, want := range []float64{5 - half, 5 + half} {
			hit := result.Hits[i]
			if math.Abs(hit.Parameter-want) > 1e-12 || hit.Residual > 1e-12 {
				t.Fatal(hit, want)
			}
		}
	}
}
func TestLineSurfaceFailureContracts(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewCylinderSurface(frame, 1)
	line, _ := NewLine(Vec3{-1e200, 0, 0}, Vec3{1, 0, 0})
	_, e := IntersectLineCylinder(line, surface, LineSurfaceOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("unresolvable distinct float64 roots must fail", e)
	}
	line, _ = NewLine(Vec3{}, Vec3{1, 0, 0})
	for _, options := range []LineSurfaceOptions{{MaxPrecision: 32}, {DistanceTolerance: -1}, {UseParameterRange: true, MinParameter: 2, MaxParameter: 1}} {
		if _, e := IntersectLineCylinder(line, surface, options); !errors.Is(e, ErrInvalid) {
			t.Fatal(options, e)
		}
	}
}

func TestLineCylinderTinyTransverseDirection(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewCylinderSurface(frame, 1)
	line, _ := NewLine(Vec3{}, Vec3{1e-200, 0, 1})
	result, e := IntersectLineCylinder(line, surface, LineSurfaceOptions{})
	if e != nil || len(result.Hits) != 2 || len(result.Coincident) != 0 {
		t.Fatal(result, e)
	}
	for _, hit := range result.Hits {
		if math.Abs(math.Abs(hit.Parameter)/1e200-1) > 1e-15 || hit.Residual > 1e-15 {
			t.Fatal(hit)
		}
	}
}
func TestRandomLineConeOracle(t *testing.T) {
	random := rand.New(rand.NewSource(31907))
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	surface, _ := NewConeSurfaceSlope(frame, .5)
	for trial := 0; trial < 120; trial++ {
		height := .5 + 8*random.Float64()
		radius := height * .5
		angle := random.Float64() * 2 * math.Pi
		direction := Vec3{math.Cos(angle), math.Sin(angle), 0}
		normal := Vec3{-direction.Y, direction.X, 0}
		offset := (2*random.Float64() - 1) * radius * .8
		origin := normal.Scale(offset).Add(direction.Scale(-10))
		origin.Z = height
		line, _ := NewLine(origin, direction)
		result, e := IntersectLineCone(line, surface, LineSurfaceOptions{})
		if e != nil || len(result.Hits) != 2 {
			t.Fatal(result, e)
		}
		half := math.Sqrt(radius*radius - offset*offset)
		for i, want := range []float64{10 - half, 10 + half} {
			if math.Abs(result.Hits[i].Parameter-want) > 1e-12 || result.Hits[i].Residual > 1e-12 {
				t.Fatal(result.Hits[i], want)
			}
		}
	}
}
func TestLineSurfaceRigidFrameInvariance(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{2, -3, 5}, Vec3{1, 2, 3})
	cylinder, _ := NewCylinderSurface(frame, 2)
	cone, _ := NewConeSurfaceSlope(frame, .5)
	line, _ := NewLine(frame.ToWorld(Vec3{-5, .4, 4}), frame.XAxis())
	for _, coneTest := range []bool{false, true} {
		var result LineSurfaceResult
		var e error
		if coneTest {
			result, e = IntersectLineCone(line, cone, LineSurfaceOptions{})
		} else {
			result, e = IntersectLineCylinder(line, cylinder, LineSurfaceOptions{})
		}
		if e != nil || len(result.Hits) != 2 {
			t.Fatal(result, e)
		}
		half := math.Sqrt(4 - .16)
		for i, x := range []float64{-half, half} {
			near(t, frame.ToLocal(result.Hits[i].Point), Vec3{x, .4, 4}, 2e-13)
		}
	}
}
