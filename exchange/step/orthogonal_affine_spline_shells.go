package step

import (
	"context"
	"io"
	"math/big"
)

// OrthogonalAffineSplineShellBody uses the same preserved source topology and
// derived immutable planar Solid as AxisAlignedSplineShellBody, but is accepted
// through the separate common-orthogonal-affine support proof.
type OrthogonalAffineSplineShellBody = AxisAlignedSplineShellBody
type OrthogonalAffineSplineShellModel = AxisAlignedSplineShellModel

type exactAxis [3]*big.Rat

func axisDot(a, b exactAxis) *big.Rat {
	r := new(big.Rat)
	for k := 0; k < 3; k++ {
		r.Add(r, new(big.Rat).Mul(a[k], b[k]))
	}
	return r
}
func axisParallel(a, b exactAxis) bool {
	for k := 0; k < 3; k++ {
		i, j := (k+1)%3, (k+2)%3
		if new(big.Rat).Mul(a[i], b[j]).Cmp(new(big.Rat).Mul(a[j], b[i])) != 0 {
			return false
		}
	}
	return true
}
func commonOrthogonalAffineSupports(faces []OrthogonalSplineFace) bool {
	return affineSplineSupports(faces, true)
}

func affineSplineSupports(faces []OrthogonalSplineFace, orthogonal bool) bool {
	families := []exactAxis{}
	for _, face := range faces {
		if face.Geometry == nil {
			return false
		}
		d := face.Geometry.Data()
		if d.DegreeU != 1 || d.DegreeV != 1 || d.CountU != 2 || d.CountV != 2 || len(d.KnotsU) != 4 || len(d.KnotsV) != 4 || d.KnotsU[0] != d.KnotsU[1] || d.KnotsU[2] != d.KnotsU[3] || d.KnotsV[0] != d.KnotsV[1] || d.KnotsV[2] != d.KnotsV[3] {
			return false
		}
		for _, w := range d.Weights {
			if w != d.Weights[0] {
				return false
			}
		}
		p := [4]exactAxis{}
		for i, point := range d.Poles {
			p[i] = exactAxis{new(big.Rat).SetFloat64(point.X), new(big.Rat).SetFloat64(point.Y), new(big.Rat).SetFloat64(point.Z)}
		}
		u, v := exactAxis{}, exactAxis{}
		for k := 0; k < 3; k++ {
			u[k] = new(big.Rat).Sub(p[2][k], p[0][k])
			v[k] = new(big.Rat).Sub(p[1][k], p[0][k])
			want := new(big.Rat).Add(p[0][k], new(big.Rat).Add(u[k], v[k]))
			if want.Cmp(p[3][k]) != 0 {
				return false
			}
		}
		if axisDot(u, u).Sign() == 0 || axisDot(v, v).Sign() == 0 || axisParallel(u, v) || orthogonal && axisDot(u, v).Sign() != 0 {
			return false
		}
		if !orthogonal {
			continue
		}
		for _, axis := range []exactAxis{u, v} {
			found := false
			for _, family := range families {
				if axisParallel(axis, family) {
					found = true
					break
				}
				if axisDot(axis, family).Sign() != 0 {
					return false
				}
			}
			if !found {
				if len(families) == 3 {
					return false
				}
				families = append(families, axis)
			}
		}
	}
	return len(faces) > 0 && (!orthogonal || len(families) == 3)
}

// ReadOrthogonalAffineSplineShells preserves model-space source coordinates and
// units while proving every affine support axis belongs to three common exact
// orthogonal direction families. Axis lengths need not be one or equal. This
// does not infer a rigid placement from approximately orthogonal coordinates.
func ReadOrthogonalAffineSplineShells(r io.Reader, l Limits) (OrthogonalAffineSplineShellModel, error) {
	return ReadOrthogonalAffineSplineShellsContext(context.Background(), r, l)
}
func ReadOrthogonalAffineSplineShellsContext(ctx context.Context, r io.Reader, l Limits) (OrthogonalAffineSplineShellModel, error) {
	return readAffineSplineShellsContext(ctx, r, l, supportOrthogonal)
}

// WriteOrthogonalAffineSplineShells emits the preserved source graph, then
// requires exact support-axis, edge, topology and global planar embedding proofs
// on the staged output. The derived Solid field is not serialized.
func WriteOrthogonalAffineSplineShells(w io.Writer, m OrthogonalAffineSplineShellModel) error {
	graph := OrthogonalSplineOpenShellModel{LengthUnit: m.LengthUnit}
	for _, body := range m.Bodies {
		graph.Shells = append(graph.Shells, OrthogonalSplineOpenShell{Name: body.Name, Faces: body.Faces, FaceHoles: body.FaceHoles, FaceIDs: body.FaceIDs, Uses: body.Uses})
	}
	return writeOrthogonalSplineShells(w, graph, true, supportOrthogonal)
}
