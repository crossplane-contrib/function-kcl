package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/go-logr/logr/testr"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
)

const testGeneric = "Unexpected error."

func TestErrorSanitizerMessage(t *testing.T) {
	// The error strings are verbatim kcl-go v0.12.5 output.
	cases := map[string]struct {
		err  string
		want string
	}{
		"MarkedAssert": {
			err:  "EvaluationError\n---> File prog.k:2: [user] baseDomain is required when DNS is enabled\n\n",
			want: "baseDomain is required when DNS is enabled",
		},
		"MarkedAssertMultiline": {
			err:  "EvaluationError\n---> File prog.k:1: [user] line one\nline two\n\n",
			want: "line one\nline two",
		},
		"MarkedAssertNoTrailingBlankLine": {
			err:  "EvaluationError\n---> File prog.k:2: [user] message",
			want: "message",
		},
		"MarkedSchemaCheck": {
			err:  "EvaluationError\n---> File prog.k:5:5: Instance check failed\n\n---> File prog.k:4: Check failed on the condition: [user] n must be positive\n\n",
			want: "n must be positive",
		},
		"MarkerWithNoMessage": {
			err:  "EvaluationError\n---> File prog.k:2: [user]\n\n",
			want: testGeneric,
		},
		"UnmarkedAssert": {
			err:  "EvaluationError\n---> File prog.k:335: Networking mode is public but hostedClusterConfig.dnsSplit.enabled is not true in environment config\n\n",
			want: testGeneric,
		},
		"AssertWithoutMessage": {
			err:  "EvaluationError\n---> File prog.k:2: \n\n",
			want: testGeneric,
		},
		"RuntimeErrorNone": {
			err:  "EvaluationError\n---> File prog.k:2: invalid value 'NoneType' to load attribute 'foo'\n\n",
			want: testGeneric,
		},
		"RuntimeErrorIndex": {
			err:  "EvaluationError\n---> File prog.k:2: list index out of range: 5\n\n",
			want: testGeneric,
		},
		"TypeError": {
			err:  "\nerror[E2G22]: TypeError\n---> File prog.k:1:1: expected int, got str(str)\n\n",
			want: testGeneric,
		},
		"CompileError": {
			err:  "\nerror[E2L23]: CompileError\n---> File prog.k:1:5: name 'c' is not defined\n\n",
			want: testGeneric,
		},
		"MarkerInsideRuntimeError": {
			// The marker is only honoured at the start of a diagnostic message.
			err:  "EvaluationError\n---> File prog.k:2: invalid value 'NoneType' to load attribute '[user] x'\n\n",
			want: testGeneric,
		},
		"NotAKCLDiagnostic": {
			err:  "failed to resolve dependency oci://example.org/mod: unauthorized",
			want: testGeneric,
		},
	}

	s := newErrorSanitizer(defaultUserErrorMarker, testGeneric)
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := s.message(errors.New(tc.err)); got != tc.want {
				t.Errorf("message(): want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestErrorSanitizerCustomMarker(t *testing.T) {
	s := newErrorSanitizer("USER:", testGeneric)
	err := errors.New("EvaluationError\n---> File prog.k:2: USER: name is required\n\n")
	if got, want := s.message(err), "name is required"; got != want {
		t.Errorf("message(): want %q, got %q", want, got)
	}
	err = errors.New("EvaluationError\n---> File prog.k:2: [user] name is required\n\n")
	if got := s.message(err); got != testGeneric {
		t.Errorf("message(): the default marker must not match a custom one, got %q", got)
	}
}

func TestNewErrorSanitizerFromEnv(t *testing.T) {
	t.Run("DisabledByDefault", func(t *testing.T) {
		if newErrorSanitizerFromEnv().enabled() {
			t.Error("sanitization must be opt-in")
		}
	})
	t.Run("Enabled", func(t *testing.T) {
		t.Setenv(envSanitizeErrors, "true")
		t.Setenv(envUserErrorMarker, "USER:")
		t.Setenv(envSanitizedErrorMessage, "Something went wrong.")
		s := newErrorSanitizerFromEnv()
		if !s.enabled() {
			t.Fatal("want sanitization enabled")
		}
		if got, want := s.message(errors.New("EvaluationError\n---> File prog.k:1: oops\n\n")), "Something went wrong."; got != want {
			t.Errorf("generic message: want %q, got %q", want, got)
		}
		if got, want := s.message(errors.New("EvaluationError\n---> File prog.k:1: USER: bad input\n\n")), "bad input"; got != want {
			t.Errorf("user message: want %q, got %q", want, got)
		}
	})
	t.Run("InvalidBool", func(t *testing.T) {
		t.Setenv(envSanitizeErrors, "yes-please")
		if newErrorSanitizerFromEnv().enabled() {
			t.Error("an unparseable value must leave sanitization disabled")
		}
	})
}

func TestRunFunctionSanitizesKCLErrors(t *testing.T) {
	cases := map[string]struct {
		sanitizer *errorSanitizer
		source    string
		want      string
		wantRawIn string
	}{
		"DisabledKeepsRawError": {
			source:    "assert False, \"[user] name is required\"\nitems = []\n",
			wantRawIn: "failed to run kcl function pipelines: EvaluationError\n---> File prog.k:1: [user] name is required",
		},
		"MarkedAssert": {
			sanitizer: newErrorSanitizer(defaultUserErrorMarker, testGeneric),
			source:    "assert False, \"[user] name is required\"\nitems = []\n",
			want:      "name is required",
		},
		"UnmarkedAssert": {
			sanitizer: newErrorSanitizer(defaultUserErrorMarker, testGeneric),
			source:    "assert False, \"hostedClusterConfig.dnsSplit.enabled is not true in environment config\"\nitems = []\n",
			want:      testGeneric,
		},
		"RuntimeError": {
			sanitizer: newErrorSanitizer(defaultUserErrorMarker, testGeneric),
			source:    "a = None\nb = a.foo\nitems = []\n",
			want:      testGeneric,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srcJSON, err := json.Marshal(tc.source)
			if err != nil {
				t.Fatalf("marshal source: %v", err)
			}
			req := &fnv1.RunFunctionRequest{
				Meta: &fnv1.RequestMeta{Tag: "sanitize"},
				Input: resource.MustStructJSON(`{
					"apiVersion": "krm.kcl.dev/v1alpha1",
					"kind": "KCLInput",
					"metadata": {"name": "basic"},
					"spec": {"target": "Default", "source": ` + string(srcJSON) + `}
				}`),
				Observed: &fnv1.State{
					Composite: &fnv1.Resource{
						Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"}}`),
					},
				},
			}

			f := &Function{log: logging.NewLogrLogger(testr.New(t)), sanitizer: tc.sanitizer}
			rsp, err := f.RunFunction(context.Background(), req)
			if err != nil {
				t.Fatalf("f.RunFunction: unexpected error: %v", err)
			}

			var fatal []string
			for _, r := range rsp.GetResults() {
				if r.GetSeverity() == fnv1.Severity_SEVERITY_FATAL {
					fatal = append(fatal, r.GetMessage())
				}
			}
			if len(fatal) != 1 {
				t.Fatalf("want exactly one fatal result, got %q", fatal)
			}
			if tc.wantRawIn != "" {
				if !strings.Contains(fatal[0], tc.wantRawIn) {
					t.Errorf("want the raw error %q in the result, got %q", tc.wantRawIn, fatal[0])
				}
				return
			}
			if fatal[0] != tc.want {
				t.Errorf("fatal result: want %q, got %q", tc.want, fatal[0])
			}
		})
	}
}
