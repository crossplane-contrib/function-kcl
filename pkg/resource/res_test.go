package resource

import (
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
