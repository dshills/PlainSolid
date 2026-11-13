package document

import (
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"sort"
)

// FromSplineShellSTEP imports standalone certified bodies into shared-capable
// trimmed assets. Faces are authoritative and are revalidated before storage.
func FromSplineShellSTEP(m step.SplineShellModel, historyLimit int) (*Document, error) {
	if len(m.Bodies) == 0 || len(m.Bodies) > 10000 || m.LengthUnit <= 0 || math.IsNaN(m.LengthUnit) || math.IsInf(m.LengthUnit, 0) {
		return nil, fmt.Errorf("document: empty spline shell model")
	}
	plain := step.TrimmedModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		if len(b.Faces) < 4 || len(b.Faces) > 64 {
			return nil, fmt.Errorf("document: spline shell face budget")
		}
		surfaces := []*geom.BSplineSurface{}
		domains := []geom.ParameterRectangle{}
		reversed := []bool{}
		for _, f := range b.Faces {
			p, e := geom.NewSurfacePatch(f.Geometry, geom.SurfacePatchOptions{})
			if e != nil {
				return nil, e
			}
			if _, e := p.Trim(f.Domain); e != nil {
				return nil, e
			}
			surfaces = append(surfaces, f.Geometry)
			domains = append(domains, f.Domain)
			reversed = append(reversed, false)
		}
		solid, _, e := trimmed.SewSplineRectangles(context.Background(), surfaces, domains, reversed, trimmed.SewOptions{Tolerance: 1e-10, MaxFaces: 64, MaxUses: 256})
		if e != nil {
			return nil, e
		}
		proof, e := solid.ValidateSplineRectangleEmbedding(context.Background(), trimmed.SplineRectangleEmbeddingOptions{})
		if e != nil || !proof.Complete {
			return nil, fmt.Errorf("document: unresolved spline shell embedding: %v", e)
		}
		plain.Bodies = append(plain.Bodies, step.TrimmedBody{Name: b.Name, Solid: solid})
	}
	d, e := FromTrimmedSTEP(plain, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	s.Labels = map[string]Label{}
	for i, b := range m.Bodies {
		id := fmt.Sprintf("body-%06d", i+1)
		s.Labels["step:"+id] = Label{Name: b.Name, Target: AssetLabel, TargetID: id, Attributes: map[string]Attribute{"step.body.name": {Kind: "string", Text: b.Name}}}
	}
	return New(s, historyLimit)
}
func splineRigidPlacement(m geom.Affine) bool {
	if !m.IsFinite() {
		return false
	}
	x, y, z := m.ApplyVector(geom.Vec3{X: 1}), m.ApplyVector(geom.Vec3{Y: 1}), m.ApplyVector(geom.Vec3{Z: 1})
	return math.Abs(x.Length()-1) <= 1e-10 && math.Abs(y.Length()-1) <= 1e-10 && math.Abs(z.Length()-1) <= 1e-10 && math.Abs(x.Dot(y)) <= 1e-10 && math.Abs(x.Dot(z)) <= 1e-10 && math.Abs(y.Dot(z)) <= 1e-10 && math.Abs(m.Determinant()-1) <= 1e-10
}

// WriteSplineShellSTEP writes each explicitly placed instance as a standalone
// body. It does not infer STEP assembly semantics. Parent/local rigid maps are
// applied in exact homogeneous arithmetic; nonrepresentable results fail.
func (s State) WriteSplineShellSTEP(w io.Writer) error {
	if w == nil {
		return fmt.Errorf("document: nil spline shell writer")
	}
	if len(s.Features) > 0 {
		var e error
		s, e = s.EvaluateFeatures(context.Background())
		if e != nil {
			return e
		}
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) > 10000 {
		return fmt.Errorf("document: spline shell node budget")
	}
	ids := []string{}
	for id, n := range s.Nodes {
		if n.Asset != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	m := step.SplineShellModel{LengthUnit: s.LengthUnit}
	validated := map[string]bool{}
	for _, id := range ids {
		node := s.Nodes[id]
		solid, ok := s.Trimmed[node.Asset]
		if !ok {
			return fmt.Errorf("document: spline shell requires a trimmed asset for %q", id)
		}
		if !validated[node.Asset] {
			proof, e := solid.ValidateSplineRectangleEmbedding(context.Background(), trimmed.SplineRectangleEmbeddingOptions{})
			if e != nil || !proof.Complete {
				return fmt.Errorf("document: spline shell asset %q lacks embedding proof: %v", node.Asset, e)
			}
			validated[node.Asset] = true
		}
		chain := []geom.Affine{}
		for current := id; current != ""; {
			n := s.Nodes[current]
			if !splineRigidPlacement(n.Transform) || len(chain) >= 256 {
				return fmt.Errorf("document: spline shell placement must be bounded proper rigid")
			}
			chain = append(chain, n.Transform)
			current = n.Parent
		}
		name := node.Name
		if name == "" {
			name = id
		}
		body := step.SplineShellBody{Name: name}
		for _, face := range solid.Faces() {
			base, mapping := parametric.SurfaceMapping(face.Surface)
			support, ok := base.(*geom.BSplineSurface)
			if !ok || face.Reversed || len(face.Outer) != 4 || len(face.Holes) != 0 {
				return fmt.Errorf("document: unsupported spline shell face")
			}
			d := geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}
			for _, u := range face.Outer {
				if u.Curve != nil {
					return fmt.Errorf("document: spline shell requires straight UV trims")
				}
				for _, p := range []geom.Vec2{u.UVStart, u.UVEnd} {
					d.UMin = math.Min(d.UMin, p.X)
					d.UMax = math.Max(d.UMax, p.X)
					d.VMin = math.Min(d.VMin, p.Y)
					d.VMax = math.Max(d.VMax, p.Y)
				}
			}
			patch, e := geom.NewSurfacePatch(support, geom.SurfacePatchOptions{})
			if e != nil {
				return e
			}
			patch, e = patch.Transformed(mapping)
			if e != nil {
				return e
			}
			for _, placement := range chain {
				patch, e = patch.Transformed(placement)
				if e != nil {
					return e
				}
			}
			placed, e := patch.ToBSplineSurface()
			if e != nil {
				return e
			}
			body.Faces = append(body.Faces, step.BoundedSplineFace{Geometry: placed, Domain: d})
		}
		m.Bodies = append(m.Bodies, body)
	}
	return step.WriteSplineShells(w, m)
}
