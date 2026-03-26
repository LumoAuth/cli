package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Format represents an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Printer handles output formatting.
type Printer struct {
	format Format
	quiet  bool
}

// NewPrinter creates a new output printer.
func NewPrinter(format string, quiet bool) *Printer {
	f := Format(strings.ToLower(format))

	// Auto-detect: if not a TTY, default to JSON for piping
	if f == FormatTable && !isTerminal() {
		f = FormatJSON
	}

	return &Printer{format: f, quiet: quiet}
}

// PrintResult outputs data in the configured format.
// In table mode, auto-detects whether data is an object (key-value) or array (table rows).
func (p *Printer) PrintResult(data interface{}) {
	switch p.format {
	case FormatJSON:
		p.printJSON(data)
	case FormatYAML:
		p.printYAML(data)
	default:
		p.printTableAuto(data)
	}
}

// printTableAuto renders JSON data as a human-readable table.
// For objects: key-value pairs. For arrays of objects: columnar table.
func (p *Printer) printTableAuto(data interface{}) {
	// If data is json.RawMessage, parse it first
	raw, isRaw := data.(json.RawMessage)
	if isRaw {
		var parsed interface{}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			p.printJSON(data)
			return
		}
		data = parsed
	}

	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this is an API response with "data" key
		if inner, ok := v["data"]; ok {
			p.printTableAuto(inner)
			// Print pagination if meta exists
			if meta, ok := v["meta"].(map[string]interface{}); ok {
				total, _ := meta["total"].(float64)
				page, _ := meta["page"].(float64)
				totalPages, _ := meta["totalPages"].(float64)
				if total > 0 {
					p.PrintPagination(int(total), int(page), int(totalPages))
				}
			}
			return
		}
		// Single object — render as key-value table
		p.printKeyValue(v)

	case []interface{}:
		if len(v) == 0 {
			fmt.Println("No results found.")
			return
		}
		// Array of objects — render as columnar table
		if first, ok := v[0].(map[string]interface{}); ok {
			p.printObjectArray(first, v)
		} else {
			// Array of primitives
			for _, item := range v {
				fmt.Println(formatValue(item))
			}
		}

	default:
		// Primitive value
		fmt.Println(formatValue(data))
	}
}

// printKeyValue renders a single object as aligned key-value pairs.
func (p *Printer) printKeyValue(obj map[string]interface{}) {
	// Collect keys and sort them
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find max key length for alignment
	maxKey := 0
	for _, k := range keys {
		label := toLabel(k)
		if len(label) > maxKey {
			maxKey = len(label)
		}
	}

	for _, k := range keys {
		label := toLabel(k)
		val := obj[k]
		valStr := formatValue(val)
		fmt.Printf("%-*s  %s\n", maxKey, label+":", valStr)
	}
}

// printObjectArray renders an array of objects as a columnar table.
func (p *Printer) printObjectArray(first map[string]interface{}, items []interface{}) {
	// Determine columns from the first object
	headers := make([]string, 0, len(first))
	for k := range first {
		headers = append(headers, k)
	}
	sort.Strings(headers)

	// Build rows
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = formatValue(obj[h])
		}
		rows = append(rows, row)
	}

	// Convert headers to uppercase labels
	labels := make([]string, len(headers))
	for i, h := range headers {
		labels[i] = strings.ToUpper(toLabel(h))
	}

	p.PrintTable(labels, rows)
}

// PrintTable prints data as a formatted table.
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	if p.format == FormatJSON || p.format == FormatYAML {
		// Convert table to a list of maps for structured output
		result := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			m := make(map[string]string)
			for i, h := range headers {
				if i < len(row) {
					m[h] = row[i]
				}
			}
			result = append(result, m)
		}
		p.PrintResult(result)
		return
	}

	if len(rows) == 0 {
		fmt.Println("No results found.")
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerLine := ""
	separatorLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "  "
			separatorLine += "  "
		}
		headerLine += fmt.Sprintf("%-*s", widths[i], strings.ToUpper(h))
		separatorLine += strings.Repeat("─", widths[i])
	}
	fmt.Println(headerLine)
	fmt.Println(separatorLine)

	// Print rows
	for _, row := range rows {
		line := ""
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if i > 0 {
				line += "  "
			}
			line += fmt.Sprintf("%-*s", widths[i], cell)
		}
		fmt.Println(line)
	}
}

// PrintSuccess prints a success message (suppressed in quiet mode).
func (p *Printer) PrintSuccess(msg string) {
	if p.quiet {
		return
	}
	if p.format == FormatJSON {
		p.printJSON(map[string]string{"status": "success", "message": msg})
		return
	}
	fmt.Printf("✓ %s\n", msg)
}

// PrintError prints an error message.
func (p *Printer) PrintError(err error) {
	if p.format == FormatJSON {
		p.printJSON(map[string]interface{}{
			"error":   true,
			"message": err.Error(),
		})
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
}

// PrintPagination prints pagination info.
func (p *Printer) PrintPagination(total, page, totalPages int) {
	if p.quiet || p.format == FormatJSON || p.format == FormatYAML {
		return
	}
	if total > 0 && totalPages > 0 {
		fmt.Printf("\nShowing page %d of %d (%d total)\n", page, totalPages, total)
	}
}

// IsJSON returns true if the output format is JSON.
func (p *Printer) IsJSON() bool {
	return p.format == FormatJSON
}

// IsTable returns true if the output format is table.
func (p *Printer) IsTable() bool {
	return p.format == FormatTable
}

func (p *Printer) printJSON(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func (p *Printer) printYAML(data interface{}) {
	// Convert through JSON to ensure consistent field names
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling data: %v\n", err)
		return
	}
	var generic interface{}
	json.Unmarshal(jsonData, &generic)
	out, err := yaml.Marshal(generic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling YAML: %v\n", err)
		return
	}
	fmt.Print(string(out))
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// toLabel converts a camelCase or snake_case key to a human-readable label.
func toLabel(key string) string {
	// Handle snake_case
	key = strings.ReplaceAll(key, "_", " ")

	// Insert spaces before uppercase letters (camelCase → camel Case)
	var result strings.Builder
	for i, r := range key {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(key[i-1])
			if prev >= 'a' && prev <= 'z' {
				result.WriteRune(' ')
			}
		}
		result.WriteRune(r)
	}

	// Title case each word
	words := strings.Fields(result.String())
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// formatValue converts an interface value to a display string.
func formatValue(v interface{}) string {
	if v == nil {
		return "—"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%.2f", val)
	case string:
		if val == "" {
			return "—"
		}
		return val
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return strings.Join(parts, ", ")
	case map[string]interface{}:
		// Nested object — show as compact JSON
		b, _ := json.Marshal(val)
		s := string(b)
		if len(s) > 60 {
			return s[:57] + "..."
		}
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}
