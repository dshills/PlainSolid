package geom

import (
	"context"
	"math/big"
	"slices"
)

// ClippedPolynomialCutChamber is a positive volume over one selected
// original overlap region or one exclusive stock tile. Tile 0 identifies
// the overlap. Region is -1 on an exclusive tile. Lower identifies the
// original floor of the chamber. All chambers share the minuend roof.
type ClippedPolynomialCutChamber struct {
	Tile, Region, Component int
	Lower                   ClippedPolynomialHeightOwner
	Volume                  *big.Rat
}

// ClippedPolynomialCutComponent contains chambers connected by strictly
// positive material across open side atoms. Only point contacts can join
// the closures of different reported components.
type ClippedPolynomialCutComponent struct {
	Chambers []int
	Volume   *big.Rat
}

// ClippedPolynomialCutContact classifies the two locally separated positive
// sectors at an original interior junction. Chambers and Components are in
// original model-CCW sector order, starting at the first selected sector.
// SelfContact means both local links belong to the same material component.
type ClippedPolynomialCutContact struct {
	Junction, Vertex int
	Sectors          [2]int
	Chambers         [2]int
	Components       [2]int
	SelfContact      bool
}

// ClippedPolynomialCutStation gives the chamber incidences at an endpoint
// of the stock-side partition. Chambers connect locally in the upper layer.
// Strips gives the connected lower layer where it exists. Terminal indexes
// the original arrangement vertex if a graph branch ends here, or -1.
type ClippedPolynomialCutStation struct {
	Point     [2]*big.Rat
	Terminal  int
	Component int
	Atoms     []int
	Chambers  []int
	Strips    []int
}

// ClippedPolynomialCutOrder records the positive chamber graph and its exact
// volume and point contacts. AtomChambers identifies retained chambers on
// the left and right of each material boundary atom, with -1 for absence.
// An atom with two chamber IDs joins their open, positive height intervals.
type ClippedPolynomialCutOrder struct {
	Chambers     []ClippedPolynomialCutChamber
	AtomChambers [][2]int
	Components   []ClippedPolynomialCutComponent
	Contacts     []ClippedPolynomialCutContact
	Stations     []ClippedPolynomialCutStation
	Total        *big.Rat
}

// ClippedSplineGraphPolynomialCut gives the ordered material and component
// proof for both differences. Orders[0] is A minus B.
type ClippedSplineGraphPolynomialCut struct {
	Material        ClippedSplineGraphPolynomialMaterial
	Orders          [2]ClippedPolynomialCutOrder
	ExactOperations int
}

// DiscoverClippedSplineGraphPolynomialCut proves positive-area adjacencies
// and the local incidences of both original Cut orders. It reports interior
// self-contacts explicitly. Unresolved boundary-station links and incomplete
// material proofs fail without publishing chambers or contacts.
func DiscoverClippedSplineGraphPolynomialCut(ctx context.Context, base, a, b *SurfacePatch, axis int,
	o GraphPointContactOptions) (r ClippedSplineGraphPolynomialCut, err error) {
	o, e := alignedPolynomialProofOptions(o)
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = ClippedSplineGraphPolynomialCut{}
		}
		r.ExactOperations = min(budget.operations, budget.maxOperations)
	}()
	r.Material, e = IntegrateClippedSplineGraphPolynomialMaterial(ctx, base, a, b, axis, o)
	budget.operations = r.Material.ExactOperations
	if e != nil {
		return r, e
	}
	for owner := range r.Orders {
		r.Orders[owner], e = clippedPolynomialCutOrder(ctx, r.Material, owner, budget)
		if e != nil {
			return r, e
		}
	}
	return r, ctx.Err()
}

func clippedPolynomialCutOrder(ctx context.Context, material ClippedSplineGraphPolynomialMaterial, owner int,
	budget *polynomialBudget) (ClippedPolynomialCutOrder, error) {
	var r ClippedPolynomialCutOrder
	arrangement := material.Overlap.Projection.Arrangement
	regionIDs := make([]int, len(arrangement.Regions))
	tileIDs := make([]int, len(material.Tiles))
	for i := range regionIDs {
		regionIDs[i] = -1
	}
	for i := range tileIDs {
		tileIDs[i] = -1
	}
	lower := [2]ClippedPolynomialHeightOwner{ClippedPolynomialRoofBHeight, ClippedPolynomialRoofAHeight}[owner]
	for _, layer := range material.OverlapCut[owner] {
		regionIDs[layer.Region] = len(r.Chambers)
		r.Chambers = append(r.Chambers, ClippedPolynomialCutChamber{
			Tile: 0, Region: layer.Region, Component: -1, Lower: lower, Volume: new(big.Rat).Set(layer.Volume),
		})
	}
	for i, tile := range material.Tiles[1:] {
		if tile.RoofMask == 1<<owner {
			tileIDs[i+1] = len(r.Chambers)
			r.Chambers = append(r.Chambers, ClippedPolynomialCutChamber{
				Tile: i + 1, Region: -1, Component: -1, Lower: ClippedPolynomialBaseHeight, Volume: new(big.Rat).Set(tile.StockVolumes[owner]),
			})
		}
	}
	neighbors := make([][]int, len(r.Chambers))
	for _, atom := range material.Boundaries {
		var pair = [2]int{-1, -1}
		for side, tile := range atom.Tiles {
			switch {
			case tile == 0:
				pair[side] = regionIDs[atom.OverlapRegion]
			case tile > 0:
				pair[side] = tileIDs[tile]
			}
			if pair[side] < 0 && atom.Cut[owner][side].Upper != ClippedPolynomialNoHeight {
				return ClippedPolynomialCutOrder{}, ErrConvergence
			}
		}
		if a, b := pair[0], pair[1]; a >= 0 && b >= 0 {
			// Both open layers have the same upper graph. Their common
			// interval above the greater lower graph is strictly positive.
			neighbors[a] = append(neighbors[a], b)
			neighbors[b] = append(neighbors[b], a)
		}
		r.AtomChambers = append(r.AtomChambers, pair)
	}
	if e := r.assignComponents(ctx, neighbors, material.Cut[owner], budget); e != nil {
		return ClippedPolynomialCutOrder{}, e
	}
	for i, node := range arrangement.Nodes {
		var contact = ClippedPolynomialCutContact{Junction: i, Vertex: node.Vertex}
		n := 0
		for sector, region := range node.Regions {
			id := regionIDs[region]
			if id < 0 {
				continue
			}
			if n == 2 {
				return ClippedPolynomialCutOrder{}, ErrConvergence
			}
			contact.Sectors[n], contact.Chambers[n], contact.Components[n] = sector, id, r.Chambers[id].Component
			n++
		}
		// Away from a junction each branch separates opposite signs.
		// At a proved junction the two selected sectors alternate with
		// sectors of the other sign, even for coincident lower jets.
		if n != 2 || contact.Sectors[1]-contact.Sectors[0] != 2 || contact.Chambers[0] == contact.Chambers[1] {
			return ClippedPolynomialCutOrder{}, ErrConvergence
		}
		contact.SelfContact = contact.Components[0] == contact.Components[1]
		r.Contacts = append(r.Contacts, contact)
	}
	stations, e := clippedPolynomialCutStations(ctx, material, r)
	if e != nil {
		return ClippedPolynomialCutOrder{}, e
	}
	r.Stations = stations
	return r, nil
}

func (r *ClippedPolynomialCutOrder) assignComponents(ctx context.Context, neighbors [][]int, target *big.Rat,
	budget *polynomialBudget) error {
	r.Total = new(big.Rat)
	for start, chamber := range r.Chambers {
		if e := ctx.Err(); e != nil {
			return e
		}
		if chamber.Component >= 0 {
			continue
		}
		id := len(r.Components)
		component := ClippedPolynomialCutComponent{Volume: new(big.Rat)}
		queue := []int{start}
		r.Chambers[start].Component = id
		for i := 0; i < len(queue); i++ {
			ch := queue[i]
			if r.Chambers[ch].Volume.Sign() <= 0 {
				return ErrConvergence
			}
			component.Chambers = append(component.Chambers, ch)
			component.Volume.Add(component.Volume, r.Chambers[ch].Volume)
			if e := budget.check(component.Volume); e != nil {
				return e
			}
			for _, other := range neighbors[ch] {
				if r.Chambers[other].Component == -1 {
					r.Chambers[other].Component = id
					queue = append(queue, other)
				}
			}
		}
		slices.Sort(component.Chambers)
		r.Components = append(r.Components, component)
		r.Total.Add(r.Total, component.Volume)
		if e := budget.check(r.Total); e != nil {
			return e
		}
	}
	if r.Total.Sign() <= 0 || r.Total.Cmp(target) != 0 {
		return ErrConvergence
	}
	return nil
}

func clippedPolynomialCutStations(ctx context.Context, material ClippedSplineGraphPolynomialMaterial,
	order ClippedPolynomialCutOrder) ([]ClippedPolynomialCutStation, error) {
	keyOf := func(xy [2]*big.Rat) [2]string { return [2]string{xy[0].RatString(), xy[1].RatString()} }
	terminals := make(map[[2]string]int)
	for i, v := range material.Overlap.Projection.Arrangement.Vertices[:4] {
		terminals[keyOf(v.Parameters)] = i
	}
	ids := make(map[[2]string]int)
	var stations []ClippedPolynomialCutStation
	for i, pair := range order.AtomChambers {
		if pair == ([2]int{-1, -1}) {
			continue
		}
		for _, p := range material.Boundaries[i].Points {
			key := keyOf(p)
			id, exists := ids[key]
			if !exists {
				terminal, ok := terminals[key]
				if !ok {
					terminal = -1
				}
				id = len(stations)
				ids[key] = id
				stations = append(stations, ClippedPolynomialCutStation{Point: p, Terminal: terminal})
			}
			s := &stations[id]
			s.Atoms = append(s.Atoms, i)
			for _, ch := range pair {
				if ch >= 0 {
					s.Chambers = append(s.Chambers, ch)
				}
			}
		}
	}
	for i := range stations {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		s := &stations[i]
		slices.Sort(s.Chambers)
		s.Chambers = slices.Compact(s.Chambers)
		for _, ch := range s.Chambers {
			if order.Chambers[ch].Tile != 0 {
				s.Strips = append(s.Strips, ch)
			}
		}
		if !clippedCutStationConnected(s.Chambers, s.Atoms, order.AtomChambers) ||
			!clippedCutStationConnected(s.Strips, s.Atoms, order.AtomChambers) {
			return nil, ErrConvergence
		}
		s.Component = order.Chambers[s.Chambers[0]].Component
	}
	return stations, nil
}

// Every original tile has a single angular sector at a station. The
// interior-root proof keeps branches off the rectangle corners and gives
// at most one branch ending at any other station. The selected overlap
// chamber then also has a single sector. Since all chambers have the same
// upper roof and just two possible floors, there are two local incidence
// graphs to check: strips below the cutter and all retained chambers at
// or above it. An atom joins the sectors for every incident height in the
// closure of their open common interval, including a tapered terminal.
func clippedCutStationConnected(chambers, atoms []int, pairs [][2]int) bool {
	if len(chambers) < 2 {
		return true
	}
	visited := map[int]bool{chambers[0]: true}
	for progress := true; progress; {
		progress = false
		for _, i := range atoms {
			a, b := pairs[i][0], pairs[i][1]
			if !slices.Contains(chambers, a) || !slices.Contains(chambers, b) || visited[a] == visited[b] {
				continue
			}
			visited[a], visited[b], progress = true, true, true
		}
	}
	return len(visited) == len(chambers)
}
