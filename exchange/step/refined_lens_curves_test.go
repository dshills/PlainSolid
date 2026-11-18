package step

import (
	"bytes"
	"errors"
	"math"
	"os"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func refinedCurveLensFixture(t testing.TB) RefinedSplineLens {
	t.Helper()
	m := splineLensFixture(t)
	r := 105. / 512
	m.Radii = geom.Vec2{X: r, Y: r}
	sq := []float64{.25, -.25, .25}
	for side, sign := range []float64{1, -1} {
		p := []geom.Vec3{}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 2 + sign*(r*r-sq[i]-sq[j])/2})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if side == 0 {
			m.Upper = s
		} else {
			m.Lower = s
		}
	}
	uv, e := curvedEllipseUV(QuadraticOval{Center: m.Center, Radii: m.Radii, QuarterWeight: .5}, 0)
	if e != nil {
		t.Fatal(e)
	}
	out := RefinedSplineLens{PartitionedSplineLens: PartitionedSplineLens{Body: m}}
	for _, c := range uv {
		var curves [3]*geom.BSpline
		for role, k := range []float64{.75, .5, .25} {
			p := c.Poles()
			if role == 0 {
				for i := range p {
					p[i].Z = 2
				}
			}
			source, e := geom.NewBSpline(2, p, c.Knots(), c.Weights())
			if e != nil {
				t.Fatal(e)
			}
			refined, e := source.InsertKnot(k, 1)
			if e != nil {
				t.Fatal(e)
			}
			curves[role] = refined
		}
		out.Curves = append(out.Curves, curves)
	}
	return out
}
func TestRefinedLensCurves(t *testing.T) {
	t.Parallel()
	m := refinedCurveLensFixture(t)
	var b bytes.Buffer
	if e := WriteRefinedSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadRefinedSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, c := range m.Curves {
		for role, a := range c {
			if !reflect.DeepEqual(a.Poles(), got.Curves[i][role].Poles()) || !reflect.DeepEqual(a.Knots(), got.Curves[i][role].Knots()) || !reflect.DeepEqual(a.Weights(), got.Curves[i][role].Weights()) {
				t.Fatal("source curve basis changed")
			}
		}
	}
	if _, e = ReadPartitionedSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("Bezier reader broadened")
	}
}
func TestIndependentRefinedLensCurves(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/refined-curve-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRefinedSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteRefinedSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	copy, e := ReadRefinedSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, curves := range m.Curves {
		for owner, c := range curves {
			d := copy.Curves[i][owner]
			if !reflect.DeepEqual(c.Poles(), d.Poles()) || !reflect.DeepEqual(c.Weights(), d.Weights()) || !reflect.DeepEqual(c.Knots(), d.Knots()) {
				t.Fatal("independent source curve changed")
			}
		}
	}

}

func TestRefinedLensCurveChangesRejected(t *testing.T) {
	t.Parallel()
	for _, role := range []int{0, 1, 2} {
		m := refinedCurveLensFixture(t)
		c := m.Curves[0][role]
		p := c.Poles()
		p[1].X = math.Nextafter(p[1].X, math.Inf(1))
		var e error
		m.Curves[0][role], e = geom.NewBSpline(c.Degree(), p, c.Knots(), c.Weights())
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteRefinedSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
			t.Fatal("changed owner/model accepted", role, e)
		}
	}
	m := refinedCurveLensFixture(t)
	var b bytes.Buffer
	if e := WriteRefinedSplineLensBody(&b, m, CurvedOvalOptions{MaxOperations: 64}); e == nil || b.Len() != 0 {
		t.Fatal("budget", e)
	}
}
func TestRefinedLensWeightNormalizationRejected(t *testing.T) {
	t.Parallel()
	m := refinedCurveLensFixture(t)
	var b bytes.Buffer
	if e := WriteRefinedSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	changed := false
	for _, id := range f.Order {
		en := f.Entities[id]
		for i, r := range en.Records {
			if r.Type == "RATIONAL_B_SPLINE_CURVE" {
				r.Args = append([]Value(nil), r.Args...)
				r.Args[0].Items = append([]Value(nil), r.Args[0].Items...)
				r.Args[0].Items[len(r.Args[0].Items)-1] = Value{Kind: Number, Text: "3."}
				en.Records = append([]Record(nil), en.Records...)
				en.Records[i] = r
				f.Entities[id] = en
				changed = true
				break
			}
		}
		if changed {
			break
		}
	}
	if !changed {
		t.Fatal("weight mutation missed")
	}
	if _, e = ReadRefinedSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("lossy normalization", e)
	}
}
