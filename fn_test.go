package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"kcl-lang.io/krm-kcl/pkg/kube"
)

func TestRunFunctionSimple(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ResponseIsReturned": {
			reason: "The Function should return a fatal result if no input was specified",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "krm.kcl.dev/v1alpha1",
						"kind": "KCLInput",
						"metadata": {
							"name": "basic"
						},
						"spec": {
							"target": "Resources",
							"source": "{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n}"
						}
					}`),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"": {
								Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"Generated"}`),
							},
						},
					},
				},
			},
		},
		"DatabaseInstance": {
			reason: "The Function should return a fatal result if no input was specified",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "database-instance"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "krm.kcl.dev/v1alpha1",
						"kind": "KCLInput",
						"metadata": {
							"name": "basic"
						},
						"spec": {
							"source": "items = [{ \n    apiVersion: \"sql.gcp.upbound.io/v1beta1\"\n    kind: \"DatabaseInstance\"\n    spec: {\n        forProvider: {\n            project: \"test-project\"\n            settings: [{databaseFlags: [{\n                name: \"log_checkpoints\"\n                value: \"on\"\n            }]}]\n        }\n    }\n}]\n"
						}
					}`),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "database-instance", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"": {
								Resource: resource.MustStructJSON(`{"apiVersion": "sql.gcp.upbound.io/v1beta1", "kind": "DatabaseInstance", "spec": {"forProvider": {"project": "test-project", "settings": [{"databaseFlags": [{"name": "log_checkpoints", "value": "on"}]}]}}}`),
							},
						},
					},
				},
			},
		},
		"CustomCompositionResourceNameIsSet": {
			reason: "The Function should set value of crossplane.io/composition-resource-name annotation by krm.kcl.dev/composition-resource-name annotation ",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "krm.kcl.dev/v1alpha1",
						"kind": "KCLInput",
						"metadata": {
							"name": "basic"
						},
						"spec": {
							"target": "Default",
							"source": "{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n metadata.annotations = {\"krm.kcl.dev/composition-resource-name\": \"custom-composition-resource-name\"}\n}"
						}
					}`),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"custom-composition-resource-name": {
								Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"Generated","metadata":{"annotations":{}}}`),
							},
						},
					},
				},
			},
		},
		"MultipleResource": {
			reason: "The Function should return multiple resources with different resource names",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "multiple-resource"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "krm.kcl.dev/v1alpha1",
						"kind": "KCLInput",
						"metadata": {
							"name": "basic"
						},
						"spec": {
							"source": "items = [\n{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n metadata.annotations = {\"krm.kcl.dev/composition-resource-name\": \"custom-composition-resource-name-0\"}\n}\n{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n metadata.annotations = {\"krm.kcl.dev/composition-resource-name\": \"custom-composition-resource-name-1\"}\n}\n]\n"
						}
					}`),
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "multiple-resource", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
						},
						Resources: map[string]*fnv1beta1.Resource{
							"custom-composition-resource-name-0": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1", "kind": "Generated", "metadata": {"annotations": {}}}`),
							},
							"custom-composition-resource-name-1": {
								Resource: resource.MustStructJSON(`{"apiVersion": "example.org/v1", "kind": "Generated", "metadata": {"annotations": {}}}`),
							},
						},
					},
				},
			},
		},
		// TODO: disable the resource check, and fix the kcl dup resource evaluation issues.
		// "MultipleResourceError": {
		// 	reason: "The Function should return a fatal result if input resources have duplicate names",
		// 	args: args{
		// 		req: &fnv1beta1.RunFunctionRequest{
		// 			Meta: &fnv1beta1.RequestMeta{Tag: "multiple-resource-error"},
		// 			Input: resource.MustStructJSON(`{
		// 				"apiVersion": "krm.kcl.dev/v1alpha1",
		// 				"kind": "KCLInput",
		// 				"metadata": {
		// 					"name": "basic"
		// 				},
		// 				"spec": {
		// 					"source": "items = [\n{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n metadata.annotations = {\"krm.kcl.dev/composition-resource-name\": \"custom-composition-resource-name\"}\n}\n{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n metadata.annotations = {\"krm.kcl.dev/composition-resource-name\": \"custom-composition-resource-name\"}\n}\n]\n"
		// 				}
		// 			}`),
		// 			Observed: &fnv1beta1.State{
		// 				Composite: &fnv1beta1.Resource{
		// 					Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
		// 				},
		// 			},
		// 		},
		// 	},
		// 	want: want{
		// 		rsp: &fnv1beta1.RunFunctionResponse{
		// 			Meta: &fnv1beta1.ResponseMeta{Tag: "multiple-resource-error", Ttl: durationpb.New(response.DefaultTTL)},
		// 			Results: []*fnv1beta1.Result{
		// 				{
		// 					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
		// 					Message:  "cannot process xr and state with the pipeline output in *v1beta1.RunFunctionResponse: duplicate resource names custom-composition-resource-name found, when returning multiple resources, you need to set different metadata.name or metadata.annotations.\"krm.kcl.dev/composition-resource-name\" to distinguish between different resources in the composition functions.",
		// 				},
		// 			}},
		// 	},
		// },
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

const (
	xrFile          = "xr.yaml"
	compositionFile = "composition.yaml"
)

func findXRandCompositionYAMLFiles(rootPath string) ([]string, error) {
	var dirs []string

	// Walk receives the root directory and a function to process each path
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil { // Handle potential errors
			return err
		}

		// Check if the current path is a directory and it contains xr.yaml file
		if info.IsDir() {
			xrPath := filepath.Join(path, xrFile)
			compositionPath := filepath.Join(path, compositionFile)
			if _, err := os.Stat(xrPath); err == nil {
				if _, err := os.Stat(compositionPath); err == nil {
					dirs = append(dirs, path) // File exists, add directory to list
				}
			}
		}

		return nil
	})

	return dirs, err
}

func readResourceFromFile(p string) (*structpb.Struct, error) {
	c, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	o, err := kube.ParseKubeObject(c)
	if err != nil {
		return nil, err
	}
	j, err := o.Node().MarshalJSON()
	if err != nil {
		return nil, err
	}
	return resource.MustStructJSON(string(j)), nil
}

func TestFunctionExamples(t *testing.T) {
	rootPath := "examples" // Change to your examples folder path
	dirs, err := findXRandCompositionYAMLFiles(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	// Print all directories containing xr.yaml file
	for _, dir := range dirs {
		xrPath := filepath.Join(dir, xrFile)
		compositionPath := filepath.Join(dir, compositionFile)
		t.Run(compositionPath, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			input, err := readResourceFromFile(xrPath)
			if err != nil {
				t.Fatal(err)
			}
			oxr, err := readResourceFromFile(compositionPath)
			if err != nil {
				t.Fatal(err)
			}
			req := &fnv1beta1.RunFunctionRequest{
				Input: input,
				// option("params").oxr
				Observed: &fnv1beta1.State{
					Composite: &fnv1beta1.Resource{
						Resource: oxr,
					},
				},
			}
			_, err = f.RunFunction(context.TODO(), req)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
