package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

const nativeClippedPolynomialCut = "trimmed-clipped-polynomial-cut-components"

// ErrClippedPolynomialCutSelfContact identifies two separated local links
// within one connected positive Cut component.
var ErrClippedPolynomialCutSelfContact = errors.New("trimmed: clipped polynomial Cut has an intra-component self-contact")

// ClippedPolynomialCutRegion identifies the source chamber of a component.
// Tile 0 denotes an original overlap region. Region is -1 on an exclusive tile.
type ClippedPolynomialCutRegion struct {
	Tile   int `json:"tile"`
	Region int `json:"region"`
}

// ClippedPolynomialCut contains separately embedded source-backed shells
// and a complete exact inter-component point contact ledger.
type ClippedPolynomialCut struct {
	native     *AlignedPolynomialCut
	footprints [2]geom.ParameterRectangle
	shells     []clippedPolynomialCutShellLedger
}

type clippedPolynomialCutShellLedger struct {
	Chambers []ClippedPolynomialCutRegion   `json:"chambers"`
	Walls    []ClippedPolynomialWallBinding `json:"walls"`
	Volume   string                         `json:"exactVolume"`
}

// Components returns the native shells in deterministic source chamber order.
func (r *ClippedPolynomialCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return r.native.Components()
}

// ExactComponentVolumes returns the exact source volume of each shell.
func (r *ClippedPolynomialCut) ExactComponentVolumes() ([]*big.Rat, *big.Rat) {
	if r == nil {
		return nil, nil
	}
	return r.native.ExactComponentVolumes()
}

// Contacts returns both original junctions and their component-local vertices.
func (r *ClippedPolynomialCut) Contacts() [2]PolynomialMixedCutContact {
	if r == nil {
		return [2]PolynomialMixedCutContact{}
	}
	return r.native.Contacts()
}

// Chambers returns the original positive regions and exclusive tiles in a shell.
func (r *ClippedPolynomialCut) Chambers(component int) []ClippedPolynomialCutRegion {
	if r == nil || component < 0 || component >= len(r.shells) {
		return nil
	}
	return slices.Clone(r.shells[component].Chambers)
}

// Walls returns the exposed original wall and source coedge bindings of a shell.
func (r *ClippedPolynomialCut) Walls(component int) []ClippedPolynomialWallBinding {
	if r == nil || component < 0 || component >= len(r.shells) {
		return nil
	}
	return cloneClippedPolynomialWallBindings(r.shells[component].Walls)
}

// CutClippedPolynomialComponents subtracts B from A on unequal axis-aligned
// original footprints. It admits two transverse crossings or a cubic crossing
// and a quadratic tangency inside the overlap. Each component must have a
// single cyclic link at each native vertex. Components can share only the
// exact indexed points proved by the original source arrangement.
func CutClippedPolynomialComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o PolynomialNodeCutOptions) (*ClippedPolynomialCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	var patches [3]*geom.SurfacePatch
	for i, source := range sources {
		p, e := geom.NewSurfacePatch(source, o.Section.Patch)
		if e != nil {
			return nil, e
		}
		patches[i] = p
	}
	cut, e := geom.DiscoverClippedSplineGraphPolynomialCut(ctx, patches[0], patches[1], patches[2], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if cut.Material.Footprints[0] == cut.Material.Footprints[1] {
		return nil, geom.ErrDomain
	}
	projection := cut.Material.Overlap.Projection
	proof := projection.Arrangement
	if !isNativeClippedPolynomialArrangement(proof, axis) {
		return nil, geom.ErrConvergence
	}
	order := cut.Orders[0]
	for _, contact := range order.Contacts {
		if contact.SelfContact {
			return nil, fmt.Errorf("%w in component %d at junction %d: %w", ErrClippedPolynomialCutSelfContact,
				contact.Components[0], contact.Junction, geom.ErrConvergence)
		}
	}
	baseChart, e := polynomialEnvelopeBaseChart(cut.Material.Overlap.BaseAlignment)
	if e != nil {
		return nil, e
	}
	roofCharts := projectedPolynomialCharts(projection)
	charts := [3]graphEnvelopeChart{baseChart, roofCharts[0], roofCharts[1]}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	ownerPieces, e := polynomialOwnerPieces(proof, projection.Owners)
	if e != nil {
		return nil, e
	}

	var material polynomialNodeCutMaterial
	var volumes []*big.Rat
	var ledgers []clippedPolynomialCutShellLedger
	var contour [][]int
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	for id, component := range order.Components {
		b := clippedStockShellBuilder{sources: sources, evaluate: evaluate,
			vertexIDs: make(map[geom.Vec3]int), edgeIDs: make(map[[2]int]int)}
		part, walls, section, e := b.buildCutShell(ctx, cut.Material, order, id, uv, ownerPieces, charts, tolerance, o.Proof)
		if e != nil {
			return nil, fmt.Errorf("trimmed: clipped Cut component %d: %w", id, e)
		}
		ledger := clippedPolynomialCutShellLedger{Walls: walls, Volume: component.Volume.RatString()}
		for _, ch := range component.Chambers {
			c := order.Chambers[ch]
			ledger.Chambers = append(ledger.Chambers, ClippedPolynomialCutRegion{Tile: c.Tile, Region: c.Region})
			if c.Region >= 0 {
				material.regions = append(material.regions, c.Region)
			}
		}
		material.components = append(material.components, part)
		contour = append(contour, section)
		ledgers = append(ledgers, ledger)
		volumes = append(volumes, component.Volume)
	}
	slices.Sort(material.regions)
	for _, contact := range order.Contacts {
		p := proof.Vertices[contact.Vertex]
		entry := TwoPolynomialNodeComponentContact{Parameters: p.Parameters, Point: p.Point, Components: contact.Components}
		if entry.Components[0] > entry.Components[1] {
			entry.Components[0], entry.Components[1] = entry.Components[1], entry.Components[0]
		}
		for j, id := range entry.Components {
			entry.Vertices[j], e = polynomialCutContactVertex(material.components[id], OpenSectionHistory{ContourEdges: contour[id]}, xyz[contact.Vertex])
			if e != nil {
				return nil, e
			}
		}
		material.contacts = append(material.contacts, entry)
	}
	if e := material.validateContactIdentities(); e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	native := &AlignedPolynomialCut{sources: sources, material: material, orders: [2]int(proof.ContactOrder),
		derivatives: [2]*big.Rat(proof.DerivativeGap), volumes: volumes, total: order.Total,
		axis: axis, tolerance: tolerance, kind: nativeClippedPolynomialCut}
	return &ClippedPolynomialCut{native: native, footprints: cut.Material.Footprints, shells: ledgers}, nil
}

func isNativeClippedPolynomialArrangement(proof geom.SplineGraphPolynomialArrangement, axis int) bool {
	return isNativeAlignedPolynomialArrangement(proof, axis) || isNativeClippedTransverseArrangement(proof, axis, 2)
}

func isNativeClippedTransverseArrangement(proof geom.SplineGraphPolynomialArrangement, axis, nodes int) bool {
	return proof.Proven && proof.FreeAxis == axis && proof.Graph.ProjectionOrientation == 1 &&
		len(proof.ContactOrder) == nodes && !slices.ContainsFunc(proof.ContactOrder, func(order int) bool { return order != 1 })
}

func (b *clippedStockShellBuilder) buildCutShell(ctx context.Context, material geom.ClippedSplineGraphPolynomialMaterial,
	order geom.ClippedPolynomialCutOrder, component int, uv []geom.Vec2, pieces [2][][]openSectionPiece, charts [3]graphEnvelopeChart,
	tolerance float64, proof geom.GraphPointContactOptions) (Solid, []ClippedPolynomialWallBinding, []int, error) {
	e := b.prepareSpans(ctx, material.Boundaries, func(id int, atom geom.ClippedPolynomialStockBoundary) (uint8, [2]geom.ClippedPolynomialWall) {
		var owners uint8
		var walls [2]geom.ClippedPolynomialWall
		for side, ch := range order.AtomChambers[id] {
			if ch >= 0 && order.Chambers[ch].Component == component {
				owners |= 1<<1 | 1<<(order.Chambers[ch].Lower-1)
				walls[side] = atom.Cut[0][side]
			}
		}
		return owners, walls
	})
	if e != nil {
		return Solid{}, nil, nil, e
	}
	var section []int
	for _, id := range order.Components[component].Chambers {
		ch := order.Chambers[id]
		var selected []geom.SplineGraphPolynomialUse
		var spans []clippedStockDirectedSpan
		if ch.Region >= 0 {
			region := material.Overlap.Projection.Arrangement.Regions[ch.Region]
			selected = region.Edges
			if arc := region.Perimeter; arc != nil {
				if arc.ParameterDirection != 1 {
					return Solid{}, nil, nil, geom.ErrConvergence
				}
				spans, e = b.perimeter(0, ch.Region, uv[arc.Start], uv[arc.End])
			}
		} else {
			d := material.Tiles[ch.Tile].Physical
			corner := geom.Vec2{X: d.UMin, Y: d.VMin}
			spans, e = b.perimeter(ch.Tile, -1, corner, corner)
		}
		if e != nil {
			return Solid{}, nil, nil, e
		}
		for layer, owner := range [2]int{1, int(ch.Lower - 1)} {
			var loop []Use
			if len(selected) != 0 {
				var newEdges []int
				loop, newEdges, e = b.graphContourUses(owner, selected, pieces[owner-1])
				if e != nil {
					return Solid{}, nil, nil, e
				}
				section = append(section, newEdges...)
			}
			for _, span := range spans {
				loop = append(loop, b.trace(owner, span))
			}
			b.cap(owner, loop, charts[owner], layer == 1)
		}
	}
	walls, e := b.addWalls(ctx)
	if e != nil {
		return Solid{}, nil, nil, e
	}
	part, e := NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
	if e != nil {
		return Solid{}, nil, nil, e
	}
	top := part.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 {
		return Solid{}, nil, nil, geom.ErrConvergence
	}
	if e := validateExactGraphPartitionEdges(ctx, part, proof); e != nil {
		return Solid{}, nil, nil, e
	}
	if e := validateExactSewnVertexLinks(ctx, part); e != nil {
		return Solid{}, nil, nil, e
	}
	return part, walls, section, nil
}
