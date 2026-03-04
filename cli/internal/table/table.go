// Package table provides a simple ASCII table printer for CLI output.
package table

import (
	"fmt"
	"io"
	"strings"
)

// Table renders a fixed-column ASCII table to an io.Writer.
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

// New creates a Table with the given column headers.
func New(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{headers: headers, widths: widths}
}

// AddRow appends a row to the table. Values are coerced to strings.
// If the row has fewer values than headers, the remaining cells are empty.
// If it has more, the extras are silently dropped.
func (t *Table) AddRow(values ...string) {
	row := make([]string, len(t.headers))
	for i := range t.headers {
		if i < len(values) {
			row[i] = values[i]
		}
		if len(row[i]) > t.widths[i] {
			t.widths[i] = len(row[i])
		}
	}
	t.rows = append(t.rows, row)
}

// Render writes the formatted table to w.
func (t *Table) Render(w io.Writer) {
	sep := t.separator()
	fmt.Fprintln(w, sep)
	fmt.Fprintln(w, t.formatRow(t.headers))
	fmt.Fprintln(w, sep)
	for _, row := range t.rows {
		fmt.Fprintln(w, t.formatRow(row))
	}
	fmt.Fprintln(w, sep)
}

func (t *Table) formatRow(values []string) string {
	parts := make([]string, len(t.headers))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%-*s", t.widths[i], v)
	}
	return "| " + strings.Join(parts, " | ") + " |"
}

func (t *Table) separator() string {
	parts := make([]string, len(t.headers))
	for i, w := range t.widths {
		parts[i] = strings.Repeat("-", w+2)
	}
	return "+" + strings.Join(parts, "+") + "+"
}

// PrintMap renders a single map as a two-column key/value table.
func PrintMap(w io.Writer, data map[string]any) {
	t := New("KEY", "VALUE")
	for k, v := range data {
		t.AddRow(k, fmt.Sprintf("%v", v))
	}
	t.Render(w)
}

// PrintList renders a slice of maps as a table, using the first map's keys as
// column headers (in insertion order is not guaranteed — use PrintListOrdered
// for deterministic columns).
func PrintList(w io.Writer, items []map[string]any, columns []string) {
	t := New(columns...)
	for _, item := range items {
		row := make([]string, len(columns))
		for i, col := range columns {
			if v, ok := item[col]; ok {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		t.AddRow(row...)
	}
	t.Render(w)
}
