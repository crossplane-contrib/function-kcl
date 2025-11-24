package resource

import (
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

// RequiredResourcesRequirements defines the requirements for required resources.
type RequiredResourcesRequirements map[string]RequiredResourcesRequirement

// RequiredResourcesRequirement defines a single requirement for required resources.
// Needed to have camelCase keys instead of the snake_case keys as defined
// through json tags by fnv1.ResourceSelector.
type RequiredResourcesRequirement struct {
	// APIVersion of the resource.
	APIVersion string `json:"apiVersion"`
	// Kind of the resource.
	Kind string `json:"kind"`
	// MatchLabels defines the labels to match the resource, if Name is empty.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// Name defines the name to match the resource.
	// If defined, MatchLabels is ignored.
	Name string `json:"name,omitempty"`
	// Namespace defines the namespace to match a namespace scoped resource, if set.
	// If empty, the resource is assumed to be cluster scoped.
	Namespace string `json:"namespace,omitempty"`
}

// ToResourceSelector converts the RequiredResourcesRequirement to a fnv1.ResourceSelector.
func (e *RequiredResourcesRequirement) ToResourceSelector() *fnv1.ResourceSelector {
	out := &fnv1.ResourceSelector{
		ApiVersion: e.APIVersion,
		Kind:       e.Kind,
	}
	if e.Namespace != "" {
		out.Namespace = &e.Namespace
	}
	if e.Name == "" {
		out.Match = &fnv1.ResourceSelector_MatchLabels{
			MatchLabels: &fnv1.MatchLabels{Labels: e.MatchLabels},
		}
		return out
	}

	out.Match = &fnv1.ResourceSelector_MatchName{
		MatchName: e.Name,
	}
	return out
}
