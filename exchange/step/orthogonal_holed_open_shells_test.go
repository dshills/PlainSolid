package step

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func holedOpenShellFixture(t testing.TB) OrthogonalSplineOpenShellModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/orthogonal-holed-open-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestOrthogonalHoledOpenShell(t *testing.T) {
	t.Parallel()
	m := holedOpenShellFixture(t)
	s := m.Shells[0]
	if len(s.Faces) != 2 || len(s.FaceHoles) != 2 || len(s.FaceHoles[0]) != 1 || len(s.FaceHoles[1]) != 0 || len(s.Uses) != 14 {
		t.Fatal(s)
	}
	holeUses := 0
	for _, u := range s.Uses {
		if u.Loop == 1 {
			if u.Face != 0 {
				t.Fatal(u)
			}
			holeUses++
		}
	}
	if holeUses != 4 {
		t.Fatal(holeUses)
	}
	var b bytes.Buffer
	if e := WriteOrthogonalSplineOpenShells(&b, m); e != nil {
		t.Fatal(e)
	}
	round, e := ReadOrthogonalSplineOpenShells(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.FaceHoles, round.Shells[0].FaceHoles) {
		t.Fatal("hole geometry changed")
	}
}
func TestOrthogonalHoledOpenShellReject(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"unowned", "loop-index", "nested"} {
		t.Run(kind, func(t *testing.T) {
			m := holedOpenShellFixture(t)
			s := &m.Shells[0]
			switch kind {
			case "unowned":
				for i, u := range s.Uses {
					if u.Loop == 1 {
						s.Uses = append(s.Uses[:i], s.Uses[i+1:]...)
						break
					}
				}
			case "loop-index":
				for i, u := range s.Uses {
					if u.Loop == 1 {
						s.Uses[i].Loop = 2
						break
					}
				}
			case "nested":
				s.FaceHoles[0] = append(s.FaceHoles[0], []geom.Vec2{{X: .75, Y: .75}, {X: 1.25, Y: .75}, {X: 1.25, Y: 1.25}, {X: .75, Y: 1.25}})
			}
			var b bytes.Buffer
			if e := WriteOrthogonalSplineOpenShells(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("bad hole topology emitted", e)
			}
		})
	}
	raw, e := os.ReadFile("testdata/orthogonal-holed-open-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m := holedOpenShellFixture(t)
	for _, u := range m.Shells[0].Uses {
		if u.Loop != 1 {
			continue
		}
		_, _, g, _ := analyticEdge(f, u.EdgeID)
		sc, _ := simple(f, g, "SURFACE_CURVE", 4)
		pcs, _ := listRefs(sc.Args[2], 1)
		pc, _ := simple(f, pcs[0], "PCURVE", 3)
		owner, _ := ref(pc.Args[1])
		neighbor, _ := simple(f, m.Shells[0].FaceIDs[1], "ADVANCED_FACE", 4)
		other, _ := ref(neighbor.Args[2])
		from := "PCURVE('',#" + strconv.FormatInt(owner, 10) + ",#" + strconv.FormatInt(pc.Args[2].Ref, 10) + ")"
		to := "PCURVE('',#" + strconv.FormatInt(other, 10) + ",#" + strconv.FormatInt(pc.Args[2].Ref, 10) + ")"
		bad := strings.Replace(string(raw), from, to, 1)
		if bad == string(raw) {
			t.Fatal("missed owner mutation")
		}
		if _, e = ReadOrthogonalSplineOpenShells(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("hole with wrong owner accepted")
		}
		break
	}
}
func TestOCCTOrthogonalHoledOpenShell(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-orthogonal-holed-open-shell")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalSplineOpenShells(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, c := openShellArea(t, source), openShellArea(t, path)
	if math.Abs(a-c) > 1e-8 {
		t.Fatal(a, c)
	}
}
