package geom

import (
	"errors"
	"math"
	"math/rand"
	"sort"
	"testing"
)

func TestLineTorusCrossingsAndBounds(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	torus, _ := NewTorusSurface(frame, 3, 1)
	line, _ := NewLine(Vec3{}, Vec3{1, 0, 0})
	result, e := IntersectLineTorus(line, torus, LineTorusOptions{})
	if e != nil || len(result.Hits) != 4 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{-4, -2, 2, 4} {
		hit := result.Hits[i]
		if math.Abs(hit.Parameter-want) > 1e-9 || hit.ParameterMin > want || hit.ParameterMax < want || hit.Multiplicity != 1 || !hit.Crossing || hit.Tangent || hit.Residual > 1e-9 {
			t.Fatal(hit, want)
		}
	}
	result, e = IntersectLineTorus(line, torus, LineTorusOptions{UseParameterRange: true, MinParameter: 2, MaxParameter: 4})
	if e != nil || len(result.Hits) != 2 || result.Hits[0].Parameter != 2 || result.Hits[1].Parameter != 4 {
		t.Fatal(result, e)
	}
	result, e = IntersectLineTorus(line, torus, LineTorusOptions{UseParameterRange: true, MinParameter: 2, MaxParameter: 2})
	if e != nil || len(result.Hits) != 1 || result.Hits[0].Parameter != 2 {
		t.Fatal(result, e)
	}
}
func TestLineTorusMultiplicity(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	torus, _ := NewTorusSurface(frame, 3, 1)
	for _, tt := range []struct {
		origin, direction   Vec3
		count, multiplicity int
	}{{Vec3{0, 4, 0}, Vec3{1, 0, 0}, 1, 2}, {Vec3{0, 2, 0}, Vec3{1, 0, 0}, 3, 2}, {Vec3{3, 0, 1}, Vec3{0, 1, 0}, 1, 4}} {
		line, _ := NewLine(tt.origin, tt.direction)
		result, e := IntersectLineTorus(line, torus, LineTorusOptions{})
		if e != nil || len(result.Hits) != tt.count {
			t.Fatal(tt, result, e)
		}
		found := false
		for _, hit := range result.Hits {
			if hit.Parameter == 0 {
				found = true
				if hit.Multiplicity != tt.multiplicity || !hit.Tangent || hit.Crossing || hit.Residual > 1e-12 {
					t.Fatal(hit)
				}
			}
		}
		if !found {
			t.Fatal(result)
		}
	}
}
func TestLineTorusAxialAndNearTangency(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	torus, _ := NewTorusSurface(frame, 3, 1)
	line, _ := NewLine(Vec3{3, 0, 0}, Vec3{0, 0, 1})
	result, e := IntersectLineTorus(line, torus, LineTorusOptions{})
	if e != nil || len(result.Hits) != 2 || math.Abs(result.Hits[0].Parameter+1) > 1e-9 || math.Abs(result.Hits[1].Parameter-1) > 1e-9 {
		t.Fatal(result, e)
	}
	line, _ = NewLine(Vec3{}, Vec3{0, 0, 1})
	result, e = IntersectLineTorus(line, torus, LineTorusOptions{})
	if e != nil || len(result.Hits) != 0 {
		t.Fatal(result, e)
	}
	for _, tt := range []struct {
		y     float64
		count int
	}{{math.Nextafter(4, 0), 2}, {math.Nextafter(4, 5), 0}} {
		line, _ = NewLine(Vec3{0, tt.y, 0}, Vec3{1, 0, 0})
		result, e = IntersectLineTorus(line, torus, LineTorusOptions{})
		if e != nil || len(result.Hits) != tt.count {
			t.Fatal(tt, result, e)
		}
	}
}
func TestRandomLineTorusCircleOracle(t *testing.T) {
	random := rand.New(rand.NewSource(31337))
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	torus, _ := NewTorusSurface(frame, 3, 1)
	for trial := 0; trial < 80; trial++ {
		y := random.Float64()*9 - 4.5
		line, _ := NewLine(Vec3{-6, y, 0}, Vec3{1, 0, 0})
		want := []float64{}
		for _, radius := range []float64{2, 4} {
			if math.Abs(y) < radius {
				half := math.Sqrt(radius*radius - y*y)
				want = append(want, 6-half, 6+half)
			}
		}
		sort.Float64s(want)
		result, e := IntersectLineTorus(line, torus, LineTorusOptions{})
		if e != nil || len(result.Hits) != len(want) {
			t.Fatal(trial, result, e, want)
		}
		for i, value := range want {
			if math.Abs(result.Hits[i].Parameter-value) > 1e-8 || result.Hits[i].Multiplicity != 1 {
				t.Fatal(result.Hits[i], value)
			}
		}
	}
}
func TestLineTorusRigidFrameAndFailures(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{2, -3, 5}, Vec3{1, 2, 3})
	torus, _ := NewTorusSurface(frame, 3, 1)
	line, _ := NewLine(frame.ToWorld(Vec3{-6, .7, 0}), frame.XAxis())
	result, e := IntersectLineTorus(line, torus, LineTorusOptions{})
	if e != nil || len(result.Hits) != 4 {
		t.Fatal(result, e)
	}
	for _, hit := range result.Hits {
		local := frame.ToLocal(hit.Point)
		if math.Abs(math.Abs(math.Hypot(local.X, local.Y)-3)-1) > 1e-9 {
			t.Fatal(hit)
		}
	}
	for _, options := range []LineTorusOptions{{MaxOperations: 1}, {MaxSubdivisions: 1}, {MaxCoefficientBits: 64}} {
		if _, e = IntersectLineTorus(line, torus, options); !errors.Is(e, ErrConvergence) {
			t.Fatal(options, e)
		}
	}
	if _, e = IntersectLineTorus(line, torus, LineTorusOptions{UseParameterRange: true, MinParameter: 1, MaxParameter: 0}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestLineTorusObliqueAndLargeBound(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	torus, _ := NewTorusSurface(frame, 3, 1)
	line, _ := NewLine(Vec3{4, 0, 0}, Vec3{-4, 3, 1})
	result, e := IntersectLineTorus(line, torus, LineTorusOptions{})
	if e != nil {
		t.Fatal(result, e)
	}
	originFound, otherFound := false, false
	for _, hit := range result.Hits {
		if math.Abs(hit.Parameter) < 1e-9 {
			originFound = true
		}
		if math.Abs(hit.Parameter-math.Sqrt(26)) < 1e-9 {
			otherFound = true
		}
	}
	if !originFound || !otherFound {
		t.Fatal(result)
	}
	large, _ := NewTorusSurface(frame, 3e100, 1e100)
	line, _ = NewLine(Vec3{}, Vec3{1, 0, 0})
	result, e = IntersectLineTorus(line, large, LineTorusOptions{DistanceTolerance: 1e88})
	if e != nil || len(result.Hits) != 4 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{-4, -2, 2, 4} {
		if math.Abs(result.Hits[i].Parameter/1e100-want) > 1e-10 {
			t.Fatal(result.Hits[i], want)
		}
	}
}
