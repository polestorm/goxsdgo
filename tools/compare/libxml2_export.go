//go:build libxml2

package main

// Plik z //export: preambuła może mieć tylko deklaracje, więc funkcje C są w libxml2.go.

/*
#include <stdint.h>
*/
import "C"

import "runtime/cgo"

// compareCollect odbiera jeden błąd z funkcji C compare_serror (libxml2.go) i dopisuje go do zbieracza.
//
//export compareCollect
func compareCollect(h C.uintptr_t, code, domain, level, line C.int, node, msg *C.char) {
	c := cgo.Handle(h).Value().(*collector)
	e := libError{Code: int(code), Domain: int(domain), Severity: libSeverity(level), Line: int(line)}
	if node != nil {
		e.NodeName = C.GoString(node)
	}
	if msg != nil {
		e.Message = C.GoString(msg)
	}
	c.errs = append(c.errs, e)
}

// collector zbiera błędy jednego wywołania libxml2.
type collector struct {
	errs []libError
}
