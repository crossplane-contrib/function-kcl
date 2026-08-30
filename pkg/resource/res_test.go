package resource

import (
	"strings"
	"testing"

	res "github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestObjToRawExtension(t *testing.T) {
	u := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"some-key": "some-value",
			},
		},
	}
	tests := []struct {
		input    interface{}
		expected []byte
		wantErr  bool
	}{
		{
			input:    nil,
			expected: nil,
			wantErr:  false,
		},
		{
			input:    struct{ Name string }{Name: "test"},
			expected: []byte(`{"Name":"test"}`),
			wantErr:  false,
		},
		{
			input: map[string]res.Extra{
				"test": {
					Resource: &u,
				},
			},
			expected: []byte(`{"test":{"Resource":{"apiVersion":"v1","data":{"some-key":"some-value"},"kind":"ConfigMap","metadata":{"name":"my-config","namespace":"default"}}}}`),
			wantErr:  false,
		},
		{
			input:    make(chan int),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := ObjToRawExtension(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ObjToRawExtension() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !equal(got.Raw, tt.expected) {
				t.Errorf("ObjToRawExtension() = %s, want %s", got.Raw, tt.expected)
			}
		})
	}
}

func equal(a, b []byte) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && string(a) == string(b))
}

func TestSetData(t *testing.T) {
	type args struct {
		data      any
		path      string
		o         any
		overwrite bool
	}
	tests := []struct {
		name     string
		args     args
		expected *res.DesiredComposed
		wantErr  bool
	}{
		{
			name: "Should create a new element on existing array",
			args: args{
				data: "c",
				path: ".some-array[2]",
				o: &res.DesiredComposed{
					Resource: &composed.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: map[string]interface{}{
								"some-array": []interface{}{"a", "b"},
							},
						},
					},
				},
				overwrite: true,
			},
			expected: &res.DesiredComposed{
				Resource: &composed.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: map[string]interface{}{
							"some-array": []interface{}{"a", "b", "c"},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetData(tt.args.data, tt.args.path, tt.args.o, tt.args.overwrite); (err != nil) != tt.wantErr {
				t.Errorf("SetData() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.args.o, tt.expected); diff != "" {
				t.Errorf("SetData(): -want rsp, +got rsp:\n%s", diff)
			}
		})
	}
}

func TestCheckDuplicateName(t *testing.T) {
	newCD := func(kind, name, annotation string) *res.DesiredComposed {
		metadata := map[string]interface{}{}
		if name != "" {
			metadata["name"] = name
		}
		if annotation != "" {
			metadata["annotations"] = map[string]interface{}{
				AnnotationKeyCompositionResourceName: annotation,
			}
		}
		return &res.DesiredComposed{
			Resource: &composed.Unstructured{
				Unstructured: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       kind,
						"metadata":   metadata,
					},
				},
			},
		}
	}

	tests := []struct {
		name    string
		kind    string
		objName string
		annot   string
		wantErr string
	}{
		{
			name:    "Named via metadata.name is accepted",
			kind:    "Bucket",
			objName: "my-bucket",
		},
		{
			name:  "Named via the composition resource name annotation is accepted",
			kind:  "Bucket",
			annot: "bucket",
		},
		{
			// Without this guard the resource is keyed on "" and Crossplane
			// later fails with an opaque "composed resource without required
			// composition-resource-name" error. See issue #199.
			name:    "Unnamed resource is rejected with an actionable error",
			kind:    "Release",
			wantErr: `composed resource of kind "Release" has no composition resource name`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := newCD(tt.kind, tt.objName, tt.annot)
			checker := newDuplicateChecker()
			err := checker.CheckDuplicateName(cd, res.Name(GetResourceName(cd)))

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("CheckDuplicateName() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckDuplicateName() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CheckDuplicateName() error = %q, want it to contain %q", err, tt.wantErr)
			}
			// The message must point the author at the annotation to set.
			if !strings.Contains(err.Error(), AnnotationKeyCompositionResourceName) {
				t.Errorf("CheckDuplicateName() error = %q, want it to mention %q", err, AnnotationKeyCompositionResourceName)
			}
		})
	}
}
