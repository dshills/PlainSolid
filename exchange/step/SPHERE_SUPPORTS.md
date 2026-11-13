# Exact standalone spherical supports

`ReadSphereSupports` and `WriteSphereSupports` exchange a generic
`REPRESENTATION`/`GEOMETRIC_SET` of complete `SPHERICAL_SURFACE` supports with
explicit placement frames and units. `Geometry` returns `geom.SphereSurface`.
U is longitude around the source pole axis, starting at its reference meridian;
V is latitude in [-π/2,π/2]. Longitude is periodic and the poles are parameter
singularities. No finite trim, seam topology, shell orientation or material
closure is inferred from these analytic conventions.

The scope requires perpendicular signed coordinate unit axis/reference vectors,
a positive radius, exact decimal-to-float64 geometry and uniquely owned source
dependencies. Normalization, projection and unit rescaling are not performed.
All surface, placement and datum names plus representation/set names survive.
The ordinary strict unit checker and existing exact coordinate/scalar validators
are reused. Unused entities, shared dependencies, implicit directions, trims,
products, assemblies and topology graphs fail. Limits include 64 supports and
512 entities in addition to parser byte/value/depth limits. Output is staged
and strictly re-read before caller I/O.

The independently authored OCCT fixture and optional
`PLAINSOLID_OCCT_SPHERE_SUPPORT` helper qualify both axis signs, translated
centers and a nondefault meridian by comparing points and both derivatives.
They validate analytic entity transfer only, not bounded faces or solid shape
transfer. Preflight hints remain declaration-level routing, not acceptance.
