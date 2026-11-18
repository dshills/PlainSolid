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

func variableClippedCommonVolume(axis, layout int, gap, roots []*big.Rat) *big.Rat {
	zero, one, start, end := new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 8), big.NewRat(7, 8)
	ul, uh, vl, vh := start, end, zero, one
	if layout == 1 {
		ul, uh, vl, vh = zero, one, start, end
	} else if layout == 2 {
		vl, vh = start, end
	}
	// Both stocks share the same original affine base. Integrate B-base
	// over the physical overlap using the rectangle mean of its bilinear
	// height, then subtract the narrow original bands in which A is lower.
	u := new(big.Rat).Mul(new(big.Rat).Add(ul, uh), big.NewRat(1, 2))
	v := new(big.Rat).Mul(new(big.Rat).Add(vl, vh), big.NewRat(1, 2))
	su, sv := big.NewRat(1, 4), big.NewRat(3, 16)
	if axis == 1 {
		su, sv = sv, su
	}
	volume := big.NewRat(23, 8)
	volume.Add(volume, new(big.Rat).Mul(new(big.Rat).Mul(u, v), big.NewRat(1, 64)))
	volume.Sub(volume, new(big.Rat).Mul(su, u)).Sub(volume, new(big.Rat).Mul(sv, v))
	volume.Mul(volume, new(big.Rat).Sub(uh, ul)).Mul(volume, new(big.Rat).Sub(vh, vl)).Mul(volume, big.NewRat(6, 1))
	stations := append([]*big.Rat{ul}, roots...)
	stations = append(stations, uh)
	cube := multiplyCutOraclePolynomials(multiplyCutOraclePolynomials(gap, gap), gap)
	for i := 1; i < len(stations); i++ {
		defect := integralCutOracle(cube, stations[i-1], stations[i])
		volume.Sub(volume, defect.Abs(defect).Mul(defect, big.NewRat(105, 128)))
	}
	return volume
}

func TestNativeClippedPolynomialVariableTransverseCommon(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for _, n := range []int{1, 4, 5} {
		for axis := range 2 {
			roof, gap, roots := variableClippedTransverseRoof(t, axis, n)
			for layout := range 3 {
				stock := clippedTransverseFuseStock(t, axis, layout, roof)
				want, _ := variableClippedCommonVolume(axis, layout, gap, roots).Float64()
				for order := range 2 {
					t.Run(fmt.Sprintf("nodes%d/axis%d/layout%d/order%d", n, axis, layout, order), func(t *testing.T) {
						r, err := CommonClippedPolynomialTransverseEnvelope(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, o)
						if err != nil {
							t.Fatal(err)
						}
						s, h, contacts := r.Boundary(), r.History(), r.Contacts()
						top := s.Topology()
						if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
							len(contacts) != n || len(h.RoofFaces) != n+3 || len(h.Edges) != 2*(n+1) ||
							len(h.Midpoints) != 2*(n-1) || len(h.Vertices) != n+4 {
							t.Fatal("one complete original shell, every contact and all roof sectors", top, h)
						}
						if s.faces[h.Base].Surface != stock[0] {
							t.Fatal("original base chart changed")
						}
						sources := [2]*geom.BSplineSurface{stock[1+order], stock[2-order]}
						for i, id := range h.RoofFaces {
							if s.faces[id].Surface != sources[h.RoofOwners[i]] {
								t.Fatalf("original roof owner changed for face %d", id)
							}
						}
						mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
						if err != nil || math.Abs(mass.Volume-want) > 1e-8 {
							t.Fatal("original material integral", mass.Volume, want, err)
						}
						for i, c := range contacts {
							station, factor := big.NewRat(2, 1), big.NewRat(2, 1)
							if axis == 1 {
								station, factor = big.NewRat(-1, 1), big.NewRat(3, 1)
							}
							station.Add(station, factor.Mul(factor, roots[i]))
							if c.Order != 1 || c.DerivativeGap.Sign() == 0 || c.Point[axis].Cmp(station) != 0 {
								t.Fatalf("original transverse crossing %d: %v, station %s", i, c, station)
							}
							for _, id := range c.Rays {
								e := s.edges[id]
								if (e.Start == c.Vertex) == (e.End == c.Vertex) {
									t.Fatalf("original ray %d is not incident to contact vertex %d", id, c.Vertex)
								}
							}
						}
						var raw, echo bytes.Buffer
						if err := r.WriteJSON(ctx, &raw); err != nil {
							t.Fatal(err)
						}
						loaded, err := ReadClippedPolynomialTransverseCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
						if err != nil || !reflect.DeepEqual(contacts, loaded.Contacts()) || !reflect.DeepEqual(h, loaded.History()) {
							t.Fatal("original complete native contact and shell ledger replay", err)
						}
						if err := loaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
							t.Fatal("canonical variable-count original-source record", err)
						}
						if axis == 0 && order == 0 && layout == 2 && n >= 4 {
							checkClippedShellDisplay(t, s, want)
							var forged clippedPolynomialCommonRecipe
							if err := json.Unmarshal(raw.Bytes(), &forged); err != nil {
								t.Fatal(err)
							}
							forged.Ledger.Contacts = forged.Ledger.Contacts[:n-1]
							data, err := json.Marshal(forged)
							if err != nil {
								t.Fatal(err)
							}
							bad, err := ReadClippedPolynomialTransverseCommonJSON(ctx, bytes.NewReader(data), JSONLimits{}, o)
							if bad != nil || !errors.Is(err, geom.ErrConvergence) {
								t.Fatal("complete source must detect the omitted crossing", bad, err)
							}
						}
					})
				}
			}
		}
	}
}

func TestNativeClippedPolynomialVariableTransverseCommonMatchesFixedCounts(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	_, two, _ := nativeWideTwoPolynomialNodeFixture(t, 0)
	stock := clippedTransverseFuseStock(t, 0, 1, two)
	fixedTwo, err := CommonClippedPolynomialEnvelope(ctx, stock[0], stock[1], stock[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	r, err := CommonClippedPolynomialTransverseEnvelope(ctx, stock[0], stock[1], stock[2], 0, 1e-9, o)
	contacts := fixedTwo.Contacts()
	if err != nil || !reflect.DeepEqual(contacts[:], r.Contacts()) ||
		!reflect.DeepEqual(fixedTwo.Boundary().Topology(), r.Boundary().Topology()) {
		t.Fatal("discovery must retain the exact two-contact shell", err)
	}
	_, three, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	stock = clippedTransverseFuseStock(t, 0, 1, three)
	fixedThree, err := CommonClippedPolynomialThreeTransverseEnvelope(ctx, stock[0], stock[1], stock[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	r, err = CommonClippedPolynomialTransverseEnvelope(ctx, stock[0], stock[1], stock[2], 0, 1e-9, o)
	if err != nil || !reflect.DeepEqual(fixedThree.Contacts(), r.Contacts()) || !reflect.DeepEqual(fixedThree.History(), r.History()) {
		t.Fatal("discovery must retain the exact three-contact shell ledger", err)
	}
}
