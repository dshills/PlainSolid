//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func authoredStationProfile(t *testing.T) *geom.BSpline {
	t.Helper()
	xy := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var poles []geom.Vec3
	for _, p := range xy {
		poles = append(poles, geom.Vec3{X: .25 + 5*p[0], Y: -.5 + 5*p[1]})
	}
	profile, e := geom.NewBSpline(2, poles, []float64{-2, -2, -2, -1, -1, 1, 1, 4, 4, 8, 8, 8}, []float64{1, .5, 1, .75, 1, .75, 1, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	return profile
}

func TestAuthoredRationalProfileStationLoft(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	profile := authoredStationProfile(t)
	stations := []WrappedRationalProfileStation{{r(0, 1), r(3, 5)}, {r(1, 1), r(1, 5)}, {r(3, 1), r(2, 5)}}
	body, history, e := LoftWrappedRationalProfileStations(ctx, profile, stations, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(history.WallFaces) != 2 || len(history.StationRims) != 3 {
		t.Fatal(history)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil || !reflect.DeepEqual(proof.History, history) {
		t.Fatal(proof, e)
	}
	for i, want := range []*big.Rat{r(3, 1), r(1, 1), r(2, 1)} {
		if proof.Scales[i].Cmp(want) != 0 {
			t.Fatal("rational scale product", proof.Scales)
		}
	}
	edges := body.Edges()
	cuts := []float64{-2, -1, 1, 4, 8}
	for _, rim := range history.StationRims {
		for quarter, edge := range rim {
			if edges[edge].Range.Min != cuts[quarter] || edges[edge].Range.Max != cuts[quarter+1] {
				t.Fatal("lost original rim parameter range", edges[edge].Range)
			}
		}
	}
	original, e := geom.NewExactSplineCurve(profile, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, station := range stations {
		section, e := body.SectionWrappedRationalStationLoftRational(ctx, station.Height, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		tx := new(big.Rat).Mul(r(1, 4), new(big.Rat).Sub(r(1, 1), station.Scale))
		ty := new(big.Rat).Mul(r(-1, 2), new(big.Rat).Sub(r(1, 1), station.Scale))
		expected, e := original.TransformRational(ctx, [3][4]*big.Rat{{station.Scale, r(0, 1), r(0, 1), tx}, {r(0, 1), station.Scale, r(0, 1), ty}, {r(0, 1), r(0, 1), r(1, 1), station.Height}}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		same, e := geom.CompareExactCurvesContext(ctx, section.Outer.Section.Curve, expected, geom.ExactCurveComparisonOptions{})
		if e != nil || !same.Complete || !same.Equal {
			t.Fatal("full authored profile identity", i, same, e)
		}
		lo, hi := section.Outer.Section.Curve.Domain()
		if lo != -2 || hi != 8 {
			t.Fatal("lost authored domain")
		}
	}
	for _, face := range history.WallFaces {
		wall := body.Faces()[face].Surface.(*geom.WrappedSurfacePatch)
		bs, e := wall.FundamentalPatch().ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		data := bs.Data()
		if !reflect.DeepEqual(data.KnotsU, profile.Knots()) {
			t.Fatal("changed quadrant basis")
		}
		for i, w := range profile.Weights() {
			if data.Weights[2*i] != w || data.Weights[2*i+1] != w {
				t.Fatal("changed stored source weight")
			}
		}
	}
	mass, e := body.BoundWrappedRationalStationMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: r(1, 25)})
	if e != nil {
		t.Fatal(e)
	}
	area, _, _ := independentStationProfileMoments([4]float64{.5, .75, .75, .75}, 4096)
	lo, _ := mass.Volume.Lower.Float64()
	hi, _ := mass.Volume.Upper.Float64()
	if 9*area < lo || 9*area > hi || math.Abs(9*area-9*math.Pi) < .1 {
		t.Fatal("noncircular independent volume", lo, 9*area, hi)
	}
	stations[0].Scale.SetInt64(99)
	poles := profile.Poles()
	poles[0].X = 99
	again, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil || again.Scales[0].Cmp(r(3, 1)) != 0 {
		t.Fatal("input alias", again, e)
	}
}

func TestAuthoredRationalProfileStationGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	profile := authoredStationProfile(t)
	for _, stations := range [][]WrappedRationalProfileStation{
		make([]WrappedRationalProfileStation, 18),
		{{r(0, 1), r(1, 1)}},
		{{r(0, 1), r(1, 1)}, {r(0, 1), r(1, 1)}},
		{{r(0, 1), r(1, 1)}, {r(1, 1), r(0, 1)}},
		{{r(0, 1), r(1, 1)}, {r(1, 1), nil}},
	} {
		body, h, e := LoftWrappedRationalProfileStations(ctx, profile, stations, 1e-8, geom.SurfacePatchOptions{})
		if e == nil || len(body.Faces()) != 0 || len(h.WallFaces) != 0 {
			t.Fatal("non-atomic station failure", e)
		}
	}
	stations := []WrappedRationalProfileStation{{r(0, 1), r(1, 1)}, {r(1, 1), r(1, 3)}}
	if body, h, e := LoftWrappedRationalProfileStations(ctx, profile, stations, 1e-8, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrUnrepresentable) || len(body.Faces()) != 0 || len(h.WallFaces) != 0 {
		t.Fatal("rounded nonrepresentable native pole", e)
	}
	stations[1].Scale = r(1, 1)
	stations[1].Height = r(1, 3)
	if _, _, e := LoftWrappedRationalProfileStations(ctx, profile, stations, 1e-8, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded native height", e)
	}
	stations[1].Height = r(1, 1)
	poles := profile.Poles()
	poles[1].Y += .125 // same endpoints, different unsupported interior
	bad, e := geom.NewBSpline(2, poles, profile.Knots(), profile.Weights())
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e := LoftWrappedRationalProfileStations(ctx, bad, stations, 1e-8, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("unsupported interior accepted")
	}
	for _, o := range []geom.SurfacePatchOptions{{MaxOperations: 1}, {MaxPoles: 4}} {
		body, h, e := LoftWrappedRationalProfileStations(ctx, profile, stations, 1e-8, o)
		if e == nil || len(body.Faces()) != 0 || len(h.WallFaces) != 0 {
			t.Fatal("quota", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := LoftWrappedRationalProfileStations(cancelled, profile, stations, 1e-8, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
