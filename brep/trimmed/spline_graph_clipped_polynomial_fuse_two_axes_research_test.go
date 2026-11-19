//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialFuseTwoAxes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for layout := range 2 {
		for axis := range 2 {
			for profile := range 2 {
				s := twoAxisClippedFuseStock(t, axis, profile, layout)
				for order := range 2 {
					out, e := FuseClippedPolynomialEnvelope(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, o)
					if e != nil {
						t.Fatal(layout, axis, profile, order, e)
					}
					checkClippedFuseTwoAxisBase(t, out, layout)
					checkClippedFuseCompoundCorners(t, out, layout, axis, order)
					checkClippedFuseTwoAxisMeasures(t, out, layout, axis, profile)
					var raw bytes.Buffer
					if e := out.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					reloaded, e := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
					if e != nil || !reflect.DeepEqual(out.ledger, reloaded.ledger) || !reflect.DeepEqual(out.Walls(), reloaded.Walls()) ||
						out.ExactVolume().Cmp(reloaded.ExactVolume()) != 0 {
						t.Fatal("concave base and compound step incidence survive original chart reproof", layout, axis, profile, order, e)
					}
					var canonical bytes.Buffer
					if e := reloaded.WriteJSON(ctx, &canonical); e != nil || !bytes.Equal(raw.Bytes(), canonical.Bytes()) {
						t.Fatal("canonical two-axis source recipe", e)
					}
				}
			}
		}
	}
}

func checkClippedFuseTwoAxisBase(t *testing.T, out *ClippedPolynomialFuse, layout int) {
	t.Helper()
	s := out.Boundary()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
		len(s.vertices) != 35 || len(s.edges) != 58 || len(s.faces) != 25 {
		t.Fatal("the two-axis shell is closed and connected", top)
	}
	base := s.faces[out.ledger.Base]
	if base.Surface != out.sources[0] || len(base.Holes) != 0 || len(out.ledger.RoofFaces) != 9 {
		t.Fatal("one concave original base and nine original roof trims", out.ledger)
	}
	for i, id := range out.ledger.RoofFaces {
		if s.faces[id].Surface != out.sources[1+out.ledger.RoofOwners[i]] {
			t.Fatal("unmodified original roof support", i)
		}
	}
	ring := make([]geom.Vec2, len(base.Outer))
	for i, use := range base.Outer {
		edge := s.edges[use.Edge]
		id := edge.Start
		if use.Reversed {
			id = edge.End
		}
		p := s.vertices[id]
		ring[i] = geom.Vec2{X: p.X, Y: p.Y}
	}
	twiceArea := new(big.Rat)
	var cornerSigns []int
	for i, p := range ring {
		q, r := ring[(i+1)%len(ring)], ring[(i+2)%len(ring)]
		cross := func(a, b geom.Vec2) *big.Rat {
			return new(big.Rat).Sub(new(big.Rat).Mul(ratInset(a.X), ratInset(b.Y)), new(big.Rat).Mul(ratInset(a.Y), ratInset(b.X)))
		}
		twiceArea.Add(twiceArea, cross(p, q))
		if turn := cross(sub2(q, p), sub2(r, q)).Sign(); turn != 0 {
			cornerSigns = append(cornerSigns, turn)
		}
		if layout == 1 && p == (geom.Vec2{X: 2, Y: -1}) {
			t.Fatal("the empty southwest bounding-box corner is outside this union outline")
		}
	}
	var reentrant int
	for _, sign := range cornerSigns {
		if sign != twiceArea.Sign() {
			reentrant++
		}
	}
	// Two 7/8 square unit footprints minus a 3/4 square overlap gives
	// 31/32 of the bounding rectangle. Physical XY has determinant six.
	if len(cornerSigns) != 8 || reentrant != 2 || new(big.Rat).Abs(twiceArea).Cmp(big.NewRat(93, 8)) != 0 {
		t.Fatal("exact concave original base outline and physical coverage", cornerSigns, twiceArea)
	}
}

func checkClippedFuseCompoundCorners(t *testing.T, out *ClippedPolynomialFuse, layout, axis, order int) {
	t.Helper()
	physical := func(u, v float64) geom.Vec2 {
		if axis == 1 {
			u, v = v, u
		}
		return geom.Vec2{X: 2 + 2*u, Y: -1 + 3*v}
	}
	polarity := [2][2]float64{{.875, .125}, {.125, .875}}[layout]
	riserCorner, exteriorCorner := physical(.875, polarity[0]), physical(.875, polarity[1])
	base := geom.ClippedPolynomialBaseHeight
	a := geom.ClippedPolynomialRoofAHeight + geom.ClippedPolynomialHeightOwner(order)
	b := geom.ClippedPolynomialRoofBHeight - geom.ClippedPolynomialHeightOwner(order)

	var ri []int
	outside := make(map[[2]geom.ClippedPolynomialHeightOwner][]int)
	s := out.Boundary()
	for _, wall := range out.Walls() {
		for end, p := range wall.Points {
			if p != riserCorner && p != exteriorCorner {
				continue
			}
			if p == riserCorner && wall.Low == b && wall.High == a {
				if wall.EndingRoofs != 1<<order || len(wall.Stations[end]) != 1 {
					t.Fatal("two perpendicular risers terminate on the same original stock wall corner", wall, end)
				}
				ri = append(ri, wall.Stations[end][0])
			}
			if p == exteriorCorner {
				key := [2]geom.ClippedPolynomialHeightOwner{wall.Low, wall.High}
				if _, ok := outside[key]; ok {
					t.Fatal("ambiguous material at the compound vertical station", wall)
				}
				outside[key] = wall.Stations[end]
			}
			for i, owner := range [2]geom.ClippedPolynomialHeightOwner{wall.Low, wall.High} {
				f := s.faces[wall.Face]
				local := f.Outer[0]
				if i == 1 {
					local = f.Outer[1+len(wall.Stations[1])]
				}
				trim, ok := graphWallSourceTrim(local.Curve, i == 1, true)
				if !ok || trim.surface != out.sources[owner-1] || local.Edge != wall.TraceEdges[i] ||
					trim.a != wall.OwnerUV[i][0] || trim.b != wall.OwnerUV[i][1] {
					t.Fatal("compound wall coedge keeps the exact lower or upper source clock", wall, owner)
				}
			}
		}
	}
	if len(ri) != 2 || ri[0] != ri[1] {
		t.Fatal("orthogonal interior risers share their full vertical coedge", ri, riserCorner)
	}
	full, floor, step := outside[[2]geom.ClippedPolynomialHeightOwner{base, a}],
		outside[[2]geom.ClippedPolynomialHeightOwner{base, b}], outside[[2]geom.ClippedPolynomialHeightOwner{b, a}]
	if len(outside) != 3 || len(full) != 2 || len(floor) != 1 || len(step) != 1 || full[0] != floor[0] || full[1] != step[0] {
		t.Fatal("exterior wall splits at the continuing roof and shares both resulting station edges", exteriorCorner, outside)
	}
}

func checkClippedFuseTwoAxisMeasures(t *testing.T, out *ClippedPolynomialFuse, layout, axis, profile int) {
	t.Helper()
	// Independently integrate each original A and B footprint. Subtract
	// Common on the 3/4-square overlap using B-base and the negative gap
	// layers |f-g|³/6 times 105/128 and the physical determinant six.
	volumes := [2][2]string{
		{"648922216120422983239/41216943789694779392", "648876579147894187591/41216943789694779392"},
		{"655114280320634152519/41216943789694779392", "655027089903515035207/41216943789694779392"},
	}
	want, _ := new(big.Rat).SetString(volumes[layout][profile])
	if out.ExactVolume().Cmp(want) != 0 || want.Sign() <= 0 {
		t.Fatal("two-axis exact material agrees with independent stock integrals", out.ExactVolume(), want)
	}
	// Independent 32-point Gauss-Legendre integration on the original
	// derivative equations and heights. Include the wall height jump on
	// each physical grid line for the adjacent active stock columns.
	areas := [2][2][2]float64{
		{{39.275147310970262, 39.507435732923859}, {39.265910450569919, 39.500445183102293}},
		{{39.618139244353372, 39.872176607076540}, {39.601990108561473, 39.859783510191129}},
	}
	value, _ := want.Float64()
	area := areas[layout][profile][axis]
	p, e := out.Boundary().MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(p.Volume-value) > 1e-8 || math.Abs(p.Area-area) > 1e-8 {
		t.Fatal("two-axis native boundary measures match independent references", p.Volume, value, p.Area, area, e)
	}
}
