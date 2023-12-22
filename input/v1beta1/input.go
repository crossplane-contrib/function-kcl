// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=template.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"kcl-lang.io/crossplane-kcl/pkg/resource"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// KCLInput can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type KCLInput struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RunSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

func (in KCLInput) Validate() error {
	if in.Spec.Source == "" {
		return field.Required(field.NewPath("spec.source"), "kcl source cannot be empty")
	}

	switch in.Spec.Target {
	// Allowed targets
	case resource.PatchDesired, resource.Resources, resource.XR:
	case resource.PatchResources:
		if len(in.Spec.Resources) == 0 {
			return field.Required(field.NewPath("spec.Resources"), fmt.Sprintf("%s target requires at least one resource", resource.PatchResources))
		}

		for i, r := range in.Spec.Resources {
			if r.Name == "" {
				return field.Required(field.NewPath("spec.Resources").Index(i).Child("name"), "name cannot be empty")
			}
			if r.Base == nil {
				return field.Required(field.NewPath("spec.Resources").Index(i).Child("base"), "base cannot be empty")
			}
		}
	default:
		return field.Required(field.NewPath("spec.target"), fmt.Sprintf("invalid target: %s", in.Spec.Target))
	}

	return nil
}

// RunSpec defines the desired state of Crossplane KCL function.
type RunSpec struct {
	// Source is a required field for providing a KCL script inline.
	Source string `json:"source" yaml:"source"`
	// Params are the parameters in key-value pairs format.
	Params map[string]runtime.RawExtension `json:"params,omitempty" yaml:"params,omitempty"`
	// Resources is a list of resources to patch and create
	// This is utilized when a Target is set to PatchResources
	Resources ResourceList `json:"resources,omitempty"`
	// Target determines what object the export output should be applied to
	// +kubebuilder:default:=Resources
	// +kubebuilder:validation:Enum:=PatchDesired;PatchResources;Resources;XR
	Target resource.Target `json:"target"`
}

type ResourceList []Resource

type Resource struct {
	// Name is a unique identifier for this entry in a ResourceList
	Name string `json:"name"`
	// Base of the composed resource that patches will be applied to.
	// According to the patches and transforms functions, this may be ommited on
	// occassion by a previous pipeline
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	// +optional
	Base *runtime.RawExtension `json:"base,omitempty"`
}
