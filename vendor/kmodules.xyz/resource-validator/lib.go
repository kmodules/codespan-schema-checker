package resourcevalidator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiservervalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Validator interface {
	Validate(ctx context.Context, obj *unstructured.Unstructured) field.ErrorList
	ValidateTypeMeta(ctx context.Context, obj *unstructured.Unstructured) field.ErrorList
}

func New(namespaceScoped bool, kind schema.GroupVersionKind, validationSchema *apiextensionsv1.CustomResourceValidation) (Validator, error) {
	var internal *apiextensions.CustomResourceValidation
	if validationSchema != nil {
		internal = &apiextensions.CustomResourceValidation{}
		if err := apiextensionsv1.Convert_v1_CustomResourceValidation_To_apiextensions_CustomResourceValidation(validationSchema, internal, nil); err != nil {
			return nil, fmt.Errorf("failed to convert CRD validation to internal version: %v", err)
		}
	}
	schemaValidator, _, err := apiservervalidation.NewSchemaValidator(internal.OpenAPIV3Schema)
	if err != nil {
		return nil, err
	}

	return &customResourceValidator{
		namespaceScoped: namespaceScoped,
		kind:            kind,
		schemaValidator: schemaValidator,
	}, nil
}
