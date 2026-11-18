package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func clippedFourTransverseRoofFixture(t *testing.T, axis int) *geom.BSplineSurface {
	t.Helper()
	_, roof, _ := nativeQuarticRoofPairFixture(t, axis, big.NewRat(1, 1024),
		[4][2]int64{{1, 4}, {3, 8}, {5, 8}, {3, 4}})
	return roof
}

func TestNativeClippedPolynomialFourTransverseFuse(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	// From the independent source equations f=1/4+u⁴/8,
	// g-f=(u-1/4)(u-3/8)(u-5/8)(u-3/4). Integrate both original
	// stock heights, subtract the original overlap's lower envelope,
	// and split the between-branch height integral at all four roots.
	volumes := [3]string{
		"5205974907560435/314460325543936",
		"2570722139241407/157230162771968",
		"9945805867513675/628920651087872",
	}
	// Independent 32-point Gauss-Legendre integration of the original
	// derivatives and the difference in ceiling heights on each wall.
	areas := [3][2]float64{
		{40.256084657152932, 40.615810037556955},
		{39.788493372566357, 40.116221267008456},
		{39.343369777998944, 39.616844616204325},
	}
	stations := [2][4]string{
		{"5/2", "11/4", "13/4", "7/2"},
		{"-1/4", "1/8", "7/8", "5/4"},
	}
	for axis := range 2 {
		original := clippedFourTransverseRoofFixture(t, axis)
		for layout := range 3 {
			src := clippedTransverseFuseStock(t, axis, layout, original)
			want, _ := new(big.Rat).SetString(volumes[layout])
			volume, _ := want.Float64()
			for order := range 2 {
				r, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, o)
				if err != nil {
					t.Fatal(axis, layout, order, err)
				}
				s := r.Boundary()
				mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil || r.ExactVolume().Cmp(want) != 0 || math.Abs(mass.Volume-volume) > 1e-8 ||
					math.Abs(mass.Area-areas[layout][axis]) > 1e-8 {
					t.Fatal("independent original stock measures", axis, layout, order, r.ExactVolume(), mass, want, areas[layout][axis], err)
				}
				top := s.Topology()
				h := r.ledger
				if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || s.faces[h.Base].Surface != src[0] ||
					len(h.Contacts) != 4 || len(h.Edges) != 10 || len(h.Vertices) != 8 || len(h.Midpoints) != 6 {
					t.Fatal("closed topology must retain four original crossings and all six intermediate edge splits", top, h)
				}
				for i, contact := range h.Contacts {
					if contact.Order != 1 || contact.Point[axis] != stations[axis][i] || len(contact.Rays) != 4 || len(contact.Sectors) != 4 {
						t.Fatal("original transverse contact must have all four native rays and roof sectors", axis, i, contact)
					}
				}
				for i, face := range h.RoofFaces {
					if s.faces[face].Surface != r.sources[h.RoofOwners[i]+1] {
						t.Fatal("unmodified original roof chart", i, face)
					}
				}
				var raw, echo bytes.Buffer
				if err := r.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				reloaded, err := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if err != nil || !reflect.DeepEqual(h, reloaded.ledger) || !reflect.DeepEqual(r.Walls(), reloaded.Walls()) {
					t.Fatal("original walls and all four contacts must re-prove", axis, layout, order, err)
				}
				if err := reloaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical four-contact Fuse receipt", err)
				}
				if axis == 0 && layout == 0 && order == 0 {
					var altered clippedPolynomialFuseRecipe
					if err := json.Unmarshal(raw.Bytes(), &altered); err != nil {
						t.Fatal(err)
					}
					altered.Ledger.Contacts = altered.Ledger.Contacts[:3]
					data, err := json.Marshal(altered)
					if err != nil {
						t.Fatal(err)
					}
					invalid, err := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(data), JSONLimits{}, o)
					if invalid != nil || !errors.Is(err, geom.ErrConvergence) {
						t.Fatal("a fourth omitted original contact must fail source replay", err)
					}
				}
			}
		}
	}
}

func TestNativeClippedPolynomialFourTransverseFuseClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := range 2 {
		roof := clippedFourTransverseRoofFixture(t, axis)
		for layout := range 3 {
			stocks := clippedTransverseFuseStock(t, axis, layout, roof)
			for order := range 2 {
				t.Run(fmt.Sprintf("axis%d/layout%d/order%d", axis, layout, order), func(t *testing.T) {
					fuse, err := FuseClippedPolynomialEnvelope(ctx, stocks[0], stocks[1+order], stocks[2-order], axis,
						1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
					if err != nil {
						t.Fatal(err)
					}
					checkClippedFuseDisplay(t, fuse)
				})
			}
		}
	}
}
