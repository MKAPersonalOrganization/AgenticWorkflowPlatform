// Package userrequirements is the user-facing promise test suite.
//
// Each file in this package maps to a feature described in the product docs
// (see frontend/ui/docs) and asserts the behaviour the docs promise to
// users. Unlike unit tests that cover internal invariants, these tests
// treat the running service as a black box: every test drives the
// /debug/agent:run endpoint (POST to 127.0.0.1:4015 by default) with a
// complete agent configuration and asserts only what a user can observe —
// the response body, the event trace, and whatever JSON the model returns.
//
// Promises covered
//
// Each *_test.go file starts with a comment block listing the doc paragraph
// it exercises and the specific claims it pins down. When a claim here
// regresses, the failing test name points directly at the sentence in the
// docs that is no longer true.
//
// Running
//
// The suite is gated behind the "integration" build tag so `go test ./...`
// stays fast. To run it:
//
//	task up:cluster                           # bring up the stack (exposes :4015)
//	OPENAI_API_KEY=sk-... \
//	    go test -tags integration -v ./userrequirements/...
//
// A test skips (rather than fails) when prerequisites are missing —
// OPENAI_API_KEY unset, or the debug endpoint unreachable — so the suite
// is safe to run everywhere.
package userrequirements
