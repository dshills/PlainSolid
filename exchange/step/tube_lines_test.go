package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func tubeLineFixture(t testing.TB, kind string) TubeLine {
	t.Helper()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	if kind == "curved" {
		a, e = geom.NewBSplineSurface(2, 1, 3, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .5}, {X: .5, Y: 1}, {X: .75, Z: 1}, {X: .75, Y: 1, Z: 1}}, []float64{0, 0, 0, 1, 1, 1}, k, nil)
		if e != nil {
			t.Fatal(e)
		}
	}
	bp := []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 2}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 2}}
	if kind == "nonbinary" {
		bp = []geom.Vec3{{Z: -1}, {Y: 1, Z: 2}, {X: 1, Z: -1}, {X: 1, Y: 1, Z: 2}}
	}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, bp, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch := func(s *geom.BSplineSurface) intersect.Patch {
		return intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
	}
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	segments, e := intersect.RebuildTubeCurves(context.Background(), intersect.TubeRecipeData{A: patch(a), B: patch(b), Cells: []intersect.TubeRecipeCell{{DomainA: d, DomainB: d}}, FreeAxis: 0})
	if e != nil || len(segments) != 1 {
		t.Fatal(e, len(segments))
	}
	start := [4]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(1, 4), big.NewRat(1, 3)}
	end := [4]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(3, 4), big.NewRat(1, 3)}
	if kind == "curved" {
		end[3] = big.NewRat(2, 3)
	}
	if kind == "nonbinary" {
		start[1] = big.NewRat(1, 3)
		end[1] = big.NewRat(1, 3)
	}
	return TubeLine{Name: "Certified line", Segment: segments[0], Start: start, End: end}
}
func TestWriteTubeLines(t *testing.T) {
	line := tubeLineFixture(t, "")
	m := TubeLineModel{Lines: []TubeLine{line}, LengthUnit: .001}
	var b bytes.Buffer
	if e := WriteTubeLines(context.Background(), &b, m, TubeLineOptions{}); e != nil {
		t.Fatal(e)
	}
	read, e := ReadCurves(&b, Limits{})
	if e != nil || len(read.Curves) != 1 || read.LengthUnit != .001 || read.Curves[0].Name != line.Name {
		t.Fatal(read, e)
	}
	g := read.Curves[0].Geometry
	if g.Degree() != 1 {
		t.Fatal(g.Degree())
	}
	for _, u := range []float64{0, .25, .5, 1} {
		p, e := g.Eval(u)
		if e != nil || p != (geom.Vec3{X: .25 + .5*u, Y: .5}) {
			t.Fatal(p, e)
		}
	}
	if line.Start[3].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("input rationals mutated")
	}
}
func TestWriteTubeLinesRejection(t *testing.T) {
	for _, kind := range []string{"curved", "nonbinary", "wrong", "partial", "reverse", "zero", "cancel", "budget", "count"} {
		t.Run(kind, func(t *testing.T) {
			line := tubeLineFixture(t, kind)
			m := TubeLineModel{Lines: []TubeLine{line}, LengthUnit: .001}
			o := TubeLineOptions{}
			ctx := context.Background()
			switch kind {
			case "wrong":
				m.Lines[0].End[1] = big.NewRat(3, 4)
			case "partial":
				m.Lines[0].End[0] = big.NewRat(1, 2)
			case "reverse":
				m.Lines[0].Start, m.Lines[0].End = m.Lines[0].End, m.Lines[0].Start
			case "zero":
				m.Lines[0].Segment = intersect.TubeSegment{}
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "budget":
				o.Proof.MaxOperations = 1
			case "count":
				o.MaxSegments = 1
				m.Lines = append(m.Lines, line)
			}
			var b bytes.Buffer
			e := WriteTubeLines(ctx, &b, m, o)
			if e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
			if kind == "nonbinary" && !errors.Is(e, geom.ErrUnrepresentable) {
				t.Fatal(e)
			}
		})
	}
	// Validation of a later segment must not expose an earlier partial geometry.
	good := tubeLineFixture(t, "")
	bad := tubeLineFixture(t, "curved")
	var b bytes.Buffer
	if e := WriteTubeLines(context.Background(), &b, TubeLineModel{Lines: []TubeLine{good, bad}, LengthUnit: .001}, TubeLineOptions{}); e == nil || b.Len() != 0 {
		t.Fatal(e, b.Len())
	}
}
func TestOCCTTubeLines(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_ORACLE") == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT oracle")
	}
	line := tubeLineFixture(t, "")
	var b bytes.Buffer
	if e := WriteTubeLines(context.Background(), &b, TubeLineModel{Lines: []TubeLine{line}, LengthUnit: .001}, TubeLineOptions{}); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "line.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "measure.json")
	stepOracle(t, out, "step-curves", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Edges  int
		Length float64
	}
	if e = json.Unmarshal(raw, &got); e != nil || got.Edges != 1 || math.Abs(got.Length-.5) > 1e-12 {
		t.Fatal(got, e)
	}
}

func TestWriteTubeLinesReversedTraversalAndIO(t *testing.T) {
	line := tubeLineFixture(t, "")
	recipe, _, _ := line.Segment.TrimA().Recipe()
	data := recipe.Data()
	data.Oriented = []intersect.TubeDirection{{FreeAxis: 0, Reversed: true}}
	segments, e := intersect.RebuildTubeCurves(context.Background(), data)
	if e != nil || len(segments) != 1 {
		t.Fatal(e)
	}
	line.Segment = segments[0]
	line.Start, line.End = line.End, line.Start
	m := TubeLineModel{Lines: []TubeLine{line}, LengthUnit: .001}
	var b bytes.Buffer
	if e = WriteTubeLines(context.Background(), &b, m, TubeLineOptions{}); e != nil {
		t.Fatal(e)
	}
	read, e := ReadCurves(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := read.Curves[0].Geometry.Eval(0)
	if e != nil || p != (geom.Vec3{X: .75, Y: .5}) {
		t.Fatal(p, e)
	}
	if e = WriteTubeLines(context.Background(), failWriter{}, m, TubeLineOptions{}); e == nil {
		t.Fatal("writer failure lost")
	}
	if e = WriteTubeLines(context.Background(), shortWriter{}, m, TubeLineOptions{}); e == nil {
		t.Fatal("short write lost")
	}
}
