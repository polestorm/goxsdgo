//go:build libxml2

package main

/*
#cgo pkg-config: libxml-2.0
#include <stdint.h>
#include <stdlib.h>
#include <libxml/parser.h>
#include <libxml/tree.h>
#include <libxml/xmlerror.h>
#include <libxml/xmlschemas.h>
#include <libxml/xmlreader.h>
#include <libxml/catalog.h>

extern void compareCollect(uintptr_t h, int code, int domain, int level, int line, char *node, char *msg);

// sink: uchwyt zbieracza po stronie Go i liczba zebranych wpisów (do przerwania walidacji strumieniowej).
typedef struct {
	uintptr_t h;
	int count;
} sink;

// compare_serror to xmlStructuredErrorFunc: przekazuje pola xmlError do Go. Nazwa węzła tylko dla elementu
// i atrybutu.
static void compare_serror(void *data, const xmlError *e) {
	sink *s = (sink *)data;
	const char *node = NULL;
	if (e->node != NULL) {
		xmlNode *n = (xmlNode *)e->node;
		if (n->type == XML_ELEMENT_NODE || n->type == XML_ATTRIBUTE_NODE) {
			node = (const char *)n->name;
		}
	}
	s->count++;
	compareCollect(s->h, e->code, e->domain, (int)e->level, e->line, (char *)node, (char *)e->message);
}

// compare_silent wycisza domyślne wypisywanie błędów na stderr.
static void compare_silent(void *data, const xmlError *e) { (void)data; (void)e; }

static void compare_init(void) {
	xmlInitParser();
	xmlSetStructuredErrorFunc(NULL, compare_silent);
}

static xmlSchema *compare_parse_schema(xmlSchemaParserCtxt *pc, sink *s) {
	if (pc == NULL) {
		return NULL;
	}
	xmlSchemaSetParserStructuredErrors(pc, compare_serror, s);
	xmlSchema *schema = xmlSchemaParse(pc);
	xmlSchemaFreeParserCtxt(pc);
	return schema;
}

static xmlSchema *compare_schema_file(const char *path, sink *s) {
	return compare_parse_schema(xmlSchemaNewParserCtxt(path), s);
}

static xmlSchema *compare_schema_mem(const char *buf, int len, sink *s) {
	return compare_parse_schema(xmlSchemaNewMemParserCtxt(buf, len), s);
}

// compare_validate_mem: drzewo dokumentu, potem walidacja. Zwraca -2, gdy dokument nie jest poprawnie
// uformowany, inaczej wynik xmlSchemaValidateDoc (0 poprawny, > 0 błędy, < 0 błąd wewnętrzny).
static int compare_validate_mem(xmlSchema *schema, const char *buf, int len, int options, sink *s) {
	xmlParserCtxt *pc = xmlNewParserCtxt();
	if (pc == NULL) {
		return -1;
	}
	xmlCtxtSetErrorHandler(pc, compare_serror, s);
	xmlDoc *doc = xmlCtxtReadMemory(pc, buf, len, NULL, NULL, options);
	xmlFreeParserCtxt(pc);
	if (doc == NULL) {
		return -2;
	}
	xmlSchemaValidCtxt *vc = xmlSchemaNewValidCtxt(schema);
	if (vc == NULL) {
		xmlFreeDoc(doc);
		return -1;
	}
	xmlSchemaSetValidStructuredErrors(vc, compare_serror, s);
	int ret = xmlSchemaValidateDoc(vc, doc);
	xmlSchemaFreeValidCtxt(vc);
	xmlFreeDoc(doc);
	return ret;
}

// compare_validate_file: walidacja strumieniowa przez xmlTextReader. Zwraca 0 po przeczytaniu całości,
// 1 po przerwaniu na max wpisach (max > 0), -1 przy błędzie czytnika albo walidacji.
static int compare_validate_file(xmlSchema *schema, const char *path, int options, int max, sink *s) {
	xmlTextReader *r = xmlReaderForFile(path, NULL, options);
	if (r == NULL) {
		return -1;
	}
	xmlTextReaderSetStructuredErrorHandler(r, compare_serror, s);
	xmlSchemaValidCtxt *vc = xmlSchemaNewValidCtxt(schema);
	if (vc == NULL || xmlTextReaderSchemaValidateCtxt(r, vc, 0) != 0) {
		if (vc != NULL) {
			xmlSchemaFreeValidCtxt(vc);
		}
		xmlFreeTextReader(r);
		return -1;
	}
	int ret;
	while ((ret = xmlTextReaderRead(r)) == 1) {
		if (max > 0 && s->count >= max) {
			break;
		}
	}
	int stopped = ret == 1;
	xmlFreeTextReader(r);
	xmlSchemaFreeValidCtxt(vc);
	if (stopped) {
		return 1;
	}
	return ret == 0 ? 0 : -1;
}
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// docOptions: opcje parsera dokumentu (xmlParserOption): bez sieci, numery wierszy powyżej 65535.
const docOptions = C.XML_PARSE_NONET | C.XML_PARSE_BIG_LINES

func init() { C.compare_init() }

// libSchema to schemat skompilowany przez libxml2.
type libSchema struct {
	p *C.xmlSchema
}

// call uruchamia f z nowym zbieraczem; wskaźnik sink żyje w pamięci C na czas wywołania.
func call(f func(s *C.sink)) []libError {
	c := &collector{}
	h := cgo.NewHandle(c)
	defer h.Delete()
	s := (*C.sink)(C.malloc(C.size_t(unsafe.Sizeof(C.sink{}))))
	defer C.free(unsafe.Pointer(s))
	s.h = C.uintptr_t(h)
	s.count = 0
	f(s)
	return c.errs
}

// loadCatalogFile dodaje katalog XML (OASIS) do katalogu globalnego libxml2: adresy importów → pliki.
func loadCatalogFile(path string) error {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	if C.xmlLoadCatalog(cp) != 0 {
		return fmt.Errorf("libxml2: cannot load catalog %s", path)
	}
	return nil
}

func schemaResult(p *C.xmlSchema, errs []libError) (*libSchema, error) {
	if p == nil {
		return nil, &schemaParseError{Errors: errs}
	}
	return &libSchema{p: p}, nil
}

// parseSchemaFile kompiluje schemat z jednego pliku (importy przez katalog).
func parseSchemaFile(path string) (*libSchema, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var p *C.xmlSchema
	errs := call(func(s *C.sink) { p = C.compare_schema_file(cp, s) })
	return schemaResult(p, errs)
}

// parseSchemaBytes kompiluje schemat z pamięci.
func parseSchemaBytes(b []byte) (*libSchema, error) {
	cb := C.CBytes(b)
	defer C.free(cb)
	var p *C.xmlSchema
	errs := call(func(s *C.sink) { p = C.compare_schema_mem((*C.char)(cb), C.int(len(b)), s) })
	return schemaResult(p, errs)
}

func (s *libSchema) Close() {
	if s != nil && s.p != nil {
		C.xmlSchemaFree(s.p)
		s.p = nil
	}
}

// Validate buduje drzewo dokumentu i waliduje je. Dokument źle uformowany daje jeden wpis: pierwszy błąd
// składni z poziomem fatal.
func (s *libSchema) Validate(doc []byte) ([]libError, error) {
	if len(doc) >= 1<<31 {
		return nil, fmt.Errorf("libxml2: document too large for tree validation")
	}
	cb := C.CBytes(doc)
	defer C.free(cb)
	var ret C.int
	errs := call(func(sk *C.sink) { ret = C.compare_validate_mem(s.p, (*C.char)(cb), C.int(len(doc)), docOptions, sk) })
	switch {
	case ret == -2:
		return parseFailure(errs), nil
	case ret < 0:
		return errs, fmt.Errorf("libxml2: internal validation error %d", int(ret))
	}
	return parseFailure(errs), nil
}

// ValidateFile waliduje plik strumieniowo; po maxErrors wpisach (0: bez limitu) przerywa i zwraca
// errTooManyErrors razem z zebranymi błędami.
func (s *libSchema) ValidateFile(path string, maxErrors int) ([]libError, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var ret C.int
	errs := call(func(sk *C.sink) { ret = C.compare_validate_file(s.p, cp, docOptions, C.int(maxErrors), sk) })
	switch ret {
	case 1:
		return errs, errTooManyErrors
	case -1:
		if hasErrors(errs) {
			return parseFailure(errs), nil
		}
		return errs, fmt.Errorf("libxml2: cannot read %s", path)
	}
	return parseFailure(errs), nil
}
