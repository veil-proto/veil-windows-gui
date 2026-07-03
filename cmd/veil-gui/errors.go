//go:build windows

package main

import "errors"

// Sentinel errors for URI-based import. Returned by importFromURI* helpers so
// the UI layer can produce meaningful messages without inspecting the error
// string.
var (
	errInvalidURI = errors.New("invalid file URI")
	errNotConf    = errors.New("selected file is not a .conf config")
)
