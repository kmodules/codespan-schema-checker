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
	"io"
	"os"
	"path/filepath"
	"strings"

	p "kmodules.xyz/client-go/tools/parser"
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
	"gomodules.xyz/logs"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2/klogr"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	kubedbcatalog "kubedb.dev/installer/catalog/kubedb"
	kubevaultcatalog "kubevault.dev/installer/catalog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	stashcatalog "stash.appscode.dev/installer/catalog"
)

type Location struct {
	App     string
	Version string
}

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
	logger = NewLogger(os.Stderr)
	f      cmdutil.Factory
	kc     client.Client

	stashCatalog = map[Location]string{}
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "codespan-schema-checker",
		Short: "Check schema of Kubernetes resources inside markdown code blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, addon := range stashcatalog.Load().Addons {
				app := strings.ReplaceAll(addon.Name, "-", "")
				for _, v := range addon.Versions {
					stashCatalog[Location{
						App:     app,
						Version: toVersion(app, v),
					}] = v
				}
			}

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

			return logger.Result()
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
	cfg, _ := f.ToRESTConfig()
	cfg.QPS = 1000
	cfg.Burst = 1000
	kc = MustClient(cfg)

	flags.AddGoFlagSet(flag.CommandLine)

	flags.StringVar(&filename, "content", filename, "Path to directory where markdown files reside")

	logs.Init(rootCmd, false)

	utilruntime.Must(rootCmd.Execute())
}

func NewClient(cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	log.SetLogger(klogr.New())
	// cfg := ctrl.GetConfigOrDie()
	// cfg.QPS = 100
	// cfg.Burst = 100

	hc, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(cfg, hc)
	if err != nil {
		return nil, err
	}

	return client.New(cfg, client.Options{
		Scheme: scheme,
		Mapper: mapper,
		//Opts: client.WarningHandlerOptions{
		//	SuppressWarnings:   false,
		//	AllowDuplicateLogs: false,
		//},
	})
}

func MustClient(cfg *rest.Config) client.Client {
	kc, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}
	return kc
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
		if err != nil && !runtime.IsMissingKind(err) && !p.IsYAMLSyntaxError(err) {
			// err
			logger.Log(err)
		}
	}
	return ast.WalkContinue, nil
}

type codeExtractor struct{}

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
		logger.Init(path)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		err = p.ProcessResources(content, checkObject)
		if err != nil && !runtime.IsMissingKind(err) && !p.IsYAMLSyntaxError(err) {
			return err
		}
	} else if ext == ".md" && filepath.Base(path) != "_index.md" {
		logger.Init(path)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(content)
		page, err := hp.ParseFrontMatterAndContent(buf)
		if err != nil {
			return err
		}
		err = md.Convert(page.Content, io.Discard)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkObject(ri p.ResourceInfo) error {
	obj := ri.Object

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	err = resourcevalidator.ValidateSchema(f, data)
	if err != nil {
		logger.Log(err)
		return nil
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := kc.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		logger.Log(err)
		return nil
	}
	gvr := mapping.Resource

	var crdSchema *v1.CustomResourceValidation

	var crd v1.CustomResourceDefinition
	crdName := fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group)
	err = kc.Get(context.TODO(), client.ObjectKey{Name: crdName}, &crd)
	if apierrors.IsNotFound(err) {
		// get resource descriptor from groupVersionResource
		rd, err := reg.LoadByGVR(gvr)
		if err != nil {
			logger.Log(err)
		}
		crdSchema = rd.Spec.Validation
	} else if err != nil {
		logger.Log(err)
		return nil
	} else {
		for _, v := range crd.Spec.Versions {
			if v.Name == gvk.Version && v.Schema != nil {
				crdSchema = v.Schema
			}
		}
	}

	if crdSchema != nil {
		validator, err := resourcevalidator.New(mapping.Scope.Name() == meta.RESTScopeNameNamespace, gvk, crdSchema)
		if err != nil {
			return err
		}
		errList := validator.Validate(context.TODO(), obj)
		if len(errList) > 0 {
			logger.Log(errList.ToAggregate())
			return nil
		}
	}

	if gvr.Group == "kubedb.com" {
		dbVersion, _, err := unstructured.NestedString(obj.Object, "spec", "version")
		if err != nil {
			logger.Log(err)
			return nil
		}
		kind := obj.GetKind()
		if kind == "RedisSentinel" {
			kind = "Redis"
		}
		if dbVersion != "" && !sets.NewString(kubedbcatalog.ActiveDBVersions()[kind]...).Has(dbVersion) {
			logger.Log(fmt.Errorf("using unknown %s version %s", obj.GetKind(), dbVersion))
			return nil
		}
	} else if gvr.Group == "catalog.kubedb.com" {
		if !sets.NewString(kubedbcatalog.ActiveDBVersions()[strings.TrimSuffix(obj.GetKind(), "Version")]...).Has(obj.GetName()) {
			logger.Log(fmt.Errorf("using unknown %s version %s", obj.GetKind(), obj.GetName()))
			return nil
		}
		backupTask, _, _ := unstructured.NestedString(obj.Object, "spec", "stash", "addon", "backupTask", "name")
		if err := checkStashTaskName(backupTask); err != nil {
			logger.Log(err)
			return nil
		}
		restoreTask, _, _ := unstructured.NestedString(obj.Object, "spec", "stash", "addon", "restoreTask", "name")
		if err := checkStashTaskName(restoreTask); err != nil {
			logger.Log(err)
			return nil
		}
	} else if gvr.Group == "stash.appscode.com" {
		taskName, _, err := unstructured.NestedString(obj.Object, "spec", "task", "name")
		if err != nil {
			logger.Log(err)
			return nil
		}
		if err := checkStashTaskName(taskName); err != nil {
			logger.Log(err)
			return nil
		}
	} else if gvr.Group == "kubevault.com" {
		appVersion, _, err := unstructured.NestedString(obj.Object, "spec", "version")
		if err != nil {
			logger.Log(err)
			return nil
		}
		if appVersion != "" && !sets.NewString(kubevaultcatalog.ActiveVersions()[obj.GetKind()]...).Has(appVersion) {
			logger.Log(fmt.Errorf("using unknown %s version %s", obj.GetKind(), appVersion))
			return nil
		}
	} else if gvr.Group == "catalog.kubevault.com" {
		if !sets.NewString(kubevaultcatalog.ActiveVersions()[strings.TrimSuffix(obj.GetKind(), "Version")]...).Has(obj.GetName()) {
			logger.Log(fmt.Errorf("using unknown %s version %s", obj.GetKind(), obj.GetName()))
			return nil
		}
	}
	return nil
}

func checkStashTaskName(taskName string) error {
	if taskName != "" && !strings.Contains(taskName, "{{<") {
		parts := strings.SplitN(taskName, "-", 3)
		if len(parts) != 3 {
			return nil // pvc-backup
		}
		loc := Location{
			App:     parts[0],
			Version: parts[2],
		}
		if _, ok := stashCatalog[loc]; !ok {
			return fmt.Errorf("%+v has no matching image tag for task %s", loc, taskName)
		}
	}
	return nil
}

func toVersion(app, v string) string {
	idx := strings.IndexRune(v, '-')
	if idx == -1 {
		return v
	}
	v2 := v[:idx]

	if app == "postgres" {
		if strings.HasPrefix(v2, "9.6.") {
			return v2
		}
		parts := strings.Split(v2, ".")
		return parts[0] + "." + parts[1]
	}
	return v2
}
