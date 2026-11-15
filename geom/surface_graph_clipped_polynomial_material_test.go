package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestIntegrateClippedSplineGraphPolynomialMaterial(t *testing.T) {
	ctx := context.Background()
	for layout, ranges := range [][2][2]float64{{{0, .875}, {.125, 1}}, {{0, 1}, {.125, .875}}, {{0, .875}, {.125, 1}}} {
		for axis := range 2 {
			if layout == 2 && axis == 1 {
				continue
			}
			src := clippedPolynomialMaterialFixture(t, layout, axis, ranges)
			for order := range 2 {
				pair := [3]*SurfacePatch{src[0], src[1+order], src[2-order]}
				r, e := IntegrateClippedSplineGraphPolynomialMaterial(ctx, pair[0], pair[1], pair[2], axis, GraphPointContactOptions{})
				if e != nil || !r.Overlap.Projection.Arrangement.Proven || r.ExactOperations <= r.Overlap.ExactOperations {
					t.Fatal("exact material coverage", layout, axis, order, e)
				}
				checkClippedMaterialIntegrals(t, r, layout, axis, order)
				checkClippedMaterialBoundaries(t, r, pair, [3]int{3, 2, 4}[layout])
			}
		}
	}
}

func clippedPolynomialMaterialFixture(t *testing.T, layout, axis int, ranges [2][2]float64) [3]*SurfacePatch {
	t.Helper()
	s := clippedPolynomialFixture(t, axis, ranges)
	var trims [2]ParameterRectangle
	if layout == 1 {
		trims[1] = s[2].Domain()
		trims[1].VMin, trims[1].VMax = -.75, .75
	}
	if layout == 2 {
		trims[0], trims[1] = s[1].Domain(), s[2].Domain()
		trims[0].VMin, trims[1].VMin = 8.25, -.75
	}
	for i, d := range trims {
		if d.Valid() {
			var e error
			s[i+1], e = s[i+1].Trim(d)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	return s
}

func checkClippedMaterialIntegrals(t *testing.T, r ClippedSplineGraphPolynomialMaterial, layout, axis, order int) {
	t.Helper()
	// Independently integrate in unit physical x,y with Jacobian 6.
	// B=3+xy/64 and base=1/8+x/4+3y/16. A-B=(v-f)(v-g),
	// f=(2+3u-3u²+u³)/8 and g=f-(u-1/4)^3(u-3/4)^2,
	// with u free and v dependent. The v antiderivative is
	// v³/3-(f+g)v²/2+fgv. The three negative intervals integrate
	// |g-f|³/6 between free stations (1/8,1/4,3/4,7/8).
	var stocks = [3][2][2][]string{
		{
			{{"116961675429/9395240960", "490586450251/225485783040"}, {"6129/512", "15693/8192"}},
			{{"116961675429/9395240960", "485962230091/225485783040"}, {"6129/512", "15861/8192"}},
		},
		{
			{{"346260806127/37580963840", "490586450251/225485783040", "448956860221/225485783040", "119372908197/75161927680", "123798882981/75161927680"}, {"18387/2048"}},
			{{"346260806127/37580963840", "29389147/13762560", "30037711/13762560", "479796603211/300647710720", "447415453501/300647710720"}, {"18387/2048"}},
		},
		{
			{{"346260806127/37580963840", "485124896587/257698037760", "119372908197/75161927680"}, {"18387/2048", "875091/524288", "47583/32768"}},
		},
	}[layout][axis]
	var totals = [3][2][4]string{
		{
			{"134912913456862300307/11270258067494666240", "111848545689491438299/6762154840496799744", "17946879650624928475/6762154840496799744", "21589863274581882733/11270258067494666240"},
			{"134912913456862300307/11270258067494666240", "111848545689491438299/6762154840496799744", "17808202647060052699/6762154840496799744", "21820991613856675693/11270258067494666240"},
		},
		{
			{"101184685089833584787/11270258067494666240", "561792589236291855943/33810774202483998720", "258238533933033416263/33810774202483998720", "11252561773/11270258067494666240"},
			{"101184685089833584787/11270258067494666240", "561792589236291855943/33810774202483998720", "258238533933033416263/33810774202483998720", "11252561773/11270258067494666240"},
		},
		{
			{"101184685089833584787/11270258067494666240", "534402697196450571847/33810774202483998720", "125317725088583476807/33810774202483998720", "35176972279455446893/11270258067494666240"},
		},
	}[layout][axis]
	layerByOwner := [2][]string{
		{"1473424056523944013/22540516134989332480", "1861680768425145065/4508103226997866496"},
		{"112545421/22540516134989332480", "1/5248122880", "18097610829/22540516134989332480"},
	}
	if layout != 0 {
		layerByOwner[0] = []string{"399325343200047181/22540516134989332480", "4913505515115970189/22540516134989332480"}
	}
	wantRegions := [2][]int{{0, 4}, {1, 2, 3}}
	if len(r.Tiles) != len(stocks[0])+len(stocks[1])-1 {
		t.Fatal("disjoint complement strip count", len(r.Tiles), stocks)
	}
	if order == 1 {
		stocks[0], stocks[1] = stocks[1], stocks[0]
		layerByOwner[0], layerByOwner[1] = layerByOwner[1], layerByOwner[0]
		wantRegions[0], wantRegions[1] = wantRegions[1], wantRegions[0]
		totals[2], totals[3] = totals[3], totals[2]
	}
	check := func(q *big.Rat, expected string) {
		t.Helper()
		if q == nil || q.RatString() != expected {
			t.Fatal("independent exact physical volume", layout, axis, order, q, expected)
		}
	}
	counts := [2]int{}
	for i, tile := range r.Tiles {
		if i == 0 && tile.RoofMask != 3 || i > 0 && tile.RoofMask != 1 && tile.RoofMask != 2 {
			t.Fatal("stock membership", tile.RoofMask)
		}
		for k, q := range tile.StockVolumes {
			if tile.RoofMask&(1<<k) != 0 {
				check(q, stocks[k][counts[k]])
				counts[k]++
			} else if q != nil || tile.SourceDomains[1+k].Valid() {
				t.Fatal("unexpected source on an exclusive strip", k, tile)
			}
		}
	}
	for owner, layers := range r.OverlapCut {
		if len(layers) != len(wantRegions[owner]) {
			t.Fatal("retained difference components", owner, layers)
		}
		for i, layer := range layers {
			if layer.Region != wantRegions[owner][i] || r.Overlap.Projection.Arrangement.Regions[layer.Region].HeightSide != 1-2*owner {
				t.Fatal("ordered signed original region incidence", owner, layer)
			}
			check(layer.Volume, layerByOwner[owner][i])
		}
	}
	for i, q := range [4]*big.Rat{r.Common, r.Fuse, r.Cut[0], r.Cut[1]} {
		check(q, totals[i])
	}
}

func checkClippedMaterialBoundaries(t *testing.T, r ClippedSplineGraphPolynomialMaterial, src [3]*SurfacePatch, wantSteps int) {
	t.Helper()
	ctx := context.Background()
	var steps int
	ends := make(map[[2]string]bool)
	lengths := make([][4]big.Rat, len(r.Tiles))
	for _, edge := range r.Boundaries {
		vary := 0
		if edge.Points[0][0].Cmp(edge.Points[1][0]) == 0 {
			vary = 1
		}
		fixed := edge.Points[0][1-vary]
		if fixed.Cmp(edge.Points[1][1-vary]) != 0 || edge.Points[0][vary].Cmp(edge.Points[1][vary]) >= 0 {
			t.Fatal("canonical direction on each boundary atom", edge)
		}
		var mask, ending uint8
		for side, tile := range edge.Tiles {
			if tile < 0 {
				continue
			}
			d := r.Tiles[tile].Physical
			compass := [2][2]int{{0, 2}, {1, 3}}[vary][side]
			fixedLimit := [4]float64{d.VMin, d.UMax, d.VMax, d.UMin}[compass]
			lo, hi := d.UMin, d.UMax
			if vary == 1 {
				lo, hi = d.VMin, d.VMax
			}
			if fixed.Cmp(new(big.Rat).SetFloat64(fixedLimit)) != 0 ||
				edge.Points[0][vary].Cmp(new(big.Rat).SetFloat64(lo)) < 0 ||
				edge.Points[1][vary].Cmp(new(big.Rat).SetFloat64(hi)) > 0 {
				t.Fatal("directed left/right incidence is a whole tile side restriction", tile, edge)
			}
			length := new(big.Rat).Sub(edge.Points[1][vary], edge.Points[0][vary])
			lengths[tile][compass].Add(&lengths[tile][compass], length)
			mask |= r.Tiles[tile].RoofMask
			ending ^= r.Tiles[tile].RoofMask
		}
		if edge.EndingRoofs != ending {
			t.Fatal("exact original wall owner mask", edge.EndingRoofs, ending)
		}
		var heights [3]*big.Rat
		for i, owner := range src {
			if i > 0 && mask&(1<<(i-1)) == 0 {
				if edge.OwnerUV[i] != ([2][2]*big.Rat{}) {
					t.Fatal("absent stock supplied a boundary clock")
				}
				continue
			}
			for j, uv := range edge.OwnerUV[i] {
				p, e := owner.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil || p.Point[0].Cmp(edge.Points[j][0]) != 0 || p.Point[1].Cmp(edge.Points[j][1]) != 0 {
					t.Fatal("atom endpoint lifts on its original source", i, edge, e)
				}
				ends[[2]string{p.Point[0].RatString(), p.Point[1].RatString()}] = true
			}
			mid := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1)) }
			x, y := edge.OwnerUV[i][0], edge.OwnerUV[i][1]
			p, e := owner.EvaluateRational(ctx, mid(x[0], y[0]), mid(x[1], y[1]), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			heights[i] = p.Point[2]
		}
		if edge.OverlapRegion >= 0 {
			region := r.Overlap.Projection.Arrangement.Regions[edge.OverlapRegion]
			if heights[1].Cmp(heights[2]) != region.HeightSide {
				t.Fatal("the perimeter arc retains exact original roof order", edge)
			}
		}
		for _, w := range edge.Fuse {
			if w.Lower > ClippedPolynomialBaseHeight {
				if edge.OverlapRegion < 0 || w.Lower == w.Upper || heights[w.Lower-1].Cmp(heights[w.Upper-1]) >= 0 {
					t.Fatal("a Fuse step must lie between the two exact original roofs", edge)
				}
				steps++
			}
		}
		testClippedMaterialWallIntervals(t, r.Tiles, edge, heights)
	}
	for i, tile := range r.Tiles {
		d := tile.Physical
		wantCorners := map[[2]float64]bool{{d.UMin, d.VMin}: true, {d.UMin, d.VMax}: true, {d.UMax, d.VMin}: true, {d.UMax, d.VMax}: true}
		for k, uv := range tile.SourceDomains {
			if k > 0 && tile.RoofMask&(1<<(k-1)) == 0 {
				continue
			}
			corners := make(map[[2]float64]bool)
			for _, u := range [2]float64{uv.UMin, uv.UMax} {
				for _, v := range [2]float64{uv.VMin, uv.VMax} {
					p, e := src[k].EvaluateRational(ctx, new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					x, xexact := p.Point[0].Float64()
					y, yexact := p.Point[1].Float64()
					if !xexact || !yexact {
						t.Fatal("original clocks lift losslessly to the physical tile corners", i, k)
					}
					corners[[2]float64{x, y}] = true
				}
			}
			if !reflect.DeepEqual(corners, wantCorners) {
				t.Fatal("original base and roof chart restrictions cover the same tile", i, k, corners, wantCorners)
			}
		}
		for side, endpoints := range [4][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}, {d.UMin, d.UMax}, {d.VMin, d.VMax}} {
			want := new(big.Rat).Sub(new(big.Rat).SetFloat64(endpoints[1]), new(big.Rat).SetFloat64(endpoints[0]))
			if lengths[i][side].Cmp(want) != 0 {
				t.Fatal("wall atoms exactly exhaust every tile edge", i, side, &lengths[i][side], want)
			}
		}
	}
	if steps != wantSteps {
		t.Fatal("exact Fuse step incidence", steps, wantSteps)
	}
	for _, v := range r.Overlap.Projection.Arrangement.Vertices[:4] {
		if !ends[[2]string{v.Point[0].RatString(), v.Point[1].RatString()}] {
			t.Fatal("a branch terminal must split any incident wall", v)
		}
	}
}

// Compare wall occupancy to the Boolean evaluation of the adjacent stocks
// at an exact height in every open slab of this atom. Source heights above
// were evaluated independently on the original patches.
func testClippedMaterialWallIntervals(t *testing.T, tiles []ClippedPolynomialStockTile, edge ClippedPolynomialStockBoundary, h [3]*big.Rat) {
	t.Helper()
	for _, pair := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		a, b := h[pair[0]], h[pair[1]]
		if a == nil || b == nil {
			continue
		}
		z := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
		if h[1] != nil && z.Cmp(h[1]) == 0 || h[2] != nil && z.Cmp(h[2]) == 0 {
			continue
		}
		var kept [2][4]bool
		for side, tile := range edge.Tiles {
			if tile == -1 {
				continue
			}
			mask := tiles[tile].RoofMask
			a := mask&1 != 0 && z.Cmp(h[1]) < 0
			b := mask&2 != 0 && z.Cmp(h[2]) < 0
			kept[side] = [4]bool{a && b, a || b, a && !b, b && !a}
		}
		for op, walls := range [4][2]ClippedPolynomialWall{edge.Common, edge.Fuse, edge.Cut[0], edge.Cut[1]} {
			for side, wall := range walls {
				got := wall.Upper != 0 && z.Cmp(h[wall.Lower-1]) > 0 && z.Cmp(h[wall.Upper-1]) < 0
				if got != (kept[side][op] && !kept[1-side][op]) {
					t.Fatal("exact exposed material over the boundary", op, side, z, wall, edge)
				}
			}
		}
	}
}

func TestIntegrateClippedSplineGraphPolynomialMaterialRefusals(t *testing.T) {
	ctx := context.Background()
	s := clippedPolynomialFixture(t, 0, [2][2]float64{{0, .875}, {.125, 1}})
	proof, e := IntegrateClippedSplineGraphPolynomialMaterial(ctx, s[0], s[1], s[2], 0, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntegrateClippedSplineGraphPolynomialMaterial(ctx, s[0], s[1], s[2], 0, GraphPointContactOptions{MaxOperations: proof.ExactOperations})
	if e != nil || r.ExactOperations != proof.ExactOperations || !reflect.DeepEqual(r.Tiles, proof.Tiles) ||
		!reflect.DeepEqual(r.Boundaries, proof.Boundaries) || !reflect.DeepEqual(r.OverlapCut, proof.OverlapCut) {
		t.Fatal("same exact ledgers at the complete cumulative budget", e)
	}
	short, e := s[0].Trim(proof.Overlap.RestrictedDomains[0])
	if e != nil {
		t.Fatal(e)
	}
	shift := Identity()
	shift[0][3] = 2
	disjoint, e := s[2].Transformed(shift)
	if e != nil {
		t.Fatal(e)
	}
	narrow := clippedPolynomialFixture(t, 0, [2][2]float64{{0, .875}, {.375, .625}})
	// This exact base has the original height and affine XY map with a
	// positive nonconstant homogeneous weight field. It passes geometric
	// clipping, but lies outside the constant-weight integration contract.
	rationalBase := *s[0]
	rationalBase.degreeU, rationalBase.countU = 2, 3
	rationalBase.knotsU = []float64{8, 8, 8, 10, 10, 10}
	rationalBase.net = make([]exactHomogeneous, 6)
	for j := range 2 {
		for k, h0 := range s[0].net[j] {
			h1 := s[0].net[2+j][k]
			rationalBase.net[j][k] = h0
			rationalBase.net[2+j][k] = new(big.Rat).Add(h0, new(big.Rat).Quo(h1, big.NewRat(2, 1)))
			rationalBase.net[4+j][k] = new(big.Rat).Mul(big.NewRat(2, 1), h1)
		}
	}
	if e := rationalBase.computeBounds(&polynomialBudget{maxOperations: 1000, maxBits: 1024}); e != nil {
		t.Fatal(e)
	}
	if r, e := DiscoverClippedSplineGraphPolynomialArrangement(ctx, &rationalBase, s[1], s[2], 0, GraphPointContactOptions{}); e != nil || !r.Projection.Arrangement.Proven {
		t.Fatal("geometric clipping still proves this rational base", e)
	}
	for _, tc := range []struct {
		base, b *SurfacePatch
		o       GraphPointContactOptions
		want    error
	}{
		{short, s[2], GraphPointContactOptions{}, ErrDomain},
		{&rationalBase, s[2], GraphPointContactOptions{}, ErrConvergence},
		{s[0], disjoint, GraphPointContactOptions{}, ErrDomain},
		{s[0], narrow[2], GraphPointContactOptions{}, ErrConvergence},
		{s[0], s[2], GraphPointContactOptions{MaxOperations: proof.ExactOperations - 1}, ErrConvergence},
		{s[0], s[2], GraphPointContactOptions{MaxCells: 1}, ErrConvergence},
	} {
		r, e := IntegrateClippedSplineGraphPolynomialMaterial(ctx, tc.base, s[1], tc.b, 0, tc.o)
		if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, ClippedSplineGraphPolynomialMaterial{ExactOperations: r.ExactOperations}) {
			t.Fatal("coverage and resource failures publish no partial stock or wall ledger", tc.want, r.ExactOperations, e)
		}
	}
}
