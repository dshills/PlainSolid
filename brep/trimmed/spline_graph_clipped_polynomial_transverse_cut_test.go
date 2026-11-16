package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func variableClippedTransverseRoof(t *testing.T, axis, nodes int) (*geom.BSplineSurface, []*big.Rat, []*big.Rat) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	var roots, gap []*big.Rat
	switch nodes {
	case 1:
		roots = []*big.Rat{big.NewRat(1, 2)}
		gap = []*big.Rat{big.NewRat(-1, 64), big.NewRat(1, 32), big.NewRat(-1, 64), big.NewRat(1, 32)}
	case 4, 5:
		roots = []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(5, 8), big.NewRat(3, 4)}
		gap = []*big.Rat{big.NewRat(1, 1)}
		if nodes == 5 {
			roots = append(append([]*big.Rat(nil), roots[:2]...), big.NewRat(1, 2), roots[2], roots[3])
			gap[0].Neg(gap[0])
		} else {
			f = []*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
		}
		for _, root := range roots {
			gap = multiplyCutOraclePolynomials(gap, []*big.Rat{new(big.Rat).Neg(root), big.NewRat(1, 1)})
		}
	default:
		t.Fatalf("unsupported independent fixture of %d crossings", nodes)
	}
	g := make([]*big.Rat, len(gap))
	for i, q := range gap {
		g[i] = new(big.Rat).Set(q)
		if i < len(f) {
			g[i].Add(g[i], f[i])
		}
	}
	_, roof, _ := twoPolynomialRoofFixture(t, axis, f, g)
	return roof, gap, roots
}

func multiplyCutOraclePolynomials(a, b []*big.Rat) []*big.Rat {
	product := make([]*big.Rat, len(a)+len(b)-1)
	for i := range product {
		product[i] = new(big.Rat)
	}
	for i, x := range a {
		for j, y := range b {
			product[i+j].Add(product[i+j], new(big.Rat).Mul(x, y))
		}
	}
	return product
}

func integralCutOracle(a []*big.Rat, lo, hi *big.Rat) *big.Rat {
	eval := func(x *big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := len(a) - 1; i >= 0; i-- {
			v.Mul(v, x).Add(v, new(big.Rat).Quo(a[i], big.NewRat(int64(i+1), 1)))
		}
		return v.Mul(v, x)
	}
	return new(big.Rat).Sub(eval(hi), eval(lo))
}

func variableClippedTransverseCutVolumes(axis, layout int, gap, roots []*big.Rat) []*big.Rat {
	freeLo, freeHi := big.NewRat(1, 8), big.NewRat(7, 8)
	if layout == 1 {
		freeLo, freeHi = new(big.Rat), big.NewRat(1, 1)
	}
	stations := append([]*big.Rat{freeLo}, roots...)
	stations = append(stations, freeHi)
	cube := multiplyCutOraclePolynomials(multiplyCutOraclePolynomials(gap, gap), gap)
	volumes := make([]*big.Rat, len(stations)-1)
	for i := range volumes {
		// The B-minus-A height is (105/128)(v-f)(g-v) with the
		// branches sorted. Integrating the v fiber gives |g-f|³/6;
		// the two original physical axis scales multiply to six.
		v := integralCutOracle(cube, stations[i], stations[i+1])
		volumes[i] = v.Abs(v).Mul(v, big.NewRat(105, 128))
	}
	// Original B has height 3+u*v/64 above a sloping stock base
	// 1/8+su*u+sv*v. Integrate its positive exclusive strip directly.
	su, sv := big.NewRat(1, 4), big.NewRat(3, 16)
	if axis == 1 {
		su, sv = sv, su
	}
	ul, uh, vl, vh := big.NewRat(7, 8), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)
	if layout == 1 {
		ul, uh, vl, vh = new(big.Rat), big.NewRat(1, 1), big.NewRat(7, 8), big.NewRat(1, 1)
	}
	mean := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1)) }
	u, v := mean(ul, uh), mean(vl, vh)
	height := big.NewRat(23, 8)
	height.Add(height, new(big.Rat).Mul(new(big.Rat).Mul(u, v), big.NewRat(1, 64)))
	height.Sub(height, new(big.Rat).Mul(su, u)).Sub(height, new(big.Rat).Mul(sv, v))
	strip := height.Mul(height, new(big.Rat).Sub(uh, ul)).Mul(height, new(big.Rat).Sub(vh, vl)).Mul(height, big.NewRat(6, 1))
	if layout == 1 {
		return append(volumes, strip)
	}
	last := volumes[len(volumes)-1]
	last.Add(last, strip)
	return volumes
}

func TestNativeClippedPolynomialVariableTransverseCut(t *testing.T) {
	ctx := context.Background()
	opts := PolynomialNodeCutOptions{}
	for _, nodes := range []int{1, 4, 5} {
		for axis := range 2 {
			roof, gap, roots := variableClippedTransverseRoof(t, axis, nodes)
			for layout := range 2 {
				t.Run(fmt.Sprintf("nodes%d/axis%d/layout%d", nodes, axis, layout), func(t *testing.T) {
					stocks := clippedTransverseFuseStock(t, axis, layout, roof)
					cut, err := CutClippedPolynomialTransverseComponents(ctx, stocks[0], stocks[2], stocks[1], axis, 1e-9, opts)
					if err != nil {
						t.Fatal(err)
					}
					components, contacts := cut.Components(), cut.Contacts()
					vols, total := cut.ExactComponentVolumes()
					expected := variableClippedTransverseCutVolumes(axis, layout, gap, roots)
					if len(components) != len(expected) || len(contacts) != nodes || len(vols) != len(expected) {
						t.Fatal("original slab volume or contact ledger length changed", len(components), len(contacts), vols, expected)
					}
					sum := new(big.Rat)
					for i, v := range vols {
						s := components[i]
						if v.Cmp(expected[i]) != 0 || v.Sign() <= 0 || len(s.Topology().Shells) != 1 ||
							s.Topology().EulerCharacteristic != 2 || len(cut.Chambers(i)) == 0 {
							t.Fatal("independent component volume and closed topology", i, v, expected[i], s.Topology())
						}
						sum.Add(sum, v)
					}
					if sum.Cmp(total) != 0 {
						t.Fatal("total must include every separate slab and exclusive strip", sum, total)
					}
					for i, c := range contacts {
						start, scale := big.NewRat(2, 1), big.NewRat(2, 1)
						if axis == 1 {
							start, scale = big.NewRat(-1, 1), big.NewRat(3, 1)
						}
						station := new(big.Rat).Add(start, scale.Mul(scale, roots[i]))
						if c.Order != 1 || c.Point[axis].Cmp(station) != 0 || c.DerivativeGap.Sign() == 0 ||
							c.Components != ([2]int{i, i + 1}) {
							t.Fatal("crossing must join the original adjacent slab occurrences", i, c, station)
						}
					}
					var receipt, echo bytes.Buffer
					if err := cut.WriteJSON(ctx, &receipt); err != nil {
						t.Fatal(err)
					}
					loaded, err := ReadClippedPolynomialTransverseCutJSON(ctx, bytes.NewReader(receipt.Bytes()), JSONLimits{}, opts)
					if err != nil || !reflect.DeepEqual(contacts, loaded.Contacts()) ||
						!reflect.DeepEqual(cut.proof.shells, loaded.proof.shells) {
						t.Fatal("source replay must prove every original chart, chamber and junction", err)
					}
					if err := loaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(echo.Bytes(), receipt.Bytes()) {
						t.Fatal("variable count receipt must be deterministic", err)
					}
					if axis == 0 && layout == 0 {
						invalid, err := CutClippedPolynomialTransverseComponents(ctx, stocks[0], stocks[1], stocks[2], axis, 1e-9, opts)
						if invalid != nil || !errors.Is(err, ErrClippedPolynomialCutSelfContact) {
							t.Fatal("a free-axis exterior strip creates separate sectors within the same component", err)
						}
					}
					if nodes == 4 && axis == 0 && layout == 1 {
						checkVariableClippedCutDisplayAndRefusal(t, cut, receipt.Bytes())
					}
				})
			}
		}
	}
}

func checkVariableClippedCutDisplayAndRefusal(t *testing.T, cut *ClippedPolynomialTransverseCut, receipt []byte) {
	t.Helper()
	ctx := context.Background()
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	ms, err := cut.Tessellate(ctx, o)
	if err != nil || len(ms) != len(cut.Components()) {
		t.Fatal("closed source-contact display", len(ms), err)
	}
	var bad clippedPolynomialCutRecipe
	if err := json.Unmarshal(receipt, &bad); err != nil {
		t.Fatal(err)
	}
	// Forge a *consistent* lower recorded count, so structural validation
	// alone can no longer distinguish it. Re-discovery must find the fourth.
	bad.Orders, bad.DerivativeGaps, bad.Contacts = bad.Orders[:3], bad.DerivativeGaps[:3], bad.Contacts[:3]
	bad.Regions = bad.Regions[:4]
	encoded, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := ReadClippedPolynomialTransverseCutJSON(ctx, bytes.NewReader(encoded), JSONLimits{}, PolynomialNodeCutOptions{})
	if rejected != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("forged complete-looking contact count was accepted", err)
	}
}

func TestNativeClippedPolynomialVariableTransverseCutMatchesFixedCounts(t *testing.T) {
	ctx := context.Background()
	opts := PolynomialNodeCutOptions{}
	_, two, _ := nativeWideTwoPolynomialNodeFixture(t, 0)
	stocks := clippedTransverseFuseStock(t, 0, 1, two)
	fixedTwo, err := CutClippedPolynomialComponents(ctx, stocks[0], stocks[2], stocks[1], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	variable, err := CutClippedPolynomialTransverseComponents(ctx, stocks[0], stocks[2], stocks[1], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	contacts := fixedTwo.Contacts()
	v, total := fixedTwo.ExactComponentVolumes()
	got, sum := variable.ExactComponentVolumes()
	if !reflect.DeepEqual(contacts[:], variable.Contacts()) || !reflect.DeepEqual(v, got) || total.Cmp(sum) != 0 {
		t.Fatal("automatic discovery must keep the two original contacts and component measures")
	}
	_, three, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	stocks = clippedTransverseFuseStock(t, 0, 1, three)
	fixedThree, err := CutClippedPolynomialThreeTransverseComponents(ctx, stocks[0], stocks[2], stocks[1], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	variable, err = CutClippedPolynomialTransverseComponents(ctx, stocks[0], stocks[2], stocks[1], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	v, total = fixedThree.ExactComponentVolumes()
	got, sum = variable.ExactComponentVolumes()
	if !reflect.DeepEqual(fixedThree.Contacts(), variable.Contacts()) || !reflect.DeepEqual(v, got) || total.Cmp(sum) != 0 {
		t.Fatal("automatic discovery must keep the three original contacts and component measures")
	}
}
