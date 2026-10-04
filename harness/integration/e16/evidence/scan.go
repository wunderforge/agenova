// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Scan mode searches archived campaign evidence for the Slice 4 token values.
// The runner reads the values back from the cluster and passes them on stdin,
// never in argv, one "label value" line each:
//
//	valid-fixture <the fixture's copy of the valid token>
//	valid         <the install namespace's copy of the valid token>
//	wrong         <the wrong token>
//
// (one space between label and value). Output names files, never a matched
// value or its label: "leak <path>" or "leak <zip path>!<entry>" per hit, then
// a summary line. It exits 0 when clean, 1 on any leak and 2 on any error.

var tokenLabels = []string{"valid-fixture", "valid", "wrong"}

var tokenValue = regexp.MustCompile(`^[0-9a-f]{64}$`)

// scanChunk is how much of a file is read at a time; files are streamed so
// large image archives are never loaded whole.
const scanChunk = 1 << 20

func scanMain(roots []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(roots) == 0 {
		fmt.Fprintln(stderr, "usage: evidence scan <root>... (stdin: one \"<label> <value>\" line each for valid-fixture, valid and wrong)")
		return 2
	}
	values, err := readTokens(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "[fail] scan:", err)
		return 2
	}
	distinct := []string{values["valid"], values["wrong"]}
	m := newMatcher(distinct)
	if err := m.selfTest(distinct); err != nil {
		fmt.Fprintln(stderr, "[fail] scan:", err)
		return 2
	}
	s := &scanner{m: m, buf: m.buffer(), out: stdout}
	for _, root := range roots {
		if err := s.root(root); err != nil {
			fmt.Fprintln(stderr, "[fail] scan:", err)
			return 2
		}
	}
	fmt.Fprintf(stdout, "scanned files=%d bytes=%d zip-entries=%d patterns=%d roots=%d\n", s.files, s.bytes, s.entries, len(m.patterns), len(roots))
	if s.leaks > 0 {
		fmt.Fprintf(stderr, "[fail] scan: %d token leaks\n", s.leaks)
		return 1
	}
	fmt.Fprintln(stderr, "[pass] scan: no token value in the scanned evidence")
	return 0
}

// readTokens reads exactly one line per label. Errors name a line number or
// a label, never what the line holds.
func readTokens(r io.Reader) (map[string]string, error) {
	values := map[string]string{}
	lines := bufio.NewScanner(r)
	n := 0
	for lines.Scan() {
		n++
		label, value, ok := strings.Cut(lines.Text(), " ")
		switch {
		case lines.Text() == "":
			return nil, fmt.Errorf("token input line %d is blank", n)
		case !ok || !contains(tokenLabels, label):
			return nil, fmt.Errorf("token input line %d is not \"<label> <value>\" with a label of %s", n, strings.Join(tokenLabels, ", "))
		case values[label] != "":
			return nil, fmt.Errorf("token input repeats %s", label)
		case !tokenValue.MatchString(value):
			return nil, fmt.Errorf("token input %s is not 64 lowercase hex characters", label)
		}
		values[label] = value
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("token input: %v", err)
	}
	if n == 0 {
		return nil, errors.New("token input is empty")
	}
	for _, label := range tokenLabels {
		if values[label] == "" {
			return nil, fmt.Errorf("token input has no %s line", label)
		}
	}
	if values["valid"] != values["valid-fixture"] {
		return nil, errors.New("token input: valid differs from valid-fixture, so the install namespace and the fixture hold different tokens")
	}
	if values["wrong"] == values["valid"] {
		return nil, errors.New("token input: wrong equals valid")
	}
	return values, nil
}

// matcher holds the byte strings that reveal a token value.
type matcher struct {
	patterns [][]byte
	longest  int
}

// newMatcher builds, for each value: the value itself; each 32-character
// half, so a value that was cut or split still shows; and the stable core of
// its standard base64 encoding at each of the three byte alignments it can
// have inside a longer encoded blob.
func newMatcher(values []string) *matcher {
	m := &matcher{}
	seen := map[string]bool{}
	add := func(p string) {
		if seen[p] {
			return
		}
		seen[p] = true
		m.patterns = append(m.patterns, []byte(p))
		if len(p) > m.longest {
			m.longest = len(p)
		}
	}
	for _, v := range values {
		add(v)
		add(v[:len(v)/2])
		add(v[len(v)/2:])
		for offset := 0; offset < 3; offset++ {
			add(base64Core(v, offset))
		}
	}
	return m
}

// base64Core returns the part of the standard base64 encoding of value that
// is the same whatever bytes surround it, when value starts at a byte offset
// congruent to offset modulo 3 inside a longer blob. Output character i
// encodes input bits [6i, 6i+6) and value fills bits [8*offset, 8*offset+8n),
// so characters ceil(8*offset/6) up to floor((8*offset+8n)/6) depend on value
// alone; those before depend on the preceding bytes and those after on the
// next byte or the padding.
//
// For a hex value this core is also its URL-safe base64 core. The two
// alphabets differ only in characters 62 and 63, and every 6-bit group lying
// wholly inside lowercase hex bytes (0x30-0x39, 0x61-0x66) is at most 57, so
// the core never holds either character. The self-test checks both.
func base64Core(value string, offset int) string {
	encoded := base64.StdEncoding.EncodeToString(append(make([]byte, offset), value...))
	return encoded[(8*offset+5)/6 : (8*offset+8*len(value))/6]
}

func (m *matcher) match(data []byte) bool {
	for _, p := range m.patterns {
		if bytes.Contains(data, p) {
			return true
		}
	}
	return false
}

// buffer returns a read buffer for one chunk after the overlap window.
func (m *matcher) buffer() []byte { return make([]byte, m.longest-1+scanChunk) }

// scan streams r and reports whether it holds any pattern, and how many bytes
// it read. Each chunk is searched with the last longest-1 bytes of the one
// before: a match that crosses a read boundary has at most that many bytes
// before it, so a value split across two reads is still found. It reads to
// the end even after a hit, so read errors and byte counts stay complete.
func (m *matcher) scan(r io.Reader, buf []byte) (bool, int64, error) {
	keep := m.longest - 1
	hit, carry := false, 0
	var total int64
	for {
		n, err := r.Read(buf[carry : carry+scanChunk])
		total += int64(n)
		end := carry + n
		if n > 0 && !hit {
			hit = m.match(buf[:end])
		}
		carry = end
		if end > keep {
			copy(buf, buf[end-keep:end])
			carry = keep
		}
		if errors.Is(err, io.EOF) {
			return hit, total, nil
		}
		if err != nil {
			return hit, total, err
		}
	}
}

// scanZip scans every entry of a zip archive and its name, calling found for
// each entry with a hit. Nested archives are scanned as bytes only.
func (m *matcher) scanZip(r io.ReaderAt, size int64, buf []byte, found func(name string)) (int, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return 0, fmt.Errorf("unreadable zip archive: %v", err)
	}
	for i, entry := range archive.File {
		content, err := entry.Open()
		if err != nil {
			return i, fmt.Errorf("zip entry %s: %v", m.show(entry.Name), err)
		}
		hit, _, err := m.scan(content, buf)
		content.Close()
		if err != nil {
			return i + 1, fmt.Errorf("zip entry %s: %v", m.show(entry.Name), err)
		}
		if hit || m.match([]byte(entry.Name)) {
			found(entry.Name)
		}
	}
	return len(archive.File), nil
}

// show prints a path or entry name, unless it holds a pattern itself.
func (m *matcher) show(name string) string {
	if m.match([]byte(name)) {
		return "[name withheld: it contains a token pattern]"
	}
	if !utf8.ValidString(name) || strings.ContainsFunc(name, unicode.IsControl) {
		return strconv.Quote(name)
	}
	return name
}

// selfTest proves the matcher finds every form of every value, embedded in
// bytes other than those the patterns were built with, before any evidence
// is reported clean.
func (m *matcher) selfTest(values []string) error {
	buf := m.buffer()
	found := func(sample []byte) bool {
		hit, _, err := m.scan(bytes.NewReader(sample), buf)
		return err == nil && hit
	}
	if found([]byte(strings.Repeat("-", 256))) {
		return errors.New("matcher self-test: a sample without any value matched")
	}
	text := func(s string) []byte { return []byte("evidence line: " + s + " :end\n") }
	type form struct {
		name   string
		sample []byte
	}
	for _, v := range values {
		forms := []form{{"raw value", text(v)}, {"first half", text(v[:len(v)/2])}, {"second half", text(v[len(v)/2:])}}
		for offset := 0; offset < 3; offset++ {
			// The value starts at a byte offset congruent to offset modulo 3,
			// between bytes the patterns were not built with.
			blob := append(append(bytes.Repeat([]byte{0xfb}, 3+offset), v...), 0xff, 0xfe, 0xfd, 0xfc, 0xfb)
			forms = append(forms,
				form{fmt.Sprintf("standard base64 at offset %d", offset), text(base64.StdEncoding.EncodeToString(blob))},
				form{fmt.Sprintf("URL-safe base64 at offset %d", offset), text(base64.URLEncoding.EncodeToString(blob))})
		}
		for _, form := range forms {
			if !found(form.sample) {
				return fmt.Errorf("matcher self-test missed the %s", form.name)
			}
		}
		var archive bytes.Buffer
		w := zip.NewWriter(&archive)
		entry, err := w.CreateHeader(&zip.FileHeader{Name: "trace/network.txt", Method: zip.Deflate})
		if err == nil {
			_, err = entry.Write(text(v))
		}
		if err == nil {
			err = w.Close()
		}
		if err != nil {
			return fmt.Errorf("matcher self-test: build zip sample: %v", err)
		}
		hits := 0
		entries, err := m.scanZip(bytes.NewReader(archive.Bytes()), int64(archive.Len()), buf, func(string) { hits++ })
		if err != nil || entries != 1 || hits != 1 {
			return errors.New("matcher self-test missed a value inside a deflated zip entry")
		}
	}
	return nil
}

type scanner struct {
	m                     *matcher
	buf                   []byte
	out                   io.Writer
	files, entries, leaks int
	bytes                 int64
}

func (s *scanner) leak(where string) {
	fmt.Fprintln(s.out, "leak", where)
	s.leaks++
}

// root walks one evidence directory without following links. A link, a
// device, socket or pipe, or anything unreadable stops the scan, because it
// would leave evidence unscanned.
func (s *scanner) root(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("root %s: %s", s.m.show(root), cause(err))
	}
	if !info.IsDir() {
		return fmt.Errorf("root %s is not a directory", s.m.show(root))
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %s", s.m.show(path), cause(err))
		}
		switch kind := d.Type(); {
		case kind&fs.ModeSymlink != 0:
			return fmt.Errorf("%s is a symbolic link", s.m.show(path))
		case kind.IsDir():
			if s.m.match([]byte(path)) {
				s.leak(s.m.show(path))
			}
			return nil
		case !kind.IsRegular():
			return fmt.Errorf("%s is not a regular file (%s)", s.m.show(path), kind)
		}
		return s.file(path)
	})
}

var zipMagic = [][]byte{[]byte("PK\x03\x04"), []byte("PK\x05\x06")}

// file scans one regular file and, when its content is a zip archive, every
// entry in it. The type is checked again with Lstat and after opening, so a
// file swapped for a link or a pipe is never followed or read.
func (s *scanner) file(path string) error {
	before, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%s: %s", s.m.show(path), cause(err))
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file (%s)", s.m.show(path), before.Mode().Type())
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: %s", s.m.show(path), cause(err))
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%s: %s", s.m.show(path), cause(err))
	}
	if !info.Mode().IsRegular() || !os.SameFile(before, info) {
		return fmt.Errorf("%s changed while it was opened", s.m.show(path))
	}
	head := make([]byte, 4)
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: %s", s.m.show(path), cause(err))
	}
	hit, read, err := s.m.scan(f, s.buf)
	s.files++
	s.bytes += read
	if err != nil {
		return fmt.Errorf("%s: %s", s.m.show(path), cause(err))
	}
	if hit || s.m.match([]byte(path)) {
		s.leak(s.m.show(path))
	}
	isZip := false
	for _, magic := range zipMagic {
		isZip = isZip || (n == len(magic) && bytes.Equal(head, magic))
	}
	if !isZip {
		return nil
	}
	entries, err := s.m.scanZip(f, info.Size(), s.buf, func(name string) { s.leak(s.m.show(path) + "!" + s.m.show(name)) })
	s.entries += entries
	if err != nil {
		return fmt.Errorf("%s: %v", s.m.show(path), err)
	}
	return nil
}

// cause drops the path from a file system error, so a message names a path
// only through show.
func cause(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Op + ": " + pathErr.Err.Error()
	}
	return err.Error()
}
