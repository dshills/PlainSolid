package occtcheck

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

func oracleWarpedPrism(t *testing.T) trimmed.Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var reversed []bool
	var domains []geom.ParameterRectangle
	add := func(eval func(float64, float64) geom.Vec3, n int, reverse bool) {
		var knots []float64
		for i := 0; i < n; i++ {
			knots = append(knots, 0)
		}
		for i := 0; i < n; i++ {
			knots = append(knots, 1)
		}
		var poles []geom.Vec3
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				poles = append(poles, eval(float64(i)/float64(n-1), float64(j)/float64(n-1)))
			}
		}
		s, e := geom.NewBSplineSurface(n-1, n-1, n, n, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, s)
		domains = append(domains, geom.ParameterRectangle{})
		reversed = append(reversed, reverse)
	}
	add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: 2 * u, Y: 2 * v} }, 2, true)
	add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: 2 * u, Y: 2 * v, Z: 1 + u*v} }, 3, false)
	for _, x := range []float64{0, 2} {
		add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: x, Y: 2 * u, Z: v * (1 + x*u/2)} }, 2, x == 0)
	}
	for _, y := range []float64{0, 2} {
		add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: 2 * u, Y: y, Z: v * (1 + u*y/2)} }, 2, y == 2)
	}
	s, _, e := trimmed.SewSplineRectangles(context.Background(), surfaces, domains, reversed, trimmed.SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestOCCTSeparatedSplineShellBooleans(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to optional OCCT executable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "spline-shell.jsonl")
	if out, e := exec.CommandContext(ctx, program, path, "spline-shell-boolean").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, out)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type shellRecord struct {
		record
		Shells int
	}
	refs := map[string]shellRecord{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var r shellRecord
		if e = json.Unmarshal(scanner.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[r.Name]; ok {
			t.Fatal("duplicate")
		}
		refs[r.Name] = r
	}
	if e = scanner.Err(); e != nil {
		t.Fatal(e)
	}
	if len(refs) != 15 {
		t.Fatal("missing records")
	}
	a := oracleWarpedPrism(t)
	for _, nested := range []bool{true, false} {
		m := geom.Identity()
		prefix := "disjoint-"
		m[0][3] = 3
		if nested {
			prefix = "nested-"
			m[0][0], m[1][1], m[2][2] = .25, .25, .25
			m[0][3], m[1][3], m[2][3] = .5, .5, .25
		}
		b, e := a.Transformed(m)
		if e != nil {
			t.Fatal(e)
		}
		for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
			name := prefix + []string{"union", "common", "cut"}[i]
			t.Run(name, func(t *testing.T) {
				s, h, e := trimmed.CombineSeparatedSplineShells(ctx, a, b, op, trimmed.SplineShellBooleanOptions{})
				if e != nil {
					t.Fatal(e)
				}
				ref := refs[name]
				if len(s.Topology().Shells) != ref.Shells {
					t.Fatal("shells", len(s.Topology().Shells), ref.Shells)
				}
				if ref.Volume == 0 {
					if len(s.Faces()) != 0 || ref.Area != 0 {
						t.Fatal("empty common mismatch")
					}
					return
				}
				p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				compare(t, ref.record, p.Area, p.Volume, p.Centroid)
				if nested && op == parametric.Difference && !h.BReversed {
					t.Fatal("cavity orientation ancestry")
				}
			})
		}
	}

	mirror := geom.Identity()
	mirror[0][0] = -1
	mirror[0][3] = 4
	joined, e := a.Transformed(mirror)
	if e != nil {
		t.Fatal(e)
	}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		name := "joined-" + []string{"union", "common", "cut"}[i]
		t.Run(name, func(t *testing.T) {
			s, _, e := trimmed.CombineFaceJoinedSplineShells(ctx, a, joined, op, trimmed.SplineShellJoinOptions{})
			if e != nil {
				t.Fatal(e)
			}
			ref := refs[name]
			if len(s.Topology().Shells) != ref.Shells {
				t.Fatal("joined shells")
			}
			if ref.Volume == 0 {
				if len(s.Faces()) != 0 {
					t.Fatal("empty")
				}
				return
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, ref.record, p.Area, p.Volume, p.Centroid)
		})
	}

	insetMap := geom.Identity()
	insetMap[0][0], insetMap[1][1], insetMap[2][2] = .5, .5, -.5
	insetMap[0][3], insetMap[1][3] = .5, .5
	inset, e := a.Transformed(insetMap)
	if e != nil {
		t.Fatal(e)
	}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		name := "inset-" + []string{"union", "common", "cut"}[i]
		t.Run(name, func(t *testing.T) {
			s, _, e := trimmed.CombineInsetJoinedSplineShells(ctx, a, inset, op, trimmed.SplineShellJoinOptions{})
			if e != nil {
				t.Fatal(e)
			}
			ref := refs[name]
			if len(s.Topology().Shells) != ref.Shells {
				t.Fatal("inset shells")
			}
			if ref.Volume == 0 {
				if len(s.Faces()) != 0 {
					t.Fatal("empty inset")
				}
				return
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, ref.record, p.Area, p.Volume, p.Centroid)
			if op == parametric.Union {
				if len(s.Faces()) != 11 {
					t.Fatal("inset faces")
				}
			}
		})
	}

	overlapMap := geom.Identity()
	overlapMap[2][2] = -1
	overlapMap[0][3], overlapMap[1][3] = 1, .5
	overlap, e := a.Transformed(overlapMap)
	if e != nil {
		t.Fatal(e)
	}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		name := "overlap-" + []string{"union", "common", "cut"}[i]
		t.Run(name, func(t *testing.T) {
			s, _, e := trimmed.CombineOverlappingInterfaceSplineShells(ctx, a, overlap, op, trimmed.SplineShellJoinOptions{})
			if e != nil {
				t.Fatal(e)
			}
			ref := refs[name]
			if len(s.Topology().Shells) != ref.Shells {
				t.Fatal("overlap shells")
			}
			if ref.Volume == 0 {
				if len(s.Faces()) != 0 {
					t.Fatal("empty overlap")
				}
				return
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, ref.record, p.Area, p.Volume, p.Centroid)
		})
	}
}
