package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
	"sort"
)

// CoincidentRectangleOverlapOptions bounds an exact shared-chart arrangement.
// MaxLoops counts both original outer/hole collections (default16,max32).
// MaxCells counts the full Cartesian grid (default4096,max65536). Support and
// each original UV validation retain their independent bounded proof options.
type CoincidentRectangleOverlapOptions struct {
	Injectivity        geom.SurfaceInjectivityOptions
	Coincidence        intersect.SplineCoincidenceOptions
	Trims              trimvalidate.Options
	MaxLoops, MaxCells int
}
type CoincidentUVRectangle struct{ Min, Max [2]*big.Rat }
type CoincidentTrimOwner struct{ Source, Face, Loop, Use, Edge int }
type CoincidentOverlapTile struct {
	A, B           CoincidentUVRectangle
	BoundaryOwners [4][]CoincidentTrimOwner
} // left,right,bottom,top
type CoincidentOverlapSegment struct {
	A, B   [2][2]*big.Rat
	Owners []CoincidentTrimOwner
}
type CoincidentOverlapPoint struct {
	A, B   [2]*big.Rat
	Owners []CoincidentTrimOwner
}

// CoincidentRectangleOverlap is an exact closed-set intersection ledger.
// Tiles have disjoint interiors. Segments/points exclude sets already covered
// by higher-dimensional tiles/segments. UVArea is measured separately in the
// two original charts, not in model-space surface area. Original source IDs
// remain attached; artificial arrangement cuts have no boundary owner.
// Complete also requires whole-support injectivity, so distinct parameter
// cells cannot hide additional model contacts. Shell embedding and Boolean
// material selection remain outside this report.
type CoincidentRectangleOverlap struct {
	Injectivity geom.SurfaceInjectivity
	Faces       [2]int
	Map         intersect.SplineChartMap
	Tiles       []CoincidentOverlapTile
	Segments    []CoincidentOverlapSegment
	Points      []CoincidentOverlapPoint
	UVArea      [2]*big.Rat
	Complete    bool
	Cells       int
}

func (a Solid) IntersectCoincidentRectangularTrims(ctx context.Context, faceA int, b Solid, faceB int, o CoincidentRectangleOverlapOptions) (CoincidentRectangleOverlap, error) {
	fail := func(e error) (CoincidentRectangleOverlap, error) { return CoincidentRectangleOverlap{}, e }
	if ctx == nil || faceA < 0 || faceA >= len(a.faces) || faceB < 0 || faceB >= len(b.faces) {
		return fail(geom.ErrInvalid)
	}
	if o.MaxLoops == 0 {
		o.MaxLoops = 16
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxLoops < 2 || o.MaxLoops > 32 || o.MaxCells < 1 || o.MaxCells > 65536 {
		return fail(geom.ErrInvalid)
	}
	faces := [2]Face{a.faces[faceA], b.faces[faceB]}
	ids := [2]int{faceA, faceB}
	if 2+len(faces[0].Holes)+len(faces[1].Holes) > o.MaxLoops {
		return fail(parametric.ErrBudget)
	}
	var patches [2]*geom.SurfacePatch
	for i, f := range faces {
		loops := append([][]Use{f.Outer}, f.Holes...)
		for _, loop := range loops {
			if len(loop) != 4 {
				return fail(geom.ErrConvergence)
			}
			for _, u := range loop {
				if u.Curve != nil || u.UVStart.X != u.UVEnd.X && u.UVStart.Y != u.UVEnd.Y {
					return fail(geom.ErrConvergence)
				}
			}
		}
		p, e := exactFaceSupportPatch(f, o.Coincidence.Patches)
		if e != nil {
			return fail(e)
		}
		patches[i] = p
		bounds, e := faceUVBounds(f)
		if e != nil {
			return fail(e)
		}
		d := p.Domain()
		if bounds.UMin < d.UMin || bounds.UMax > d.UMax || bounds.VMin < d.VMin || bounds.VMax > d.VMax {
			return fail(geom.ErrDomain)
		}
		proof, e := ValidateFaceTrimLoops(ctx, f, o.Trims)
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	proof, e := intersect.FindExactSplineCoincidence(ctx, patches[0], patches[1], o.Coincidence)
	if e != nil {
		return fail(e)
	}
	if !proof.Found {
		return fail(geom.ErrConvergence)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	injective, e := geom.CertifySurfaceInjectivity(patches[0], o.Injectivity)
	if e != nil {
		return fail(e)
	}
	if !injective.ProvenInjective {
		return fail(geom.ErrConvergence)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	toB := splineCoincidenceUVMatrix(proof.Map)
	toA := splineCoincidenceUVMatrix(proof.Map.Inverse())
	type boundary struct {
		ends  [2][2]*big.Rat
		owner CoincidentTrimOwner
	}
	var rectangles [2][]CoincidentUVRectangle
	var boundaries []boundary
	var axes [2][]*big.Rat
	for source, f := range faces {
		for li, loop := range append([][]Use{f.Outer}, f.Holes...) {
			var r CoincidentUVRectangle
			for ui, u := range loop {
				endpoints := [2][2]*big.Rat{{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}, {ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}}
				if source == 1 {
					for k := range endpoints {
						endpoints[k] = coincidentMapPoint(toA, endpoints[k])
					}
				}
				boundaries = append(boundaries, boundary{endpoints, CoincidentTrimOwner{source, ids[source], li, ui, u.Edge}})
				for _, p := range endpoints {
					for k := range p {
						if r.Min[k] == nil || p[k].Cmp(r.Min[k]) < 0 {
							r.Min[k] = new(big.Rat).Set(p[k])
						}
						if r.Max[k] == nil || p[k].Cmp(r.Max[k]) > 0 {
							r.Max[k] = new(big.Rat).Set(p[k])
						}
					}
				}
			}
			for k := 0; k < 2; k++ {
				if r.Min[k].Cmp(r.Max[k]) >= 0 {
					return fail(geom.ErrDegenerate)
				}
				axes[k] = append(axes[k], r.Min[k], r.Max[k])
			}
			rectangles[source] = append(rectangles[source], r)
		}
	}
	for k := range axes {
		sort.Slice(axes[k], func(i, j int) bool { return axes[k][i].Cmp(axes[k][j]) < 0 })
		out := axes[k][:0]
		for _, x := range axes[k] {
			if len(out) == 0 || out[len(out)-1].Cmp(x) != 0 {
				out = append(out, x)
			}
		}
		axes[k] = out
	}
	nx, ny := len(axes[0]), len(axes[1])
	if (nx - 1) > o.MaxCells/(ny-1) {
		return fail(parametric.ErrBudget)
	}
	r := CoincidentRectangleOverlap{Injectivity: injective, Faces: ids, Map: proof.Map, UVArea: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Cells: (nx - 1) * (ny - 1)}
	inside := func(p [2]*big.Rat) bool {
		for _, rr := range rectangles {
			if !coincidentRectangleContains(rr[0], p, false) {
				return false
			}
			for _, h := range rr[1:] {
				if coincidentRectangleContains(h, p, true) {
					return false
				}
			}
		}
		return true
	}
	owners := func(p [2]*big.Rat) []CoincidentTrimOwner {
		var found []CoincidentTrimOwner
		for _, s := range boundaries {
			on := true
			for k := range p {
				lo, hi := s.ends[0][k], s.ends[1][k]
				if lo.Cmp(hi) > 0 {
					lo, hi = hi, lo
				}
				on = on && p[k].Cmp(lo) >= 0 && p[k].Cmp(hi) <= 0
			}
			if on {
				found = append(found, s.owner)
			}
		}
		return found
	}
	midpoint := func(x, y *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Add(x, y), big.NewRat(2, 1)) }
	retained := make([]bool, (nx-1)*(ny-1))
	at := func(i, j int) bool { return i >= 0 && j >= 0 && i < nx-1 && j < ny-1 && retained[i*(ny-1)+j] }
	for i := 0; i < nx-1; i++ {
		for j := 0; j < ny-1; j++ {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			x, y := midpoint(axes[0][i], axes[0][i+1]), midpoint(axes[1][j], axes[1][j+1])
			if !inside([2]*big.Rat{x, y}) {
				continue
			}
			retained[i*(ny-1)+j] = true
			tile := CoincidentOverlapTile{A: coincidentCopyRectangle(CoincidentUVRectangle{[2]*big.Rat{axes[0][i], axes[1][j]}, [2]*big.Rat{axes[0][i+1], axes[1][j+1]}})}
			tile.B = coincidentMappedRectangle(toB, tile.A)
			for k, p := range [4][2]*big.Rat{{axes[0][i], y}, {axes[0][i+1], y}, {x, axes[1][j]}, {x, axes[1][j+1]}} {
				tile.BoundaryOwners[k] = owners(p)
			}
			r.Tiles = append(r.Tiles, tile)
			r.UVArea[0].Add(r.UVArea[0], coincidentRectangleArea(tile.A))
			r.UVArea[1].Add(r.UVArea[1], coincidentRectangleArea(tile.B))
		}
	}
	covered := make([]bool, nx*ny)
	addSegment := func(i, j, k, l int) {
		p, q := [2]*big.Rat{axes[0][i], axes[1][j]}, [2]*big.Rat{axes[0][k], axes[1][l]}
		mid := [2]*big.Rat{midpoint(p[0], q[0]), midpoint(p[1], q[1])}
		if !inside(mid) {
			return
		}
		a := [2][2]*big.Rat{coincidentCopyPoint(p), coincidentCopyPoint(q)}
		r.Segments = append(r.Segments, CoincidentOverlapSegment{a, [2][2]*big.Rat{coincidentMapPoint(toB, p), coincidentMapPoint(toB, q)}, owners(mid)})
		covered[i*ny+j] = true
		covered[k*ny+l] = true
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if i+1 < nx && !at(i, j) && !at(i, j-1) {
				addSegment(i, j, i+1, j)
			}
			if j+1 < ny && !at(i, j) && !at(i-1, j) {
				addSegment(i, j, i, j+1)
			}
		}
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if covered[i*ny+j] || at(i, j) || at(i-1, j) || at(i, j-1) || at(i-1, j-1) {
				continue
			}
			p := [2]*big.Rat{axes[0][i], axes[1][j]}
			if inside(p) {
				r.Points = append(r.Points, CoincidentOverlapPoint{coincidentCopyPoint(p), coincidentMapPoint(toB, p), owners(p)})
			}
		}
	}
	r.Complete = true
	return r, nil
}
func coincidentMapPoint(m [3][4]*big.Rat, p [2]*big.Rat) (r [2]*big.Rat) {
	for i := range r {
		r[i] = new(big.Rat).Add(m[i][3], new(big.Rat).Add(new(big.Rat).Mul(m[i][0], p[0]), new(big.Rat).Mul(m[i][1], p[1])))
	}
	return
}
func coincidentCopyPoint(p [2]*big.Rat) [2]*big.Rat {
	return [2]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1])}
}
func coincidentCopyRectangle(r CoincidentUVRectangle) CoincidentUVRectangle {
	return CoincidentUVRectangle{coincidentCopyPoint(r.Min), coincidentCopyPoint(r.Max)}
}
func coincidentMappedRectangle(m [3][4]*big.Rat, r CoincidentUVRectangle) CoincidentUVRectangle {
	a, b := coincidentMapPoint(m, r.Min), coincidentMapPoint(m, r.Max)
	for k := range a {
		if a[k].Cmp(b[k]) > 0 {
			a[k], b[k] = b[k], a[k]
		}
	}
	return CoincidentUVRectangle{a, b}
}
func coincidentRectangleContains(r CoincidentUVRectangle, p [2]*big.Rat, strict bool) bool {
	for k := range p {
		a, b := p[k].Cmp(r.Min[k]), p[k].Cmp(r.Max[k])
		if a < 0 || b > 0 || strict && (a == 0 || b == 0) {
			return false
		}
	}
	return true
}
func coincidentRectangleArea(r CoincidentUVRectangle) *big.Rat {
	return new(big.Rat).Mul(new(big.Rat).Sub(r.Max[0], r.Min[0]), new(big.Rat).Sub(r.Max[1], r.Min[1]))
}
