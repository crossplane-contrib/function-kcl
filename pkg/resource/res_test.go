package resource

import (
	"testing"

	res "github.com/crossplane/function-sdk-go/resource"
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
