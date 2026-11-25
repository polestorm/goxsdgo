package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func tarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestExtractStripsAndHashes(t *testing.T) {
	dir := t.TempDir()
	data := tarGz(t, map[string]string{"top/a.xsd": "a", "top/sub/b.xml": "b"})
	if err := extractTarGz(bytes.NewReader(data), dir, 1); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "sub", "b.xml")); err != nil || string(b) != "b" {
		t.Fatalf("sub/b.xml: %q, %v", b, err)
	}
	sum1, n, err := treeSum(dir)
	if err != nil || n != 2 {
		t.Fatalf("treeSum: %d files, %v", n, err)
	}
	// Ta sama zawartość spakowana inaczej (inna kolejność wpisów) daje tę samą sumę drzewa.
	dir2 := t.TempDir()
	extractTarGz(bytes.NewReader(tarGz(t, map[string]string{"x/sub/b.xml": "b", "x/a.xsd": "a"})), dir2, 1)
	if sum2, _, _ := treeSum(dir2); sum2 != sum1 {
		t.Fatalf("tree sum depends on packing: %s vs %s", sum1, sum2)
	}
	// Ta sama wartość jest w test_fetch.py: obie wersje narzędzia muszą liczyć sumę drzewa tak samo.
	if sum1 != treeSumReference {
		t.Fatalf("tree sum %s, reference %s", sum1, treeSumReference)
	}
}

// Suma drzewa przez dowiązanie do katalogu jest taka sama jak bez niego (zgodnie z fetch.py).
func TestTreeSumFollowsSymlinkedDir(t *testing.T) {
	dir := t.TempDir()
	if err := extractTarGz(bytes.NewReader(tarGz(t, map[string]string{"top/a.xsd": "a", "top/sub/b.xml": "b"})), dir, 1); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("symlinks not available:", err)
	}
	a, _, err1 := treeSum(dir)
	b, _, err2 := treeSum(link)
	if err1 != nil || err2 != nil || a != b {
		t.Fatalf("treeSum via symlink: %s %v vs %s %v", b, err2, a, err1)
	}
}

func TestExtractRejectsEscapingPaths(t *testing.T) {
	for _, name := range []string{"top/../../evil", "/abs/evil", "c:/evil"} {
		dir := t.TempDir()
		err := extractTarGz(bytes.NewReader(tarGz(t, map[string]string{name: "x"})), dir, 0)
		if err == nil {
			t.Errorf("%q: accepted", name)
		}
	}
}

func TestLooksLikeHTML(t *testing.T) {
	cases := map[string]bool{
		"\xef\xbb\xbf<?xml version=\"1.0\"?><a/>": false,
		"<xsd:schema/>":                    false,
		"\n<!DOCTYPE html><html>":          true,
		"<html><head><title>Nieprawidłowy": true,
	}
	for in, want := range cases {
		if got := looksLikeHTML([]byte(in)); got != want {
			t.Errorf("%q: %v, want %v", in, got, want)
		}
	}
}

const treeSumReference = "af2a37d086ae3007af6beb69df9cf0d1569699b41a4e2e0134a877a5db68afd0"
