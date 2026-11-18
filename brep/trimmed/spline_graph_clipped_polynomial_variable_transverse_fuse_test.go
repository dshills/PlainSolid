package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func variableClippedFuseVolume(axis, layout int, gap, roots []*big.Rat) *big.Rat {
	z, p, q, one := new(big.Rat), big.NewRat(1, 8), big.NewRat(7, 8), big.NewRat(1, 1)
	rectangles := [3][2][4]*big.Rat{
		{{z, q, z, one}, {p, one, z, one}},
		{{z, one, z, q}, {z, one, p, one}},
		{{z, q, z, q}, {p, one, p, one}},
	}
	a, b := rectangles[layout][0], rectangles[layout][1]
	du, dv := big.NewRat(1, 4), big.NewRat(3, 16)
	if axis == 1 {
		du, dv = dv, du
	}
	bilinearStock := func(box [4]*big.Rat) *big.Rat {
		u := new(big.Rat).Quo(new(big.Rat).Add(box[0], box[1]), big.NewRat(2, 1))
		v := new(big.Rat).Quo(new(big.Rat).Add(box[2], box[3]), big.NewRat(2, 1))
		mean := big.NewRat(23, 8)
		mean.Add(mean, new(big.Rat).Mul(new(big.Rat).Mul(u, v), big.NewRat(1, 64)))
		mean.Sub(mean, new(big.Rat).Mul(u, du)).Sub(mean, new(big.Rat).Mul(v, dv))
		return mean.Mul(mean, new(big.Rat).Sub(box[1], box[0])).Mul(mean, new(big.Rat).Sub(box[3], box[2])).Mul(mean, big.NewRat(6, 1))
	}
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g, sum := make([]*big.Rat, len(gap)), make([]*big.Rat, len(gap))
	for i, coefficient := range gap {
		g[i], sum[i] = new(big.Rat).Set(coefficient), new(big.Rat).Set(coefficient)
		if i < len(f) {
			g[i].Add(g[i], f[i])
			sum[i].Add(g[i], f[i])
		}
	}
	fg := multiplyCutOraclePolynomials(f, g)
	// The original A-minus-B height divided by 105/128 is
	// (v-f)(v-g). Its v antiderivative is v³/3-(f+g)v²/2+fg*v.
	cap := make([]*big.Rat, len(fg))
	for i, coefficient := range fg {
		cap[i] = new(big.Rat).Set(coefficient)
	}
	v0, v1 := a[2], a[3]
	dv2 := new(big.Rat).Sub(new(big.Rat).Mul(v1, v1), new(big.Rat).Mul(v0, v0))
	dv3 := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Mul(v1, v1), v1), new(big.Rat).Mul(new(big.Rat).Mul(v0, v0), v0))
	for i := range cap {
		cap[i].Mul(cap[i], new(big.Rat).Sub(v1, v0))
		if i < len(sum) {
			cap[i].Sub(cap[i], new(big.Rat).Mul(sum[i], new(big.Rat).Mul(dv2, big.NewRat(1, 2))))
		}
	}
	cap[0].Add(cap[0], new(big.Rat).Mul(dv3, big.NewRat(1, 3)))
	raised := integralCutOracle(cap, a[0], a[1])
	raised.Mul(raised, big.NewRat(6*105, 128))
	all := new(big.Rat).Add(bilinearStock(a), bilinearStock(b))
	all.Add(all, raised)
	return all.Sub(all, variableClippedCommonVolume(axis, layout, gap, roots))
}

func TestNativeClippedPolynomialVariableTransverseFuse(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for _, count := range []int{1, 5} {
		for axis := range 2 {
			roof, gap, roots := variableClippedTransverseRoof(t, axis, count)
			for layout := range 3 {
				stocks := clippedTransverseFuseStock(t, axis, layout, roof)
				want := variableClippedFuseVolume(axis, layout, gap, roots)
				wantFloat, _ := want.Float64()
				for order := range 2 {
					t.Run(fmt.Sprintf("nodes%d/axis%d/layout%d/order%d", count, axis, layout, order), func(t *testing.T) {
						out, err := FuseClippedPolynomialEnvelope(ctx, stocks[0], stocks[1+order], stocks[2-order], axis, 1e-9, opts)
						if err != nil {
							t.Fatal(err)
						}
						body, receipt := out.Boundary(), out.ledger
						props, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
						if err != nil || out.ExactVolume().Cmp(want) != 0 || math.Abs(props.Volume-wantFloat) > 1e-8 {
							t.Fatal("independent original union material", out.ExactVolume(), props.Volume, want, err)
						}
						top := body.Topology()
						if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
							len(receipt.Contacts) != count || len(receipt.Edges) != 2*(count+1) {
							t.Fatal("complete source shell topology and original vertices", top, receipt)
						}
						var raw, echo bytes.Buffer
						if err := out.WriteJSON(ctx, &raw); err != nil {
							t.Fatal(err)
						}
						reopened, err := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
						if err != nil || !reflect.DeepEqual(reopened.ledger, receipt) || !reflect.DeepEqual(reopened.Walls(), out.Walls()) {
							t.Fatal("complete source reuse and all original wall bindings", err)
						}
						if err := reopened.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
							t.Fatal("canonical source replay", err)
						}
						if axis == 0 && layout == 2 && order == 0 {
							checkClippedFuseDisplay(t, out)
						}
					})
				}
			}
		}
	}
}
