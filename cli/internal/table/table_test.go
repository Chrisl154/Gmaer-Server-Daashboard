package table

import (
	"strings"
	"testing"
)

func TestNew_HeaderWidths(t *testing.T) {
	tbl := New("ID", "Name", "State")
	want := []int{2, 4, 5}
	for i, w := range want {
		if tbl.widths[i] != w {
			t.Errorf("widths[%d] = %d, want %d", i, tbl.widths[i], w)
		}
	}
}

func TestAddRow_Basic(t *testing.T) {
	tbl := New("ID", "Name")
	tbl.AddRow("1", "Hi")
	if tbl.widths[0] != 2 || tbl.widths[1] != 4 {
		t.Errorf("widths changed unexpectedly: %v", tbl.widths)
	}
}

func TestAddRow_WiderValue(t *testing.T) {
	tbl := New("ID", "Name")
	tbl.AddRow("srv-001", "My Server Name")
	if tbl.widths[0] != 7 {
		t.Errorf("widths[0] = %d, want 7", tbl.widths[0])
	}
	if tbl.widths[1] != 14 {
		t.Errorf("widths[1] = %d, want 14", tbl.widths[1])
	}
}

func TestAddRow_FewerValues(t *testing.T) {
	tbl := New("ID", "Name", "State")
	tbl.AddRow("1")
	row := tbl.rows[0]
	if len(row) != 3 {
		t.Fatalf("row length = %d, want 3", len(row))
	}
	if row[1] != "" || row[2] != "" {
		t.Errorf("missing cells should be empty strings, got %q %q", row[1], row[2])
	}
}

func TestAddRow_ExtraValues(t *testing.T) {
	tbl := New("ID", "Name")
	tbl.AddRow("1", "Alpha", "extra", "also-extra")
	row := tbl.rows[0]
	if len(row) != 2 {
		t.Errorf("row length = %d, want 2 (extras must be dropped)", len(row))
	}
}

func TestRender_EmptyTable(t *testing.T) {
	tbl := New("ID", "Name", "State")
	var buf strings.Builder
	tbl.Render(&buf)

	lines := nonEmpty(strings.Split(buf.String(), "\n"))
	// sep + header + sep + closing sep = 4 non-empty lines
	if len(lines) != 4 {
		t.Errorf("empty table rendered %d non-empty lines, want 4:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[1], "ID") || !strings.Contains(lines[1], "Name") || !strings.Contains(lines[1], "State") {
		t.Errorf("header line missing column names: %q", lines[1])
	}
}

func TestRender_WithRows(t *testing.T) {
	tbl := New("ID", "Name")
	tbl.AddRow("srv-1", "Alpha Server")
	tbl.AddRow("srv-2", "Beta Server")
	var buf strings.Builder
	tbl.Render(&buf)

	out := buf.String()
	for _, want := range []string{"ID", "Name", "srv-1", "Alpha Server", "srv-2", "Beta Server"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestRender_ColumnAlignment(t *testing.T) {
	tbl := New("ID", "Name")
	tbl.AddRow("srv-1", "Short")
	tbl.AddRow("srv-2", "A Much Longer Name")
	var buf strings.Builder
	tbl.Render(&buf)

	lines := nonEmpty(strings.Split(buf.String(), "\n"))
	if len(lines) == 0 {
		t.Fatal("no output lines")
	}
	wantLen := len(lines[0])
	for i, line := range lines {
		if len(line) != wantLen {
			t.Errorf("line %d length = %d, want %d: %q", i, len(line), wantLen, line)
		}
	}
}

func TestPrintMap(t *testing.T) {
	data := map[string]any{
		"host": "localhost",
		"port": 25565,
	}
	var buf strings.Builder
	PrintMap(&buf, data)

	out := buf.String()
	for _, want := range []string{"host", "localhost", "port", "25565"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintMap output missing %q", want)
		}
	}
}

func TestPrintList(t *testing.T) {
	items := []map[string]any{
		{"id": "srv-1", "name": "Alpha"},
		{"id": "srv-2", "name": "Beta"},
	}
	var buf strings.Builder
	PrintList(&buf, items, []string{"id", "name"})

	out := buf.String()
	for _, want := range []string{"srv-1", "Alpha", "srv-2", "Beta"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintList output missing %q", want)
		}
	}
}

// nonEmpty filters out blank strings from a slice.
func nonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
