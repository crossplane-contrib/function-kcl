package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

// When a KCL program fails, function-kcl returns a fatal result whose message is
// the raw KCL diagnostic, e.g.
//
//	failed to run kcl function pipelines: EvaluationError
//	---> File prog.k:335: Networking mode is public but ... is not true in environment config
//
// Crossplane copies that message verbatim into the composite's Synced condition,
// and in platforms that expose composites to end users (directly, or synced into
// another control plane) it becomes user-visible. That leaks implementation
// details: source line numbers, internal config names, runtime errors such as
// "invalid value 'NoneType' to load attribute 'foo'".
//
// A trailing pipeline step cannot fix this: a fatal result stops the pipeline,
// so no later function runs. The function that produced the error has to clean
// it up itself.
//
// The KCL runtime returns a deliberate `assert cond, "msg"` and an unexpected
// runtime failure in the same format, so the two cannot be told apart by parsing
// alone. Authors therefore opt a message in by prefixing it with a marker:
//
//	assert _baseDomain, "[user] baseDomain is required when DNS is enabled"
//
// With sanitization enabled, a KCL error whose diagnostic message starts with
// the marker is returned with only the text after the marker. Any other KCL
// error is returned as a generic message. The full error is always logged, so
// operators keep the detail.
//
// Opt-in via FUNCTION_KCL_SANITIZE_ERRORS=true. The marker and the generic
// message are configurable.
const (
	envSanitizeErrors        = "FUNCTION_KCL_SANITIZE_ERRORS"
	envUserErrorMarker       = "FUNCTION_KCL_USER_ERROR_MARKER"
	envSanitizedErrorMessage = "FUNCTION_KCL_SANITIZED_ERROR_MESSAGE"

	defaultUserErrorMarker       = "[user]"
	defaultSanitizedErrorMessage = "Unexpected error while composing resources. Contact your platform administrator."
)

// errorSanitizer rewrites KCL errors into user-safe messages. A nil
// *errorSanitizer means sanitization is disabled.
type errorSanitizer struct {
	generic string
	// user matches the marker at the start of a KCL diagnostic message: right
	// after the `---> File <path>:<line>[:<col>]: ` header, or after the
	// `Check failed on the condition: ` prefix that schema check blocks add.
	// Anchoring there means a marker that merely appears inside a runtime
	// error (e.g. in an interpolated value) is not treated as an opt-in.
	user *regexp.Regexp
}

func newErrorSanitizerFromEnv() *errorSanitizer {
	if !envBool(envSanitizeErrors, false) {
		return nil
	}
	return newErrorSanitizer(envString(envUserErrorMarker, defaultUserErrorMarker), envString(envSanitizedErrorMessage, defaultSanitizedErrorMessage))
}

// newErrorSanitizer returns a sanitizer that passes through diagnostics opted in
// with marker and replaces everything else with generic.
func newErrorSanitizer(marker, generic string) *errorSanitizer {
	return &errorSanitizer{
		generic: generic,
		// (?m) anchors ^ at each line, so the match starts at a diagnostic
		// header; (?s) lets a multi-line assert message through. A KCL
		// diagnostic ends at the next blank line.
		user: regexp.MustCompile(`(?ms)^---> File [^\n]*?:\d+(?::\d+)?: (?:Check failed on the condition: )?` + regexp.QuoteMeta(marker) + `(.*?)(?:\n\n|\z)`),
	}
}

func (s *errorSanitizer) enabled() bool { return s != nil }

// message returns the user-safe message for a KCL error.
func (s *errorSanitizer) message(err error) string {
	m := s.user.FindStringSubmatch(err.Error())
	if m == nil {
		return s.generic
	}
	if msg := strings.TrimSpace(m[1]); msg != "" {
		return msg
	}
	return s.generic
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
