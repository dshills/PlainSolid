package boolean

import "context"

// arrangement is the one record that the intersection stage builds and
// the building stage reads (section 7.2). While the passes of section 7.3
// run it holds nodes that may still merge and, for every input edge, the
// nodes on it. seal then fixes it: the nodes become points, the node
// lists become spans, overlaps become bundles, and every list below is
// rewritten over points and elements. Nothing else passes between the
// two stages (AR8).
type arrangement struct {
	fabric // points, elements and face boundaries, filled by seal

	ctx    context.Context
	t      tolerances
	limits Options
	tests  int // geometric pair tests so far (MaxPairs)

	// While the passes run.
	nodes   []node       // AR1
	live    int          // nodes not merged into another
	vnode   [2][]int     // the node of each input vertex
	stops   [2][][]stop  // AR2: the nodes on each input edge, in order
	owners  [2][][2]int  // the faces of the two uses of each input edge
	borders [2][][][]leg // the boundary of each input face as legs
	laps    []lap        // overlaps of an edge of A with an edge of B (X3)
	lies    []lying      // parts of edges that lie inside a face (X5)
	loose   [2][][]int   // nodes inside each face with no span in it (X4, X5)
	touches []touch      // touching contacts as found (X4, X5)

	// After seal.
	spans    []span     // AR2
	bundles  []bundle   // AR3
	traces   []trace    // AR4
	inner    [2][][]int // AR5: elements inside each face
	along    [2][][]int // AR5: spans of the other operand along each face's boundary
	free     [2][][]int // AR5: points inside each face with no span in it
	pairs    []samePair // AR6
	contacts []contact  // AR7
}

// lap is an overlap of an edge of A with an edge of B (CC4): the nodes at
// the two ends of the overlap interval, in the direction of A's edge.
type lap struct {
	edges   [2]int
	ends    [2]int
	opposed bool
}

// lying is a span of an edge that lies on the carrier of a face of the
// other operand and inside its trimmed region (X5).
type lying struct {
	op, edge int
	ends     [2]int // nodes
	face     int
}

// touch is a touching contact as a pass finds it: a node or a lying
// span, with the faces of A and of B that meet there. A touching trace
// is its own record.
type touch struct {
	node, lie int // one is set, the other is -1
	faces     [2][]int
}

// trace is a curve where a face of A and a face of B meet that is not
// already an edge of either (AR4). A closed trace has equal ends.
type trace struct {
	c        curve
	t0, t1   float64
	ends     [2]int // nodes; points after seal
	faces    [2]int
	touching bool
	elem     int // its element, set by seal
}

// samePair is a face of A and a face of B on one surface (AR6).
type samePair struct {
	faces    [2]int
	opposite bool // the outward normals are opposite
}

// contact is a touching contact of the sealed arrangement (AR7): a point
// or an element, with the faces of A and of B that meet there.
type contact struct {
	point, elem int // one is set, the other is -1
	faces       [2][]int
}

// newArrangement starts the arrangement of two operands: one node for
// every vertex and, on every edge, the nodes of its two ends.
func newArrangement(ctx context.Context, ops [2]*operand, t tolerances, o Options) (*arrangement, error) {
	a := &arrangement{ctx: ctx, t: t, limits: o}
	a.ops = ops
	for i, op := range ops {
		for v, p := range op.verts {
			n, err := a.add(p)
			if err != nil {
				return nil, err
			}
			a.nodes[n].verts[i] = []int{v}
			a.vnode[i] = append(a.vnode[i], n)
		}
		for _, e := range op.edges {
			a.stops[i] = append(a.stops[i], []stop{{a.vnode[i][e.start], e.t0}, {a.vnode[i][e.end], e.t1}})
		}
		a.owners[i] = filled(op.edges, [2]int{-1, -1})
		for f, face := range op.faces {
			var border [][]leg
			for _, loop := range face.loops {
				var legs []leg
				for _, u := range loop {
					e := op.edges[u.edge]
					l := leg{e.c, e.t0, e.t1}
					if u.reversed {
						l = l.back()
					}
					legs = append(legs, l)
					if a.owners[i][u.edge][0] < 0 {
						a.owners[i][u.edge][0] = f
					} else {
						a.owners[i][u.edge][1] = f
					}
				}
				border = append(border, legs)
			}
			a.borders[i] = append(a.borders[i], border)
		}
		a.loose[i] = table[[]int](op.faces)
	}
	return a, nil
}

// cyclic is entry k of a list read as a ring.
func cyclic[T any](list []T, k int) T { return list[(k%len(list)+len(list))%len(list)] }

// lie records a span that lies in a face and returns the record's index.
func (a *arrangement) lie(l lying) int {
	a.lies = append(a.lies, l)
	return len(a.lies) - 1
}

// tick counts one geometric pair test (section 10) and checks the
// context, which takes precedence over the budget.
func (a *arrangement) tick() error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	if a.tests++; a.tests > a.limits.MaxPairs {
		return fail(ErrBudget, "arrangement", "more than MaxPairs %d pair tests", a.limits.MaxPairs)
	}
	return nil
}

// lower runs the passes below face with face, in the order of section
// 7.3, so that a later pass attaches what it finds to nodes that exist.
func (a *arrangement) lower() error {
	for _, pass := range []func() error{a.vertexWithVertex, a.vertexWithEdge, a.edgeWithEdge, a.vertexWithFace, a.edgeWithFace} {
		if err := pass(); err != nil {
			return err
		}
	}
	return nil
}
