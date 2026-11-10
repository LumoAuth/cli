package output

import "testing"

func TestDefaultFormatIsATableOnATerminalAndJSONInAPipe(t *testing.T) {
	cases := []struct {
		format   string
		terminal bool
		want     Format
	}{
		{"", true, FormatTable},
		{"", false, FormatJSON},
		{"auto", true, FormatTable},
		{"auto", false, FormatJSON},
		{" AUTO ", false, FormatJSON},
		// An explicit format is used as given, also in a pipe.
		{"table", false, FormatTable},
		{"json", true, FormatJSON},
		{"YAML", false, FormatYAML},
	}
	for _, tc := range cases {
		got := newPrinter(tc.format, false, tc.terminal).format
		if got != tc.want {
			t.Errorf("newPrinter(%q, terminal=%v) = %q, want %q", tc.format, tc.terminal, got, tc.want)
		}
	}
}
