// Package guard holds architectural invariant tests for aictl — most notably
// the no-network rule (NFR-1): aictl performs no network/LLM calls in its core
// path. It intentionally exposes no runtime API; the enforcement lives in the
// test file.
package guard
