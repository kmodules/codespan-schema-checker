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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"kmodules.xyz/client-go/logs"
	p "kmodules.xyz/client-go/tools/parser"
	"kmodules.xyz/resource-metadata/apis/meta/v1alpha1"
	"kmodules.xyz/resource-metadata/hub"
	resourcevalidator "kmodules.xyz/resource-validator"

	hp "github.com/gohugoio/hugo/parser/pageparser"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cliflag "k8s.io/component-base/cli/flag"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var (
	filename string
	reg      = hub.NewRegistryOfKnownResources()
	md       = goldmark.New(
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
	f cmdutil.Factory
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "build input_dir out_dir",
		Short: "Build yamls",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := os.Stat(filename)
			if os.IsNotExist(err) {
				return err
			}
			if info.IsDir() {
				err = filepath.Walk(filename, check)
				if err != nil {
					return err
				}
			} else {
				err = check(filename, info, nil)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}
	flags := rootCmd.Flags()
	// Normalize all flags that are coming from other packages or pre-configurations
	// a.k.a. change all "_" to "-". e.g. glog package
	flags.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(flags)
	f = cmdutil.NewFactory(matchVersionKubeConfigFlags)

	flags.AddGoFlagSet(flag.CommandLine)

	flags.StringVar(&filename, "content", "/personal/AppsCode/websites/kubedb/content/docs/v2021.01.26/guides/mongodb/quickstart/quickstart.md", "Path to directory where markdown files reside")

	logs.ParseFlags()

	utilruntime.Must(rootCmd.Execute())
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

func (r *CodeExtractor) extractCode(_ util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		var buf bytes.Buffer
		l := n.Lines().Len()
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			buf.Write(line.Value(source))
		}
		err := p.ProcessResources(buf.Bytes(), checkObject)
		if err != nil && !runtime.IsMissingKind(err) && !IsYAMLSyntaxError(err) {
			_, _ = fmt.Fprintln(os.Stderr, err)
			// fmt.Println(err)
			// return err
		}
	}
	return ast.WalkContinue, nil
}

func (r *CodeExtractor) writeLines(w util.BufWriter, source []byte, n ast.Node) {
	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
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

func check(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	ext := filepath.Ext(info.Name())
	if ext == ".yaml" || ext == ".yml" || ext == ".json" {
		fmt.Println("_____________________________________________________________________________________________")
		fmt.Println(path)
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		err = p.ProcessResources(content, checkObject)
		if err != nil && !runtime.IsMissingKind(err) && !IsYAMLSyntaxError(err) {
			return err
		}
	} else if ext == ".md" && filepath.Base(path) != "_index.md" {
		fmt.Println("_____________________________________________________________________________________________")
		fmt.Println(path)
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(content)
		page, err := hp.ParseFrontMatterAndContent(buf)
		if err != nil {
			return err
		}
		err = md.Convert(page.Content, ioutil.Discard)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkObject(obj *unstructured.Unstructured) error {
	gvr, err := reg.GVR(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return err
	}
	rd, err := reg.LoadByGVR(gvr)
	if err != nil {
		return err
	}
	if v1alpha1.IsOfficialType(rd.Spec.Resource.Group) {
		return nil
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	err = resourcevalidator.ValidateSchema(f, data)
	if err != nil {
		return err
	}

	if rd.Spec.Validation != nil {
		validator, err := resourcevalidator.New(rd.Spec.Resource.Scope == v1alpha1.NamespaceScoped, schema.GroupVersionKind{
			Group:   rd.Spec.Resource.Group,
			Version: rd.Spec.Resource.Version,
			Kind:    rd.Spec.Resource.Kind,
		}, rd.Spec.Validation)
		if err != nil {
			return err
		}
		errList := validator.Validate(context.TODO(), obj)
		if len(errList) > 0 {
			// err
			fmt.Fprintln(os.Stderr, errList.ToAggregate().Error())
		}
	}
	return nil
}

func IsYAMLSyntaxError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(yaml.YAMLSyntaxError)
	return ok
}
