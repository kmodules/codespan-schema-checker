package resourcevalidator

import (
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/validation"
)

func ValidateSchema(f cmdutil.Factory, obj []byte) error {
	schema := validation.ConjunctiveSchema{
		validation.NewSchemaValidation(f),
		validation.NoDoubleKeySchema{},
	}
	return schema.ValidateBytes(obj)
}
