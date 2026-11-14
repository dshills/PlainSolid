package step

import (
	"bytes"
	"context"
	"errors"
	"io"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// PlanarSplineFamily explicitly distinguishes original planar faces from the
// certified rectangular spline-shell material family. Assembly nodes have no body.
type PlanarSplineFamily string

const (
	PlanarSplineNode   PlanarSplineFamily = "assembly"
	PlanarSplinePlanar PlanarSplineFamily = "planar"
	PlanarSplineSpline PlanarSplineFamily = "spline"
)

type PlanarSplinePart struct {
	ID, Name string
	Family   PlanarSplineFamily
	Planar   []Body
	Splines  []SplineShellBody
}
type PlanarSplineAssemblyModel struct {
	Products    []PlanarSplinePart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

// PlanarSplineProduct retains independently decoded original local geometry and
// actual source body IDs. Occurrence reuse never duplicates these local bodies.
type PlanarSplineProduct struct {
	Family        PlanarSplineFamily
	Planar        []Body
	Splines       []SplineShellBody
	SourceBodyIDs []int64
}
type PlanarSplineAssembly struct {
	Assembly Assembly
	Products map[int64]PlanarSplineProduct
}

func planarSplineFailure() error {
	return errors.New("step: unsupported mixed planar/spline material assembly")
}
func planarSplineRigid(m geom.Affine) bool {
	if !m.IsFinite() || m.Determinant() != 1 {
		return false
	}
	var col [3]int
	for i := 0; i < 3; i++ {
		n := 0
		for j := 0; j < 3; j++ {
			if m[i][j] != 0 {
				if m[i][j] != 1 && m[i][j] != -1 {
					return false
				}
				n++
				col[j]++
			}
		}
		if n != 1 {
			return false
		}
	}
	return col == [3]int{1, 1, 1}
}

// WritePlanarSplineAssembly stages distinct advanced planar and certified spline
// products in one explicit AP214 occurrence graph. This subset uses proper
// signed-coordinate rigid poses; local geometry and names remain unflattened.
func WritePlanarSplineAssembly(w io.Writer, m PlanarSplineAssemblyModel) error {
	if w == nil {
		return planarSplineFailure()
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, len(m.Products))
	np, ns := 0, 0
	for _, o := range m.Occurrences {
		if !planarSplineRigid(o.Transform) {
			return planarSplineFailure()
		}
	}
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		switch p.Family {
		case PlanarSplineNode:
			if len(p.Planar)+len(p.Splines) != 0 {
				return planarSplineFailure()
			}
		case PlanarSplinePlanar:
			if len(p.Planar) == 0 || len(p.Splines) != 0 {
				return planarSplineFailure()
			}
			counts[i] = len(p.Planar)
			np++
			for _, b := range p.Planar {
				if e := poly.ValidateEmbeddedBoundary(context.Background(), b.Solid, poly.Options{}); e != nil {
					return e
				}
			}
		case PlanarSplineSpline:
			if len(p.Splines) == 0 || len(p.Planar) != 0 {
				return planarSplineFailure()
			}
			counts[i] = len(p.Splines)
			ns++
		default:
			return planarSplineFailure()
		}
	}
	if np == 0 || ns == 0 {
		return planarSplineFailure()
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		p := m.Products[i]
		var e error
		if p.Family == PlanarSplinePlanar {
			e = WriteAdvanced(&b, Model{Bodies: p.Planar, LengthUnit: m.LengthUnit})
		} else {
			e = WriteSplineShells(&b, SplineShellModel{Bodies: p.Splines, LengthUnit: m.LengthUnit})
		}
		if e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	})
	if e != nil {
		return e
	}
	if _, e = ReadPlanarSplineAssembly(bytes.NewReader(staged.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		return io.ErrShortWrite
	}
	return e
}
func planarSplineBodyFamily(f *File, id int64) (PlanarSplineFamily, error) {
	b, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return "", e
	}
	sid, e := ref(b.Args[1])
	if e != nil {
		return "", e
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return "", e
	}
	faces, e := listRefs(shell.Args[1], 1)
	if e != nil {
		return "", e
	}
	kind := PlanarSplineFamily("")
	for _, fid := range faces {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return "", e
		}
		s, e := ref(face.Args[2])
		if e != nil {
			return "", e
		}
		k := PlanarSplineFamily("")
		for _, r := range f.Entities[s].Records {
			if r.Type == "PLANE" {
				k = PlanarSplinePlanar
			}
			if r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" {
				k = PlanarSplineSpline
			}
		}
		if k == "" || kind != "" && kind != k {
			return "", planarSplineFailure()
		}
		kind = k
	}
	return kind, nil
}
func planarSplineOwnedType(t string) bool {
	switch t {
	case "FACETED_BREP", "BREP_WITH_VOIDS", "FACE", "POLY_LOOP", "CIRCLE", "ELLIPSE", "LINE", "TRIMMED_CURVE", "CYLINDRICAL_SURFACE", "CONICAL_SURFACE", "SPHERICAL_SURFACE", "TOROIDAL_SURFACE", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "SURFACE_CURVE", "PCURVE", "PLANE", "B_SPLINE_SURFACE_WITH_KNOTS", "B_SPLINE_CURVE_WITH_KNOTS":
		return true
	}
	return false
}

// ReadPlanarSplineAssembly classifies each product from its actual full shell
// support graph, then invokes exactly one material decoder. It rejects mixed
// support families inside a body/product, shared geometry across product owners,
// unowned material entities, noncardinal poses, and cumulative resource overflow.
func ReadPlanarSplineAssembly(r io.Reader, l Limits) (PlanarSplineAssembly, error) {
	fail := func() (PlanarSplineAssembly, error) { return PlanarSplineAssembly{}, planarSplineFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return PlanarSplineAssembly{}, e
	}
	for _, o := range a.Occurrences {
		if !planarSplineRigid(o.Transform) {
			return fail()
		}
	}
	out := PlanarSplineAssembly{Assembly: a, Products: map[int64]PlanarSplineProduct{}}
	remaining := l
	owned := map[int64]int64{}
	seen := map[int64]bool{}
	np, ns := 0, 0
	for _, p := range a.Products {
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return fail()
		}
		product := PlanarSplineProduct{Family: PlanarSplineNode, SourceBodyIDs: append([]int64(nil), ids...)}
		for i, id := range ids {
			if seen[id] || !advanced[i] || remaining.MaxSolids <= 0 {
				return fail()
			}
			seen[id] = true
			k, e := planarSplineBodyFamily(a.Source, id)
			if e != nil {
				return fail()
			}
			if product.Family != PlanarSplineNode && product.Family != k {
				return fail()
			}
			product.Family = k
			if k == PlanarSplinePlanar {
				b, e := readBody(a.Source, id, remaining.MaxVertices, remaining.MaxFaces, remaining.MaxEdgeUses, true)
				if e != nil {
					return PlanarSplineAssembly{}, e
				}
				if e = poly.ValidateEmbeddedBoundary(context.Background(), b.Solid, poly.Options{}); e != nil {
					return PlanarSplineAssembly{}, e
				}
				product.Planar = append(product.Planar, b)
				remaining.MaxVertices -= len(b.Solid.Vertices())
				remaining.MaxFaces -= len(b.Solid.Faces())
				for _, f := range b.Solid.Faces() {
					remaining.MaxEdgeUses -= len(f.Boundary)
					for _, h := range f.Holes {
						remaining.MaxEdgeUses -= len(h)
					}
				}
				np++
			} else {
				b, n, e := decodeSplineShell(a.Source, id, remaining)
				if e != nil {
					return PlanarSplineAssembly{}, e
				}
				product.Splines = append(product.Splines, b)
				remaining.MaxVertices -= n
				remaining.MaxFaces -= len(b.Faces)
				remaining.MaxEdgeUses -= 4 * len(b.Faces)
				ns++
			}
			remaining.MaxSolids--
			if remaining.MaxVertices < 0 || remaining.MaxFaces < 0 || remaining.MaxEdgeUses < 0 {
				return fail()
			}
			// Traverse bounded parsed references; record material ownership separately
			// from common unit/representation contexts, which may legitimately be shared.
			todo := []int64{id}
			visited := map[int64]bool{}
			var add func(Value)
			add = func(v Value) {
				if v.Kind == Reference {
					todo = append(todo, v.Ref)
				}
				for _, x := range v.Items {
					add(x)
				}
			}
			for len(todo) > 0 {
				x := todo[len(todo)-1]
				todo = todo[:len(todo)-1]
				if visited[x] {
					continue
				}
				visited[x] = true
				for _, rec := range a.Source.Entities[x].Records {
					if planarSplineOwnedType(rec.Type) {
						if prior := owned[x]; prior != 0 && prior != p.DefinitionID {
							return fail()
						}
						owned[x] = p.DefinitionID
					}
					for _, v := range rec.Args {
						add(v)
					}
				}
			}
		}
		out.Products[p.DefinitionID] = product
	}
	if np == 0 || ns == 0 {
		return fail()
	}
	for id, ent := range a.Source.Entities {
		for _, rec := range ent.Records {
			if planarSplineOwnedType(rec.Type) && owned[id] == 0 {
				return fail()
			}
		}
	}
	return out, nil
}
