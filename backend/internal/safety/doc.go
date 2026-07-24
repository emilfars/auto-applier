// Package safety holds allowlist-free static guards that enforce the product's
// Prime Directive on the backend: the system never submits a job application to
// a third-party portal. The guards live as tests (see safety_test.go).
package safety
