package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
	"os"
	"path/filepath"
)

func typed(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// testSessionForm builds the modal with two projects, so the project field is
// exercised alongside the rest.
func testSessionForm() *form {
	return newSessionForm([]choice{
		{label: "alpha", value: "p1", help: "/repos/alpha"},
		{label: "beta", value: "p2", help: "/repos/beta"},
	}, 0, true, "claude")
}

// TestFormKeepsWordsThatNameKeys is the regression for the input-eating bug.
//
// Bubble Tea reports the runes of one read as a single KeyRunes message whose
// String() is the typed text, and both our own dispatch and bubbles/textinput
// used to match key bindings against that string. Every word below names a
// key, and each one silently disappeared.
func TestFormKeepsWordsThatNameKeys(t *testing.T) {
	for _, title := range []string{
		"wire up the smoke test",
		"track down the leak",
		"end of quarter report",
		"delete the stale rows",
		"home page redesign",
		"tab through the fields",
		"space out the retries",
	} {
		t.Run(title, func(t *testing.T) {
			f := testSessionForm()
			f.update(typed(title))
			if got := f.fields[sessionFieldTitle].value(); got != title {
				t.Errorf("title = %q, want %q", got, title)
			}
		})
	}
}

// TestFormNavigationStillWorks guards the other side of the same fix: the real
// keys must still navigate, now that typed text no longer does.
func TestFormNavigationStillWorks(t *testing.T) {
	f := testSessionForm()
	// Focus starts on the title, not the project: the project is usually
	// already right and the title always has to be typed.
	if f.index != sessionFieldTitle {
		t.Fatalf("form starts on field %d, want %d", f.index, sessionFieldTitle)
	}

	f.update(tea.KeyMsg{Type: tea.KeyTab})
	if f.index != sessionFieldWorkingCopy {
		t.Errorf("after tab, field = %d, want %d", f.index, sessionFieldWorkingCopy)
	}

	f.update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if f.index != sessionFieldTitle {
		t.Errorf("after shift+tab, field = %d, want %d", f.index, sessionFieldTitle)
	}

	_, _, cancel, _ := f.update(tea.KeyMsg{Type: tea.KeyEsc})
	if !cancel {
		t.Error("esc did not cancel the form")
	}
}

// TestFormChoiceDefaultsToWorktree pins the default working copy. An isolated
// worktree is the safe default: it cannot disturb the project checkout.
func TestFormChoiceDefaultsToWorktree(t *testing.T) {
	f := testSessionForm()
	if got := f.fields[sessionFieldWorkingCopy].value(); got != "worktree" {
		t.Fatalf("default working copy = %q, want worktree", got)
	}

	f.update(tea.KeyMsg{Type: tea.KeyTab})
	f.update(tea.KeyMsg{Type: tea.KeyRight})
	if got := f.fields[sessionFieldWorkingCopy].value(); got != "cwd" {
		t.Errorf("after →, working copy = %q, want cwd", got)
	}
}

// TestFormProjectFieldSelects covers the field that makes ^g n usable from
// inside a running session: the project is a choice, not fixed context.
func TestFormProjectFieldSelects(t *testing.T) {
	f := testSessionForm()
	if got := f.fields[sessionFieldProject].value(); got != "p1" {
		t.Fatalf("default project = %q, want p1", got)
	}

	f.update(tea.KeyMsg{Type: tea.KeyShiftTab}) // title -> project
	if f.index != sessionFieldProject {
		t.Fatalf("shift+tab from title landed on %d, want %d", f.index, sessionFieldProject)
	}
	f.update(tea.KeyMsg{Type: tea.KeyRight})
	if got := f.fields[sessionFieldProject].value(); got != "p2" {
		t.Errorf("after →, project = %q, want p2", got)
	}
}

// TestFormOpensOnRequestedProject checks the caller's default survives, so
// ^g n inside a session preselects that session's project.
func TestFormOpensOnRequestedProject(t *testing.T) {
	f := newSessionForm([]choice{
		{label: "alpha", value: "p1"},
		{label: "beta", value: "p2"},
		{label: "gamma", value: "p3"},
	}, 2, true, "claude")
	if got := f.fields[sessionFieldProject].value(); got != "p3" {
		t.Errorf("preselected project = %q, want p3", got)
	}
}

// TestFormRequiresTitle checks that submitting an empty required field is
// refused and says why, rather than creating a nameless session.
func TestFormRequiresTitle(t *testing.T) {
	f := testSessionForm()
	_, submit, _, _ := f.update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if submit {
		t.Error("empty form submitted")
	}
	if f.problem == "" {
		t.Error("no problem reported for the empty required field")
	}
}

// TestCyclerLinesAlignWhenChoicesOverflow is the regression for a mangled
// chip. The overflow cycler is a three-line bordered box, and it was indented
// with a "  " string prefix — which shifts only the first line, leaving the
// border hanging off to the left.
func TestCyclerLinesAlignWhenChoicesOverflow(t *testing.T) {
	many := make([]choice, 0, 12)
	for _, n := range []string{
		"api-gateway", "billing-service", "content-pipeline", "design-system",
		"edge-router", "feature-flags", "graph-explorer", "image-resizer",
		"job-scheduler", "kv-store", "log-shipper", "mail-relay",
	} {
		many = append(many, choice{label: n, value: n})
	}
	f := newSessionForm(many, 0, true, "claude")
	s := newStyles(paletteFor("cinder"))

	out := renderChoices(s, &f.fields[sessionFieldProject], 60)
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected the bordered cycler, got %d line(s): %q", len(lines), out)
	}

	indent := func(l string) int {
		return len(l) - len(strings.TrimLeft(l, " "))
	}
	first := indent(lines[0])
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if got := indent(l); got != first {
			t.Errorf("line %d is indented %d, line 0 is indented %d — the border is ragged:\n%s",
				i, got, first, out)
		}
	}
}

// TestProjectFieldOpensThePicker covers the escape hatch for a long list:
// stepping with ←/→ does not scale past a handful, so Enter defers to the
// picker instead of advancing the form.
func TestProjectFieldOpensThePicker(t *testing.T) {
	f := testSessionForm()
	f.focus(sessionFieldProject)

	_, submit, cancel, pick := f.update(tea.KeyMsg{Type: tea.KeyEnter})
	if !pick {
		t.Error("enter on the project field did not ask for the picker")
	}
	if submit || cancel {
		t.Error("enter on the project field submitted or cancelled the form")
	}
	if f.index != sessionFieldProject {
		t.Errorf("focus moved to %d; the picker should be shown instead", f.index)
	}
}

// TestNonPickableFieldStillAdvances keeps Enter's ordinary meaning everywhere
// else, so the two-option working-copy field is unaffected.
func TestNonPickableFieldStillAdvances(t *testing.T) {
	f := testSessionForm()
	f.focus(sessionFieldTitle)

	_, _, _, pick := f.update(tea.KeyMsg{Type: tea.KeyEnter})
	if pick {
		t.Error("a non-pickable field asked for the picker")
	}
	if f.index != sessionFieldWorkingCopy {
		t.Errorf("enter left focus at %d, want it advanced to %d", f.index, sessionFieldWorkingCopy)
	}
}

// TestProjectNameFallsBackToTheDirectory covers the sidebar label. The list
// showed filepath.Base of the path because that was the only name a project
// could have; leaving the field empty must still produce that, so an
// unlabelled project is never a blank row.
func TestProjectNameFallsBackToTheDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "api-gateway")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(&store.State{}, "bash", nil)
	next, _ := m.addProject(dir, "", "")
	got := next.(Model).state.Projects

	if len(got) != 1 {
		t.Fatalf("projects = %d, want 1", len(got))
	}
	if got[0].Name != "api-gateway" {
		t.Errorf("name = %q, want the directory name api-gateway", got[0].Name)
	}
}

// TestProjectNameIsWhatYouTyped is the ask: the sidebar shows the label, not
// the directory it happens to live in.
func TestProjectNameIsWhatYouTyped(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "some-checkout-dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(&store.State{}, "bash", nil)
	m.width, m.height = 100, 30
	next, _ := m.addProject(dir, "Payments", "invoices and dunning")
	m = next.(Model)

	if got := m.state.Projects[0].Name; got != "Payments" {
		t.Fatalf("name = %q, want Payments", got)
	}
	list := m.renderProjectList(30, 12)
	if !strings.Contains(list, "Payments") {
		t.Errorf("sidebar does not show the label:\n%s", list)
	}
	if strings.Contains(list, "some-checkout-dir") {
		t.Errorf("sidebar still shows the directory name:\n%s", list)
	}
}

// TestProjectFormReadsItsFieldsByName pins the indices. commitForm reads the
// form positionally, so inserting Name between the path and the description
// would otherwise have stored the name as the description with nothing failing.
func TestProjectFormReadsItsFieldsByName(t *testing.T) {
	f := newProjectForm("/tmp/api-gateway")
	f.fields[projectFieldName].input.SetValue("Payments")
	f.fields[projectFieldDescription].input.SetValue("invoices and dunning")

	if got := f.fields[projectFieldPath].value(); got != "/tmp/api-gateway" {
		t.Errorf("path field = %q", got)
	}
	if got := f.fields[projectFieldName].value(); got != "Payments" {
		t.Errorf("name field = %q", got)
	}
	if got := f.fields[projectFieldDescription].value(); got != "invoices and dunning" {
		t.Errorf("description field = %q", got)
	}
	// The placeholder advertises the fallback, so an empty field is not a
	// blank the user has to guess about.
	if got := f.fields[projectFieldName].input.Placeholder; got != "api-gateway" {
		t.Errorf("name placeholder = %q, want the directory name", got)
	}
}

// TestRenameProjectKeepsTheSameProject is the point of editing by id. A form
// left open while the cursor moves must still write to the row it was opened
// on, not to whatever is selected when ^s lands.
func TestRenameProjectKeepsTheSameProject(t *testing.T) {
	st := &store.State{}
	first := st.AddProject(store.Project{Name: "api-gateway", Path: "/code/api-gateway"})
	st.AddProject(store.Project{Name: "billing-service", Path: "/code/billing-service"})

	m := New(st, "bash", nil)
	m.form = editProjectForm(first)
	m.projectIx = 1 // the cursor moves after the form opens

	next, _ := m.renameProject(m.form.subject, "Gateway", "request routing")
	got := next.(Model).state

	if got.Projects[0].Name != "Gateway" {
		t.Errorf("first project = %q, want Gateway", got.Projects[0].Name)
	}
	if got.Projects[1].Name != "billing-service" {
		t.Errorf("the selected project was renamed instead: %q", got.Projects[1].Name)
	}
	if got.Projects[0].Description != "request routing" {
		t.Errorf("description = %q", got.Projects[0].Description)
	}
}

// TestRenameToEmptyFallsBackToTheDirectory keeps the sidebar from going blank
// when the field is cleared, matching what registering does.
func TestRenameToEmptyFallsBackToTheDirectory(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "Payments", Path: "/code/billing-service"})

	m := New(st, "bash", nil)
	next, _ := m.renameProject(p.ID, "", "")
	if got := next.(Model).state.Projects[0].Name; got != "billing-service" {
		t.Errorf("name = %q, want the directory name billing-service", got)
	}
}

// TestEditProjectFormPrefillsWhatIsThere pins the two fields and their order.
// The rename form has its own indices because it has no path field, and
// sharing the add form's would write the name into the description.
func TestEditProjectFormPrefillsWhatIsThere(t *testing.T) {
	p := &store.Project{
		ID: "p1", Name: "Payments", Path: "/code/billing-service",
		Description: "invoices and dunning",
	}
	f := editProjectForm(p)

	if f.subject != "p1" {
		t.Errorf("subject = %q, want the project id", f.subject)
	}
	if got := f.fields[editFieldName].value(); got != "Payments" {
		t.Errorf("name field = %q", got)
	}
	if got := f.fields[editFieldDescription].value(); got != "invoices and dunning" {
		t.Errorf("description field = %q", got)
	}
	if got := f.fields[editFieldName].input.Placeholder; got != "billing-service" {
		t.Errorf("placeholder = %q, want the directory name", got)
	}
}
