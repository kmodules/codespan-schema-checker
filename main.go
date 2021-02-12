/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func main() {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithExtensions(Strikethrough),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(sample), &buf); err != nil {
		panic(err)
	}
	fmt.Println(buf.String())
}

// CodeExtractor is a renderer.NodeRenderer implementation that
// renders Strikethrough nodes.
type CodeExtractor struct {
	html.Config
}

// NewCodeExtractor returns a new CodeExtractor.
func NewCodeExtractor(opts ...html.Option) renderer.NodeRenderer {
	r := &CodeExtractor{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *CodeExtractor) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeBlock, r.extractCode)
	reg.Register(ast.KindFencedCodeBlock, r.extractCode)
}

func (r *CodeExtractor) extractCode(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<pre><code>")
		r.writeLines(w, source, n)
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

func (r *CodeExtractor) writeLines(w util.BufWriter, source []byte, n ast.Node) {
	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		fmt.Println(line.Start, line.Stop, line.Padding)

		r.Writer.RawWrite(w, line.Value(source))
	}
}

type codeExtractor struct {
}

// Strikethrough is an extension that allow you to use codeExtractor expression like '~~text~~' .
var Strikethrough = &codeExtractor{}

func (e *codeExtractor) Extend(m goldmark.Markdown) {
	//m.Parser().AddOptions(parser.WithInlineParsers(
	//	util.Prioritized(NewStrikethroughParser(), 500),
	//))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewCodeExtractor(), 10),
	))
}
