package geom

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestClippedPolynomialCutComponentGraph(t *testing.T) {
	ctx := context.Background()
	ranges := [3][2][2]float64{{{0, .875}, {.125, 1}}, {{0, 1}, {.125, .875}}, {{0, .875}, {.125, 1}}}
	// Integrate the original positive sectors using
	// v³/3-(f+g)v²/2+fgv and the physical determinant six.
	// Add A-only or B-only stock integrals over their whole rectangular
	// strips to the connected sector. The two tiny leading B components
	// never attach to an exclusive strip in these footprints.
	higher := [3][2]string{
		{"17946879650624928475/6762154840496799744", "17808202647060052699/6762154840496799744"},
		{"258238533933033416263/33810774202483998720", "258238533933033416263/33810774202483998720"},
		{"125317725088583476807/33810774202483998720"},
	}
	last := [3][2]string{
		{"43179726544756252749/22540516134989332480", "43641983223305838669/22540516134989332480"},
		{"18097610829/22540516134989332480", "18097610829/22540516134989332480"},
		{"70353944554503381069/22540516134989332480"},
	}
	for layout, physical := range ranges {
		for axis := range 2 {
			if axis == 1 && layout == 2 {
				continue
			}
			s := clippedPolynomialMaterialFixture(t, layout, axis, physical)
			for operandOrder := range 2 {
				p, e := DiscoverClippedSplineGraphPolynomialCut(ctx, s[0], s[1+operandOrder], s[2-operandOrder], axis, GraphPointContactOptions{})
				if e != nil || p.ExactOperations <= p.Material.ExactOperations {
					t.Fatal("complete original solid material connectivity", layout, axis, operandOrder, e)
				}
				checkClippedCutOrder(t, p, operandOrder,
					[][]int{{0, 1, 2}}, []string{higher[layout][axis]}, true, [3]int{3, 6, 4}[layout])
				tail := [3][]int{{2, 3}, {2}, {2, 3, 4}}[layout]
				checkClippedCutOrder(t, p, 1-operandOrder, [][]int{{0}, {1}, tail},
					[]string{"112545421/22540516134989332480", "1/5248122880", last[layout][axis]}, false, 2+(len(tail)))
			}
		}
	}
}

func checkClippedCutOrder(t *testing.T, p ClippedSplineGraphPolynomialCut, owner int, groups [][]int, volumes []string, self bool, chambers int) {
	t.Helper()
	r := p.Orders[owner]
	if len(r.Chambers) != chambers || len(r.Components) != len(groups) || len(r.Contacts) != 2 {
		t.Fatal("chamber components and exact contact count", len(r.Chambers), r.Components, r.Contacts)
	}
	if self {
		groups[0] = make([]int, chambers)
		for i := range groups[0] {
			groups[0][i] = i
		}
	}
	for i, c := range r.Components {
		if !slices.Equal(c.Chambers, groups[i]) || c.Volume.RatString() != volumes[i] {
			t.Fatal("independent exact component volume and chamber membership", i, c, volumes[i], groups[i])
		}
	}
	for i, contact := range r.Contacts {
		node := p.Material.Overlap.Projection.Arrangement.Nodes[i]
		if contact.Junction != i || contact.Vertex != node.Vertex || contact.SelfContact != self || contact.Sectors[1]-contact.Sectors[0] != 2 {
			t.Fatal("original separate local links with exact global classification", i, contact)
		}
		c := contact.Components
		if c[0] > c[1] {
			c[0], c[1] = c[1], c[0]
		}
		if self && c != ([2]int{0, 0}) || !self && c != ([2]int{i, i + 1}) {
			t.Fatal("source-model contact graph", i, c)
		}
		for j, ch := range contact.Chambers {
			if r.Chambers[ch].Region != node.Regions[contact.Sectors[j]] || r.Chambers[ch].Component != contact.Components[j] {
				t.Fatal("positive rays lead to recorded original region and component", i, j, contact)
			}
		}
	}
	var joins, stripJoins, compound int
	for i, pair := range r.AtomChambers {
		atom := p.Material.Boundaries[i]
		for side, ch := range pair {
			tile := atom.Tiles[side]
			if ch < 0 {
				if tile == 0 && p.Material.Overlap.Projection.Arrangement.Regions[atom.OverlapRegion].HeightSide == 1-2*owner ||
					tile > 0 && p.Material.Tiles[tile].RoofMask == 1<<owner {
					t.Fatal("positive stock atom side omitted from chamber graph", i, side)
				}
				continue
			}
			if r.Chambers[ch].Tile != tile || (tile == 0 && r.Chambers[ch].Region != atom.OverlapRegion) {
				t.Fatal("chamber keeps its original directed atom incidence", i, side, ch)
			}
		}
		if a, b := pair[0], pair[1]; a >= 0 && b >= 0 {
			joins++
			if r.Chambers[a].Tile != 0 && r.Chambers[b].Tile != 0 {
				stripJoins++
			}
			if r.Chambers[a].Component != r.Chambers[b].Component {
				t.Fatal("open material adjacency must lie in one component", pair)
			}
		}
	}
	terminals := map[int]bool{}
	for _, station := range r.Stations {
		if station.Terminal >= 0 {
			terminals[station.Terminal] = true
			if len(station.Chambers) > 1 && len(station.Strips) != 1 {
				t.Fatal("one overlap germ joins the exclusive positive strip at terminal", station)
			}
		}
		if len(station.Strips) > 1 {
			compound++
		}
		for _, ch := range station.Chambers {
			if r.Chambers[ch].Component != station.Component {
				t.Fatal("one connected local link at each stock-side station", station)
			}
		}
	}
	if len(terminals) != 4 || self && joins < 2 || (!self && chambers > 3 && joins == 0) ||
		stripJoins > 0 && compound == 0 {
		t.Fatal("incomplete original side and terminal local links", joins, stripJoins, compound, terminals)
	}
}

func TestClippedPolynomialCutComponentGraphRefusals(t *testing.T) {
	ctx := context.Background()
	s := clippedPolynomialFixture(t, 0, [2][2]float64{{0, .875}, {.125, 1}})
	p, e := DiscoverClippedSplineGraphPolynomialCut(ctx, s[0], s[1], s[2], 0, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := DiscoverClippedSplineGraphPolynomialCut(ctx, s[0], s[1], s[2], 0, GraphPointContactOptions{MaxOperations: p.ExactOperations})
	if e != nil || !reflect.DeepEqual(q.Orders, p.Orders) || q.ExactOperations != p.ExactOperations {
		t.Fatal("same complete component proof at the exact quota", e)
	}
	short, e := s[0].Trim(p.Material.Overlap.RestrictedDomains[0])
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		base *SurfacePatch
		o    GraphPointContactOptions
		err  error
	}{
		{s[0], GraphPointContactOptions{MaxOperations: p.ExactOperations - 1}, ErrConvergence},
		{short, GraphPointContactOptions{}, ErrDomain},
	} {
		q, e := DiscoverClippedSplineGraphPolynomialCut(ctx, tc.base, s[1], s[2], 0, tc.o)
		if !errors.Is(e, tc.err) || !reflect.DeepEqual(q, ClippedSplineGraphPolynomialCut{ExactOperations: q.ExactOperations}) {
			t.Fatal("resource and exclusive strip source failures publish no component graph", tc.err, e)
		}
	}
}
