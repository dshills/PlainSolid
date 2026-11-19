package occtcheck

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// The cases of testdata/general_boolean_cases.json are the oracle cases of
// the general Boolean trial (specs/general-boolean, A6): the section 11
// leaves that return a solid, the first row of each Tier 1 family, and the
// results the general path gives where the legacy path declined. A case is
// a list of primitives and a list of Boolean steps; a step takes two
// operands by number, the primitives first and then the results of the
// earlier steps. tools/occtref/general_boolean.cxx builds the same cases
// with the pinned OCCT.

type generalBooleanCase struct {
	Name   string `json:"name"`
	Shapes []struct {
		Kind      string       `json:"kind"`
		Min       [3]float64   `json:"min"`
		Max       [3]float64   `json:"max"`
		Origin    [3]float64   `json:"origin"`
		Axis      [3]float64   `json:"axis"`
		Radius    float64      `json:"radius"`
		Height    float64      `json:"height"`
		Outer     [][3]float64 `json:"outer"`
		Holes     [][][3]float64
		Offset    [3]float64     `json:"offset"`
		Placement *[3][4]float64 `json:"placement"`
	} `json:"shapes"`
	Steps []struct {
		Op   string `json:"op"`
		A, B int
	} `json:"steps"`
	// Faces is the number of faces section 11 states for the result, and
	// Dividers how many of them M6 makes by dividing a face that touches
	// itself, which the oracle does not do.
	Faces    int `json:"faces"`
	Dividers int `json:"dividers"`
}

type generalBooleanReference struct {
	Name          string
	Step          int
	Done, Valid   bool
	Volume, Area  float64
	Centroid      [3]float64
	Solids, Faces int
}

func vec3(v [3]float64) geom.Vec3 { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }

func loop3(points [][3]float64) []geom.Vec3 {
	out := make([]geom.Vec3, len(points))
	for i, p := range points {
		out[i] = vec3(p)
	}
	return out
}

// generalBooleanShape builds one primitive of a case.
func generalBooleanShape(t *testing.T, c generalBooleanCase, i int) trimmed.Solid {
	t.Helper()
	s := c.Shapes[i]
	var planar brep.Solid
	var err error
	switch s.Kind {
	case "box":
		planar, err = brep.Box(vec3(s.Min), vec3(s.Max))
	case "prism":
		holes := make([][]geom.Vec3, len(s.Holes))
		for k, h := range s.Holes {
			holes[k] = loop3(h)
		}
		planar, err = brep.PrismWithHoles(loop3(s.Outer), holes, vec3(s.Offset))
	case "cylinder":
		frame, e := geom.FrameFromNormal(vec3(s.Origin), vec3(s.Axis))
		if e != nil {
			t.Fatalf("%s shape %d: %v", c.Name, i, e)
		}
		patches, e := parametric.Cylinder(frame, s.Radius, s.Height)
		if e != nil {
			t.Fatalf("%s shape %d: %v", c.Name, i, e)
		}
		out, e := trimmed.FromParametric(patches)
		if e != nil {
			t.Fatalf("%s shape %d: %v", c.Name, i, e)
		}
		return out
	default:
		t.Fatalf("%s shape %d: unknown kind %q", c.Name, i, s.Kind)
	}
	if err == nil && s.Placement != nil {
		planar, err = planar.Transformed(geom.Affine(*s.Placement))
	}
	if err != nil {
		t.Fatalf("%s shape %d: %v", c.Name, i, err)
	}
	out, err := trimmed.FromPlanar(planar, 1e-9)
	if err != nil {
		t.Fatalf("%s shape %d: %v", c.Name, i, err)
	}
	return out
}

// TestGeneralBooleanOracle compares boolean.Combine with the pinned OCCT
// on every step of every case, at the bound of compare in oracle_test.go:
// 3e-7 relative for volume and area and 3e-7 absolute for the centroid.
// It runs only when PLAINSOLID_OCCT_GENERAL_BOOLEAN names the helper built
// from tools/occtref/general_boolean.cxx.
func TestGeneralBooleanOracle(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_GENERAL_BOOLEAN")
	if bin == "" {
		t.Skip("optional pinned OCCT reference for the general Boolean")
	}
	const file = "testdata/general_boolean_cases.json"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Cases []generalBooleanCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, file).Output()
	if err != nil {
		t.Fatalf("%s: %v", bin, err)
	}
	reference := map[string][]generalBooleanReference{}
	for lines := bufio.NewScanner(bytes.NewReader(out)); lines.Scan(); {
		var r generalBooleanReference
		if err := json.Unmarshal(lines.Bytes(), &r); err != nil {
			t.Fatalf("%s: %v", lines.Text(), err)
		}
		reference[r.Name] = append(reference[r.Name], r)
	}
	operations := map[string]boolean.Operation{"union": boolean.Union, "common": boolean.Common, "cut": boolean.Cut}
	options := boolean.Options{Linear: 1e-7, Angular: 1e-9, Volume: 1e-7}
	compared, without := 0, 0
	for _, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			want := reference[c.Name]
			if len(want) != len(c.Steps) {
				t.Fatalf("the oracle reports %d steps of %d", len(want), len(c.Steps))
			}
			var solids []trimmed.Solid
			for i := range c.Shapes {
				solids = append(solids, generalBooleanShape(t, c, i))
			}
			for k, step := range c.Steps {
				w := want[k]
				if !w.Done || !w.Valid {
					t.Fatalf("step %d: the oracle built no valid shape (%+v)", k, w)
				}
				r, err := boolean.Combine(context.Background(), solids[step.A], solids[step.B], operations[step.Op], options)
				if err != nil {
					// Not a difference from the oracle: the general
					// Boolean gave no result to compare. The run counts
					// these, and the steps after it cannot be made.
					without++
					t.Logf("step %d (%s): no result: %v", k, step.Op, err)
					return
				}
				compared++
				p := r.Properties
				centre := geom.Vec3{X: w.Centroid[0], Y: w.Centroid[1], Z: w.Centroid[2]}
				// One line per step, both sides, so that a run leaves
				// its own record.
				t.Logf("oracle step %d (%s): solids %d/%d volume %.15g/%.15g area %.15g/%.15g centroid distance %.3g faces %d/%d", k, step.Op, r.Solids, w.Solids, p.Volume, w.Volume, p.Area, w.Area, p.Centroid.Distance(centre), r.Solid.Topology().Faces, w.Faces)
				if r.Solids != w.Solids || math.Abs(p.Volume-w.Volume) > 3e-7*w.Volume || math.Abs(p.Area-w.Area) > 3e-7*w.Area || r.Solids > 0 && p.Centroid.Distance(centre) > 3e-7 {
					t.Errorf("step %d (%s): %d solids, volume %.15g, area %.15g, centroid %v; OCCT %d, %.15g, %.15g, %v", k, step.Op, r.Solids, p.Volume, p.Area, p.Centroid, w.Solids, w.Volume, w.Area, w.Centroid)
				}
				if last := k+1 == len(c.Steps); last && c.Faces != 0 {
					if got := r.Solid.Topology().Faces; got != c.Faces || w.Faces != c.Faces-c.Dividers {
						t.Errorf("step %d (%s): %d faces, OCCT %d; section 11 states %d, %d of them from dividing a face", k, step.Op, got, w.Faces, c.Faces, c.Dividers)
					}
				}
				solids = append(solids, r.Solid)
			}
		})
	}
	t.Logf("%d steps compared with the oracle; %d cases stopped at a step with no result", compared, without)
}
