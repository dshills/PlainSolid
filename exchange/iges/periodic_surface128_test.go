package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"plainsolid/geom"
)

type periodic128Case struct {
	name             string
	flags            [2]bool
	expanded, cyclic [2]int
	degrees          [2]int
	bounds           [4]string
}

var periodic128Cases = []periodic128Case{
	{"u", [2]bool{true, false}, [2]int{9, 4}, [2]int{7, 4}, [2]int{3, 2}, [4]string{"-5/4", "27/4", "5/2", "15/2"}},
	{"v", [2]bool{false, true}, [2]int{4, 9}, [2]int{4, 7}, [2]int{2, 3}, [4]string{"5/2", "15/2", "-5/4", "27/4"}},
	{"uv", [2]bool{true, true}, [2]int{9, 7}, [2]int{7, 5}, [2]int{3, 2}, [4]string{"-5/4", "27/4", "48/5", "84/5"}},
}

func TestReadExactPeriodicSurface128(t *testing.T) {
	for _, tc := range periodic128Cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := periodic128Fixture(t, tc.name)
			m, e := Read(bytes.NewReader(raw), Limits{MaxControlPoints: tc.expanded[0] * tc.expanded[1]})
			if e != nil || len(m.Entities) != 1 || m.LengthUnit != .001 {
				t.Fatal("periodic author did not import as one raw millimetre support", e)
			}
			entity := m.Entities[0]
			s := entity.Surface
			exact, e := s.ExactPeriodicGeometry()
			if e != nil {
				t.Fatal(e)
			}
			d := s.Geometry.Data()
			u, v, net := exact.Data()
			if [2]bool{s.PeriodicU, s.PeriodicV} != tc.flags || [2]bool{s.ClosedU, s.ClosedV} != tc.flags ||
				[2]int{d.CountU, d.CountV} != tc.expanded || [2]int{u.Count, v.Count} != tc.cyclic ||
				[2]int{u.Degree, v.Degree} != tc.degrees || len(net) != tc.cyclic[0]*tc.cyclic[1] {
				t.Fatal("changed original periodic flag, degree, or net dimensions")
			}

			var bounds [4]*big.Rat
			for i, text := range tc.bounds {
				bounds[i], _ = new(big.Rat).SetString(text)
			}
			ctx := context.Background()
			eval := func(parameter [2]*big.Rat) geom.RationalPeriodicSurfaceJet {
				t.Helper()
				jet, err := exact.EvaluateRational(ctx, parameter[0], parameter[1], geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(parameter, err)
				}
				return jet
			}
			for i, axis := range [2]geom.ExactTensorSplineAxis{u, v} {
				start, end := bounds[2*i], bounds[2*i+1]
				lo, hi := axis.Degree, axis.Count
				if axis.Periodic {
					lo, hi = 0, len(axis.Knots)-1
				}
				if axis.Periodic != tc.flags[i] || axis.Knots[lo].Cmp(start) != 0 || axis.Knots[hi].Cmp(end) != 0 {
					t.Fatal("lost exact original physical domain", i, axis.Knots)
				}
				parameters := [2]*big.Rat{
					new(big.Rat).Mul(new(big.Rat).Add(bounds[0], bounds[1]), big.NewRat(1, 2)),
					new(big.Rat).Mul(new(big.Rat).Add(bounds[2], bounds[3]), big.NewRat(1, 2)),
				}
				if !axis.Periodic {
					parameters[i] = new(big.Rat).Sub(start, big.NewRat(1, 8))
					if _, err := exact.EvaluateRational(ctx, parameters[0], parameters[1], geom.SurfacePatchOptions{}); !errors.Is(err, geom.ErrDomain) {
						t.Fatal("ordinary axis accepted wrapped evaluation", err)
					}
					continue
				}
				period := new(big.Rat).Sub(end, start)
				parameters[i] = start
				first := eval(parameters)
				parameters[i] = end
				second := eval(parameters)
				owner := [2]geom.ExactSurfaceKnotSpan{second.U, second.V}[i]
				if !samePeriodic128Jet(first, second) || owner.Cycle.Cmp(big.NewInt(1)) != 0 || owner.Domain.Lower.Cmp(end) != 0 {
					t.Fatal("lost exact seam derivative or right-hand owner", first, second)
				}
				parameters[i] = new(big.Rat).Add(start, big.NewRat(1, 8))
				inside := eval(parameters)
				parameters[i].Sub(parameters[i], new(big.Rat).Mul(big.NewRat(3, 1), period))
				repeated := eval(parameters)
				owner = [2]geom.ExactSurfaceKnotSpan{repeated.U, repeated.V}[i]
				if !samePeriodic128Jet(inside, repeated) || owner.Cycle.Cmp(big.NewInt(-3)) != 0 {
					t.Fatal("lost file-unit derivatives or physical period on another cycle")
				}
			}
			if tc.name == "uv" {
				// The pinned writer rotates the original V seam and writes
				// period 7.2 exactly in the translated decimal knot lattice.
				// A binary64 subtraction does not preserve that source value.
				if new(big.Rat).Sub(v.Knots[len(v.Knots)-1], v.Knots[0]).Cmp(big.NewRat(36, 5)) != 0 ||
					net[v.Count][3].Cmp(big.NewRat(1, 1)) != 0 || net[v.Count+1][3].Cmp(big.NewRat(11, 10)) != 0 {
					t.Fatal("rounded the source lattice or a nonbinary pole weight", v.Knots)
				}
			}
			for _, uv := range [][2]float64{{0, 0}, {1, 1}, {.28125, .6875}} {
				p, err := entity.EvalSurface(uv[0], uv[1])
				if err != nil {
					t.Fatal("bounded surface evaluation failed", err)
				}
				parameters := [2]*big.Rat{}
				for i, value := range uv {
					width := new(big.Rat).Sub(bounds[2*i+1], bounds[2*i])
					parameters[i] = new(big.Rat).Add(bounds[2*i], width.Mul(width, rat(value)))
				}
				jet := eval(parameters)
				checkPeriodic128Vector(t, jet.Point, [3]float64{p.X, p.Y, p.Z})
				uf, _ := parameters[0].Float64()
				vf, _ := parameters[1].Float64()
				du, dv, err := s.Geometry.Partials(uf, vf)
				if err != nil {
					t.Fatal("independent expanded-net partials failed", err)
				}
				checkPeriodic128Vector(t, jet.Du, [3]float64{du.X, du.Y, du.Z})
				checkPeriodic128Vector(t, jet.Dv, [3]float64{dv.X, dv.Y, dv.Z})
			}

			edited := *s
			edited.PeriodicU, edited.PeriodicV = false, false
			if _, err := edited.ExactPeriodicGeometry(); err == nil {
				t.Fatal("relabelled surface kept its exact source proof")
			}
		})
	}
}

func TestReadExactPeriodicSurface128Reject(t *testing.T) {
	raw := periodic128Fixture(t, "u")
	p := periodic128Parameters(t, raw)
	refuse := func(raw []byte, l Limits, reason string) {
		t.Helper()
		m, err := Read(bytes.NewReader(raw), l)
		if err == nil || !strings.Contains(err.Error(), reason) || !reflect.DeepEqual(m, Model{}) {
			t.Fatalf("unqualified source returned a partial model or wrong refusal: %v", err)
		}
	}
	for _, test := range []struct {
		index         int
		value, reason string
	}{
		{5, "0", "periodic surface flags"},
		{9, "1", "periodic surface flags"},
		{7, "1", "inconsistent exact polynomial weight"},
		{10, "-4.5000000000000001", "periodic knot translation"},
		{37, "1.00000000000000001", "repeated periodic surface pole or weight"},
		{87, "7.00000000000000001", "repeated periodic surface pole or weight"},
		{len(p) - 4, "-1.24999999999999999", "full original active interval"},
		{3, "2", "associativity/property pointers unsupported"},
	} {
		values := slices.Clone(p)
		if values[test.index] == test.value {
			t.Fatal("ineffective source mutant", test.index)
		}
		values[test.index] = test.value
		refuse(periodic128Records(t, values), Limits{}, test.reason)
	}
	refuse(raw, Limits{MaxControlPoints: 35}, "control-point limit")
	refuse(raw, Limits{MaxBytes: int64(len(raw) - 1)}, "byte limit")
	duplicate := string(periodic128Records(t, p, p))
	duplicate = replaceField(duplicate, 'D', 3, 1, "1")
	refuse([]byte(duplicate), Limits{}, "overlapping or inconsistent parameter pointers")
	var topology topologyAuthor
	topology.add(128, 0, strings.Join(p, ",")+";")
	topology.add(110, 0, "110,0,0,0,1,0,0;")
	topology.add(110, 0, "110,0,0,0,1,1,0;")
	topology.add(142, 0, "142,0,1,3,5,0;")
	attached, err := topology.file(.001)
	if err != nil {
		t.Fatal(err)
	}
	carrier := string(attached)
	for i := range 4 {
		carrier = replaceField(carrier, 'D', 2*i+1, 8, "00000000")
	}
	refuse([]byte(carrier), Limits{}, "periodic spline surface topology unsupported")

	values := slices.Clone(p)
	values[8] = "0"
	ordinary, err := Read(bytes.NewReader(periodic128Records(t, values)), Limits{})
	if err != nil || len(ordinary.Entities) != 1 || !ordinary.Entities[0].Surface.ClosedU {
		t.Fatal("ordinary closed source changed behavior", err)
	}
	if _, err := ordinary.Entities[0].Surface.ExactPeriodicGeometry(); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("ordinary closed metadata introduced a periodic chart", err)
	}
	if m, err := ReadCompositeCurves(bytes.NewReader(raw), Limits{}); err == nil || !reflect.DeepEqual(m, Model{}) {
		t.Fatal("standalone wire mode accepted periodic surface support", err)
	}
}

func TestReadExactPeriodicSurface128PinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT periodic surface author and reader")
	}
	for _, tc := range periodic128Cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name+".iges")
			authored := periodic128OracleReport(t, oracle, "write-"+tc.name, path)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(periodic128Parameters(t, raw), periodic128Parameters(t, periodic128Fixture(t, tc.name))) {
				t.Fatal("fresh OCCT writer changed its exact periodic parameter record")
			}
			readback := periodic128OracleReport(t, oracle, "read", path)
			if readback.ExpandedCounts != tc.expanded ||
				[2]bool{readback.RawPeriodic[0] == 1, readback.RawPeriodic[1] == 1} != tc.flags ||
				readback.RawClosed != readback.RawPeriodic {
				t.Fatal("OCCT did not find the original periodic flags and expanded control owners")
			}
			m, err := Read(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			s, err := m.Entities[0].Surface.ExactPeriodicGeometry()
			if err != nil {
				t.Fatal(err)
			}
			for _, oracle := range []periodic128Report{authored, readback} {
				if oracle.Counts != tc.cyclic || [2]bool{oracle.Periodic[0] == 1, oracle.Periodic[1] == 1} != tc.flags || len(oracle.Samples) != 9 {
					t.Fatal("pinned OCCT used a different chart or lost periodicity", oracle)
				}
				for _, point := range oracle.Samples {
					jet, err := s.EvaluateRational(context.Background(), rat(point.U), rat(point.V), geom.SurfacePatchOptions{})
					if err != nil {
						t.Fatal(point, err)
					}
					checkPeriodic128Vector(t, jet.Point, point.Point)
					checkPeriodic128Vector(t, jet.Du, point.Du)
					checkPeriodic128Vector(t, jet.Dv, point.Dv)
				}
			}
		})
	}
}

func samePeriodic128Jet(a, b geom.RationalPeriodicSurfaceJet) bool {
	for i := range a.Point {
		if a.Point[i].Cmp(b.Point[i]) != 0 || a.Du[i].Cmp(b.Du[i]) != 0 || a.Dv[i].Cmp(b.Dv[i]) != 0 {
			return false
		}
	}
	return true
}

func checkPeriodic128Vector(t *testing.T, got [3]*big.Rat, want [3]float64) {
	t.Helper()
	for i, q := range got {
		x, _ := q.Float64()
		if math.Abs(x-want[i]) > 3e-11*math.Max(1, math.Abs(want[i])) {
			t.Fatalf("wrong physical derivative or point: %v vs %v", got, want)
		}
	}
}

func periodic128Fixture(t testing.TB, name string) []byte {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-periodic-128-" + name + ".iges")
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func periodic128Parameters(t testing.TB, raw []byte) []string {
	t.Helper()
	var text strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if len(line) == 80 && line[72] == 'P' {
			text.WriteString(line[:64])
		}
	}
	budget := 1000
	values, err := parseParameters(text.String(), ',', ';', &budget)
	if err != nil {
		t.Fatal(err)
	}
	fields := make([]string, len(values))
	for i, p := range values {
		fields[i] = p.text
	}
	return fields
}

func periodic128Records(t *testing.T, entities ...[]string) []byte {
	t.Helper()
	var a topologyAuthor
	for _, fields := range entities {
		a.add(128, 0, strings.Join(fields, ",")+";")
	}
	raw, err := a.file(.001)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for i := range entities {
		source = replaceField(source, 'D', 2*i+1, 8, "00000000")
	}
	return []byte(source)
}

type periodic128Report struct {
	Periodic, Counts                       [2]int
	RawPeriodic, RawClosed, ExpandedCounts [2]int
	Bounds                                 [4]float64
	Samples                                []struct {
		U, V          float64
		Point, Du, Dv [3]float64
	}
}

func periodic128OracleReport(t *testing.T, oracle, mode, path string) periodic128Report {
	t.Helper()
	out, err := exec.Command(oracle, mode, path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT %s: %v\n%s", mode, err, out)
	}
	var report periodic128Report
	for _, line := range bytes.Split(out, []byte("\n")) {
		if bytes.HasPrefix(line, []byte(`{"rawPeriodic":`)) || bytes.HasPrefix(line, []byte(`{"periodic":`)) {
			if err := json.Unmarshal(line, &report); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(report.Samples) == 0 {
		t.Fatalf("missing pinned OCCT surface report: %s", out)
	}
	return report
}
