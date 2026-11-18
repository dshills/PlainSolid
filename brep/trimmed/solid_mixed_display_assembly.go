//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MixedDisplayAssemblyOptions caps the complete collection before aggregation.
// Defaults: 16 occurrences, 100000 vertices/triangles, 2016 asset pairs, 16384 bits.
// Validation independently bounds the final exact global contact/nesting proof,
// retaining its own default 4096-triangle limit unless explicitly configured.
// This constructor performs no source remeshing or Boolean operation.
type MixedDisplayAssemblyOptions struct {
	MaxAssets, MaxVertices, MaxTriangles, MaxAssetPairs, MaxCoefficientBits int
	Validation                                                              collision.LocationOptions
}
type MixedDisplayTriangleOwner struct{ Asset, Face int }

// MixedDisplaySeparation proves positive source-image envelope separation:
// DisplayGap² > RequiredGapSquared = 2*(squaredDeviationA+squaredDeviationB).
// This sufficient axis-aligned condition may reject separated sources.
type MixedDisplaySeparation struct {
	AssetA, AssetB, Axis           int
	DisplayGap, RequiredGapSquared *big.Rat
}

// MixedDisplayAssembly is an immutable indexed display of a disjoint collection
// of exact source occurrences. Native identity is (asset slot, original ID).
// Distinct slots may reuse one prepared source. No vertices are welded, no
// nesting between slots is accepted, and this is not a Boolean/native union.
// The source-image proximity proof does not establish ideal material volume.
type MixedDisplayAssembly struct {
	assets      []*PlacedStoredMixedDisplay
	display     *placement.PreparedClosedDisplay
	owners      []MixedDisplayTriangleOwner
	offsets     []int
	squared     *big.Rat
	separations []MixedDisplaySeparation
}

func (a *MixedDisplayAssembly) Mesh() mesh.Mesh {
	if a == nil {
		return mesh.Mesh{}
	}
	return a.display.Mesh()
}
func (a *MixedDisplayAssembly) Shells() []collision.ShellInfo {
	if a == nil {
		return nil
	}
	return a.display.Shells()
}
func (a *MixedDisplayAssembly) TriangleOwners() []MixedDisplayTriangleOwner {
	if a == nil {
		return nil
	}
	return append([]MixedDisplayTriangleOwner(nil), a.owners...)
}
func (a *MixedDisplayAssembly) SquaredDistance() *big.Rat {
	if a == nil || a.squared == nil {
		return nil
	}
	return new(big.Rat).Set(a.squared)
}
func (a *MixedDisplayAssembly) Asset(i int) (*PlacedStoredMixedDisplay, bool) {
	if a == nil || i < 0 || i >= len(a.assets) {
		return nil, false
	}
	return a.assets[i], true
}
func (a *MixedDisplayAssembly) Separations() []MixedDisplaySeparation {
	if a == nil {
		return nil
	}
	out := append([]MixedDisplaySeparation(nil), a.separations...)
	for i := range out {
		out[i].DisplayGap = new(big.Rat).Set(out[i].DisplayGap)
		out[i].RequiredGapSquared = new(big.Rat).Set(out[i].RequiredGapSquared)
	}
	return out
}

// Ancestry returns original edge parameters with GLOBAL assembly vertex IDs.
func (a *MixedDisplayAssembly) Ancestry(i int) (StoredDisplayAncestry, bool) {
	asset, ok := a.Asset(i)
	if !ok {
		return StoredDisplayAncestry{}, false
	}
	out := asset.Ancestry()
	offset := a.offsets[i]
	for _, rows := range [][][]int{out.FaceVertices, out.EdgeVertices} {
		for _, row := range rows {
			for j := range row {
				row[j] += offset
			}
		}
	}
	for k, v := range out.CornerVertices {
		out.CornerVertices[k] = v + offset
	}
	return out, true
}

func AssemblePlacedMixedDisplays(ctx context.Context, assets []*PlacedStoredMixedDisplay, o MixedDisplayAssemblyOptions) (*MixedDisplayAssembly, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxAssets == 0 {
		o.MaxAssets = 16
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 100000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = collision.MaxTriangles
	}
	if o.MaxAssetPairs == 0 {
		o.MaxAssetPairs = 2016
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxAssets < 1 || o.MaxAssets > 64 || o.MaxVertices < 1 || o.MaxVertices > 100000 || o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.MaxAssetPairs < 1 || o.MaxAssetPairs > 2016 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 262144 {
		return nil, geom.ErrInvalid
	}
	if len(assets) == 0 {
		return nil, geom.ErrInvalid
	}
	if len(assets) > o.MaxAssets || len(assets)*(len(assets)-1)/2 > o.MaxAssetPairs {
		return nil, geom.ErrConvergence
	}
	check := func(q *big.Rat) bool {
		return q != nil && q.Num().BitLen() <= o.MaxCoefficientBits && q.Denom().BitLen() <= o.MaxCoefficientBits
	}
	out := &MixedDisplayAssembly{assets: append([]*PlacedStoredMixedDisplay(nil), assets...), squared: new(big.Rat)}
	var combined mesh.Mesh
	boxes := make([][2][3]*big.Rat, len(assets))
	for i, asset := range assets {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if asset == nil || asset.display == nil || asset.source == nil || !check(asset.squared) || asset.squared.Sign() < 0 {
			return nil, geom.ErrInvalid
		}
		m := asset.display.Mesh()
		if len(m.Vertices) == 0 || len(m.Triangles) == 0 {
			return nil, geom.ErrInvalid
		}
		if len(m.Vertices) > o.MaxVertices-len(combined.Vertices) || len(m.Triangles) > o.MaxTriangles-len(combined.Triangles) {
			return nil, geom.ErrConvergence
		}
		if asset.squared.Cmp(out.squared) > 0 {
			out.squared.Set(asset.squared)
		}
		for _, v := range m.Vertices {
			for axis, f := range []float64{v.X, v.Y, v.Z} {
				q := new(big.Rat).SetFloat64(f)
				if !check(q) {
					return nil, geom.ErrConvergence
				}
				if boxes[i][0][axis] == nil || q.Cmp(boxes[i][0][axis]) < 0 {
					boxes[i][0][axis] = q
				}
				if boxes[i][1][axis] == nil || q.Cmp(boxes[i][1][axis]) > 0 {
					boxes[i][1][axis] = q
				}
			}
		}
		offset := len(combined.Vertices)
		out.offsets = append(out.offsets, offset)
		combined.Vertices = append(combined.Vertices, m.Vertices...)
		faces := asset.display.Faces()
		if len(faces) != len(m.Triangles) {
			return nil, geom.ErrInvalid
		}
		for j, tr := range m.Triangles {
			combined.Triangles = append(combined.Triangles, [3]int{tr[0] + offset, tr[1] + offset, tr[2] + offset})
			out.owners = append(out.owners, MixedDisplayTriangleOwner{i, faces[j]})
		}
	}
	for i := range assets {
		for j := i + 1; j < len(assets); j++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			required := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(assets[i].squared, assets[j].squared))
			if !check(required) {
				return nil, geom.ErrConvergence
			}
			proved := false
			for axis := 0; axis < 3; axis++ {
				gap := new(big.Rat).Sub(boxes[j][0][axis], boxes[i][1][axis])
				if gap.Sign() <= 0 {
					gap.Sub(boxes[i][0][axis], boxes[j][1][axis])
				}
				if gap.Sign() <= 0 {
					continue
				}
				squared := new(big.Rat).Mul(gap, gap)
				if !check(gap) || !check(squared) {
					return nil, geom.ErrConvergence
				}
				if squared.Cmp(required) > 0 {
					out.separations = append(out.separations, MixedDisplaySeparation{i, j, axis, gap, required})
					proved = true
					break
				}
			}
			if !proved {
				return nil, geom.ErrConvergence
			}
		}
	}
	display, e := placement.PrepareClosedDisplay(ctx, combined, nil, placement.ClosedDisplayOptions{Rounding: placement.MeshRoundingOptions{MaxVertices: o.MaxVertices, MaxTriangles: o.MaxTriangles}, Validation: o.Validation})
	if e != nil {
		return nil, e
	}
	shells := display.Shells()
	if len(shells) != len(assets) {
		return nil, geom.ErrDomain
	}
	seen := make([]bool, len(assets))
	for _, shell := range shells {
		if shell.Parent != -1 || shell.Depth != 0 || len(shell.Triangles) == 0 {
			return nil, geom.ErrDomain
		}
		owner := out.owners[shell.Triangles[0]].Asset
		if seen[owner] {
			return nil, geom.ErrDomain
		}
		seen[owner] = true
		for _, tri := range shell.Triangles {
			if out.owners[tri].Asset != owner {
				return nil, geom.ErrDomain
			}
		}
	}
	out.display = display
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
