#!/usr/bin/env python3
"""Pobiera dane testowe opisane w testdata/sources.json: zestawy W3C, schematy MF i przykłady MF.

Odpowiednik narzędzia w Go (main.go w tym katalogu) dla osób bez Go, np. na Windows. Oba czytają ten sam manifest,
sprawdzają te same sumy SHA-256 i liczą sumę drzewa tak samo, więc dają te same pliki. Wymaga Pythona 3.9+
i tylko biblioteki standardowej (ADR 0003).

Użycie (z dowolnego katalogu; katalog główny repozytorium jest ustalany z położenia skryptu):

    python tools/fetch/fetch.py                  # pobiera wszystko, czego brakuje
    python tools/fetch/fetch.py --set mf-schemas # jeden zestaw
    python tools/fetch/fetch.py --verify         # tylko sprawdza to, co już pobrane
    python tools/fetch/fetch.py --from DIR       # brakujące pliki zestawów "files" kopiuje z DIR
    python tools/fetch/fetch.py --record         # dopisuje do manifestu brakujące sumy (nowe wpisy)

Na Windows zamiast "python" bywa "py".
"""

import argparse
import concurrent.futures
import hashlib
import io
import json
import os
import posixpath
import re
import shutil
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request

# Ogranicza rozmiar jednego pobrania (archiwum W3C ma kilka MB); ta sama wartość co w main.go.
MAX_DOWNLOAD = 256 << 20
USER_AGENT = "goxsdgo-fetch"
TIMEOUT = 300


class FetchError(Exception):
    pass


def hex_sum(data):
    return hashlib.sha256(data).hexdigest()


def check_rel_path(p):
    """Przyjmuje tylko ścieżkę względną w postaci kanonicznej, bez wyjścia poza katalog (jak fs.ValidPath w Go)."""
    if not p or "\\" in p or p.startswith("/") or ":" in p:
        raise FetchError("unsafe path %r" % p)
    for part in p.split("/"):
        if part in ("", ".", ".."):
            raise FetchError("unsafe path %r" % p)


def looks_like_html(data):
    """Wykrywa „miękkie 404”: gov.pl i crd.gov.pl zwracają stronę HTML zamiast brakującego pliku."""
    if data.startswith(b"\xef\xbb\xbf"):
        data = data[3:]
    head = data.lstrip(b" \t\r\n")[:64].lower()
    return head.startswith(b"<!doctype html") or head.startswith(b"<html")


def download(url):
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            if resp.status != 200:
                raise FetchError("%s: HTTP %d" % (url, resp.status))
            data = resp.read(MAX_DOWNLOAD + 1)
    except urllib.error.HTTPError as e:
        raise FetchError("%s: HTTP %d" % (url, e.code))
    except (urllib.error.URLError, OSError) as e:
        raise FetchError("%s: %s" % (url, e))
    if len(data) > MAX_DOWNLOAD:
        raise FetchError("%s: larger than %d bytes" % (url, MAX_DOWNLOAD))
    return data


def write_file(target, data):
    os.makedirs(os.path.dirname(target), exist_ok=True)
    with open(target, "wb") as f:
        f.write(data)


def extract_tar_gz(data, dest, strip):
    """Rozpakowuje tylko zwykłe pliki i katalogi; pierwsze `strip` składników ścieżki jest pomijane."""
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
        for m in tf:
            name = m.name[2:] if m.name.startswith("./") else m.name
            name = posixpath.normpath(name)
            parts = name.split("/")
            if len(parts) <= strip:
                continue
            rel = "/".join(parts[strip:])
            check_rel_path(rel)
            target = os.path.join(dest, *rel.split("/"))
            if m.isdir():
                os.makedirs(target, exist_ok=True)
            elif m.isreg():
                if m.size > MAX_DOWNLOAD:
                    raise FetchError("archive entry %r too large" % m.name)
                write_file(target, tf.extractfile(m).read())
            # dowiązania, urządzenia, nagłówki pax: pomijamy


def tree_sum(directory):
    """Suma drzewa: SHA-256 z posortowanych wierszy „<sha256 pliku>  <ścieżka>\\n”, jak w main.go i sha256sum."""
    lines = []
    for root, _, files in os.walk(directory):
        for fn in files:
            p = os.path.join(root, fn)
            if os.path.islink(p) or not os.path.isfile(p):
                continue
            with open(p, "rb") as f:
                h = hex_sum(f.read())
            rel = os.path.relpath(p, directory).replace(os.sep, "/")
            lines.append(h + "  " + rel + "\n")
    if not lines:
        raise FetchError("empty")
    # Go sortuje bajty UTF-8; kolejność bajtów UTF-8 jest tą samą co kolejność punktów kodowych.
    lines.sort()
    h = hashlib.sha256()
    for line in lines:
        h.update(line.encode("utf-8"))
    return h.hexdigest(), len(lines)


class Fetcher:
    def __init__(self, root, from_dir, verify, record):
        self.root = root
        self.from_dir = from_dir
        self.verify = verify
        self.record = record

    def dest(self, s):
        return os.path.join(self.root, *s["dest"].split("/"))

    def archive_set(self, s):
        dest = self.dest(s)
        name = s["name"]
        pinned = bool(s.get("tree_sha256"))
        if os.path.isdir(dest) and pinned:
            try:
                got, n = tree_sum(dest)
            except FetchError:
                got, n = "", 0
            if got == s["tree_sha256"] and n == s.get("file_count"):
                print("%s: ok (%d files, present)" % (name, n))
                return []
            if self.verify:
                return ["tree checksum mismatch in %s: %d files, %s" % (s["dest"], n, got)]
        elif self.verify:
            return ["%s missing or not pinned" % s["dest"]]

        print("%s: downloading %s" % (name, s["url"]), flush=True)
        try:
            data = download(s["url"])
        except FetchError as e:
            return [str(e)]
        got_archive = hex_sum(data)
        if s.get("sha256") and s["sha256"] != got_archive:
            return ["archive checksum %s, manifest %s" % (got_archive, s["sha256"])]
        parent = os.path.dirname(dest)
        os.makedirs(parent, exist_ok=True)
        tmp = tempfile.mkdtemp(prefix=".fetch-" + name + "-", dir=parent)
        try:
            extract_tar_gz(data, tmp, s.get("strip", 0))
            got, n = tree_sum(tmp)
            if not pinned or s.get("sha256") == "":
                if not self.record:
                    return ["not pinned (archive %s, tree %s, %d files); run with --record" % (got_archive, got, n)]
                s["tree_sha256"], s["file_count"] = got, n
                if "sha256" in s:
                    s["sha256"] = got_archive
                print("%s: recorded tree %s, %d files" % (name, got, n))
            elif got != s["tree_sha256"] or n != s.get("file_count"):
                return ["tree checksum %s (%d files), manifest %s (%d files)"
                        % (got, n, s["tree_sha256"], s.get("file_count", 0))]
            if os.path.exists(dest):
                shutil.rmtree(dest)
            os.replace(tmp, dest)
        except (FetchError, tarfile.TarError, OSError) as e:
            return [str(e)]
        finally:
            if os.path.exists(tmp):
                shutil.rmtree(tmp, ignore_errors=True)
        print("%s: ok (%d files)" % (name, n))
        return []

    def fetch_file(self, f, dest):
        check_rel_path(f["path"])
        target = os.path.join(dest, *f["path"].split("/"))
        if f.get("sha256") and os.path.isfile(target):
            with open(target, "rb") as fh:
                if hex_sum(fh.read()) == f["sha256"]:
                    return "present"
        if self.verify:
            raise FetchError("missing or checksum mismatch")
        tried = []

        def accept(data, where):
            if looks_like_html(data):
                tried.append(where + ": HTML page instead of a file")
                return False
            got = hex_sum(data)
            if not f.get("sha256"):
                if not self.record:
                    tried.append(where + ": no checksum in manifest, got " + got + " (run with --record)")
                    return False
                f["sha256"] = got
            elif got != f["sha256"]:
                tried.append(where + ": checksum " + got)
                return False
            write_file(target, data)
            return True

        for url in f.get("urls") or []:
            try:
                data = download(url)
            except FetchError as e:
                tried.append(str(e))
                continue
            if accept(data, url):
                return "download"
        if self.from_dir:
            local = os.path.join(self.from_dir, *f["path"].split("/"))
            try:
                with open(local, "rb") as fh:
                    data = fh.read()
            except OSError as e:
                tried.append(str(e))
            else:
                if accept(data, local):
                    return "local"
        if not tried:
            raise FetchError("no source (no URL; use --from)")
        raise FetchError("; ".join(tried))

    def files_set(self, s):
        dest = self.dest(s)
        files = s.get("files") or []
        errors = []
        counts = {"present": 0, "download": 0, "local": 0}
        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
            futures = [pool.submit(self.fetch_file, f, dest) for f in files]
            for f, fut in zip(files, futures):
                try:
                    counts[fut.result()] += 1
                except (FetchError, OSError) as e:
                    errors.append("%s: %s" % (f["path"], e))
        print("%s: %d/%d ok (present %d, downloaded %d, local copy %d)"
              % (s["name"], len(files) - len(errors), len(files),
                 counts["present"], counts["download"], counts["local"]))
        return errors


def main(argv=None):
    here = os.path.dirname(os.path.abspath(__file__))
    ap = argparse.ArgumentParser(description="Fetch goxsdgo test data listed in testdata/sources.json.")
    ap.add_argument("--root", default=os.path.normpath(os.path.join(here, "..", "..")), help="repository root")
    ap.add_argument("--manifest", default="testdata/sources.json", help="manifest path, relative to --root")
    ap.add_argument("--set", dest="only", default="", help="fetch only this set")
    ap.add_argument("--from", dest="from_dir", default="",
                    help='directory with local copies for sets of kind "files" (checked by SHA-256)')
    ap.add_argument("--verify", action="store_true", help="do not download, only verify what is present")
    ap.add_argument("--record", action="store_true", help="write missing checksums into the manifest")
    a = ap.parse_args(argv)

    mpath = os.path.join(a.root, *a.manifest.split("/"))
    with open(mpath, encoding="utf-8") as fh:
        manifest = json.load(fh)
    fetcher = Fetcher(a.root, a.from_dir, a.verify, a.record)
    failed = []
    found = False
    for s in manifest["sets"]:
        if a.only and s["name"] != a.only:
            continue
        found = True
        if s["kind"] == "archive":
            errs = fetcher.archive_set(s)
        elif s["kind"] == "files":
            errs = fetcher.files_set(s)
        else:
            errs = ["unknown kind %r" % s["kind"]]
        failed += [s["name"] + ": " + e for e in errs]
    if not found:
        print("fetch: no set named %r" % a.only, file=sys.stderr)
        return 1
    if a.record:
        with open(mpath, "w", encoding="utf-8", newline="\n") as fh:
            json.dump(manifest, fh, ensure_ascii=False, indent=2)
            fh.write("\n")
    if failed:
        print("\n%d problem(s):" % len(failed), file=sys.stderr)
        for line in failed:
            print("  " + line, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
