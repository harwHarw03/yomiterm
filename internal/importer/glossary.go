package importer

import (
	"encoding/json"
	"strings"
)

// GlossaryView holds extracted definitions, examples, and notes from a Yomitan glossary_json array.
type GlossaryView struct {
	Definitions []string `json:"definitions"`
	Examples    []string `json:"examples,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

// ParseGlossary parses a Yomitan glossary_json string into structured definitions, examples, and notes.
func ParseGlossary(glossaryJSON string) GlossaryView {
	var view GlossaryView
	var items []any
	if err := json.Unmarshal([]byte(glossaryJSON), &items); err != nil {
		if glossaryJSON != "" {
			view.Definitions = append(view.Definitions, glossaryJSON)
		}
		return view
	}

	for _, item := range items {
		switch v := item.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				view.Definitions = append(view.Definitions, trimmed)
			}
		case map[string]any:
			typ, _ := v["type"].(string)
			switch typ {
			case "text":
				if txt, _ := v["text"].(string); strings.TrimSpace(txt) != "" {
					view.Definitions = append(view.Definitions, strings.TrimSpace(txt))
				}
			case "structured-content":
				extractStructuredContent(v["content"], &view, "")
			default:
				// Fallback: try walking content if present
				if c, ok := v["content"]; ok {
					extractStructuredContent(c, &view, "")
				}
			}
		}
	}
	return view
}

func extractStructuredContent(node any, view *GlossaryView, section string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case string:
		t := strings.TrimSpace(n)
		if t != "" {
			appendToSection(view, section, t)
		}
	case []any:
		// If we are inside an inline container or list, check if children are inline text or block items
		if isInlineArray(n) {
			t := strings.TrimSpace(flattenInline(n))
			if t != "" {
				appendToSection(view, section, t)
			}
			return
		}
		for _, child := range n {
			extractStructuredContent(child, view, section)
		}
	case map[string]any:
		tag, _ := n["tag"].(string)
		if dataMap, ok := n["data"].(map[string]any); ok {
			if dc, ok := dataMap["content"].(string); ok && dc != "" {
				section = dc
			}
		}

		// Handle ruby tags inline: 漢字(かんじ)
		if tag == "ruby" || tag == "span" || tag == "a" {
			t := strings.TrimSpace(flattenInline(n))
			if t != "" {
				appendToSection(view, section, t)
			}
			return
		}

		if tag == "li" {
			t := strings.TrimSpace(flattenInline(n["content"]))
			if t != "" {
				appendToSection(view, section, t)
			}
			return
		}

		if tag == "tr" {
			t := strings.TrimSpace(flattenTableRow(n["content"]))
			if t != "" {
				appendToSection(view, section, t)
			}
			return
		}

		if content, ok := n["content"]; ok {
			extractStructuredContent(content, view, section)
		}
	}
}

func appendToSection(view *GlossaryView, section, text string) {
	switch section {
	case "examples", "example":
		view.Examples = append(view.Examples, text)
	case "infoGlossary", "attribution":
		view.Notes = append(view.Notes, text)
	default:
		view.Definitions = append(view.Definitions, text)
	}
}

func isInlineArray(arr []any) bool {
	for _, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		switch tag {
		case "ul", "ol", "li", "div", "p", "table", "tr":
			return false
		}
	}
	return true
}

func flattenInline(node any) string {
	if node == nil {
		return ""
	}
	switch n := node.(type) {
	case string:
		return n
	case []any:
		var b strings.Builder
		for _, child := range n {
			b.WriteString(flattenInline(child))
		}
		return b.String()
	case map[string]any:
		tag, _ := n["tag"].(string)
		if tag == "br" {
			return "\n"
		}
		if tag == "rt" {
			inner := flattenInline(n["content"])
			if inner == "" {
				return ""
			}
			return "(" + inner + ")"
		}
		if tag == "rp" {
			return ""
		}
		if content, ok := n["content"]; ok {
			return flattenInline(content)
		}
	}
	return ""
}

func flattenTableRow(node any) string {
	arr, ok := node.([]any)
	if !ok {
		return flattenInline(node)
	}
	var cells []string
	for _, cell := range arr {
		s := strings.TrimSpace(flattenInline(cell))
		if s != "" {
			cells = append(cells, s)
		}
	}
	return strings.Join(cells, " | ")
}
