package geom_test

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"plainsolid/internal/testgraphfixture"
	"testing"
	"time"
)

func tripleRadialVolumeProof(t *testing.T, reverse bool) *geom.SplineGraphRadialContours {
	t.Helper()
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if reverse {
		pa, pb = pb, pa
	}
	p, e := geom.CertifySplineGraphRadialContours(context.Background(), pa, pb, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, testgraphfixture.TripleRadialRadii(), geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestTripleRadialExactVolumeBands(t *testing.T) {
	ctx := context.Background()
	want := []*big.Rat{big.NewRat(613935546875, 864691128455135232), big.NewRat(478515625, 6597069766656)}
	total := big.NewRat(63333935546875, 864691128455135232)
	for _, reverse := range []bool{false, true} {
		proof := tripleRadialVolumeProof(t, reverse)
		r, e := proof.BoundVolumeBands(ctx, []int{0, 2}, geom.RadialGraphVolumeOptions{PrecisionBits: 80})
		if e != nil {
			t.Fatal(e)
		}
		if r.PiCoefficient.Cmp(total) != 0 || len(r.Bands) != 2 || r.Volume.Lower.Sign() <= 0 {
			t.Fatal("independent exact volume coefficient", r)
		}
		width := new(big.Rat).Sub(r.Volume.Upper, r.Volume.Lower)
		if width.Cmp(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))) > 0 {
			t.Fatal("requested width", width)
		}
		for i, b := range r.Bands {
			sign := -1
			if reverse {
				sign = 1
			}
			if b.PiCoefficient.Cmp(want[i]) != 0 || b.HeightSign != sign {
				t.Fatal("independent component integral", b)
			}
		}
		lo, _ := r.Volume.Lower.Float64()
		hi, _ := r.Volume.Upper.Float64()
		v, _ := total.Float64()
		if math.Abs(lo-math.Pi*v) > 1e-18 || math.Abs(hi-math.Pi*v) > 1e-18 {
			t.Fatal("pi enclosure", lo, hi)
		}
		t.Log("positive tight volume", lo, hi, "width", width.RatString(), "work", r.ExactOperations)
		r.PiCoefficient.SetInt64(0)
		r.Bands[0].PiCoefficient.SetInt64(0)
		again, e := proof.BoundVolumeBands(ctx, []int{0, 2}, geom.RadialGraphVolumeOptions{})
		if e != nil || again.PiCoefficient.Cmp(total) != 0 {
			t.Fatal("mutable result changed proof", e)
		}
		for _, bands := range [][]int{{}, {0, 0}, {2, 0}, {0, 3}, {-1}} {
			if bad, e := proof.BoundVolumeBands(ctx, bands, geom.RadialGraphVolumeOptions{}); e == nil || bad.Volume.Lower != nil {
				t.Fatal("invalid band selection", bands, e)
			}
		}
		if bad, e := proof.BoundVolumeBands(ctx, []int{0, 2}, geom.RadialGraphVolumeOptions{MaxOperations: 1}); e == nil || bad.Volume.Lower != nil {
			t.Fatal("work quota", e)
		}
		if bad, e := proof.BoundVolumeBands(ctx, []int{0, 2}, geom.RadialGraphVolumeOptions{PrecisionBits: 80, MaxCoefficientBits: 64}); e == nil || bad.Volume.Lower != nil {
			t.Fatal("bit quota", e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if bad, e := proof.BoundVolumeBands(cancelled, []int{0, 2}, geom.RadialGraphVolumeOptions{}); e != context.Canceled || bad.Volume.Lower != nil {
			t.Fatal("cancel", e)
		}
	}
}

func TestRadialVolumeRejectsVariableDenominator(t *testing.T) {
	ctx := context.Background()
	a, _, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	height, e := testgraphfixture.TripleRadialHeightControls()
	if e != nil {
		t.Fatal(e)
	}
	var controls [][4]*big.Rat
	for i := int64(0); i <= 8; i++ {
		for j := int64(0); j <= 8; j++ {
			w := big.NewRat(8+i, 8)
			x := new(big.Rat).Add(big.NewRat(i, 8), big.NewRat(i*(i-1), 56))
			y := new(big.Rat).Mul(big.NewRat(j, 8), w)
			controls = append(controls, [4]*big.Rat{x, y, new(big.Rat).SetFloat64(height[i][j]), w})
		}
	}
	pb, e := geom.NewExactHomogeneousBezierSurface(ctx, 8, 8, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := geom.CertifySplineGraphRadialContours(ctx, pa, pb, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, testgraphfixture.TripleRadialRadii(), geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal("valid complete radial numerator", e)
	}
	if r, e := proof.BoundVolumeBands(ctx, []int{0, 2}, geom.RadialGraphVolumeOptions{}); e == nil || r.Volume.Lower != nil {
		t.Fatal("numerator mistaken for height", e)
	}
	if r, e := proof.PlanarMomentsBands(ctx, []int{0, 2}, geom.RadialGraphMomentOptions{}); e == nil || r.Total.VolumePiCoefficient != nil || len(r.Bands) != 0 {
		t.Fatal("moments require actual radial height", e)
	}

	if r, e := proof.MassMomentsBands(ctx, []int{0, 2}, geom.RadialGraphMomentOptions{}); e == nil || r.Height.CentroidZ != nil || len(r.Bands) != 0 {
		t.Fatal("centroid requires original polynomial height", e)
	}

}

func TestTripleRadialVolumeOCCT(t *testing.T) {
	path := os.Getenv("PLAINSOLID_OCCT_TRIPLE_RADIAL")
	if path == "" {
		t.Skip("set PLAINSOLID_OCCT_TRIPLE_RADIAL to independent tools/occtref/triple_radial_roofs executable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, path).Output()
	if e != nil {
		t.Fatal(e)
	}
	var reference struct {
		Volume        float64
		Faces, Solids int
		Components    []struct {
			Faces, Holes int
			Volume       float64
		}
	}
	if e = json.Unmarshal(data, &reference); e != nil {
		t.Fatal(e)
	}
	proof := tripleRadialVolumeProof(t, false)
	r, e := proof.BoundVolumeBands(ctx, []int{0, 2}, geom.RadialGraphVolumeOptions{PrecisionBits: 80})
	if e != nil {
		t.Fatal(e)
	}
	if reference.Faces != 4 || reference.Solids != 2 || len(reference.Components) != 2 {
		t.Fatal("independent topology", string(data))
	}
	want, _ := r.Volume.Lower.Float64()
	if math.Abs(reference.Volume-want) > 1e-10 {
		t.Fatal("independent total", reference.Volume, want)
	}
	seen := [2]bool{}
	for _, component := range reference.Components {
		i := 0
		if component.Holes == 2 {
			i = 1
		} else if component.Holes != 0 {
			t.Fatal("independent holes", component)
		}
		v, _ := r.Bands[i].Volume.Lower.Float64()
		if seen[i] || component.Faces != 2 || math.Abs(component.Volume-v) > 1e-10 {
			t.Fatal("independent component", component, v)
		}
		seen[i] = true
	}
	t.Log("independent OCCT total", reference.Volume, "exact-source integral", want)
}
