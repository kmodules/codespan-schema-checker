/*
Copyright 2018 The Kubernetes Authors.

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

package resourcevalidator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
)

type customResourceValidator struct {
	namespaceScoped bool
	kind            schema.GroupVersionKind
	schemaValidator apiextensionsvalidation.SchemaValidator
}

func (a customResourceValidator) Validate(ctx context.Context, obj *unstructured.Unstructured) field.ErrorList {
	if errs := a.ValidateTypeMeta(ctx, obj); len(errs) > 0 {
		return errs
	}

	var allErrs field.ErrorList

	allErrs = append(allErrs, validation.ValidateObjectMetaAccessor(obj, a.namespaceScoped, validation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apiextensionsvalidation.ValidateCustomResource(nil, obj.UnstructuredContent(), a.schemaValidator)...)

	return allErrs
}

func (a customResourceValidator) ValidateTypeMeta(ctx context.Context, obj *unstructured.Unstructured) field.ErrorList {
	typeAccessor, err := meta.TypeAccessor(obj)
	if err != nil {
		return field.ErrorList{field.Invalid(field.NewPath("kind"), nil, err.Error())}
	}

	var allErrs field.ErrorList
	if typeAccessor.GetKind() != a.kind.Kind {
		allErrs = append(allErrs, field.Invalid(field.NewPath("kind"), typeAccessor.GetKind(), fmt.Sprintf("must be %v", a.kind.Kind)))
	}

	apiVersion := a.kind.Version
	if a.kind.Group != "" {
		apiVersion = a.kind.Group + "/" + a.kind.Version
	}
	if typeAccessor.GetAPIVersion() != apiVersion {
		allErrs = append(allErrs, field.Invalid(field.NewPath("apiVersion"), typeAccessor.GetAPIVersion(), fmt.Sprintf("must be %v", a.kind.Group+"/"+a.kind.Version)))
	}
	return allErrs
}
