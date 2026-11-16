// Package solid implements regularized Boolean operations on unions of
// axis-aligned boxes. It does not implement general B-rep or mesh Booleans.
package solid

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"plainsolid/geom"
	"plainsolid/mesh"
)

var ErrLimit = errors.New("solid: coordinate grid exceeds cell or work budget")

// Options bounds coordinate-grid work. Zero selects one million cells.
type Options struct {
	MaxCells int
	// MaxWork bounds occupancy cell visits across all input boxes; zero selects
	// twenty million. This separately bounds repeated overlapping input work.
	MaxWork int
}

func (o Options) limit() (int, error) {
	if o.MaxCells < 0 || o.MaxWork < 0 {
		return 0, errors.New("solid: negative cell budget")
	}
	if o.MaxCells == 0 {
		return 1000000, nil
	}
	return o.MaxCells, nil
}

// Solid stores nonoverlapping positive-volume cells. Its zero value is empty.
// Methods do not mutate inputs and are safe for concurrent use.
type Solid struct{ cells []geom.Box }

// New normalizes overlapping boxes into disjoint cells. Zero-thickness boxes
// are rejected. An empty argument list constructs the empty solid.
func New(boxes ...geom.Box) (Solid, error) { return NewWithOptions(Options{}, boxes...) }
func NewWithOptions(options Options, boxes ...geom.Box) (Solid, error) {
	for i, b := range boxes {
		if !validBox(b) {
			return Solid{}, fmt.Errorf("solid: invalid box %d", i)
		}
	}
	return combine(boxes, nil, Union, options)
}
func validBox(b geom.Box) bool {
	return b.Min.IsFinite() && b.Max.IsFinite() && b.Min.X < b.Max.X && b.Min.Y < b.Max.Y && b.Min.Z < b.Max.Z && finite(b.Max.X-b.Min.X) && finite(b.Max.Y-b.Min.Y) && finite(b.Max.Z-b.Min.Z)
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Cells returns an independent copy of the solid's disjoint cells.
func (s Solid) Cells() []geom.Box { return append([]geom.Box(nil), s.cells...) }

// CellCount reports the number of disjoint cells without copying them.
func (s Solid) CellCount() int { return len(s.cells) }

func (s Solid) Empty() bool { return len(s.cells) == 0 }
func (s Solid) Volume() float64 {
	sum := 0.
	for _, b := range s.cells {
		sum += (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y) * (b.Max.Z - b.Min.Z)
	}
	return sum
}
func (s Solid) Bounds() (geom.Box, error) {
	if s.Empty() {
		return geom.Box{}, errors.New("solid: empty bounds")
	}
	r := s.cells[0]
	for _, b := range s.cells[1:] {
		r.Min = geom.Vec3{X: math.Min(r.Min.X, b.Min.X), Y: math.Min(r.Min.Y, b.Min.Y), Z: math.Min(r.Min.Z, b.Min.Z)}
		r.Max = geom.Vec3{X: math.Max(r.Max.X, b.Max.X), Y: math.Max(r.Max.Y, b.Max.Y), Z: math.Max(r.Max.Z, b.Max.Z)}
	}
	return r, nil
}

// Contains includes boundary points; nonfinite points are never contained.
func (s Solid) Contains(p geom.Vec3) bool {
	if !p.IsFinite() {
		return false
	}
	for _, b := range s.cells {
		if p.X >= b.Min.X && p.X <= b.Max.X && p.Y >= b.Min.Y && p.Y <= b.Max.Y && p.Z >= b.Min.Z && p.Z <= b.Max.Z {
			return true
		}
	}
	return false
}

// Centroid returns the volume-weighted center, rejecting empty/overflowed data.
func (s Solid) Centroid() (geom.Vec3, error) {
	v := s.Volume()
	if v <= 0 || !finite(v) {
		return geom.Vec3{}, errors.New("solid: undefined centroid")
	}
	r := geom.Vec3{}
	for _, b := range s.cells {
		w := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y) * (b.Max.Z - b.Min.Z) / v
		r = r.Add(b.Min.Scale(.5).Add(b.Max.Scale(.5)).Scale(w))
	}
	if !r.IsFinite() {
		return geom.Vec3{}, errors.New("solid: centroid overflow")
	}
	return r, nil
}

type Operation uint8

const (
	Union Operation = iota
	Intersection
	Difference
)

// Boolean computes a regularized set operation: isolated faces/edges/points
// have no volume and are omitted. Coordinates are not tolerance-snapped.
func (a Solid) Boolean(b Solid, op Operation, options Options) (Solid, error) {
	return combine(a.cells, b.cells, op, options)
}
func (a Solid) Union(b Solid) (Solid, error)        { return a.Boolean(b, Union, Options{}) }
func (a Solid) Intersection(b Solid) (Solid, error) { return a.Boolean(b, Intersection, Options{}) }
func (a Solid) Difference(b Solid) (Solid, error)   { return a.Boolean(b, Difference, Options{}) }

type grid struct {
	x, y, z    []float64
	nx, ny, nz int
}

func coordinates(a, b []geom.Box, options Options) (grid, error) {
	limit, err := options.limit()
	if err != nil {
		return grid{}, err
	}
	g := grid{}
	for _, bs := range [][]geom.Box{a, b} {
		for _, v := range bs {
			g.x = append(g.x, v.Min.X, v.Max.X)
			g.y = append(g.y, v.Min.Y, v.Max.Y)
			g.z = append(g.z, v.Min.Z, v.Max.Z)
		}
	}
	g.x = unique(g.x)
	g.y = unique(g.y)
	g.z = unique(g.z)
	if len(g.x) == 0 {
		return g, nil
	}
	g.nx = len(g.x) - 1
	g.ny = len(g.y) - 1
	g.nz = len(g.z) - 1
	n := 1
	for _, v := range []int{g.nx, g.ny, g.nz} {
		if v == 0 {
			return grid{}, errors.New("solid: degenerate grid")
		}
		if n > limit/v {
			return grid{}, ErrLimit
		}
		n *= v
	}
	workLimit := options.MaxWork
	if workLimit == 0 {
		workLimit = 20000000
	}
	work := 0
	for _, boxes := range [][]geom.Box{a, b} {
		for _, box := range boxes {
			counts := []int{sort.SearchFloat64s(g.x, box.Max.X) - sort.SearchFloat64s(g.x, box.Min.X), sort.SearchFloat64s(g.y, box.Max.Y) - sort.SearchFloat64s(g.y, box.Min.Y), sort.SearchFloat64s(g.z, box.Max.Z) - sort.SearchFloat64s(g.z, box.Min.Z)}
			visits := 1
			for _, count := range counts {
				if count <= 0 || visits > (workLimit-work)/count {
					return grid{}, ErrLimit
				}
				visits *= count
			}
			work += visits
		}
	}
	return g, nil
}
func unique(v []float64) []float64 {
	sort.Float64s(v)
	n := 0
	for _, x := range v {
		if n == 0 || x != v[n-1] {
			v[n] = x
			n++
		}
	}
	return v[:n]
}
func (g grid) index(i, j, k int) int { return (i*g.ny+j)*g.nz + k }
func (g grid) occupancy(bs []geom.Box) []bool {
	v := make([]bool, g.nx*g.ny*g.nz)
	for _, b := range bs {
		ilo := sort.SearchFloat64s(g.x, b.Min.X)
		ihi := sort.SearchFloat64s(g.x, b.Max.X)
		jlo := sort.SearchFloat64s(g.y, b.Min.Y)
		jhi := sort.SearchFloat64s(g.y, b.Max.Y)
		klo := sort.SearchFloat64s(g.z, b.Min.Z)
		khi := sort.SearchFloat64s(g.z, b.Max.Z)
		for i := ilo; i < ihi; i++ {
			for j := jlo; j < jhi; j++ {
				for k := klo; k < khi; k++ {
					v[g.index(i, j, k)] = true
				}
			}
		}
	}
	return v
}
func combine(a, b []geom.Box, op Operation, options Options) (Solid, error) {
	if op > Difference {
		return Solid{}, errors.New("solid: unknown Boolean operation")
	}
	g, err := coordinates(a, b, options)
	if err != nil {
		return Solid{}, err
	}
	av, bv := g.occupancy(a), g.occupancy(b)
	r := Solid{}
	for i := 0; i < g.nx; i++ {
		for j := 0; j < g.ny; j++ {
			for k := 0; k < g.nz; k++ {
				n := g.index(i, j, k)
				keep := false
				switch op {
				case Union:
					keep = av[n] || bv[n]
				case Intersection:
					keep = av[n] && bv[n]
				case Difference:
					keep = av[n] && !bv[n]
				}
				if keep {
					cell := geom.Box{Min: geom.Vec3{X: g.x[i], Y: g.y[j], Z: g.z[k]}, Max: geom.Vec3{X: g.x[i+1], Y: g.y[j+1], Z: g.z[k+1]}}
					volume := (cell.Max.X - cell.Min.X) * (cell.Max.Y - cell.Min.Y) * (cell.Max.Z - cell.Min.Z)
					if !finite(volume) || volume == 0 {
						return Solid{}, errors.New("solid: cell volume outside float64 range")
					}
					r.cells = append(r.cells, cell)
				}
			}
		}
	}
	if !finite(r.Volume()) {
		return Solid{}, errors.New("solid: volume overflow")
	}
	return r, nil
}

// Mesh triangulates exposed grid faces, sharing vertices and removing internal
// faces. Edge/point-touching components can produce nonmanifold boundaries;
// callers can use mesh.ValidateSolid to detect these.
func (s Solid) Mesh(options Options) (mesh.Mesh, error) {
	vertices, quads, err := s.boundaryQuads(options)
	if err != nil {
		return mesh.Mesh{}, err
	}
	m := mesh.Mesh{Vertices: vertices, Triangles: make([][3]int, 0, 2*len(quads))}
	for _, f := range quads {
		m.Triangles = append(m.Triangles, [3]int{f[0], f[1], f[2]}, [3]int{f[0], f[2], f[3]})
	}
	return m, m.Validate()
}

func (s Solid) boundaryQuads(options Options) ([]geom.Vec3, [][4]int, error) {
	g, err := coordinates(s.cells, nil, options)
	if err != nil {
		return nil, nil, err
	}
	occ := g.occupancy(s.cells)
	var points []geom.Vec3
	var quads [][4]int
	vertices := map[[3]int]int{}
	vertex := func(p [3]int) int {
		if v, ok := vertices[p]; ok {
			return v
		}
		v := len(points)
		vertices[p] = v
		points = append(points, geom.Vec3{X: g.x[p[0]], Y: g.y[p[1]], Z: g.z[p[2]]})
		return v
	}
	face := func(a, b, c, d [3]int) {
		ia, ib, ic, id := vertex(a), vertex(b), vertex(c), vertex(d)
		quads = append(quads, [4]int{ia, ib, ic, id})
	}
	has := func(i, j, k int) bool {
		return i >= 0 && j >= 0 && k >= 0 && i < g.nx && j < g.ny && k < g.nz && occ[g.index(i, j, k)]
	}
	for i := 0; i < g.nx; i++ {
		for j := 0; j < g.ny; j++ {
			for k := 0; k < g.nz; k++ {
				if !has(i, j, k) {
					continue
				}
				p000 := [3]int{i, j, k}
				p100 := [3]int{i + 1, j, k}
				p010 := [3]int{i, j + 1, k}
				p110 := [3]int{i + 1, j + 1, k}
				p001 := [3]int{i, j, k + 1}
				p101 := [3]int{i + 1, j, k + 1}
				p011 := [3]int{i, j + 1, k + 1}
				p111 := [3]int{i + 1, j + 1, k + 1}
				if !has(i-1, j, k) {
					face(p000, p001, p011, p010)
				}
				if !has(i+1, j, k) {
					face(p100, p110, p111, p101)
				}
				if !has(i, j-1, k) {
					face(p000, p100, p101, p001)
				}
				if !has(i, j+1, k) {
					face(p010, p011, p111, p110)
				}
				if !has(i, j, k-1) {
					face(p000, p010, p110, p100)
				}
				if !has(i, j, k+1) {
					face(p001, p101, p111, p011)
				}
			}
		}
	}
	return points, quads, nil
}
