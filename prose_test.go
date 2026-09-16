package main

// Prose is the one place in this repository with no compiler. An identifier
// spelled wrong in code fails the build; the same mistake in a doc comment, a
// README table or an architecture note passes every gate and is read as fact by
// the next person. This test closes that gap for the half of it a machine can
// judge: whether a name written in prose exists at all.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// camelCase is the shape that distinguishes a Go identifier from an English
// word without needing a dictionary. sessionLine matches; "focus" and "Session"
// do not, and neither does ordinary prose — which is what keeps the check quiet
// enough to be worth having. Single-word identifiers are out of reach for the
// same reason, and that is the accepted cost.
//
// Every capital must be followed by lowercase. That is what separates a Go name
// from an acronym: this repository writes several of those often, and a check
// that flagged them would be turned off within a week.
var camelCase = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:[A-Z][a-z0-9]+)+\b`)

// backticked pulls the code spans out of markdown. A name outside them is
// English, and a name inside one is a claim about this tree.
var backticked = regexp.MustCompile("`([^`]+)`")

// miss is one name, in one file, that resolves to nothing.
type miss struct{ file, name string }

// scanProse reports every name written in prose under root that is not an
// identifier somewhere in root's Go sources.
//
// It reads names out of the syntax tree rather than out of the file text. Text
// would include the comments themselves, so a misspelling written twice would
// vouch for itself and the check would pass on exactly the case it exists to
// catch.
//
// Taking a root rather than walking the working directory is what lets the
// planted-misspelling test run the real scanner over a fixture, instead of the
// discrimination being something a human checked once and wrote down.
func scanProse(t *testing.T, root string) []miss {
	t.Helper()

	var goFiles, mdFiles []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "assets") {
			return filepath.SkipDir
		}
		switch filepath.Ext(path) {
		case ".go":
			goFiles = append(goFiles, path)
		case ".md":
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	// Without this the whole check passes by finding nothing — a walk that
	// starts in the wrong place, or a skip rule that grows too broad, would
	// turn into a green test rather than a red one.
	if len(goFiles) == 0 {
		t.Fatalf("no Go sources under %s; the walk is wrong, not the tree", root)
	}

	fset := token.NewFileSet()
	inCode := map[string]bool{}
	comments := map[string][]string{}          // file -> comment text
	inLiterals := map[string]map[string]bool{} // file -> names inside its own string literals

	for _, path := range goFiles {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		inLiterals[path] = map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Ident:
				inCode[v.Name] = true
			case *ast.BasicLit:
				// A name a file only ever writes into a string is still a name
				// that file's comments may explain: the SVG attributes ansisvg
				// emits are not Go symbols, and its comments discuss them by
				// name. Scoped to the file, so one literal cannot vouch for
				// prose anywhere else in the tree.
				for _, name := range camelCase.FindAllString(v.Value, -1) {
					inLiterals[path][name] = true
				}
			}
			return true
		})
		for _, g := range f.Comments {
			comments[path] = append(comments[path], g.Text())
		}
	}

	// A name that prefixes a real identifier is naming a family rather than a
	// symbol — "workingCopy indices" heads workingCopyWorktree and
	// workingCopyDir. Accepting it keeps the check honest about what it can
	// know, and costs little: a misspelling is not a prefix of what it meant.
	prefixes := func(name string) bool {
		for known := range inCode {
			if len(known) > len(name) && strings.HasPrefix(known, name) {
				return true
			}
		}
		return false
	}

	// A name is reported once per file that spells it, so a fix lands in one
	// place rather than being chased through a list of repeats.
	var missing []miss
	seen := map[miss]bool{}
	note := func(file, name string) {
		m := miss{file, name}
		if inCode[name] || inLiterals[file][name] || seen[m] || prefixes(name) {
			return
		}
		seen[m] = true
		missing = append(missing, m)
	}

	for path, texts := range comments {
		for _, text := range texts {
			for _, name := range camelCase.FindAllString(text, -1) {
				note(path, name)
			}
		}
	}

	for _, path := range mdFiles {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, span := range backticked.FindAllStringSubmatch(string(body), -1) {
			// A qualified name arrives as ui.coordArgs or gitx.Diff; the
			// package half is not declared in this module's own trees, so only
			// the selector is checked.
			for _, name := range camelCase.FindAllString(span[1], -1) {
				note(path, name)
			}
		}
	}

	sort.Slice(missing, func(i, j int) bool {
		if missing[i].file != missing[j].file {
			return missing[i].file < missing[j].file
		}
		return missing[i].name < missing[j].name
	})
	return missing
}

// TestProseNamesIdentifiersThatExist is the regression for a class of defect
// this repository kept producing: a comment or a document naming a function
// that is not there.
//
// What it cannot judge is whether a claim about a real symbol is true —
// "sessionLine draws no marker" names something that exists and was wrong
// anyway. That half stays a reading job.
//
// It reads every markdown file on disk, not only the tracked ones, so an
// uncommitted note is checked here and not in CI. That is deliberate: working
// instructions go stale the same way published ones do, and a local-only
// failure is still a true one.
func TestProseNamesIdentifiersThatExist(t *testing.T) {
	for _, m := range scanProse(t, ".") {
		t.Errorf("%s names %q, which is not an identifier anywhere in this module",
			m.file, m.name)
	}
}

// TestProseScanReportsAPlantedMisspelling proves the scanner discriminates.
//
// Without it the test above is a green light of unknown value: it passes on a
// clean tree whether or not it can see anything. This runs the real scanner
// over a fixture whose comment names a function that does not exist.
//
// The fixture is written to a temporary directory rather than committed. A
// committed one would put the misspelling into this module's own sources, where
// it would resolve — and the commit-msg hook, which greps the tree rather than
// parsing it, would then treat the typo as a real symbol forever.
func TestProseScanReportsAPlantedMisspelling(t *testing.T) {
	dir := t.TempDir()
	const sample = `package sample

// jumpToVerandah is named here and declared nowhere.
func Real() {}
`
	// A markdown fixture too, because the two halves read different things —
	// one parses comments out of a syntax tree, the other pulls code spans out
	// of text. A single fixture would leave whichever half broke still green.
	const notes = "# Sample\n\nThe `openTheVerandah` helper does the thing.\n"

	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, m := range scanProse(t, dir) {
		got = append(got, m.name)
	}
	sort.Strings(got)
	if want := "jumpToVerandah,openTheVerandah"; strings.Join(got, ",") != want {
		t.Fatalf("scan reported %v, want both planted names", got)
	}
}

// TestCamelCaseMatchesNamesNotAcronyms covers the matcher every other check is
// built on, including the rule that made it usable. An acronym is not a Go
// name, and a check that reported one on every run is a check somebody deletes.
func TestCamelCaseMatchesNamesNotAcronyms(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"jumpToSession moves the cursor", []string{"jumpToSession"}},
		{"sessionLine and cursorMarker", []string{"sessionLine", "cursorMarker"}},
		{"resolved on macOS only", nil},
		{"the sRGB ramp", nil},
		{"focus and Session on their own", nil},
	} {
		got := camelCase.FindAllString(tc.in, -1)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("on %q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}
