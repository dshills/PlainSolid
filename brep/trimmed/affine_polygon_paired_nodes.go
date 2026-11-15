package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

// ConvexAffinePolygonSectionOwner is an exact original normalized edge
// parameter, with both incident face/local-coedge occurrences retained.
type ConvexAffinePolygonSectionOwner struct {
	Owner, Edge int
	Parameter   *big.Rat
	Occurrences [][2]int
}

func appendPolygonOccurrence(list [][2]int, pair [2]int) [][2]int {
	for _, p := range list {
		if p == pair {
			return list
		}
	}
	return append(list, pair)
}
func polygonPointKey(p [3]*big.Rat) [3]string {
	return [3]string{p[0].RatString(), p[1].RatString(), p[2].RatString()}
}
func (r *ConvexAffinePolygonCrossing) recordSectionPoint(b *nativePrismAffineBudget, ids map[[3]string]int, faces [2]int, p geom.AffineChartIntersectionEndpoint, sides [2][]int) (int, error) {
	owners := []ConvexAffinePolygonSectionOwner{}
	if len(sides[0]) > 1 || len(sides[1]) > 1 || len(sides[0])+len(sides[1]) == 0 || (!r.allowPaired && len(sides[0])+len(sides[1]) != 1) {
		return 0, geom.ErrDomain
	}
	for owner, side := range sides {
		if len(side) == 0 {
			continue
		}
		face, local := faces[owner], side[0]
		use := r.source.sources[owner].faces[face].Outer[local]
		uv := p.A
		if owner == 1 {
			uv = p.B
		}
		start, end := [2]*big.Rat{ratInset(use.UVStart.X), ratInset(use.UVStart.Y)}, [2]*big.Rat{ratInset(use.UVEnd.X), ratInset(use.UVEnd.Y)}
		axis := 0
		if start[0].Cmp(end[0]) == 0 {
			axis = 1
		}
		parameter := b.op('/', b.op('-', uv[axis], start[axis]), b.op('-', end[axis], start[axis]))
		if use.Reversed {
			parameter = b.op('-', big.NewRat(1, 1), parameter)
		}
		if b.err != nil {
			return 0, b.err
		}
		if parameter.Sign() <= 0 || parameter.Cmp(big.NewRat(1, 1)) >= 0 {
			return 0, geom.ErrDomain
		}
		owners = append(owners, ConvexAffinePolygonSectionOwner{Owner: owner, Edge: use.Edge, Parameter: parameter, Occurrences: [][2]int{{face, local}}})
	}
	token := polygonPointKey(p.Point)
	id, ok := ids[token]
	if !ok {
		id = len(r.nodes)
		ids[token] = id
		r.points = append(r.points, p.Point)
		r.nodes = append(r.nodes, ConvexAffinePolygonSectionVertex{Point: p.Point, Owners: owners})
	} else {
		node := &r.nodes[id]
		if len(node.Owners) != len(owners) {
			return 0, geom.ErrDomain
		}
		for i, owner := range owners {
			old := &node.Owners[i]
			if old.Owner != owner.Owner || old.Edge != owner.Edge || old.Parameter.Cmp(owner.Parameter) != 0 {
				return 0, geom.ErrDomain
			}
			old.Occurrences = appendPolygonOccurrence(old.Occurrences, owner.Occurrences[0])
		}
	}
	r.nodes[id].ContactPairs = appendPolygonOccurrence(r.nodes[id].ContactPairs, faces)
	return id, nil
}
func (r *ConvexAffinePolygonCrossing) finishSectionNodes(b *nativePrismAffineBudget, degree []int) error {
	if len(degree) != len(r.nodes) {
		return geom.ErrDomain
	}
	for i := range r.nodes {
		node := &r.nodes[i]
		for _, owner := range node.Owners {
			if owner.Owner < 0 || owner.Owner > 1 || owner.Edge < 0 || owner.Edge >= len(r.source.sources[owner.Owner].edges) {
				return geom.ErrDomain
			}
			source := r.source.sources[owner.Owner]
			edge := source.edges[owner.Edge]
			start, end := affineMaterialXYZ(source.vertices[edge.Start]), affineMaterialXYZ(source.vertices[edge.End])
			for k := range start {
				value := b.op('+', start[k], b.op('*', owner.Parameter, b.op('-', end[k], start[k])))
				if b.err != nil {
					return b.err
				}
				if value.Cmp(node.Point[k]) != 0 {
					return geom.ErrDomain
				}
			}
			if len(owner.Occurrences) != 2 || owner.Occurrences[0][0] == owner.Occurrences[1][0] {
				return geom.ErrDomain
			}
		}
		if len(node.Owners) == 1 {
			if degree[i] != 2 || len(node.ContactPairs) != 2 {
				return geom.ErrDomain
			}
			owner := node.Owners[0]
			node.Owner, node.Edge, node.Parameter = owner.Owner, owner.Edge, owner.Parameter
			node.Occurrences = append([][2]int(nil), owner.Occurrences...)
		} else if len(node.Owners) == 2 && r.allowPaired {
			if (degree[i] != 2 && degree[i] != 4) || len(node.ContactPairs) != 4 {
				return geom.ErrDomain
			}
			directions := [2][3]*big.Rat{}
			for i, owner := range node.Owners {
				source := r.source.sources[owner.Owner]
				edge := source.edges[owner.Edge]
				a, c := affineMaterialXYZ(source.vertices[edge.Start]), affineMaterialXYZ(source.vertices[edge.End])
				for k := range a {
					directions[i][k] = b.op('-', c[k], a[k])
				}
			}
			independent := false
			for k := 0; k < 3; k++ {
				j, l := (k+1)%3, (k+2)%3
				cross := b.op('-', b.op('*', directions[0][j], directions[1][l]), b.op('*', directions[0][l], directions[1][j]))
				independent = independent || cross.Sign() != 0
			}
			if b.err != nil {
				return b.err
			}
			if !independent {
				return geom.ErrDomain
			}
			node.Owner, node.Edge, node.Parameter = -1, -1, nil
			node.Occurrences = nil
			if node.Owners[0].Owner != 0 || node.Owners[1].Owner != 1 {
				return geom.ErrDomain
			}
			for _, a := range node.Owners[0].Occurrences {
				for _, c := range node.Owners[1].Occurrences {
					found := false
					for _, pair := range node.ContactPairs {
						found = found || pair == [2]int{a[0], c[0]}
					}
					if !found {
						return geom.ErrDomain
					}
				}
			}
		} else {
			return geom.ErrDomain
		}
	}
	return nil
}
func polygonSectionNodesEqual(a, c ConvexAffinePolygonSectionVertex) bool {
	if len(a.Owners) != len(c.Owners) || len(a.ContactPairs) != len(c.ContactPairs) || a.Owner != c.Owner || a.Edge != c.Edge {
		return false
	}
	if (a.Parameter == nil) != (c.Parameter == nil) || len(a.Occurrences) != len(c.Occurrences) {
		return false
	}
	if a.Parameter != nil && a.Parameter.Cmp(c.Parameter) != 0 {
		return false
	}
	for i, p := range a.Occurrences {
		if p != c.Occurrences[i] {
			return false
		}
	}
	for k := range a.Point {
		if a.Point[k] == nil || c.Point[k] == nil || a.Point[k].Cmp(c.Point[k]) != 0 {
			return false
		}
	}
	for i, owner := range a.Owners {
		other := c.Owners[i]
		if owner.Owner != other.Owner || owner.Edge != other.Edge || owner.Parameter == nil || other.Parameter == nil || owner.Parameter.Cmp(other.Parameter) != 0 || len(owner.Occurrences) != len(other.Occurrences) {
			return false
		}
		for j, p := range owner.Occurrences {
			if p != other.Occurrences[j] {
				return false
			}
		}
	}
	for i, p := range a.ContactPairs {
		if p != c.ContactPairs[i] {
			return false
		}
	}
	return true
}
