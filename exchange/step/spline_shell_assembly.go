package step

import (
	"bytes"
	"errors"
	"io"
)

// SplineShellAssemblyPart retains certified rectangular spline bodies in local
// product coordinates. Repeated occurrences refer to this geometry once.
type SplineShellAssemblyPart struct {
	ID, Name string
	Bodies   []SplineShellBody
}
type SplineShellAssemblyModel struct {
	Products    []SplineShellAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

// SplineShellAssembly combines the explicit product graph with fully decoded
// local bodies keyed by product-definition ID; occurrences are never flattened.
type SplineShellAssembly struct {
	Assembly      Assembly
	ProductBodies map[int64][]SplineShellBody
}

// WriteSplineShellAssembly uses the existing AP214 rigid occurrence graph and
// requires complete local shell embedding certificates. Faces are authoritative;
// callers cannot bypass validation by supplying a preconstructed Solid field.
// Geometry is staged before output and retains local control coordinates.
func WriteSplineShellAssembly(w io.Writer, m SplineShellAssemblyModel) error {
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, len(m.Products))
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		counts[i] = len(p.Bodies)
	}
	return writeAssemblyGeometry(w, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteSplineShells(&b, SplineShellModel{Bodies: m.Products[i].Bodies, LengthUnit: m.LengthUnit}); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	})
}

func (a Assembly) splineShellBodies(definition int64, l Limits) ([]SplineShellBody, int, error) {
	ids, advanced, e := a.productItems(definition)
	if e != nil {
		return nil, 0, e
	}
	if len(ids) > l.MaxSolids {
		return nil, 0, errors.New("step: spline product body limit")
	}
	out := []SplineShellBody{}
	points, faces, uses := 0, 0, 0
	seen := map[int64]bool{}
	for i, id := range ids {
		if !advanced[i] || seen[id] {
			return nil, 0, errors.New("step: duplicate or nonadvanced spline body")
		}
		seen[id] = true
		remaining := l
		remaining.MaxVertices -= points
		remaining.MaxFaces -= faces
		remaining.MaxEdgeUses -= uses
		body, n, e := decodeSplineShell(a.Source, id, remaining)
		if e != nil {
			return nil, 0, e
		}
		points += n
		faces += len(body.Faces)
		uses += 4 * len(body.Faces)
		out = append(out, body)
	}
	return out, points, nil
}

// SplineShellBodies validates one product's exact rectangular spline solids.
// Local coordinates are preserved. Assembly-only products return an empty list.
func (a Assembly) SplineShellBodies(definition int64, l Limits) ([]SplineShellBody, error) {
	l, e := l.defaults()
	if e != nil {
		return nil, e
	}
	b, _, e := a.splineShellBodies(definition, l)
	return b, e
}

// ReadSplineShellAssembly requires all represented bodies to satisfy the
// existing exact spline-shell and global embedding proofs, in addition to the
// rigid AP214/AP242 assembly graph checks. Limits apply across all local assets;
// reuse through occurrences does not duplicate body geometry or its budget.
func ReadSplineShellAssembly(r io.Reader, l Limits) (SplineShellAssembly, error) {
	l, e := l.defaults()
	if e != nil {
		return SplineShellAssembly{}, e
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return SplineShellAssembly{}, e
	}
	out := SplineShellAssembly{Assembly: a, ProductBodies: map[int64][]SplineShellBody{}}
	remaining := l
	seen := map[int64]bool{}
	count := 0
	for _, p := range a.Products {
		ids, _, e := a.productItems(p.DefinitionID)
		if e != nil {
			return SplineShellAssembly{}, e
		}
		for _, id := range ids {
			if seen[id] {
				return SplineShellAssembly{}, errors.New("step: body shared across distinct product definitions")
			}
			seen[id] = true
		}
		bodies, points, e := a.splineShellBodies(p.DefinitionID, remaining)
		if e != nil {
			return SplineShellAssembly{}, e
		}
		remaining.MaxVertices -= points
		remaining.MaxSolids -= len(bodies)
		count += len(bodies)
		for _, b := range bodies {
			remaining.MaxFaces -= len(b.Faces)
			remaining.MaxEdgeUses -= 4 * len(b.Faces)
		}
		out.ProductBodies[p.DefinitionID] = bodies
	}
	reachable := map[int64]bool{}
	todo := []int64{}
	for id := range seen {
		todo = append(todo, id)
	}
	var collect func(Value)
	collect = func(v Value) {
		if v.Kind == Reference {
			todo = append(todo, v.Ref)
		}
		for _, child := range v.Items {
			collect(child)
		}
	}
	for len(todo) > 0 {
		id := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if reachable[id] {
			continue
		}
		reachable[id] = true
		for _, rec := range a.Source.Entities[id].Records {
			for _, v := range rec.Args {
				collect(v)
			}
		}
	}
	for id, ent := range a.Source.Entities {
		for _, rec := range ent.Records {
			if (rec.Type == "ADVANCED_FACE" || rec.Type == "B_SPLINE_SURFACE_WITH_KNOTS") && !reachable[id] {
				return SplineShellAssembly{}, errors.New("step: unowned spline assembly geometry")
			}
			if rec.Type == "MANIFOLD_SOLID_BREP" && !seen[id] {
				return SplineShellAssembly{}, errors.New("step: unowned spline assembly body")
			}
		}
	}
	if count == 0 {
		return SplineShellAssembly{}, errors.New("step: empty spline shell assembly")
	}
	return out, nil
}
