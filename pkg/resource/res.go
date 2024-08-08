package resource

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"dario.cat/mergo"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	krmyaml "kcl-lang.io/krm-kcl/pkg/yaml"
)

type Target string

const (
	// PatchDesired targets existing Resources on the Desired XR
	PatchDesired Target = "PatchDesired"
	// PatchResources targets existing KCLInput.spec.Resources
	// These resources are then created similar to the Resources target
	PatchResources Target = "PatchResources"
	// Resources creates new resources that are added to the DesiredComposed Resources
	Resources Target = "Resources"
	// XR targets the existing Observed XR itself
	XR Target = "XR"
	// Default targets the existing all resources including XR, Desired XR,
	// spec.Resources and DesiredComposed Resources.
	Default Target = "Default"
)

const (
	AnnotationKeyReady                   = "krm.kcl.dev/ready"
	AnnotationKeyCompositionResourceName = "krm.kcl.dev/composition-resource-name"
	MetaApiVersion                       = "meta.krm.kcl.dev/v1alpha1"
)

type ResourceList []Resource

type Resource struct {
	// Name is a unique identifier for this entry in a ResourceList
	Name string                    `json:"name"`
	Base unstructured.Unstructured `json:"base,omitempty"`
}

func JsonByteToRawExtension(jsonByte []byte) (runtime.RawExtension, error) {
	o, err := JsonByteToUnstructured(jsonByte)
	if err != nil {
		return runtime.RawExtension{}, err
	}
	return UnstructuredToRawExtension(o)
}

func JsonByteToUnstructured(jsonByte []byte) (*unstructured.Unstructured, error) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonByte, &data)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: data}
	return u, nil
}

func UnstructuredToRawExtension(obj *unstructured.Unstructured) (runtime.RawExtension, error) {
	if obj == nil {
		return runtime.RawExtension{}, nil
	}
	raw, err := obj.MarshalJSON()
	if err != nil {
		return runtime.RawExtension{}, err
	}
	return runtime.RawExtension{Raw: raw}, nil
}

func ObjToRawExtension(obj interface{}) (runtime.RawExtension, error) {
	if obj == nil {
		return runtime.RawExtension{}, nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return runtime.RawExtension{}, err
	}
	return runtime.RawExtension{Raw: raw}, nil
}

func normalizeMap(input interface{}) (interface{}, error) {
	switch in := input.(type) {
	case map[interface{}]interface{}:
		normalized := make(map[string]interface{})
		for key, value := range in {
			strKey, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("found non-string key in the map")
			}
			normalizedValue, err := normalizeMap(value)
			if err != nil {
				return nil, err
			}
			normalized[strKey] = normalizedValue
		}
		return normalized, nil
	case []interface{}:
		for i, v := range in {
			normalizedValue, err := normalizeMap(v)
			if err != nil {
				return nil, err
			}
			in[i] = normalizedValue
		}
		return in, nil
	default:
		return input, nil
	}
}

// DataResourcesFromYaml returns the manifests list from the YAML stream data.
func DataResourcesFromYaml(in []byte) (result []unstructured.Unstructured, err error) {
	bytes, err := krmyaml.SplitDocuments(string(in))
	if err != nil {
		return
	}
	for _, b := range bytes {
		var data interface{}
		err = yaml.Unmarshal([]byte(b), &data)
		if err != nil {
			return
		}

		// Convert map[any]any to map[string]any
		normalizedData, err := normalizeMap(data)
		if err != nil {
			return nil, err
		}

		result = append(result, unstructured.Unstructured{
			Object: normalizedData.(map[string]interface{}),
		})
	}
	return
}

// DesiredMatch matches a list of data to apply to a desired resource
// This is used when targeting PatchDesired resources
type DesiredMatch map[*resource.DesiredComposed][]map[string]interface{}

// MatchResources finds and associates the data to the desired resource
// The length of the passed data should match the total count of desired match data
func MatchResources(desired map[resource.Name]*resource.DesiredComposed, data []unstructured.Unstructured) (DesiredMatch, error) {
	// Iterate over the data patches and match them to desired resources
	matches := make(DesiredMatch)
	count := 0
	// Get total count of all the match patches to apply
	// this count should match the initial count of the supplied data
	// otherwise we lost something somewhere
	for _, d := range data {
		// PatchDesired
		cd := resource.NewDesiredComposed()
		cd.Resource.Unstructured = d
		if found, ok := desired[resource.Name(GetResourceName(cd))]; ok {
			if _, ok := matches[found]; !ok {
				matches[found] = []map[string]interface{}{d.Object}
			} else {
				matches[found] = append(matches[found], d.Object)
			}
			count++
		}
	}
	if count != len(data) {
		return matches, fmt.Errorf("failed to match all resources, found %d / %d patches", count, len(data))
	}

	return matches, nil
}

type AddResourcesResult struct {
	Target   Target
	Object   any
	MsgCount int
	Msgs     []string
}

// setSuccessMsgs generates the success messages for the input data
func (r *AddResourcesResult) setSuccessMsgs() {
	r.Msgs = make([]string, r.MsgCount)
	switch r.Target {
	case Resources:
		desired := r.Object.([]unstructured.Unstructured)
		j := 0
		for _, d := range desired {
			r.Msgs[j] = fmt.Sprintf("created resource \"%s:%s\"", d.GetName(), d.GetKind())
			j++
		}
	case PatchDesired:
		desired := r.Object.([]unstructured.Unstructured)
		j := 0
		for _, d := range desired {
			r.Msgs[j] = fmt.Sprintf("updated resource \"%s:%s\"", d.GetName(), d.GetKind())
			j++
		}
	case PatchResources:
		desired := r.Object.([]unstructured.Unstructured)
		j := 0
		for _, d := range desired {
			r.Msgs[j] = fmt.Sprintf("created resource \"%s:%s\"", d.GetName(), d.GetKind())
			j++
		}
	case XR:
		o := r.Object.(*resource.Composite)
		r.Msgs[0] = fmt.Sprintf("updated xr \"%s:%s\"", o.Resource.GetName(), o.Resource.GetKind())
	}
	sort.Strings(r.Msgs)
}

type AddResourcesOptions struct {
	Basename  string
	Data      []unstructured.Unstructured
	Overwrite bool
}

// AddResourcesTo adds the given data to any allowed object passed
// Will return err if the object is not of a supported type
// For 'desired' composed resources, the Basename is used for the resource name
// For 'xr' resources, the Basename must match the xr name
// For 'existing' resources, the Basename must match the resource name
func AddResourcesTo(o any, opts *AddResourcesOptions) error {
	// Merges data with the desired composed resource
	// Values from data overwrite the desired composed resource
	merged := func(data map[string]interface{}, from *resource.DesiredComposed) map[string]interface{} {
		merged := make(map[string]interface{})
		for k, v := range from.Resource.UnstructuredContent() {
			merged[k] = v
		}
		// patch data overwrites existing desired composed data
		for k, v := range data {
			merged[k] = v
		}
		return merged
	}

	switch val := o.(type) {
	case map[resource.Name]*resource.DesiredComposed:
		// Resources
		desired := val
		for _, d := range opts.Data {
			cd := resource.NewDesiredComposed()
			cd.Resource.Unstructured = d
			name := resource.Name(GetResourceName(cd))
			d := cd.Resource.Unstructured
			// If the value exists, merge its existing value with the patches
			if v, ok := desired[name]; ok {
				mergedData := merged(cd.Resource.Object, v)
				d = unstructured.Unstructured{Object: mergedData}
			}
			desired[name] = &resource.DesiredComposed{
				Resource: &composed.Unstructured{
					Unstructured: d,
				},
			}
		}
	case DesiredMatch:
		// PatchDesired
		matches := val
		// Set the Match data on the desired resource stored as keys
		for obj, matchData := range matches {
			// There may be multiple data patches to the DesiredComposed object
			for _, d := range matchData {
				if err := SetData(d, "", obj, opts.Overwrite); err != nil {
					return errors.Wrap(err, "cannot set data existing desired composed object")
				}
			}
		}
	case *resource.Composite:
		// XR
		for _, d := range opts.Data {
			if err := SetData(d.Object, "", o, opts.Overwrite); err != nil {
				return errors.Wrap(err, "cannot set data on xr")
			}
		}
	default:
		return fmt.Errorf("cannot add configuration to %T: invalid type for obj", o)
	}
	return nil
}

var (
	errNoSuchField = "no such field"
)

// SetData is a recursive function that is intended to build a kube fieldpath valid
// JSONPath(s) of the given object, it will then copy from 'data' at the given path
// to the passed o object - at the same path, overwrite defines if this function should
// be allowed to overwrite values or not, if not return an conflicting value error
//
// If the resource to write to 'o' contains a nil .Resource, setData will return an error
// It is expected that the resource is created via composed.New() or composite.New() prior
// to calling setData
func SetData(data any, path string, o any, overwrite bool) error {
	switch val := data.(type) {
	case map[string]interface{}:
		// Check if the parent field is annotations or labels
		// if so wrap the key in [] instead of . prefix
		//
		// Check if the suffix for validation, this is because there may be metadata annotations on deeper level items
		isWrapped := false
		if strings.HasSuffix(path, ".metadata.annotations") || strings.HasSuffix(path, ".metadata.labels") {
			isWrapped = true
		}

		for key, value := range val {
			var newKey string
			if isWrapped {
				newKey = fmt.Sprintf("%s[%s]", path, key)
			} else {
				newKey = fmt.Sprintf("%s.%v", path, key)
			}
			if err := SetData(value, newKey, o, overwrite); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, value := range val {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if err := SetData(value, newPath, o, overwrite); err != nil {
				return err
			}
		}
	default:
		// Reached a leaf node, add the JSON path to the desired resource
		switch val := o.(type) {
		case *resource.DesiredComposed:
			path = strings.TrimPrefix(path, ".")

			// Currently do not allow overwriting apiVersion kind or name
			// ignore setting these again because this will conflict with the overwrite settings
			if path == "apiVersion" || path == "kind" || path == "metadata.name" {
				return nil
			}

			r := val.Resource
			if r == nil {
				return errors.New("cannot set data on a nil DesiredComposed resource")
			}

			if curVal, err := r.GetValue(path); err != nil && !strings.Contains(err.Error(), errNoSuchField) {
				return errors.Wrapf(err, "getting %s:%s in xr failed", path, data)
			} else if curVal != nil && !overwrite {
				return fmt.Errorf("%s: conflicting values %q and %q", path, curVal, data)
			}

			if err := r.SetValue(path, data); err != nil {
				return errors.Wrapf(err, "setting %s:%s in dxr failed", path, data)
			}
		case *resource.Composite:
			path = strings.TrimPrefix(path, ".")

			// The composite does not do any matching to update so there is no need to skip here
			// on apiVersion, kind or metadata.name

			r := val.Resource
			if r == nil {
				return fmt.Errorf("cannot set data on a nil XR")
			}

			if curVal, err := r.GetValue(path); err != nil && !strings.Contains(err.Error(), errNoSuchField) {
				return errors.Wrapf(err, "getting %s:%s in xr failed", path, data)
			} else if curVal != nil && !overwrite {
				return fmt.Errorf("%s: conflicting values %q and %q", path, curVal, data)
			}

			if err := r.SetValue(path, data); err != nil {
				return errors.Wrapf(err, "setting %s:%s in dxr failed", path, data)
			}
		default:
			return fmt.Errorf("cannot set data on %T: invalid type for obj", o)
		}
	}
	return nil
}

func ProcessResources(dxr *resource.Composite, oxr *resource.Composite, desired map[resource.Name]*resource.DesiredComposed, observed map[resource.Name]resource.ObservedComposed, target Target, resources ResourceList, opts *AddResourcesOptions) (AddResourcesResult, error) {
	result := AddResourcesResult{
		Target: target,
	}
	data := opts.Data
	switch target {
	case XR:
		if err := AddResourcesTo(dxr, opts); err != nil {
			return result, err
		}
		result.Object = dxr
		result.MsgCount = 1
	case PatchDesired:
		desiredMatches, err := MatchResources(desired, data)
		if err != nil {
			return result, err
		}
		if err := AddResourcesTo(desiredMatches, opts); err != nil {
			return result, err
		}
		result.Object = data
		result.MsgCount = len(data)
	case PatchResources:
		// Render the List of DesiredComposed resources from the input
		// Update the existing desired map to be created as a base
		for _, r := range resources {
			desired[resource.Name(r.Name)] = &resource.DesiredComposed{Resource: &composed.Unstructured{Unstructured: r.Base}}
		}
		// Match the data to the desired resources
		desiredMatches, err := MatchResources(desired, data)
		if err != nil {
			return result, err
		}

		if err := AddResourcesTo(desiredMatches, opts); err != nil {
			return result, err
		}
		result.Object = data
		result.MsgCount = len(data)
	case Resources:
		if err := AddResourcesTo(desired, opts); err != nil {
			return result, err
		}
		// Pass data here instead of desired
		// This is because there already may be desired objects
		result.Object = data
		result.MsgCount = len(data)
	case Default:
		checked := make(map[string]struct{})
		for _, obj := range data {
			cd := resource.NewDesiredComposed()
			cd.Resource.Unstructured = obj
			// Patch dxr
			if cd.Resource.GetAPIVersion() == oxr.Resource.GetAPIVersion() && cd.Resource.GetKind() == oxr.Resource.GetKind() {
				dst := make(map[string]any)
				if err := dxr.Resource.GetValueInto("status", &dst); err != nil && !fieldpath.IsNotFound(err) {
					return result, errors.Wrap(err, "cannot get desired composite status")
				}
				src := make(map[string]any)
				if err := cd.Resource.GetValueInto("status", &src); err != nil && !fieldpath.IsNotFound(err) {
					return result, errors.Wrap(err, "cannot get templated composite status")
				}
				if err := mergo.Merge(&dst, src, mergo.WithOverride); err != nil {
					return result, errors.Wrap(err, "cannot merge desired composite status")
				}
				if err := fieldpath.Pave(dxr.Resource.Object).SetValue("status", dst); err != nil {
					return result, errors.Wrap(err, "cannot set desired composite status")
				}
				continue
			}
			// Check the meta resource
			if cd.Resource.GetAPIVersion() == MetaApiVersion {
				switch obj.GetKind() {
				case "CompositeConnectionDetails":
					con, _ := cd.Resource.GetStringObject("data")
					for k, v := range con {
						d, _ := base64.StdEncoding.DecodeString(v) //nolint:errcheck // k8s returns secret values encoded
						dxr.ConnectionDetails[k] = d
					}
				default:
					return result, errors.Errorf("invalid kind %q for apiVersion %q - must be CompositeConnectionDetails", obj.GetKind(), MetaApiVersion)
				}
				continue
			}
			if v, found := cd.Resource.GetAnnotations()[AnnotationKeyReady]; found {
				if v != string(resource.ReadyTrue) && v != string(resource.ReadyUnspecified) && v != string(resource.ReadyFalse) {
					return result, errors.Errorf("invalid function input: invalid %q annotation value %q: must be True, False, or Unspecified", AnnotationKeyReady, v)
				}
				cd.Ready = resource.Ready(v)
				// Remove meta annotation.
				meta.RemoveAnnotations(cd.Resource, AnnotationKeyReady)
			}
			err := CheckAndSetDesired(desired, checked, cd)
			if err != nil {
				return result, err
			}
		}
		result.Object = data
		result.MsgCount = len(data)
		result.setSuccessMsgs()
		return result, nil
	}
	result.setSuccessMsgs()
	return result, nil
}

// Check the set the resource into the desired resource map.
func CheckAndSetDesired(desired map[resource.Name]*resource.DesiredComposed, checked map[string]struct{}, cd *resource.DesiredComposed) error {
	name := GetResourceName(cd)
	if _, existed := checked[name]; existed {
		return errors.Errorf("duplicate resource names %s found, when returning multiple resources, you need to set different metadata.name or matadata.annotations.\"krm.kcl.dev/composition-resource-name\" to distinguish between different resources in the composition functions.", name)
	}
	checked[name] = struct{}{}
	desired[resource.Name(name)] = cd
	return nil
}

func GetResourceName(cd *resource.DesiredComposed) string {
	name, found := cd.Resource.GetAnnotations()[AnnotationKeyCompositionResourceName]
	if !found {
		name = cd.Resource.GetName()
	}
	meta.RemoveAnnotations(cd.Resource, AnnotationKeyCompositionResourceName)
	return name
}
