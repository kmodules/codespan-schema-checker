package resourcevalidator

import (
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/validation"
	openapivalidation "k8s.io/kubectl/pkg/validation"
)

func ValidateSchema(f cmdutil.Factory, obj []byte) error {
	schema := validation.ConjunctiveSchema{
		openapivalidation.NewSchemaValidation(f),
		validation.NoDoubleKeySchema{},
	}
	return schema.ValidateBytes(obj)
}
