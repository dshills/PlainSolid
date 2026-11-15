package document

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (s State) circularInsetExportRecipe(ctx context.Context, nodeID string) (trimmed.ExactCircularSweep, trimmed.ExactCircularSweep, geom.Affine, string, error) {
	fail := func(err error) (trimmed.ExactCircularSweep, trimmed.ExactCircularSweep, geom.Affine, string, error) {
		return trimmed.ExactCircularSweep{}, trimmed.ExactCircularSweep{}, geom.Affine{}, "", err
	}
	node, ok := s.Nodes[nodeID]
	if !ok || node.Asset == "" {
		return fail(fmt.Errorf("document: node %q has no circular inset asset", nodeID))
	}
	layout, err := s.circularInsetRoles(node.Asset)
	if err != nil {
		return fail(err)
	}
	parentPose := geom.Identity()
	seen := map[string]bool{}
	for id := nodeID; id != ""; {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		n, ok := s.Nodes[id]
		if !ok || seen[id] || len(seen) >= 256 || !circularInsetSignedPose(n.Transform) {
			return fail(fmt.Errorf("document: invalid circular inset node placement %q: %w", id, geom.ErrDomain))
		}
		seen[id] = true
		parentPose, err = circularInsetExactCompose(n.Transform, parentPose)
		if err != nil {
			return fail(err)
		}
		id = n.Parent
	}
	featurePose := geom.Identity()
	for id := node.Asset; s.Features[id].Kind == TransformFeature; {
		f := s.Features[id]
		// circularInsetRoles has already proved a bounded acyclic feature
		// chain ending at the authored join.
		if !circularInsetSignedPose(f.Transform) {
			return fail(geom.ErrDomain)
		}
		featurePose, err = circularInsetExactCompose(featurePose, f.Transform)
		if err != nil {
			return fail(err)
		}
		id = f.Input
	}
	pose, err := circularInsetExactCompose(parentPose, featurePose)
	if err != nil {
		return fail(err)
	}
	if pose.Determinant() != 1 {
		return fail(fmt.Errorf("document: circular inset export needs a proper signed-coordinate placement: %w", geom.ErrDomain))
	}
	host, err := s.circularInsetExportSweep(ctx, layout.input)
	if err != nil {
		return fail(err)
	}
	guest, err := s.circularInsetExportSweep(ctx, layout.tool)
	if err != nil {
		return fail(err)
	}
	name := node.Name
	if name == "" {
		name = nodeID
	}
	return host, guest, pose, name, nil
}

func (s State) circularInsetExportSweep(ctx context.Context, id string) (trimmed.ExactCircularSweep, error) {
	var zero trimmed.ExactCircularSweep
	var offsets []Feature
	for s.Features[id].Kind == DirectedCircularExtrusionOffsetFeature {
		f := s.Features[id]
		offsets = append(offsets, f)
		id = f.Input
	}
	f := s.Features[id]
	src, _, err := evaluateDirectedCircularExtrusion(ctx, f)
	if err != nil {
		return zero, err
	}
	for j := len(offsets) - 1; j >= 0; j-- {
		f := offsets[j]
		o := trimmed.CircularSweepOffsetOptions{Sweep: trimmed.ExactCircularLoopOptions{DisplayTolerance: f.Tolerance}}
		next, err := trimmed.OffsetExactCircularSweepSide(ctx, src, f.Distance, o)
		if err != nil {
			return zero, err
		}
		src = next.Profile
	}
	return src, nil
}

func circularInsetSignedPose(m geom.Affine) bool {
	one := [3]bool{}
	for _, row := range m {
		n := 0
		for j, v := range row[:3] {
			if v == 0 {
				continue
			}
			if v != 1 && v != -1 || one[j] {
				return false
			}
			n++
			one[j] = true
		}
		if n != 1 || math.IsNaN(row[3]) || math.IsInf(row[3], 0) {
			return false
		}
	}
	return true
}

func circularInsetExactCompose(a, b geom.Affine) (geom.Affine, error) {
	a0 := a
	for i := range a0 {
		for j, v := range a[i][:3] {
			if v == 0 {
				continue
			}
			for k := 0; k < 3; k++ {
				a0[i][k] = v * b[j][k]
			}
			p := new(big.Rat).SetFloat64(a[i][3])
			q := new(big.Rat).SetFloat64(v * b[j][3])
			x, err := geom.RepresentedExactScalar(p.Add(p, q))
			if err != nil {
				return geom.Affine{}, err
			}
			a0[i][3] = x
			break
		}
	}
	return a0, nil
}

// WriteCircularInsetSTEP exports one authored inset occurrence. Both original
// profiles and feature offsets are regenerated; proper signed-coordinate
// feature and assembly placements must compose without rounding.
func (s State) WriteCircularInsetSTEP(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	host, guest, pose, name, err := s.circularInsetExportRecipe(ctx, nodeID)
	if err != nil {
		return err
	}
	x := geom.Vec3{X: 1}
	z := geom.Vec3{Z: 1}
	f, err := geom.NewFrame(pose.Apply(geom.Vec3{}), pose.ApplyVector(x), pose.ApplyVector(z))
	if err != nil {
		return err
	}
	return step.WriteExactCircularSweepInset(ctx, w, step.ExactCircularSweepInset{Name: name, Frame: f, Host: host, Guest: guest, LengthUnit: s.LengthUnit})
}

// WriteCircularInsetIGES exports one authored inset occurrence at an exact
// placement which preserves positive Z. The in-plane transform is applied to
// every original directed line and circular arc before authoring IGES.
func (s State) WriteCircularInsetIGES(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	host, guest, pose, _, err := s.circularInsetExportRecipe(ctx, nodeID)
	if err != nil {
		return err
	}
	if pose.ApplyVector(geom.Vec3{Z: 1}) != (geom.Vec3{Z: 1}) {
		return fmt.Errorf("document: circular inset IGES needs a positive-Z placement: %w", geom.ErrDomain)
	}
	apply := func(v geom.Vec3) (geom.Vec3, error) {
		p := pose.ApplyVector(v)
		var v0 [3]float64
		for k, x := range [3]float64{p.X, p.Y, p.Z} {
			a, b := new(big.Rat).SetFloat64(x), new(big.Rat).SetFloat64(pose[k][3])
			t, err := geom.RepresentedExactScalar(a.Add(a, b))
			if err != nil {
				return geom.Vec3{}, err
			}
			v0[k] = t
		}
		return geom.Vec3{X: v0[0], Y: v0[1], Z: v0[2]}, nil
	}
	for _, member := range []*trimmed.ExactCircularSweep{&host, &guest} {
		for _, v := range []*float64{&member.Bottom, &member.Top} {
			t, err := apply(geom.Vec3{Z: *v})
			if err != nil {
				return err
			}
			*v = t.Z
		}
		for _, ring := range append([][]trimmed.ExactCircularLoopSegment{member.Outer}, member.Holes...) {
			for i := range ring {
				a := &ring[i]
				points := []*geom.Vec2{&a.Start, &a.End}
				if a.Radius > 0 {
					points = append(points, &a.Center)
				}
				for _, p := range points {
					q, err := apply(geom.Vec3{X: p.X, Y: p.Y})
					if err != nil {
						return err
					}
					*p = geom.Vec2{X: q.X, Y: q.Y}
				}
			}
		}
	}
	return iges.WriteExactCircularSweepInset(ctx, w, iges.ExactCircularSweepInset{Host: host, Guest: guest, Unit: s.LengthUnit})
}
