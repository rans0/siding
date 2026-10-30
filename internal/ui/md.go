package ui

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// ponytail: the Markdown subset task docs use (headings, lists, task boxes, tables, fences,
// quotes, rules, code, bold, italics, links). No nested lists or reference links; swap in
// goldmark if docs outgrow it. All text is escaped before any tag is added.
func markdown(src string) template.HTML {
	var b strings.Builder
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); {
		l := lines[i]
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			i++
		case strings.HasPrefix(t, "```"):
			b.WriteString("<pre><code>")
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				b.WriteString(html.EscapeString(lines[i]) + "\n")
			}
			b.WriteString("</code></pre>\n")
			i++
		case heading.MatchString(t):
			m := heading.FindStringSubmatch(t)
			n := string(rune('0' + len(m[1])))
			b.WriteString("<h" + n + ">" + inline(m[2]) + "</h" + n + ">\n")
			i++
		case rule.MatchString(t):
			b.WriteString("<hr>\n")
			i++
		case strings.HasPrefix(t, "|") && i+1 < len(lines) && tableSep.MatchString(strings.TrimSpace(lines[i+1])):
			b.WriteString("<table><thead><tr>")
			for _, c := range cells(t) {
				b.WriteString("<th>" + inline(c) + "</th>")
			}
			b.WriteString("</tr></thead><tbody>\n")
			for i += 2; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				b.WriteString("<tr>")
				for _, c := range cells(strings.TrimSpace(lines[i])) {
					b.WriteString("<td>" + inline(c) + "</td>")
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</tbody></table>\n")
		case item.MatchString(t):
			tag := "ul"
			if item.FindStringSubmatch(t)[1] == "" {
				tag = "ol"
			}
			b.WriteString("<" + tag + ">\n")
			for ; i < len(lines) && item.MatchString(strings.TrimSpace(lines[i])); i++ {
				text := item.FindStringSubmatch(strings.TrimSpace(lines[i]))[3]
				if m := task.FindStringSubmatch(text); m != nil {
					box := "<input type=\"checkbox\" disabled>"
					if m[1] != " " {
						box = "<input type=\"checkbox\" disabled checked>"
					}
					b.WriteString("<li class=\"check\">" + box + " " + inline(m[2]) + "</li>\n")
					continue
				}
				b.WriteString("<li>" + inline(text) + "</li>\n")
			}
			b.WriteString("</" + tag + ">\n")
		case strings.HasPrefix(t, ">"):
			var q []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				q = append(q, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
			}
			b.WriteString("<blockquote>" + inline(strings.Join(q, " ")) + "</blockquote>\n")
		default:
			var p []string
			for ; i < len(lines) && paragraph(lines, i); i++ {
				p = append(p, strings.TrimSpace(lines[i]))
			}
			b.WriteString("<p>" + inline(strings.Join(p, " ")) + "</p>\n")
		}
	}
	return template.HTML(b.String())
}

var (
	heading  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*$`)
	rule     = regexp.MustCompile(`^(-\s*){3,}$|^(\*\s*){3,}$|^(_\s*){3,}$`)
	tableSep = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)
	item     = regexp.MustCompile(`^(?:([-*+])|\d+[.)])(\s+)(.*)$`)
	task     = regexp.MustCompile(`^\[([ xX])\]\s*(.*)$`)
	code     = regexp.MustCompile("`([^`]+)`")
	strong   = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	em       = regexp.MustCompile(`(^|[^\w*])[*_]([^*_\s][^*_]*?)[*_]($|[^\w*])`)
	link     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

// paragraph reports whether line i continues a paragraph: not blank, not another block.
func paragraph(lines []string, i int) bool {
	t := strings.TrimSpace(lines[i])
	return t != "" && !strings.HasPrefix(t, "```") && !heading.MatchString(t) && !item.MatchString(t) &&
		!strings.HasPrefix(t, ">") && !strings.HasPrefix(t, "|") && !rule.MatchString(t)
}

func cells(row string) []string {
	row = strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	out := strings.Split(row, "|")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

// inline escapes s, then marks up code spans (left as is inside), bold, italics and links.
func inline(s string) string {
	parts := code.Split(s, -1)
	spans := code.FindAllStringSubmatch(s, -1)
	var b strings.Builder
	for i, p := range parts {
		p = html.EscapeString(p)
		p = link.ReplaceAllStringFunc(p, func(m string) string {
			g := link.FindStringSubmatch(m)
			if u := html.UnescapeString(g[2]); strings.Contains(u, ":") && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
				return g[1] // no javascript: or other schemes
			}
			return `<a href="` + g[2] + `" rel="noreferrer">` + g[1] + `</a>`
		})
		p = strong.ReplaceAllString(p, "<strong>$1$2</strong>")
		p = em.ReplaceAllString(p, "$1<em>$2</em>$3")
		b.WriteString(p)
		if i < len(spans) {
			b.WriteString("<code>" + html.EscapeString(spans[i][1]) + "</code>")
		}
	}
	return b.String()
}
