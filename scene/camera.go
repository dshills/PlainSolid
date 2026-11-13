// Package scene provides camera projection and headless picking for mesh scenes.
package scene

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// Camera uses normalized screen coordinates [-1,1]² with Y up, and positive
// forward depth. A constructor validates a finite nondegenerate view volume.
type Camera struct {
	frame                    geom.Frame
	width, height, near, far float64
	perspective              bool
}

// Orthographic creates a camera looking from eye toward target. Width and
// height are full viewport sizes in world units. Up sets screen Y.
func Orthographic(eye, target, up geom.Vec3, width, height, near, far float64) (Camera, error) {
	return newCamera(eye, target, up, width, height, near, far, false)
}

// Perspective uses a vertical field of view in radians and width/height aspect.
func Perspective(eye, target, up geom.Vec3, fovy, aspect, near, far float64) (Camera, error) {
	if !finite(fovy) || fovy <= 0 || fovy >= math.Pi || !finite(aspect) || aspect <= 0 {
		return Camera{}, fmt.Errorf("scene: invalid field of view or aspect")
	}
	height := 2 * math.Tan(fovy/2)
	return newCamera(eye, target, up, height*aspect, height, near, far, true)
}

func finite(x float64) bool { return !math.IsInf(x, 0) && !math.IsNaN(x) }
func newCamera(eye, target, up geom.Vec3, w, h, n, f float64, p bool) (Camera, error) {
	if w <= 0 || h <= 0 || n <= 0 || f <= n || !finite(w) || !finite(h) || !finite(n) || !finite(f) {
		return Camera{}, fmt.Errorf("scene: invalid camera dimensions or clip distances")
	}
	// Local Z points toward the viewer, so frame X and Y are screen right/up.
	back, err := eye.Sub(target).Unit()
	if err != nil {
		return Camera{}, err
	}
	top, err := up.Unit()
	if err != nil {
		return Camera{}, err
	}
	frame, err := geom.NewFrame(eye, top.Cross(back), back)
	if err != nil {
		return Camera{}, err
	}
	return Camera{frame, w, h, n, f, p}, nil
}
func (c Camera) valid() bool { return c.width > 0 && c.height > 0 && c.near > 0 && c.far > c.near }

// Project returns normalized screen coordinates, forward depth and whether the
// point is in the closed view volume. Perspective points at or behind the
// eye return visible=false and a zero screen position.
func (c Camera) Project(point geom.Vec3) (screen geom.Vec2, depth float64, visible bool, err error) {
	if !c.valid() || !point.IsFinite() {
		return geom.Vec2{}, 0, false, fmt.Errorf("scene: invalid camera or point")
	}
	local := c.frame.ToLocal(point)
	depth = -local.Z
	if !local.IsFinite() {
		return geom.Vec2{}, 0, false, fmt.Errorf("scene: projection overflow")
	}
	x, y := local.X, local.Y
	if c.perspective {
		if depth <= 0 {
			return geom.Vec2{}, depth, false, nil
		}
		x, y = x/depth, y/depth
	}
	screen = geom.Vec2{X: 2 * (x / c.width), Y: 2 * (y / c.height)}
	if !finite(screen.X) || !finite(screen.Y) {
		return geom.Vec2{}, depth, false, fmt.Errorf("scene: projection overflow")
	}
	return screen, depth, math.Abs(screen.X) <= 1 && math.Abs(screen.Y) <= 1 && depth >= c.near && depth <= c.far, nil
}

// Ray is a unit-direction ray segment clipped to the near and far planes.
type Ray struct {
	Origin, Direction geom.Vec3
	Length            float64
}

func (c Camera) Ray(screen geom.Vec2) (Ray, error) {
	if !c.valid() || !finite(screen.X) || !finite(screen.Y) || math.Abs(screen.X) > 1 || math.Abs(screen.Y) > 1 {
		return Ray{}, fmt.Errorf("scene: invalid camera or screen coordinates")
	}
	x, y := screen.X*(c.width/2), screen.Y*(c.height/2)
	localOrigin := geom.Vec3{X: x, Y: y, Z: -c.near}
	worldDirection := c.frame.ZAxis().Scale(-1)
	length := c.far - c.near
	if c.perspective {
		localOrigin.X *= c.near
		localOrigin.Y *= c.near
		dir := geom.Vec3{X: x, Y: y, Z: -1}
		length *= dir.Length()
		worldDirection = c.frame.XAxis().Scale(x).Add(c.frame.YAxis().Scale(y)).Add(worldDirection)
	}
	o := c.frame.ToWorld(localOrigin)
	d, e := worldDirection.Unit()
	if e != nil || !o.IsFinite() || length <= 0 || !finite(length) {
		return Ray{}, fmt.Errorf("scene: unrepresentable ray")
	}
	return Ray{o, d, length}, nil
}
