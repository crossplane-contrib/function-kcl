// Package v1alpha1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=krm.kcl.dev
// +versionName=v1alpha1
package v1alpha1

import (
	"fmt"

	"github.com/crossplane-contrib/function-kcl/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

func (in *KCLInput) Validate() error {
	if in.Spec.Source == "" {
		return field.Required(field.NewPath("spec.source"), "kcl source cannot be empty")
	}

	switch in.Spec.Target {
	// Allowed targets
	case resource.Default, resource.PatchDesired, resource.Resources, resource.XR:
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
		in.Spec.Target = resource.Default
	}

	return nil
}

// RunSpec defines the desired state of Crossplane KCL function.
type RunSpec struct {
	// Source is a required field for providing a KCL script inline.
	Source string `json:"source" yaml:"source"`
	// Config is the compile config.
	Config ConfigSpec `json:"config,omitempty" yaml:"config,omitempty"`
	// Credentials for remote locations
	Credentials CredSpec `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	// Dependencies are the external dependencies for the KCL code.
	// The format of the `dependencies` field is same as the `[dependencies]` in the `kcl.mod` file
	Dependencies string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	// Params are the parameters in key-value pairs format.
	Params map[string]runtime.RawExtension `json:"params,omitempty" yaml:"params,omitempty"`
	// Resources is a list of resources to patch and create
	// This is utilized when a Target is set to PatchResources
	Resources ResourceList `json:"resources,omitempty"`
	// Target determines what object the export output should be applied to
	// +kubebuilder:default:=Resources
	// +kubebuilder:validation:Enum:=Default;PatchDesired;PatchResources;Resources;XR
	Target resource.Target `json:"target"`
}

// ConfigSpec defines the compile config.
type ConfigSpec struct {
	// Arguments is the list of top level dynamic arguments for the kcl option function, e.g., env="prod"
	Arguments []string `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	// Settings is the list of kcl setting files including all of the CLI config.
	Settings []string `json:"settings,omitempty" yaml:"settings,omitempty"`
	// Overrides is the list of override paths and values, e.g., app.image="v2"
	Overrides []string `json:"overrides,omitempty" yaml:"overrides,omitempty"`
	// PathSelectors is the list of path selectors to select output result, e.g., a.b.c
	PathSelectors []string `json:"pathSelectors,omitempty" yaml:"pathSelectors,omitempty"`
	// Vendor denotes running kcl in the vendor mode.
	Vendor bool `json:"vendor,omitempty" yaml:"vendor,omitempty"`
	// SortKeys denotes sorting the output result keys, e.g., `{b = 1, a = 2} => {a = 2, b = 1}`.
	SortKeys bool `json:"sortKeys,omitempty" yaml:"sortKeys,omitempty"`
	// ShowHidden denotes output the hidden attribute in the result.
	ShowHidden bool `json:"showHidden,omitempty" yaml:"showHidden,omitempty"`
	// DisableNone denotes running kcl and disable dumping None values.
	DisableNone bool `json:"disableNone,omitempty" yaml:"disableNone,omitempty"`
	// Debug denotes running kcl in debug mode.
	Debug bool `json:"debug,omitempty" yaml:"debug,omitempty"`
	// StrictRangeCheck performs the 32-bit strict numeric range checks on numbers.
	StrictRangeCheck bool `json:"strictRangeCheck,omitempty" yaml:"strictRangeCheck,omitempty"`
}

// CredSpec defines authentication credentials for remote locations
type CredSpec struct {
	Url      string `json:"url,omitempty" yaml:"url,omitempty"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
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
