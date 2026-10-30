package ui

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// md renders GitHub-flavored Markdown. Without goldmark's WithUnsafe, raw HTML is dropped
// and javascript: style links are blanked, so agent-written docs cannot inject script.
var md = goldmark.New(goldmark.WithExtensions(extension.GFM))

func markdown(src string) template.HTML {
	var b bytes.Buffer
	if err := md.Convert([]byte(src), &b); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(b.String())
}
