package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	runtimeresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-template-go/input/v1beta1"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type outputData struct {
	// Name is a unique identifier for this entry
	Name string `json:"name"`
	// Resource is the managed resource output.
	Resource map[string]interface{} `json:"resource"`
}

// renderFromJSON renders the supplied resource from JSON bytes.
func renderFromJSON(o runtimeresource.Object, data []byte) error {
	if err := json.Unmarshal(data, o); err != nil {
		return errors.Wrap(err, "cannot unmarshal JSON data")
	}
	return nil
}

func buildData(in []byte) (result []outputData, err error) {
	bytes, err := splitDocuments(string(in))
	if err != nil {
		return
	}
	for i, b := range bytes {
		var data map[string]interface{}
		err = yaml.Unmarshal([]byte(b), &data)
		if err != nil {
			return
		}
		result = append(result, outputData{
			Name:     fmt.Sprint(i),
			Resource: data,
		})
	}
	return
}

// desiredMatch matches a list of data to apply to a desired resource
// This is used when targeting PatchDesired resources
type desiredMatch map[*resource.DesiredComposed][]map[string]interface{}

// matchResources finds and associates the data to the desired resource
// The length of the passed data should match the total count of desired match data
func matchResources(desired map[resource.Name]*resource.DesiredComposed, data []outputData) (desiredMatch, error) {
	// Iterate over the data patches and match them to desired resources
	matches := make(desiredMatch)
	count := 0
	// Get total count of all the match patches to apply
	// this count should match the initial count of the supplied data
	// otherwise we lost something somewhere
	for _, d := range data {
		// PatchDesired
		if found, ok := desired[resource.Name(d.Name)]; ok {
			if _, ok := matches[found]; !ok {
				matches[found] = []map[string]interface{}{d.Resource}
			} else {
				matches[found] = append(matches[found], d.Resource)
			}
			count++
		}
	}
	if count != len(data) {
		return matches, fmt.Errorf("failed to match all resources, found %d / %d patches", count, len(data))
	}

	return matches, nil
}

type successOutput struct {
	target   v1beta1.Target
	object   any
	msgCount int
	msgs     []string
}

// setSuccessMsgs generates the success messages for the input data
func (output *successOutput) setSuccessMsgs() {
	output.msgs = make([]string, output.msgCount)
	switch output.target {
	case v1beta1.Resources:
		desired := output.object.([]outputData)
		j := 0
		for _, d := range desired {
			u := unstructured.Unstructured{Object: d.Resource}
			output.msgs[j] = fmt.Sprintf("created resource \"%s:%s\"", u.GetName(), u.GetKind())
			j++
		}
	case v1beta1.PatchDesired:
		desired := output.object.([]outputData)
		j := 0
		for _, d := range desired {
			u := unstructured.Unstructured{Object: d.Resource}
			output.msgs[j] = fmt.Sprintf("updated resource \"%s:%s\"", u.GetName(), u.GetKind())
			j++
		}
	case v1beta1.PatchResources:
		desired := output.object.([]outputData)
		j := 0
		for _, d := range desired {
			u := unstructured.Unstructured{Object: d.Resource}
			output.msgs[j] = fmt.Sprintf("created resource \"%s:%s\"", u.GetName(), u.GetKind())
			j++
		}
	case v1beta1.XR:
		o := output.object.(*resource.Composite)
		output.msgs[0] = fmt.Sprintf("updated xr \"%s:%s\"", o.Resource.GetName(), o.Resource.GetKind())
	}
	sort.Strings(output.msgs)
}

type addResourcesConf struct {
	basename  string
	data      []outputData
	overwrite bool
}

// addResourcesTo adds the given data to any allowed object passed
// Will return err if the object is not of a supported type
// For 'desired' composed resources, the basename is used for the resource name
// For 'xr' resources, the basename must match the xr name
// For 'existing' resources, the basename must match the resource name
func addResourcesTo(o any, conf addResourcesConf) error {
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
		for _, d := range conf.data {
			name := resource.Name(d.Name)
			u := unstructured.Unstructured{
				Object: d.Resource,
			}

			// Add the resource name as a suffix to the basename
			// if there are multiple resources to add
			if len(conf.data) > 1 {
				name = resource.Name(fmt.Sprintf("%s-%s", conf.basename, d.Name))
			}
			// If the value exists, merge its existing value with the patches
			if v, ok := desired[name]; ok {
				mergedData := merged(d.Resource, v)
				u = unstructured.Unstructured{Object: mergedData}
			}
			desired[name] = &resource.DesiredComposed{
				Resource: &composed.Unstructured{
					Unstructured: u,
				},
			}
		}
	case desiredMatch:
		// PatchDesired
		matches := val
		// Set the Match data on the desired resource stored as keys
		for obj, matchData := range matches {
			// There may be multiple data patches to the DesiredComposed object
			for _, d := range matchData {
				if err := setData(d, "", obj, conf.overwrite); err != nil {
					return errors.Wrap(err, "cannot set data existing desired composed object")
				}
			}
		}
	case *resource.Composite:
		// XR
		for _, d := range conf.data {
			if err := setData(d.Resource, "", o, conf.overwrite); err != nil {
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

// setData is a recursive function that is intended to build a kube fieldpath valid
// JSONPath(s) of the given object, it will then copy from 'data' at the given path
// to the passed o object - at the same path, overwrite defines if this function should
// be allowed to overwrite values or not, if not return an conflicting value error
//
// If the resource to write to 'o' contains a nil .Resource, setData will return an error
// It is expected that the resource is created via composed.New() or composite.New() prior
// to calling setData
func setData(data any, path string, o any, overwrite bool) error {
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
			if err := setData(value, newKey, o, overwrite); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, value := range val {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if err := setData(value, newPath, o, overwrite); err != nil {
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
