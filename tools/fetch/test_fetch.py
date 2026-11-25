"""Testy fetch.py; te same przypadki co main_test.go. Uruchomienie: python -m unittest discover tools/fetch"""

import io
import os
import tarfile
import tempfile
import unittest

import fetch


def tar_gz(entries):
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        for name, body in entries:
            info = tarfile.TarInfo(name)
            info.size = len(body)
            tf.addfile(info, io.BytesIO(body))
    return buf.getvalue()


class FetchTest(unittest.TestCase):
    def test_extract_strips_and_hashes(self):
        with tempfile.TemporaryDirectory() as d1, tempfile.TemporaryDirectory() as d2:
            fetch.extract_tar_gz(tar_gz([("top/a.xsd", b"a"), ("top/sub/b.xml", b"b")]), d1, 1)
            with open(os.path.join(d1, "sub", "b.xml"), "rb") as f:
                self.assertEqual(f.read(), b"b")
            sum1, n = fetch.tree_sum(d1)
            self.assertEqual(n, 2)
            # Ta sama zawartość spakowana inaczej daje tę samą sumę drzewa.
            fetch.extract_tar_gz(tar_gz([("x/sub/b.xml", b"b"), ("x/a.xsd", b"a")]), d2, 1)
            self.assertEqual(fetch.tree_sum(d2)[0], sum1)

    def test_tree_sum_matches_go(self):
        # Wartość policzona przez treeSum z main.go dla tego samego drzewa (a.xsd = "a", sub/b.xml = "b").
        with tempfile.TemporaryDirectory() as d:
            fetch.extract_tar_gz(tar_gz([("top/a.xsd", b"a"), ("top/sub/b.xml", b"b")]), d, 1)
            self.assertEqual(fetch.tree_sum(d)[0], GO_TREE_SUM)

    def test_extract_rejects_escaping_paths(self):
        for name in ("top/../../evil", "/abs/evil", "c:/evil"):
            with tempfile.TemporaryDirectory() as d:
                with self.assertRaises(fetch.FetchError, msg=name):
                    fetch.extract_tar_gz(tar_gz([(name, b"x")]), d, 0)

    def test_looks_like_html(self):
        cases = {
            b'\xef\xbb\xbf<?xml version="1.0"?><a/>': False,
            b"<xsd:schema/>": False,
            b"\n<!DOCTYPE html><html>": True,
            "<html><head><title>Nieprawidłowy".encode("utf-8"): True,
        }
        for data, want in cases.items():
            self.assertEqual(fetch.looks_like_html(data), want, data)


GO_TREE_SUM = "af2a37d086ae3007af6beb69df9cf0d1569699b41a4e2e0134a877a5db68afd0"

if __name__ == "__main__":
    unittest.main()
