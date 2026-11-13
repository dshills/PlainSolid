package step

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func verifyOrthogonalOpenShell(t testing.TB, raw []byte) {
	t.Helper()
	m, e := ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Shells) != 1 || len(m.Shells[0].Faces) != 2 || len(m.Shells[0].Uses) != 10 {
		t.Fatal(m)
	}
	for _, face := range m.Shells[0].Faces {
		for _, uv := range face.Boundary {
			p, e := face.Geometry.Eval(uv.X, uv.Y)
			if e != nil || p.X != uv.X || p.Y != uv.Y || p.Z != uv.X*uv.Y/16 {
				t.Fatal("source geometry changed", p, e)
			}
		}
	}
	counts := map[int64]int{}
	for _, u := range m.Shells[0].Uses {
		counts[u.EdgeID]++
	}
	shared, boundary := 0, 0
	for _, n := range counts {
		if n == 2 {
			shared++
		} else if n == 1 {
			boundary++
		} else {
			t.Fatal(n)
		}
	}
	if shared != 1 || boundary != 8 {
		t.Fatal(shared, boundary)
	}
	if _, e = ReadOrthogonalSplineFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("standalone face API widened")
	}
	if _, e = ReadSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("solid API accepted open shell")
	}
	for _, l := range []Limits{{MaxEdgeUses: 9}, {MaxFaces: 1}, {MaxVertices: 7}} {
		if _, e = ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestIndependentOrthogonalOpenShell(t *testing.T) {
	raw, e := os.ReadFile("testdata/orthogonal-open-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	verifyOrthogonalOpenShell(t, raw)
}
func TestOCCTOrthogonalOpenShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, path, "write-step-orthogonal-open-shell")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	verifyOrthogonalOpenShell(t, raw)
}
func TestOpenShellIncidenceReject(t *testing.T) {
	// Two quads share one oppositely traversed edge. Canonical source vertices
	// are retained, rather than replaced by a tolerance-based positional weld.
	base := OrthogonalSplineOpenShell{Faces: make([]OrthogonalSplineFace, 2), Uses: []OrthogonalShellUse{{0, 1, 1, 2, true, 0}, {0, 2, 2, 3, true, 0}, {0, 3, 3, 4, true, 0}, {0, 4, 4, 1, true, 0}, {1, 2, 2, 3, false, 0}, {1, 5, 2, 5, true, 0}, {1, 6, 5, 6, true, 0}, {1, 7, 6, 3, true, 0}}}
	if e := openShellIncidence(base); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"sense", "disconnected", "third-owner", "closed"} {
		t.Run(kind, func(t *testing.T) {
			s := base
			s.Uses = append([]OrthogonalShellUse(nil), base.Uses...)
			switch kind {
			case "sense":
				s.Uses[4].Forward = true
			case "disconnected":
				s.Uses[4].EdgeID = 8
			case "third-owner":
				s.Uses = append(s.Uses, s.Uses[4])
			case "closed":
				s.Uses = []OrthogonalShellUse{{0, 1, 1, 2, true, 0}, {1, 1, 1, 2, false, 0}}
			}
			if e := openShellIncidence(s); e == nil {
				t.Fatal("invalid incidence accepted")
			}
		})
	}
}

func TestOrthogonalOpenShellSourceReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/orthogonal-open-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{
		{"#16 = OPEN_SHELL('',(#17,#127));", "#16 = OPEN_SHELL('',(#17,#17));"},
		{"#46 = SURFACE_CURVE('',#47,(#50,#57),.PCURVE_S1.);", "#46 = SURFACE_CURVE('',#47,(#50),.PCURVE_S1.);"},
		{"#46 = SURFACE_CURVE('',#47,(#50,#57),.PCURVE_S1.);", "#46 = SURFACE_CURVE('',#47,(#50,#50),.PCURVE_S1.);"},
		{"#57 = PCURVE('',#58,#63);", "#57 = PCURVE('',#31,#63);"},
		{"#42 = ORIENTED_EDGE('',*,*,#43,.T.);", "#42 = ORIENTED_EDGE('',*,*,#43,.F.);"},
		{"OPEN_SHELL('',(#17,#127))", "CLOSED_SHELL('',(#17,#127))"},
	} {
		bad := strings.Replace(string(raw), pair[0], pair[1], 1)
		if bad == string(raw) {
			t.Fatal("missed mutation", pair)
		}
		if _, e := ReadOrthogonalSplineOpenShells(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
}
func FuzzOrthogonalOpenShell(f *testing.F) {
	raw, e := os.ReadFile("testdata/orthogonal-open-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	holed, e := os.ReadFile("testdata/orthogonal-holed-open-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(holed)
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxVertices: 128, MaxEdgeUses: 128})
	})
}
