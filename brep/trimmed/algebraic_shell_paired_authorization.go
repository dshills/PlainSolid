package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AffineShellImprintEdge is a complete finite original face-pair section.
// Endpoints index Vertices; it is not an approximate display segment.
type AffineShellImprintEdge struct {
	Faces    [2]int
	Vertices [2]int
	UV       [2][2][2]*big.Rat // owner, endpoint, original chart coordinate
}
type AffineShellPairedImprintNode struct {
	Vertex        int
	Owners        [2]AlgebraicShellBoundaryOwner
	IncidentPairs [4][2]int
	IncidentEdges []int
}
type AffineShellPairedImprintOptions struct {
	Embedding                          SplineRectangleEmbeddingOptions
	Patches                            geom.SurfacePatchOptions
	Contacts                           geom.AffineChartIntersectionOptions
	Endpoints                          geom.TubeAlgebraicGraphEndpointOptions
	MaxFacePairs, MaxVertices, MaxWork int
}

// AffineShellPairedImprint authorizes exact source-edge splitting on a complete
// section graph with explicitly retained two- or four-germ paired nodes. Every original face pair was intersected exactly;
// each simultaneous node rebinds both original edges and all four incident face
// pairs. It is not a material Boolean or an already split native shell.
type AffineShellPairedImprint struct {
	sources     [2]Solid
	charts      [2][]splineEmbeddingFace
	vertices    [][3]*big.Rat
	edges       []AffineShellImprintEdge
	paired      []AffineShellPairedImprintNode
	pairs, work int
}

func (r *AffineShellPairedImprint) OriginalOwner(i int) (Solid, error) {
	if r == nil || i < 0 || i > 1 {
		return Solid{}, geom.ErrInvalid
	}
	return r.sources[i], nil
}
func (r *AffineShellPairedImprint) Vertices() [][3]*big.Rat {
	if r == nil {
		return nil
	}
	out := make([][3]*big.Rat, len(r.vertices))
	for i, p := range r.vertices {
		for j, q := range p {
			out[i][j] = new(big.Rat).Set(q)
		}
	}
	return out
}
func (r *AffineShellPairedImprint) Edges() []AffineShellImprintEdge {
	if r == nil {
		return nil
	}
	out := append([]AffineShellImprintEdge(nil), r.edges...)
	for i := range out {
		for owner := 0; owner < 2; owner++ {
			for end := 0; end < 2; end++ {
				for axis := 0; axis < 2; axis++ {
					out[i].UV[owner][end][axis] = new(big.Rat).Set(out[i].UV[owner][end][axis])
				}
			}
		}
	}
	return out
}
func (r *AffineShellPairedImprint) PairedNodes() []AffineShellPairedImprintNode {
	if r == nil {
		return nil
	}
	out := append([]AffineShellPairedImprintNode(nil), r.paired...)
	for i := range out {
		out[i].IncidentEdges = append([]int(nil), out[i].IncidentEdges...)
	}
	return out
}
func (r *AffineShellPairedImprint) CompleteFacePairs() int {
	if r == nil {
		return 0
	}
	return r.pairs
}
func (r *AffineShellPairedImprint) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}

// AuthorizeAffineSplineShellPairedImprint requires actual globally affine
// original spline patches, independent shell embeddings, no coplanar contacts,
// no section segment carried by a source edge, no original vertex contacts and
// degree two at ordinary nodes and degree two or four at paired nodes. It rejects unsupported contacts rather
// than dropping them. Contact and endpoint arithmetic share MaxWork; support
// extraction and embedding retain their separately bounded options.
func AuthorizeAffineSplineShellPairedImprint(ctx context.Context, a, b Solid, o AffineShellPairedImprintOptions) (*AffineShellPairedImprint, error) {
	if ctx == nil || o.MaxFacePairs < 0 || o.MaxFacePairs > 16384 || o.MaxVertices < 0 || o.MaxVertices > 4096 || o.MaxWork < 0 || o.MaxWork > 1000000000 {
		return nil, geom.ErrInvalid
	}
	if o.Contacts.MaxOperations < 0 || o.Contacts.MaxOperations > 128000000 || o.Contacts.MaxCoefficientBits < 0 || o.Contacts.MaxCoefficientBits > 65536 || (o.Contacts.MaxCoefficientBits > 0 && o.Contacts.MaxCoefficientBits < 64) {
		return nil, geom.ErrInvalid
	}
	if o.MaxFacePairs == 0 {
		o.MaxFacePairs = 256
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 256000000
	}
	endOptions := o.Endpoints
	endOptions.MaxOperations = o.MaxWork
	budget, e := newPairedTerminalBudget(endOptions)
	if e != nil {
		return nil, e
	}
	var charts [2][]splineEmbeddingFace
	solids := [2]Solid{a, b}
	for i, s := range solids {
		if _, e = s.validateSplineRectangleEmbedding(ctx, o.Embedding, &charts[i]); e != nil {
			return nil, e
		}
	}
	if len(charts[0]) == 0 || len(charts[1]) == 0 || len(charts[0]) > o.MaxFacePairs/len(charts[1]) {
		return nil, geom.ErrConvergence
	}
	out := &AffineShellPairedImprint{sources: solids, charts: charts}
	type occurrence struct {
		faces [2]int
		uv    [2][2]*big.Rat
		masks [2]uint8
	}
	occurrences := [][]occurrence{}
	degree := []int{}
	ids := map[[3]string]int{}
	for i, p := range charts[0] {
		for j, q := range charts[1] {
			x, e := algebraicShellTracePatch(a.faces[i], p.patch.Domain())
			if e != nil {
				return nil, e
			}
			y, e := algebraicShellTracePatch(b.faces[j], q.patch.Domain())
			if e != nil {
				return nil, e
			}
			opts := o.Contacts
			if opts.MaxOperations == 0 {
				opts.MaxOperations = 8000000
			}
			if opts.MaxOperations > budget.remaining {
				opts.MaxOperations = budget.remaining
			}
			if opts.MaxOperations < 1 {
				return nil, geom.ErrConvergence
			}
			contact, e := intersect.IntersectAffinePatches(ctx, x, y, o.Patches, opts)
			if e != nil {
				return nil, fmt.Errorf("trimmed: complete affine pair [%d,%d]: %w", i, j, e)
			}
			if e = budget.debit(contact.ExactOperations()); e != nil {
				return nil, e
			}
			out.pairs++
			kind, e := contact.Kind()
			if e != nil {
				return nil, e
			}
			if kind == intersect.AffinePatchContactEmpty {
				continue
			}
			if kind == intersect.AffinePatchContactArea {
				return nil, fmt.Errorf("affine pair [%d,%d] area: %w", i, j, geom.ErrDomain)
			}
			if _, _, _, err := contact.CoplanarArea(); err == nil {
				return nil, fmt.Errorf("affine pair [%d,%d] coplanar: %w", i, j, geom.ErrDomain)
			}
			vs, _ := contact.Vertices()
			mapped := []int{}
			for _, v := range vs {
				for _, mask := range []uint8{v.BoundaryA, v.BoundaryB} {
					if mask != 0 && mask&(mask-1) != 0 {
						return nil, fmt.Errorf("affine pair [%d,%d] corner: %w", i, j, geom.ErrDomain)
					}
				}
				key := [3]string{v.Point[0].RatString(), v.Point[1].RatString(), v.Point[2].RatString()}
				id, found := ids[key]
				if !found {
					if len(ids) >= o.MaxVertices {
						return nil, geom.ErrConvergence
					}
					id = len(ids)
					ids[key] = id
					out.vertices = append(out.vertices, v.Point)
					degree = append(degree, 0)
					occurrences = append(occurrences, nil)
				}
				occurrences[id] = append(occurrences[id], occurrence{[2]int{i, j}, [2][2]*big.Rat{v.A, v.B}, [2]uint8{v.BoundaryA, v.BoundaryB}})
				mapped = append(mapped, id)
			}
			es, _ := contact.Edges()
			for _, edge := range es {
				if edge.Boundary[0] != 0 || edge.Boundary[1] != 0 {
					return nil, fmt.Errorf("affine pair [%d,%d] carried edge: %w", i, j, geom.ErrDomain)
				}
				u, v := mapped[edge.Start], mapped[edge.End]
				if u == v {
					return nil, geom.ErrInvalid
				}
				degree[u]++
				degree[v]++
				out.edges = append(out.edges, AffineShellImprintEdge{Faces: [2]int{i, j}, Vertices: [2]int{u, v}, UV: [2][2][2]*big.Rat{{vs[edge.Start].A, vs[edge.End].A}, {vs[edge.Start].B, vs[edge.End].B}}})
			}
		}
	}
	for id, n := range degree {
		if n != 2 && n != 4 {
			return nil, fmt.Errorf("affine node %d degree %d: %w", id, n, geom.ErrDomain)
		}
		var paired *occurrence
		for i := range occurrences[id] {
			c := &occurrences[id][i]
			if c.masks[0] != 0 && c.masks[1] != 0 {
				if paired == nil {
					paired = c
				}
			}
		}
		if paired == nil {
			if n != 2 {
				return nil, geom.ErrDomain
			}
			continue
		}
		row := AffineShellPairedImprintNode{Vertex: id}
		for i, edge := range out.edges {
			if edge.Vertices[0] == id || edge.Vertices[1] == id {
				row.IncidentEdges = append(row.IncidentEdges, i)
			}
		}
		var canonical *geom.ExactComposedCurveEvent
		for owner := 0; owner < 2; owner++ {
			side := geom.SurfaceEdge(0)
			for uint8(1)<<uint(side) != paired.masks[owner] {
				side++
			}
			axis := 1 - int(side)/2
			value := paired.uv[owner][axis]
			opts, e := budget.rootOptions()
			if e != nil {
				return nil, e
			}
			root, n, e := geom.NewExactRationalRoot(ctx, value, opts)
			if e != nil {
				return nil, e
			}
			if e = budget.debit(n); e != nil {
				return nil, e
			}
			f := paired.faces[owner]
			chart := charts[owner][f]
			if canonical == nil {
				canonical, e = affineShellCanonicalEvent(ctx, solids[owner], f, chart.edge[side], axis, value, out.vertices[id], budget)
				if e != nil {
					return nil, e
				}
			}
			row.Owners[owner], e = bindPairedShellEdge(ctx, solids[owner], f, chart, AlgebraicSplineBoundaryOwner{Operand: owner, Side: side, Parameter: root}, canonical, budget)
			if e != nil {
				return nil, e
			}
		}
		expected := map[[2]int]bool{}
		k := 0
		for _, u := range row.Owners[0].Incidents {
			for _, v := range row.Owners[1].Incidents {
				pair := [2]int{u.Face, v.Face}
				expected[pair] = true
				row.IncidentPairs[k] = pair
				k++
			}
		}
		if len(expected) != 4 || len(occurrences[id]) != 4 {
			return nil, geom.ErrDomain
		}
		for _, c := range occurrences[id] {
			if !expected[c.faces] {
				return nil, geom.ErrDomain
			}
			delete(expected, c.faces)
		}
		if len(expected) != 0 {
			return nil, geom.ErrDomain
		}
		out.paired = append(out.paired, row)
	}
	if len(out.paired) == 0 {
		return nil, geom.ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.used
	return out, nil
}

func affineShellCanonicalEvent(ctx context.Context, s Solid, face, edge, axis int, value *big.Rat, point [3]*big.Rat, budget *pairedTerminalBudget) (*geom.ExactComposedCurveEvent, error) {
	var selected *Use
	for i := range s.faces[face].Outer {
		u := &s.faces[face].Outer[i]
		if u.Edge == edge {
			if selected != nil {
				return nil, geom.ErrDomain
			}
			selected = u
		}
	}
	if selected == nil {
		return nil, geom.ErrDomain
	}
	p, q := selected.UVStart, selected.UVEnd
	if selected.Reversed {
		p, q = q, p
	}
	lo, hi := p.X, q.X
	if axis == 1 {
		lo, hi = p.Y, q.Y
	}
	width := new(big.Rat).Sub(ratInset(hi), ratInset(lo))
	if width.Sign() == 0 {
		return nil, geom.ErrDomain
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(value, ratInset(lo)), width)
	if t.Sign() <= 0 || t.Cmp(big.NewRat(1, 1)) >= 0 {
		return nil, geom.ErrDomain
	}
	const allowance = 250000
	if e := budget.debit(32 * allowance); e != nil {
		return nil, e
	}
	composition := budget.options.SurfaceCurveAgreementOptions
	composition.MaxOperations = allowance
	model, _, e := algebraicSeamSource(ctx, s, edge, composition)
	if e != nil {
		return nil, e
	}
	child, e := budget.endpointOptions()
	if e != nil {
		return nil, e
	}
	uv, e := model.curve.modelParameterCurve(ctx, child.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	if e = budget.debit(uv.ExactOperations()); e != nil {
		return nil, e
	}
	child, e = budget.endpointOptions()
	if e != nil {
		return nil, e
	}
	cells, e := model.patch.BuildSurfaceCurveCompositionCells(ctx, uv, child.SurfaceCurveAgreementOptions)
	if e != nil {
		return nil, e
	}
	if e = budget.debit(cells.ExactOperations); e != nil {
		return nil, e
	}
	if !cells.Complete {
		return nil, geom.ErrConvergence
	}
	opts, e := budget.rootOptions()
	if e != nil {
		return nil, e
	}
	root, n, e := geom.NewExactRationalRoot(ctx, t, opts)
	if e != nil {
		return nil, e
	}
	if e = budget.debit(n); e != nil {
		return nil, e
	}
	var event *geom.ExactComposedCurveEvent
	for _, cell := range cells.Cells {
		opts, e = budget.rootOptions()
		if e != nil {
			return nil, e
		}
		if opts.MaxOperations > 100000 {
			opts.MaxOperations = 100000
		}
		if e = budget.debit(opts.MaxOperations); e != nil {
			return nil, e
		}
		candidate, err := cell.BindRoot(ctx, root, opts)
		if errors.Is(err, geom.ErrDomain) {
			continue
		}
		if err != nil {
			return nil, err
		}
		opts, e = budget.rootOptions()
		if e != nil {
			return nil, e
		}
		witness, e := candidate.EvaluateRationalWitness(ctx, t, opts)
		if e != nil {
			return nil, e
		}
		if e = budget.debit(witness.ExactOperations); e != nil {
			return nil, e
		}
		for k := range point {
			if point[k].Cmp(witness.Point[k]) != 0 {
				return nil, geom.ErrDomain
			}
		}
		event = candidate
	}
	if event == nil {
		return nil, geom.ErrDomain
	}
	return event, nil
}
