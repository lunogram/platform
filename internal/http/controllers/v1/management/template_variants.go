//go:build !enterprise

package v1

// variantsAvailable reports whether a project may configure template variants.
//
// Variants are an enterprise capability, so open-source builds refuse to
// declare them or to create a template for one. The send path deliberately has
// no equivalent guard: with no variant configurable here, every template is the
// default variant and selection resolves to it on its own, which keeps one code
// path through the render hot loop for both builds.
const variantsAvailable = false
