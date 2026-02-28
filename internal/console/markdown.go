package console

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
)

// renderMarkdown は Markdown バイト列を HTML バイト列に変換する。
func renderMarkdown(source []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
