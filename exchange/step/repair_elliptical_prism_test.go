package step

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"

	"strings"
	"testing"
)

func ellipticalPrismFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-elliptical-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRepairEllipticalPrism(t *testing.T) {
	t.Parallel()
	raw := ellipticalPrismFixture(t)
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	m, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(raw), Limits{}, o)
	if e != nil {
		t.Fatal(e)
	}
	if m.SourceFaceIDs != ([3]int64{17, 108, 112}) || m.SourceRimPCurveIDs != ([2][2]int64{{61, 68}, {89, 96}}) || !m.Faces[0].Reversed || !m.Faces[1].Reversed || m.Faces[2].Reversed {
		t.Fatal(m)
	}
	for _, u := range []float64{0, .375, math.Pi, 2 * math.Pi} {
		for _, pair := range []struct {
			cap int
			v   float64
		}{{1, 0}, {2, -3}} {
			p, e := m.Faces[0].Surface.Eval(u, pair.v)
			if e != nil {
				t.Fatal(e)
			}
			q, e := m.Faces[pair.cap].Surface.Eval(4*math.Cos(u), 2*math.Sin(u))
			if e != nil || p.Distance(q) > 1e-13 {
				t.Fatal(p, q, e)
			}
		}
	}
	var b bytes.Buffer
	if e = WriteEllipticalPrismShellSource(&b, m.Source, Limits{}, o); e != nil {
		t.Fatal(e)
	}
	again, e := RepairRoundedEllipticalPrismShell(&b, Limits{}, o)
	if e != nil || again.Side.Height != 3 {
		t.Fatal(e)
	}

	for _, x := range [][2]string{{"(#17,#108,#112)", "(#17,#108,#108)"}, {"#68 = PCURVE('',#69,#74);", "#68 = PCURVE('',#97,#74);"}, {"#75 = ELLIPSE('',#76,4.,2.);", "#75 = ELLIPSE('',#76,4.,1.);"}, {"#71 = CARTESIAN_POINT('',(1.,5.,3.));", "#71 = CARTESIAN_POINT('',(1.,4.,3.));"}, {"#111 = ORIENTED_EDGE('',*,*,#82,.T.);", "#111 = ORIENTED_EDGE('',*,*,#54,.T.);"}, {"#108 = ADVANCED_FACE('',(#109),#97,.F.);", "#108 = ADVANCED_FACE('',(#109),#97,.T.);"}, {"#18 = FACE_BOUND('',#19,.F.);", "#18 = FACE_BOUND('',#19,.T.);"}, {"#80 = ORIENTED_EDGE('',*,*,#21,.F.);", "#80 = ORIENTED_EDGE('',*,*,#21,.T.);"}, {"6.28318530718", "6.2831853071"}} {
		s := strings.Replace(string(raw), x[0], x[1], 1)
		if s == string(raw) {
			t.Fatal("stale mutation", x)
		}
		q, e := RepairRoundedEllipticalPrismShell(strings.NewReader(s), Limits{}, o)
		if e == nil || q.Source != nil {
			t.Fatal("accepted", x)
		}
	}
}
func FuzzRepairEllipticalPrism(f *testing.F) {
	f.Add(ellipticalPrismFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		_, _ = RepairRoundedEllipticalPrismShell(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 121, MaxValues: 4096, MaxDepth: 16}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	})
}

func TestRepairEllipticalPrismRenumberedHeight(t *testing.T) {
	t.Parallel()
	s := string(ellipticalPrismFixture(t))
	s = strings.ReplaceAll(s, "(1.,5.,", "(1.,7.,")
	s = strings.ReplaceAll(s, "(0.,-3.)", "(0.,-5.)")
	re := regexp.MustCompile(`#([0-9]+)`)
	s = re.ReplaceAllStringFunc(s, func(x string) string { n, _ := strconv.Atoi(x[1:]); return fmt.Sprintf("#%d", n+1000) })
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	m, e := RepairRoundedEllipticalPrismShell(strings.NewReader(s), Limits{}, o)
	if e != nil || m.Side.Height != 5 || m.SourceFaceIDs != ([3]int64{1017, 1108, 1112}) || m.SourceEdgeIDs != ([3]int64{1021, 1054, 1082}) {
		t.Fatal(m, e)
	}
	for _, l := range []Limits{{MaxEntities: 120}, {MaxBytes: 128}, {MaxValues: 64}, {MaxDepth: 1}} {
		q, e := RepairRoundedEllipticalPrismShell(strings.NewReader(s), l, o)
		if e == nil || q.Source != nil {
			t.Fatal("limit", l)
		}
	}
	if _, e := RepairRoundedEllipticalPrismShell(strings.NewReader(s), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-14}); e == nil {
		t.Fatal("small tolerance")
	}
	var out bytes.Buffer
	bad := m.Source
	en := bad.Entities[1075]
	en.Records[0].Args[2].Text = "5"
	bad.Entities[1075] = en
	if WriteEllipticalPrismShellSource(&out, bad, Limits{}, o) == nil || out.Len() != 0 {
		t.Fatal("nonatomic source validation")
	}
}
