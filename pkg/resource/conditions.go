package resource

import (
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// Target determines which objects to set the condition on.
type BindingTarget string

const (
	// TargetComposite targets only the composite resource.
	TargetComposite BindingTarget = "Composite"

	// TargetCompositeAndClaim targets both the composite and the claim.
	TargetCompositeAndClaim BindingTarget = "CompositeAndClaim"
)

type ConditionResources []ConditionResource

// ConditionResource will set a condition on the target.
type ConditionResource struct {
	// The target(s) to receive the condition. Can be Composite or
	// CompositeAndClaim. Defaults to Composite
	Target *BindingTarget `json:"target"`
	// If true, the condition will override a condition of the same Type. Defaults
	// to false.
	Force *bool `json:"force"`
	// Condition to set.
	Condition Condition `json:"condition"`
}

// Condition allows you to specify fields to set on a composite resource and
// claim.
type Condition struct {
	// Type of the condition. Required.
	Type string `json:"type"`
	// Status of the condition. Required.
	Status metav1.ConditionStatus `json:"status"`
	// Reason of the condition. Required.
	Reason string `json:"reason"`
	// Message of the condition. Optional. A template can be used. The available
	// template variables come from capturing groups in MatchCondition message
	// regular expressions.
	Message *string `json:"message"`
}

// transformCondition converts a ConditionResource into an fnv1.Condition while mapping status and target accordingly.
func transformCondition(cs ConditionResource) *fnv1.Condition {
	c := &fnv1.Condition{
		Type:   cs.Condition.Type,
		Reason: cs.Condition.Reason,
		Target: transformTarget(cs.Target),
	}

	switch cs.Condition.Status {
	case metav1.ConditionTrue:
		c.Status = fnv1.Status_STATUS_CONDITION_TRUE
	case metav1.ConditionFalse:
		c.Status = fnv1.Status_STATUS_CONDITION_FALSE
	case metav1.ConditionUnknown:
		fallthrough
	default:
		c.Status = fnv1.Status_STATUS_CONDITION_UNKNOWN
	}

	c.Message = cs.Condition.Message

	return c
}

func transformTarget(t *BindingTarget) *fnv1.Target {
	target := ptr.Deref(t, TargetComposite)
	if target == TargetCompositeAndClaim {
		return fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum()
	}
	return fnv1.Target_TARGET_COMPOSITE.Enum()
}

// SetConditions updates the RunFunctionResponse with specified conditions from ConditionResources, ensuring no duplicates.
// It validates that system-reserved Crossplane condition types are not set and permits forced updates when specified.
func SetConditions(rsp *fnv1.RunFunctionResponse, cr ConditionResources, log logging.Logger) error {
	conditionsSet := map[string]bool{}
	// All matchConditions matched, set the desired conditions.
	for _, cs := range cr {
		if xpv1.IsSystemConditionType(xpv1.ConditionType(cs.Condition.Type)) {
			response.Fatal(rsp, errors.Errorf("cannot set ClaimCondition type: %s is a reserved Crossplane Condition", cs.Condition.Type))
			return errors.New("error updating response")
		}
		if conditionsSet[cs.Condition.Type] && (cs.Force == nil || !*cs.Force) {
			// The condition is already set and this setter is not forceful.
			log.Debug("skipping because condition is already set and setCondition is not forceful")
			continue
		}
		log.Debug("setting condition")

		c := transformCondition(cs)
		rsp.Conditions = append(rsp.Conditions, c)
		conditionsSet[cs.Condition.Type] = true
	}
	return nil
}
