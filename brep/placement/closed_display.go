package placement

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MeshRoundingOptions caps work before copying/transforming any mesh. Defaults
// are 100000 vertices/triangles and 4096 coefficient bits. Each exact point
// transform is bounded independently; the total is at most MaxVertices.
// Exact rounding-error accumulators are capped at 3*MaxCoefficientBits+4096 bits.
type MeshRoundingOptions struct{ MaxVertices, MaxTriangles, MaxCoefficientBits int }

func meshRoundingLimits(o MeshRoundingOptions) (MeshRoundingOptions, error) {
	if o.MaxVertices == 0 {
		o.MaxVertices = 100000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = collision.MaxTriangles
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxVertices < 1 || o.MaxVertices > 100000 || o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return o, geom.ErrInvalid
	}
	return o, nil
}
func cloneDisplayMesh(m mesh.Mesh) mesh.Mesh {
	return mesh.Mesh{Vertices: append([]geom.Vec3(nil), m.Vertices...), Triangles: append([][3]int(nil), m.Triangles...)}
}

// RoundedMeshImage owns its output. RoundingUpper bounds every transformed
// vertex's Euclidean error using its exact L1 error. Barycentric interpolation
// proves the same two-sided triangle-surface distance bound. This primitive
// proves neither source approximation nor closedness/embedding/nesting.
type RoundedMeshImage struct {
	Mesh          mesh.Mesh
	RoundingUpper *big.Rat
}

func (p RationalRigid) RoundMesh(ctx context.Context, m mesh.Mesh, o MeshRoundingOptions) (RoundedMeshImage, error) {
	var zero RoundedMeshImage
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	o, e := meshRoundingLimits(o)
	if e != nil {
		return zero, e
	}
	if len(m.Vertices) > o.MaxVertices || len(m.Triangles) > o.MaxTriangles {
		return zero, geom.ErrConvergence
	}
	if _, e := p.Matrix(); e != nil {
		return zero, e
	}
	if e := m.Validate(); e != nil {
		return zero, e
	}
	out := RoundedMeshImage{Mesh: cloneDisplayMesh(m), RoundingUpper: new(big.Rat)}
	for i, v := range m.Vertices {
		q, e := p.TransformPoint(ctx, [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}, false, o.MaxCoefficientBits)
		if e != nil {
			return zero, e
		}
		var f [3]float64
		for j := 0; j < 3; j++ {
			f[j], _ = q[j].Float64()
		}
		rounded := geom.Vec3{X: f[0], Y: f[1], Z: f[2]}
		if !rounded.IsFinite() {
			return zero, geom.ErrConvergence
		}
		delta := new(big.Rat)
		for j := 0; j < 3; j++ {
			d := new(big.Rat).Sub(q[j], new(big.Rat).SetFloat64(f[j]))
			delta.Add(delta, d.Abs(d))
		}
		if delta.Num().BitLen() > 3*o.MaxCoefficientBits+4096 || delta.Denom().BitLen() > 3*o.MaxCoefficientBits+4096 {
			return zero, geom.ErrConvergence
		}
		if delta.Cmp(out.RoundingUpper) > 0 {
			out.RoundingUpper.Set(delta)
		}
		out.Mesh.Vertices[i] = rounded
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

// ClosedDisplayOptions bounds each preparation or placement separately.
// Validation performs fresh exact represented topology/contact/nesting proofs;
// its ray, work and collision quotas are not approximation tolerances.
type ClosedDisplayOptions struct {
	Rounding   MeshRoundingOptions
	Validation collision.LocationOptions
}

// PreparedClosedDisplay is an immutable owned represented manifold snapshot.
// It authenticates no analytic source, trims, or source approximation error.
type PreparedClosedDisplay struct {
	display mesh.Mesh
	faces   []int
	locator *collision.PointLocator
}

func PrepareClosedDisplay(ctx context.Context, m mesh.Mesh, faces []int, o ClosedDisplayOptions) (*PreparedClosedDisplay, error) {
	limits, e := meshRoundingLimits(o.Rounding)
	if e != nil {
		return nil, e
	}
	if len(m.Vertices) > limits.MaxVertices || len(m.Triangles) > limits.MaxTriangles {
		return nil, geom.ErrConvergence
	}
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if faces != nil && len(faces) != len(m.Triangles) {
		return nil, geom.ErrInvalid
	}
	if e := m.Validate(); e != nil {
		return nil, e
	}
	owned := cloneDisplayMesh(m)
	ids := append([]int(nil), faces...)
	locator, e := collision.NewMeshPointLocator(ctx, owned, ids, o.Validation)
	if e != nil {
		return nil, e
	}
	return &PreparedClosedDisplay{owned, ids, locator}, nil
}
func (p *PreparedClosedDisplay) Mesh() mesh.Mesh {
	if p == nil {
		return mesh.Mesh{}
	}
	return cloneDisplayMesh(p.display)
}
func (p *PreparedClosedDisplay) Faces() []int {
	if p == nil {
		return nil
	}
	return append([]int(nil), p.faces...)
}
func (p *PreparedClosedDisplay) Shells() []collision.ShellInfo {
	if p == nil || p.locator == nil {
		return nil
	}
	return p.locator.Shells()
}

// PlacedClosedDisplay preserves original indices and exact component nesting,
// despite represented-coordinate rounding. Its private snapshot is immutable.
// Both endpoint meshes are embedded; no ambient-isotopy or interpolation-path
// embedding claim is made between them.
type PlacedClosedDisplay struct {
	source   *PreparedClosedDisplay
	world    *PreparedClosedDisplay
	rounding *big.Rat
	pose     RationalRigid
}

func (p *PlacedClosedDisplay) Mesh() mesh.Mesh {
	if p == nil {
		return mesh.Mesh{}
	}
	return p.world.Mesh()
}
func (p *PlacedClosedDisplay) Faces() []int {
	if p == nil {
		return nil
	}
	return p.world.Faces()
}
func (p *PlacedClosedDisplay) Shells() []collision.ShellInfo {
	if p == nil {
		return nil
	}
	return p.world.Shells()
}
func (p *PlacedClosedDisplay) RoundingUpper() *big.Rat {
	if p == nil || p.rounding == nil {
		return nil
	}
	return new(big.Rat).Set(p.rounding)
}
func (p *PlacedClosedDisplay) Pose() RationalRigid {
	if p == nil {
		return RationalRigid{}
	}
	return p.pose
}

// InheritedDistanceUpper is conditional: IF sourceUpper is an independently
// proved two-sided distance from the original analytic boundary to the prepared
// mesh, this sum bounds distance from the exact rigid source occurrence to the
// placed mesh. This method does not establish that prerequisite or certify
// material-volume error. It accepts a nonnegative rational <=65536 bits.
func (p *PlacedClosedDisplay) InheritedDistanceUpper(sourceUpper *big.Rat) (*big.Rat, error) {
	if p == nil || p.rounding == nil || sourceUpper == nil || sourceUpper.Sign() < 0 {
		return nil, geom.ErrInvalid
	}
	if e := checkRigidRat(sourceUpper, 65536); e != nil {
		return nil, e
	}
	out := new(big.Rat).Add(sourceUpper, p.rounding)
	if e := checkRigidRat(out, 270336); e != nil {
		return nil, e
	}
	return out, nil
}
func (p *PreparedClosedDisplay) Place(ctx context.Context, pose RationalRigid, o ClosedDisplayOptions) (*PlacedClosedDisplay, error) {
	if p == nil || p.locator == nil {
		return nil, geom.ErrInvalid
	}
	image, e := pose.RoundMesh(ctx, p.display, o.Rounding)
	if e != nil {
		return nil, e
	}
	world, e := PrepareClosedDisplay(ctx, image.Mesh, p.faces, o)
	if e != nil {
		return nil, e
	}
	before, after := p.locator.Shells(), world.locator.Shells()
	if len(before) != len(after) {
		return nil, geom.ErrDomain
	}
	for i, a := range before {
		b := after[i]
		if a.Parent != b.Parent || a.Depth != b.Depth || len(a.Triangles) != len(b.Triangles) {
			return nil, geom.ErrDomain
		}
		for j, id := range a.Triangles {
			if id != b.Triangles[j] {
				return nil, geom.ErrDomain
			}
		}
	}
	return &PlacedClosedDisplay{source: p, world: world, rounding: image.RoundingUpper, pose: pose}, nil
}
