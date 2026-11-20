package boolean

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// The tests of this file hold cylinders that are not an equal pair and
// whose axes are not parallel (specs/general-boolean-quartic) to the
// volume their common has by a quadrature of its own (A2).

// quarticCommon is the volume common to two cylinders of radii big and
// small whose axes pass at distance d and make an angle of the sine
// given: the section at height y along the common perpendicular is a
// parallelogram with half-widths √(big² − y²) and √(small² − (y − d)²).
// The substitution y = middle + half·sin φ takes the square roots out of
// the ends, and Simpson's rule does the rest.
func quarticCommon(big, small, d, sine float64) float64 {
	lo, hi := math.Max(-big, d-small), math.Min(big, d+small)
	middle, half := (lo+hi)/2, (hi-lo)/2
	f := func(phi float64) float64 {
		s, c := math.Sincos(phi)
		y := middle + half*s
		return math.Sqrt(math.Max(big*big-y*y, 0)*math.Max(small*small-(y-d)*(y-d), 0)) * half * c
	}
	const n = 20000
	h := math.Pi / n
	sum := f(-math.Pi/2) + f(math.Pi/2)
	for k := 1; k < n; k++ {
		sum += float64(2+2*(k%2)) * f(-math.Pi/2+h*float64(k))
	}
	return 4 / sine * sum * h / 3
}

// quarticPairs are the pairs of A1 and A2: a large cylinder about the z
// axis and a small one whose axis passes at distance d along y, in the
// plane across y, at an angle of the sine given to the first.
var quarticPairs = []struct {
	name                string
	big, small, d, sine float64
	curves              int
}{
	{"full, right angle", 5, 2, 0, 1, 2},
	{"full, skew at a slant", 5, 2, 1, 0.6, 2},
	{"partial", 5, 3, 4, 1, 1},
	{"partial, equal radii", 3, 3, 2, 1, 1},
}

func quarticCylinders(t *testing.T, m geom.Affine, big, small, d, sine float64) (cylinder, cylinder) {
	t.Helper()
	upright, err := geom.FrameFromNormal(m.Apply(geom.Vec3{}), m.ApplyVector(geom.Vec3{Z: 1}))
	if err != nil {
		t.Fatal(err)
	}
	slanted, err := geom.FrameFromNormal(m.Apply(geom.Vec3{Y: d}), m.ApplyVector(geom.Vec3{X: sine, Z: math.Sqrt(1 - sine*sine)}))
	if err != nil {
		t.Fatal(err)
	}
	return cylinder{upright, big}, cylinder{slanted, small}
}

// TestQuarticCarrier is A1: every point of a carrier is on both
// cylinders, its velocity is the derivative of its point, and param
// returns the parameter of a point, for the carrier and for it reversed.
func TestQuarticCarrier(t *testing.T) {
	tol := tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: 40}
	for _, c := range quarticPairs {
		for _, turned := range []bool{false, true} {
			m := geom.Identity()
			if turned {
				m = crossingTurn(t)
			}
			large, small := quarticCylinders(t, m, c.big, c.small, c.d, c.sine)
			pair, err := surfacePair(large, small, tol)
			if err != nil || pair.open || len(pair.curves) != c.curves {
				t.Fatalf("%s: %d curves, open %v, %v", c.name, len(pair.curves), pair.open, err)
			}
			for _, carrier := range pair.curves {
				for _, q := range []curve{carrier, carrier.flipped()} {
					for k := range 64 {
						u := 2 * math.Pi * (float64(k) + 0.25) / 64
						p := q.at(u)
						if gap := math.Max(math.Abs(large.offset(p)), math.Abs(small.offset(p))); gap > 1e-11*40 {
							t.Fatalf("%s: the point at %g is %g from a cylinder", c.name, u, gap)
						}
						const h = 1e-6
						v, slope := q.velocity(u), q.at(u+h).Sub(q.at(u-h)).Scale(1/(2*h))
						if v.Sub(slope).Length() > 1e-6*v.Length() || v.Length() == 0 {
							t.Fatalf("%s: the velocity at %g is %v, and the point moves by %v", c.name, u, v, slope)
						}
						if back := q.param(p); math.Abs(math.Remainder(back-u, 2*math.Pi)) > 1e-9 {
							t.Fatalf("%s: param gives %g for the point at %g", c.name, back, u)
						}
						// A point a little off the carrier has the
						// parameter of the point it was moved from.
						aside := p.Add(large.normal(p).Scale(2e-7))
						if back := q.param(aside); q.at(back).Distance(p) > 1e-9 {
							t.Fatalf("%s: param of a point beside %g gives a point %g away", c.name, u, q.at(back).Distance(p))
						}
					}
				}
			}
		}
	}
}

// TestQuarticBores is A2: from a prism with a bore of the large radius
// along z, a second bore of the small radius is cut across it. The two
// bores are through, so the result has the volume of the prism less
// both bores, plus what the bores have in common. The first two pairs
// are also taken as the common of the two rods.
func TestQuarticBores(t *testing.T) {
	const w, depth, h = 16., 16., 40.
	for _, c := range quarticPairs {
		for _, turned := range []bool{false, true} {
			name := c.name
			if turned {
				name += ", turned"
			}
			t.Run(name, func(t *testing.T) {
				m := geom.Identity()
				if turned {
					m = crossingTurn(t)
				}
				cosine := math.Sqrt(1 - c.sine*c.sine)
				stock := crossingStock(t, m, crossingRectangle(w, depth), -h/2, h)
				upright := crossingRod(t, m, geom.Vec3{}, geom.Vec3{Z: 1}, c.big, h+4)
				slanted := crossingRod(t, m, geom.Vec3{Y: c.d}, geom.Vec3{X: c.sine, Z: cosine}, c.small, 60)
				common := quarticCommon(c.big, c.small, c.d, c.sine)
				first, err := Combine(context.Background(), stock, upright, Cut, crossingOptions)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Combine(context.Background(), first.Solid, slanted, Cut, crossingOptions)
				if err != nil {
					t.Fatal(err)
				}
				want := w*depth*h - math.Pi*c.big*c.big*h - math.Pi*c.small*c.small*w/c.sine + common
				if got.Solids != 1 || math.Abs(got.Volume-want) > 1e-9*w*depth*h {
					t.Fatalf("%d solids, volume %.12g want %.12g", got.Solids, got.Volume, want)
				}
				if c.curves != 2 {
					return
				}
				both, err := Combine(context.Background(), upright, slanted, Common, crossingOptions)
				if err != nil {
					t.Fatal(err)
				}
				if both.Solids != 1 || math.Abs(both.Volume-common) > 1e-9*w*depth*h {
					t.Fatalf("the common of the rods: %d solids, volume %.12g want %.12g", both.Solids, both.Volume, common)
				}
			})
		}
	}
}

// TestQuarticRefusals holds the pairs out of scope to their refusals: a
// small cylinder that touches the large one from inside or nearly does,
// one that touches it from outside, and axes too nearly parallel.
func TestQuarticRefusals(t *testing.T) {
	const w, depth, h = 30., 30., 40.
	for _, c := range []struct {
		name                string
		big, small, d, sine float64
		refused             string
	}{
		{"touching from inside", 5, 2, 3, 1, "that touch"},
		{"nearly touching from inside", 5, 2, 2.9999, 1, "nearly touch from inside"},
		{"nearly touching, partial", 5, 2, 3.0001, 1, "nearly touch"},
		{"touching from outside", 5, 2, 7, 1, "that touch"},
		{"axes at a sine of 1/128", 5, 2, 4, 1. / 128, "too nearly parallel"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := geom.Identity()
			cosine := math.Sqrt(1 - c.sine*c.sine)
			stock := crossingStock(t, m, crossingRectangle(w, depth), -h/2, h)
			upright := crossingRod(t, m, geom.Vec3{}, geom.Vec3{Z: 1}, c.big, h+4)
			slanted := crossingRod(t, m, geom.Vec3{Y: c.d}, geom.Vec3{X: c.sine, Z: cosine}, c.small, 36)
			first, err := Combine(context.Background(), stock, upright, Cut, crossingOptions)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Combine(context.Background(), first.Solid, slanted, Cut, crossingOptions)
			if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), c.refused) {
				t.Fatalf("%v, want unsupported with %q", err, c.refused)
			}
		})
	}
}

// quarticBored is a prism 16 wide and deep and 40 tall with a bore of
// radius 5 along z and one of radius 2 along x across it.
func quarticBored(t *testing.T, m geom.Affine) Result {
	t.Helper()
	body := crossingStock(t, m, crossingRectangle(16, 16), -20, 40)
	var out Result
	for _, rod := range []struct {
		axis   geom.Vec3
		radius float64
	}{{geom.Vec3{Z: 1}, 5}, {geom.Vec3{X: 1}, 2}} {
		r, err := Combine(context.Background(), body, crossingRod(t, m, geom.Vec3{}, rod.axis, rod.radius, 60), Cut, crossingOptions)
		if err != nil {
			t.Fatal(err)
		}
		out, body = r, r.Solid
	}
	return out
}

// TestQuarticCutAgain is A3: a body with edges on meeting curves is cut
// by a tool that passes through them, and what the tool removes and what
// it leaves add up to the body. One tool has plane faces. The other is a
// cylinder parallel to the first bore, whose wall meets the second bore
// along meeting curves of its own that cross the ones there.
func TestQuarticCutAgain(t *testing.T) {
	for _, turned := range []bool{false, true} {
		m := geom.Identity()
		if turned {
			m = crossingTurn(t)
		}
		body := quarticBored(t, m)
		slab := crossingStock(t, m, []geom.Vec2{{X: 3, Y: -9}, {X: 12, Y: -9}, {X: 12, Y: 9}, {X: 3, Y: 9}}, -0.5, 1)
		for name, tool := range map[string]func() (Result, Result, error){
			"a slab through the curves": func() (Result, Result, error) {
				less, err := Combine(context.Background(), body.Solid, slab, Cut, crossingOptions)
				if err != nil {
					return Result{}, Result{}, err
				}
				common, err := Combine(context.Background(), body.Solid, slab, Common, crossingOptions)
				return less, common, err
			},
			"a rod along the first bore": func() (Result, Result, error) {
				rod := crossingRod(t, m, geom.Vec3{X: 5}, geom.Vec3{Z: 1}, 1, 60)
				less, err := Combine(context.Background(), body.Solid, rod, Cut, crossingOptions)
				if err != nil {
					return Result{}, Result{}, err
				}
				common, err := Combine(context.Background(), body.Solid, rod, Common, crossingOptions)
				return less, common, err
			},
		} {
			less, common, err := tool()
			if err != nil {
				t.Fatalf("%s, turned %v: %v", name, turned, err)
			}
			if gap := less.Volume + common.Volume - body.Volume; math.Abs(gap) > 1e-9*16*16*40 || common.Volume <= 0 {
				t.Fatalf("%s, turned %v: the parts are %.12g and %.12g, %g off the body", name, turned, less.Volume, common.Volume, gap)
			}
		}
	}
}

// TestQuarticClassify is A5: points of a body with faces trimmed along
// meeting curves are located by trimmed.Solid.Classify, whose rays cross
// those faces.
func TestQuarticClassify(t *testing.T) {
	body := quarticBored(t, geom.Identity()).Solid
	for _, c := range []struct {
		name  string
		p     geom.Vec3
		where parametric.Location
	}{
		{"in the material beside both bores", geom.Vec3{X: 6.5, Z: 3}, parametric.Inside},
		{"in the material between the openings", geom.Vec3{X: 4.5, Y: 4.5, Z: 1}, parametric.Inside},
		{"in the small bore", geom.Vec3{X: 6.5, Z: 1}, parametric.Outside},
		{"in the large bore", geom.Vec3{X: 1, Y: 1, Z: 10}, parametric.Outside},
		{"where the bores open into each other", geom.Vec3{X: 5, Z: 1}, parametric.Outside},
		{"on the large wall above the opening", geom.Vec3{X: 5, Z: 2.5}, parametric.Boundary},
		{"on the small wall beside the opening", geom.Vec3{X: 6, Z: 2}, parametric.Boundary},
		{"on a meeting curve", geom.Vec3{X: math.Sqrt(24), Y: 1, Z: math.Sqrt(3)}, parametric.Boundary},
		{"beyond the stock", geom.Vec3{X: 9, Z: 3}, parametric.Outside},
	} {
		got, err := body.Classify(c.p, 1e-7)
		if err != nil || got != c.where {
			t.Errorf("%s: %v, %v, want %v", c.name, got, err, c.where)
		}
	}
}
