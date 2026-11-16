package iges

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestWriteExactPeriodicSurface128(t *testing.T) {
	for _, tc := range periodic128Cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := periodic128Fixture(t, tc.name)
			m, err := Read(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			want, err := m.Entities[0].Surface.ExactPeriodicGeometry()
			if err != nil {
				t.Fatal(err)
			}
			m.Entities[0].Label, m.Entities[0].Subscript = "CYCLIC", 2
			var out bytes.Buffer
			if err := Write(&out, m); err != nil {
				t.Fatal(err)
			}
			compareExpandedPeriodic128Parameters(t, raw, out.Bytes())
			got, err := Read(bytes.NewReader(out.Bytes()), Limits{})
			if err != nil || len(got.Entities) != 1 || got.LengthUnit != m.LengthUnit ||
				got.Entities[0].Label != "CYCLIC" || got.Entities[0].Subscript != 2 {
				t.Fatal("strict reopen changed periodic type 128 or its directory", err)
			}
			compareExactPeriodic128Surfaces(t, want, got.Entities[0].Surface)
			var repeated bytes.Buffer
			if err := Write(&repeated, got); err != nil || !bytes.Equal(out.Bytes(), repeated.Bytes()) {
				t.Fatal("periodic type-128 canonical export changed after reopen", err)
			}
		})
	}
}

func TestNewExactPeriodicSurface128(t *testing.T) {
	_, independent := editedPeriodic128Model(t)
	var carrier bytes.Buffer
	if err := Write(&carrier, independent); err != nil {
		t.Fatal(err)
	}
	model, err := Read(bytes.NewReader(carrier.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	native, err := independent.Entities[0].Surface.ExactPeriodicGeometry()
	if err != nil {
		t.Fatal(err)
	}
	compareExactPeriodic128Surfaces(t, native, model.Entities[0].Surface)
}

func TestWriteExactPeriodicSurface128Reject(t *testing.T) {
	ctx := context.Background()
	raw := periodic128Fixture(t, "u")
	checked, err := Read(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := checked.Entities[0].Surface.ExactPeriodicGeometry()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, message string
		change        func(*geom.ExactTensorSplineAxis, *geom.ExactTensorSplineAxis, [][4]*big.Rat)
	}{
		{"nonterminating Cartesian pole", "bounded exact decimal", func(_, _ *geom.ExactTensorSplineAxis, c [][4]*big.Rat) {
			c[0][0], c[0][3] = big.NewRat(1, 1), big.NewRat(3, 1)
		}},
		{"nonterminating weight", "bounded exact decimal", func(_, _ *geom.ExactTensorSplineAxis, c [][4]*big.Rat) {
			for i, q := range c[0] {
				c[0][i] = new(big.Rat).Mul(q, big.NewRat(1, 13))
			}
		}},
		{"nonterminating seam phase", "bounded exact decimal", func(u, _ *geom.ExactTensorSplineAxis, _ [][4]*big.Rat) {
			for _, q := range u.Knots {
				q.Add(q, big.NewRat(1, 3))
			}
		}},
		{"coincident binary64 stations", "collapse in binary64 transfer", func(_, v *geom.ExactTensorSplineAxis, _ [][4]*big.Rat) {
			v.Knots[3], _ = new(big.Rat).SetString("2.50000000000000000001")
		}},
		{"rational weights treated as polynomial", "rational weights lost", func(_, _ *geom.ExactTensorSplineAxis, c [][4]*big.Rat) {
			for i := range c {
				for j := 0; j < 3; j++ {
					c[i][j].Quo(c[i][j], c[i][3])
				}
				c[i][3].SetInt64(1)
			}
			w := big.NewRat(100000000001, 100000000000)
			for j := 0; j < 4; j++ {
				c[0][j].Mul(c[0][j], w)
			}
		}},
		{"weight below the consumer floor", "rational weights lost", func(_, _ *geom.ExactTensorSplineAxis, c [][4]*big.Rat) {
			for j := 0; j < 4; j++ {
				c[0][j].Mul(c[0][j], big.NewRat(1, 10000000000))
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, v, c := exact.Data()
			tc.change(&u, &v, c)
			input, err := geom.NewExactPeriodicSplineSurface(ctx, u, v, c, geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal("test must provide a valid native exact shape", err)
			}
			if bad, err := NewExactPeriodicSurface128(input); bad != nil || !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), tc.message) {
				t.Fatal("writer accepted unsupported exact conversion", err)
			}
		})
	}
	if s, err := NewExactPeriodicSurface128(nil); s != nil || !errors.Is(err, geom.ErrInvalid) {
		t.Fatal("empty exact tensor became a periodic surface", err)
	}

	p := periodic128Parameters(t, raw)
	// The source reader accepts longer exact decimals. Change both
	// expanded owners of the same weight, preserving proved periodicity.
	p[30] = "1." + strings.Repeat("0", 68) + "1"
	p[37] = p[30]
	long, err := Read(bytes.NewReader(periodic128Records(t, p)), Limits{})
	if err != nil {
		t.Fatal("target input must be a valid exact periodic source", err)
	}
	long.Entities = append([]Entity{{Point: &geom.Vec3{X: 4}}}, long.Entities...)
	out := bytes.NewBufferString("unchanged")
	if err := Write(out, long); !errors.Is(err, geom.ErrUnrepresentable) || out.String() != "unchanged" {
		t.Fatal("overlong exact data published a partial file", err)
	}
	tr := geom.Translation(geom.Vec3{Y: 4})
	checked.Entities[0].Transform = &tr
	if err := Write(out, checked); err == nil || out.String() != "unchanged" {
		t.Fatal("unqualified transformed periodic output changed the writer", err)
	}
}

func TestWriteExactPeriodicSurface128UnsupportedBasis(t *testing.T) {
	cyclic := func(count, degree int) geom.ExactTensorSplineAxis {
		axis := geom.ExactTensorSplineAxis{Degree: degree, Count: count, Periodic: true, Knots: make([]*big.Rat, count+1)}
		for i := range axis.Knots {
			axis.Knots[i] = big.NewRat(int64(i), 1)
		}
		return axis
	}
	ordinary := func(discontinuous bool) geom.ExactTensorSplineAxis {
		count, values := 2, []int64{0, 0, 2, 2}
		if discontinuous {
			count, values = 4, []int64{0, 0, 1, 1, 2, 2}
		}
		axis := geom.ExactTensorSplineAxis{Degree: 1, Count: count, Knots: make([]*big.Rat, len(values))}
		for i, v := range values {
			axis.Knots[i] = big.NewRat(v, 1)
		}
		return axis
	}
	for _, tc := range []struct {
		name    string
		u, v    geom.ExactTensorSplineAxis
		cause   error
		message string
	}{
		{"degree", cyclic(27, 26), ordinary(false), geom.ErrUnrepresentable, "degree exceeds transfer limit"},
		{"shape-changing knot multiplicity", cyclic(3, 1), ordinary(true), geom.ErrUnrepresentable, "interior multiplicity exceeds transfer degree"},
		{"expanded owner budget", cyclic(64, 1), cyclic(64, 1), geom.ErrConvergence, "expanded periodic type-128 control point limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controls := make([][4]*big.Rat, tc.u.Count*tc.v.Count)
			for i := range controls {
				u, v := int64(i/tc.v.Count), int64(i%tc.v.Count)
				controls[i] = [4]*big.Rat{big.NewRat(u, 1), big.NewRat(v, 1), big.NewRat(u*v, 16), big.NewRat(1, 1)}
			}
			g, err := geom.NewExactPeriodicSplineSurface(context.Background(), tc.u, tc.v, controls, geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal("test must construct a valid native basis", err)
			}
			if s, err := NewExactPeriodicSurface128(g); s != nil || !errors.Is(err, tc.cause) || !strings.Contains(err.Error(), tc.message) {
				t.Fatal("unsupported native basis escaped the carrier limits", err)
			}
		})
	}
}

func TestWriteExactPeriodicSurface128PinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT type-128 reader")
	}
	type attributedModel struct {
		name string
		m    Model
	}
	var variants []attributedModel
	for _, tc := range periodic128Cases {
		m, err := Read(bytes.NewReader(periodic128Fixture(t, tc.name)), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		variants = append(variants, attributedModel{tc.name, m})
	}
	_, edited := editedPeriodic128Model(t)
	variants = append(variants, attributedModel{"translated-and-rephased", edited})
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Write(&out, variant.m); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), variant.name+".iges")
			if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			report := periodic128OracleReport(t, oracle, "read", path)
			s := variant.m.Entities[0].Surface
			g, err := s.ExactPeriodicGeometry()
			if err != nil {
				t.Fatal(err)
			}
			u, v, _ := g.Data()
			d := s.Geometry.Data()
			flags := [2]int{}
			if u.Periodic {
				flags[0] = 1
			}
			if v.Periodic {
				flags[1] = 1
			}
			if report.Counts != [2]int{u.Count, v.Count} || report.ExpandedCounts != [2]int{d.CountU, d.CountV} ||
				report.RawPeriodic != flags || report.RawClosed != flags || report.Periodic != flags ||
				report.Bounds != [4]float64{s.UStart, s.UEnd, s.VStart, s.VEnd} || len(report.Samples) != 9 {
				t.Fatal("pinned transfer changed the original range or cyclic owners", report)
			}
			for _, pt := range report.Samples {
				j, err := g.EvaluateRational(context.Background(), rat(pt.U), rat(pt.V), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				checkPeriodic128Vector(t, j.Point, pt.Point)
				checkPeriodic128Vector(t, j.Du, pt.Du)
				checkPeriodic128Vector(t, j.Dv, pt.Dv)
			}
		})
	}
}

func compareExpandedPeriodic128Parameters(t *testing.T, source, candidate []byte) {
	t.Helper()
	a, b := periodic128Parameters(t, source), periodic128Parameters(t, candidate)
	if len(a)+2 != len(b) || b[len(b)-2] != "0" || b[len(b)-1] != "0" || !slices.Equal(a[:10], b[:10]) {
		t.Fatal("changed the finite source control layout or property suffix")
	}
	for i := 10; i < len(a); i++ {
		x, err := igesPlanarExact(parameter{text: a[i]})
		if err != nil {
			t.Fatal(err)
		}
		y, err := igesPlanarExact(parameter{text: b[i]})
		if err != nil || x.Cmp(y) != 0 {
			t.Fatalf("changed original phase, interval or expanded owner at parameter %d: %q / %q", i, a[i], b[i])
		}
	}
}

func compareExactPeriodic128Surfaces(t *testing.T, want *geom.ExactPeriodicSplineSurface, s *Surface) {
	t.Helper()
	got, err := s.ExactPeriodicGeometry()
	if err != nil {
		t.Fatal(err)
	}
	u, v, net := want.Data()
	gu, gv, gnet := got.Data()
	if !reflect.DeepEqual(u, gu) || !reflect.DeepEqual(v, gv) || !reflect.DeepEqual(net, gnet) {
		t.Fatal("roundtrip changed exact nonuniform knots, phase or homogeneous poles")
	}
	ctx := context.Background()
	parameters := [2]*big.Rat{
		new(big.Rat).Add(u.Knots[u.Degree], big.NewRat(1, 8)),
		new(big.Rat).Add(v.Knots[v.Degree], big.NewRat(1, 8)),
	}
	for i, a := range [2]geom.ExactTensorSplineAxis{u, v} {
		if !a.Periodic {
			continue
		}
		for _, cycle := range []int64{-2, 0, 3} {
			p := parameters
			period := new(big.Rat).Sub(a.Knots[a.Count], a.Knots[0])
			p[i] = new(big.Rat).Add(a.Knots[0], new(big.Rat).Mul(period, big.NewRat(cycle, 1)))
			a, err := want.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			b, err := got.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchOptions{})
			if err != nil || !samePeriodic128Jet(a, b) || !reflect.DeepEqual(a.U, b.U) || !reflect.DeepEqual(a.V, b.V) {
				t.Fatal("exact physical seam derivatives or original cycle owner changed", err)
			}
		}
	}
}

func editedPeriodic128Model(t *testing.T) (*geom.ExactPeriodicSplineSurface, Model) {
	t.Helper()
	m, err := Read(bytes.NewReader(periodic128Fixture(t, "uv")), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	base, err := m.Entities[0].Surface.ExactPeriodicGeometry()
	if err != nil {
		t.Fatal(err)
	}
	u, v, c := base.Data()
	uShift, vShift, xShift := big.NewRat(9, 4), big.NewRat(1, 4), big.NewRat(3, 4)
	for _, q := range u.Knots {
		q.Add(q, uShift)
	}
	for _, q := range v.Knots {
		q.Add(q, vShift)
	}
	for i := range c {
		c[i][0].Add(c[i][0], new(big.Rat).Mul(c[i][3], xShift))
	}
	ctx := context.Background()
	edited, err := geom.NewExactPeriodicSplineSurface(ctx, u, v, c, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewExactPeriodicSurface128(edited)
	if err != nil {
		t.Fatal(err)
	}
	compareExactPeriodic128Surfaces(t, edited, s)

	// Translate equally in UV and model space. Both derivatives, hence
	// their ordered cross product and winding, must remain exact.
	a, err := base.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(13, 1), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := edited.EvaluateRational(ctx, new(big.Rat).Add(big.NewRat(1, 2), uShift), new(big.Rat).Add(big.NewRat(13, 1), vShift), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b.Point[0].Sub(b.Point[0], xShift)
	if !samePeriodic128Jet(a, b) {
		t.Fatal("intentional rephase and pole translation changed geometry or winding")
	}
	return edited, Model{LengthUnit: .001, Entities: []Entity{{Surface: s}}}
}
