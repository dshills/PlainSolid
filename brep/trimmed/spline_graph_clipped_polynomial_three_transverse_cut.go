package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ClippedPolynomialThreeTransverseCut retains the embedded positive shells
// and the complete native contact ledger of a three-crossing clipped Cut.
type ClippedPolynomialThreeTransverseCut struct {
	sources   [3]*geom.BSplineSurface
	cut       *clippedPolynomialCutData
	axis      int
	tolerance float64
}

// Components returns each embedded shell in source chamber order.
func (r *ClippedPolynomialThreeTransverseCut) Components() []Solid {
	if r == nil || r.cut == nil {
		return nil
	}
	return slices.Clone(r.cut.material.components)
}

// Contacts returns all original contacts in increasing free-coordinate order.
// The rational coordinates and derivatives are caller-owned copies.
func (r *ClippedPolynomialThreeTransverseCut) Contacts() []PolynomialMixedCutContact {
	if r == nil || r.cut == nil {
		return nil
	}
	return r.cut.typedContacts()
}

func (d *clippedPolynomialCutData) typedContacts() []PolynomialMixedCutContact {
	out := make([]PolynomialMixedCutContact, len(d.material.contacts))
	for i, c := range d.material.contacts {
		out[i] = PolynomialMixedCutContact{
			TwoPolynomialNodeComponentContact: c.copy(),
			Order:                             d.orders[i],
			DerivativeGap:                     new(big.Rat).Set(d.derivatives[i]),
		}
	}
	return out
}

// ExactComponentVolumes returns the original source integrals in component
// order and their exact physical total.
func (r *ClippedPolynomialThreeTransverseCut) ExactComponentVolumes() ([]*big.Rat, *big.Rat) {
	if r == nil || r.cut == nil {
		return nil, nil
	}
	return r.cut.exactVolumes()
}

func (d *clippedPolynomialCutData) exactVolumes() ([]*big.Rat, *big.Rat) {
	volumes := make([]*big.Rat, len(d.volumes))
	for i, v := range d.volumes {
		volumes[i] = new(big.Rat).Set(v)
	}
	return volumes, new(big.Rat).Set(d.total)
}

// Chambers identifies the positive original overlap regions and exclusive
// stock tiles retained in a component.
func (r *ClippedPolynomialThreeTransverseCut) Chambers(component int) []ClippedPolynomialCutRegion {
	if r == nil || r.cut == nil || component < 0 || component >= len(r.cut.shells) {
		return nil
	}
	return slices.Clone(r.cut.shells[component].Chambers)
}

// Walls returns the original exposed wall and coedge bindings of a shell.
func (r *ClippedPolynomialThreeTransverseCut) Walls(component int) []ClippedPolynomialWallBinding {
	if r == nil || r.cut == nil || component < 0 || component >= len(r.cut.shells) {
		return nil
	}
	return cloneClippedPolynomialWallBindings(r.cut.shells[component].Walls)
}

// CutClippedPolynomialThreeTransverseComponents subtracts B from A on unequal
// axis-aligned original footprints with exactly three transverse crossings
// inside their overlap. Distinct positive components may meet at the indexed
// original contact points. A component with disconnected local vertex links
// at one original contact returns ErrClippedPolynomialCutSelfContact.
func CutClippedPolynomialThreeTransverseComponents(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o PolynomialNodeCutOptions) (*ClippedPolynomialThreeTransverseCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	cut, e := buildClippedPolynomialCut(ctx, sources, axis, tolerance, o, func(proof geom.SplineGraphPolynomialArrangement, axis int) bool {
		return isNativeClippedTransverseArrangement(proof, axis, 3)
	})
	if e != nil {
		return nil, e
	}
	return &ClippedPolynomialThreeTransverseCut{sources: sources, cut: cut, axis: axis, tolerance: tolerance}, nil
}
