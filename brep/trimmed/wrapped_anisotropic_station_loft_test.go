package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"
)

func anisotropicStationBand(t *testing.T, z0, z1 float64, lower, upper [2]float64, weight float64) *geom.WrappedSurfacePatch {
	t.Helper()
	base := rationalStationBand(t, z0, z1, 1, 1, weight)
	recipe, e := base.ExportRecipe()
	if e != nil {
		t.Fatal(e)
	}
	d := recipe.Surface.Data()
	for i, p := range d.Poles {
		scale := lower
		if i%2 == 1 {
			scale = upper
		}
		d.Poles[i].X = .25 + (p.X-.25)*scale[0]
		d.Poles[i].Y = -.5 + (p.Y+.5)*scale[1]
	}
	d.KnotsU = []float64{-2, -2, -2, -1, -1, 1, 1, 4, 4, 8, 8, 8}
	d.KnotsV = []float64{2, 2, 5, 5}
	s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func TestWrappedAnisotropicNativeStations(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	a := anisotropicStationBand(t, 0, 1, [2]float64{3, 1}, [2]float64{1, 2}, .75)
	b := anisotropicStationBand(t, 1, 3, [2]float64{1, 2}, [2]float64{2, 1}, .75)
	body, h, e := NewWrappedAnisotropicStationLoft(ctx, []*geom.WrappedSurfacePatch{a, b}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if body.Topology().EulerCharacteristic != 2 || len(h.WallFaces) != 2 {
		t.Fatal(body.Topology(), h)
	}
	for _, beforeReload := range []bool{true, false} {
		if !beforeReload {
			faces := body.Faces()
			slices.Reverse(faces)
			body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			var raw bytes.Buffer
			if e = WriteJSON(&raw, body); e != nil {
				t.Fatal(e)
			}
			body, e = ReadJSONContext(ctx, &raw, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
		}
		p, e := body.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
		if e != nil || !p.Complete {
			t.Fatal(p, e)
		}
		if len(p.Scales) != 3 || p.Scales[1][0].Cmp(r(1, 1)) != 0 || p.Scales[1][1].Cmp(r(2, 1)) != 0 {
			t.Fatal(p.Scales)
		}
		if _, e = body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{}); e == nil {
			t.Fatal("homothetic gate widened")
		}
		for _, height := range []*big.Rat{r(0, 1), r(1, 3), r(1, 1), r(7, 3), r(3, 1)} {
			section, e := body.SectionWrappedAnisotropicStationRational(ctx, height, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			cuts := []float64{-2, -1, 1, 4, 8}
			for j := 0; j < 4; j++ {
				piece, e := section.Profile.Curve.Restrict(cuts[j], cuts[j+1], geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				net, _, e := piece.BezierControlNet(ctx, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, q := range net.Control {
					if q[2].Cmp(new(big.Rat).Mul(height, q[3])) != 0 {
						t.Fatal("section not exactly horizontal")
					}
				}
			}
			if height.Cmp(r(1, 1)) == 0 {
				want := append([]int(nil), p.History.WallFaces...)
				slices.Sort(want)
				if section.Cap != -1 || section.Profile.Cap != 0 || !reflect.DeepEqual(section.Faces, want) || !reflect.DeepEqual(section.Rim, p.History.StationRims[1][:]) {
					t.Fatal("crease ancestry", section)
				}
				edges := body.Edges()
				for j, id := range section.Rim {
					if edges[id].Range.Min != cuts[j] || edges[id].Range.Max != cuts[j+1] {
						t.Fatal("original edge parameters lost")
					}
				}
			}
			if height.Cmp(r(1, 3)) == 0 && (section.Profile.Scale[0].Cmp(r(7, 3)) != 0 || section.Profile.Scale[1].Cmp(r(4, 3)) != 0 || len(section.Rim) != 0) {
				t.Fatal("rational interior section", section)
			}
		}
	}
	// Independent section Jacobian: integral sx(z)*sy(z) dz=43/6,
	// integral z*sx(z)*sy(z) dz=10; normalized profile area is independent.
	properties, e := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	area := rationalQuarterProfileArea(.75, 8192)
	if math.Abs(properties.Volume-area*43/6) > 1e-6 || math.Abs(properties.Centroid.Z-60./43) > 1e-7 {
		t.Fatal("independent station integral", properties, area*43/6)
	}
}

func TestWrappedAnisotropicNativeRejects(t *testing.T) {
	ctx := context.Background()
	a := anisotropicStationBand(t, 0, 1, [2]float64{3, 1}, [2]float64{1, 2}, .75)
	b := anisotropicStationBand(t, 1, 3, [2]float64{1, 2}, [2]float64{2, 1}, .75)
	wrong := anisotropicStationBand(t, 1, 3, [2]float64{1, 2}, [2]float64{2, 1}, .5)
	gap := anisotropicStationBand(t, 2, 3, [2]float64{1, 2}, [2]float64{2, 1}, .75)
	for _, bands := range [][]*geom.WrappedSurfacePatch{{a, wrong}, {a, gap}, {b, a}, make([]*geom.WrappedSurfacePatch, 17)} {
		if got, h, e := NewWrappedAnisotropicStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{}); e == nil || len(got.Faces()) != 0 || len(h.WallFaces) != 0 {
			t.Fatal("partial/unsupported bands", e)
		}
	}
	body, _, e := NewWrappedAnisotropicStationLoft(ctx, []*geom.WrappedSurfacePatch{a, b}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bad := body
	bad.faces = body.Faces()
	bad.faces[1].Surface = wrong
	if p, e := bad.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{}); e == nil || p.Complete {
		t.Fatal("forged original trim accepted", e)
	}
	if p, e := body.SectionWrappedAnisotropicStationRational(ctx, big.NewRat(1, 3), geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || p.Profile.Curve != nil {
		t.Fatal("quota partial", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := body.ValidateWrappedAnisotropicStationLoft(cancelled, geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || p.Complete {
		t.Fatal(p, e)
	}
}
