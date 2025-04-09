/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	context "context"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	applyconfigurationsappsv1beta1 "k8s.io/client-go/applyconfigurations/apps/v1beta1"
	gentype "k8s.io/client-go/gentype"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

// DeploymentsGetter has a method to return a DeploymentInterface.
// A group's client should implement this interface.
type DeploymentsGetter interface {
	Deployments(namespace string) DeploymentInterface
}

// DeploymentInterface has methods to work with Deployment resources.
type DeploymentInterface interface {
	Create(ctx context.Context, deployment *appsv1beta1.Deployment, opts v1.CreateOptions) (*appsv1beta1.Deployment, error)
	Update(ctx context.Context, deployment *appsv1beta1.Deployment, opts v1.UpdateOptions) (*appsv1beta1.Deployment, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, deployment *appsv1beta1.Deployment, opts v1.UpdateOptions) (*appsv1beta1.Deployment, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*appsv1beta1.Deployment, error)
	List(ctx context.Context, opts v1.ListOptions) (*appsv1beta1.DeploymentList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *appsv1beta1.Deployment, err error)
	Apply(ctx context.Context, deployment *applyconfigurationsappsv1beta1.DeploymentApplyConfiguration, opts v1.ApplyOptions) (result *appsv1beta1.Deployment, err error)
	// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
	ApplyStatus(ctx context.Context, deployment *applyconfigurationsappsv1beta1.DeploymentApplyConfiguration, opts v1.ApplyOptions) (result *appsv1beta1.Deployment, err error)
	DeploymentExpansion
}

// deployments implements DeploymentInterface
type deployments struct {
	*gentype.ClientWithListAndApply[*appsv1beta1.Deployment, *appsv1beta1.DeploymentList, *applyconfigurationsappsv1beta1.DeploymentApplyConfiguration]
}

// newDeployments returns a Deployments
func newDeployments(c *AppsV1beta1Client, namespace string) *deployments {
	return &deployments{
		gentype.NewClientWithListAndApply[*appsv1beta1.Deployment, *appsv1beta1.DeploymentList, *applyconfigurationsappsv1beta1.DeploymentApplyConfiguration](
			"deployments",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *appsv1beta1.Deployment { return &appsv1beta1.Deployment{} },
			func() *appsv1beta1.DeploymentList { return &appsv1beta1.DeploymentList{} },
			gentype.PrefersProtobuf[*appsv1beta1.Deployment](),
		),
	}
}
