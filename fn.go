package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime"
	"kcl-lang.io/krm-kcl/pkg/kio"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"

	"kcl-lang.io/crossplane-kcl/input/v1beta1"
	pkgresource "kcl-lang.io/crossplane-kcl/pkg/resource"

	"sigs.k8s.io/yaml"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	log := f.log.WithValues("tag", req.GetMeta().GetTag())
	log.Info("Running Function")

	rsp := response.To(req, response.DefaultTTL)
	in := &v1beta1.KCLInput{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}
	if err := in.Validate(); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "invalid function input"))
		return rsp, nil
	}

	// The composite resource that actually exists.
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}
	if in.Spec.Params == nil {
		in.Spec.Params = make(map[string]runtime.RawExtension)
	}
	in.Spec.Params["oxr"], err = pkgresource.UnstructuredToRawExtension(&oxr.Resource.Unstructured)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}
	log = log.WithValues(
		"xr-version", oxr.Resource.GetAPIVersion(),
		"xr-kind", oxr.Resource.GetKind(),
		"xr-name", oxr.Resource.GetName(),
		"target", in.Spec.Target,
	)

	// The composite resource desired by previous functions in the pipeline.
	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get desired composite resource"))
		return rsp, nil
	}
	dxr.Resource.SetAPIVersion(oxr.Resource.GetAPIVersion())
	dxr.Resource.SetKind(oxr.Resource.GetKind())
	in.Spec.Params["dxr"], err = pkgresource.UnstructuredToRawExtension(&dxr.Resource.Unstructured)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// The composed resources desired by any previous Functions in the pipeline.
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired composed resources from %T", req))
		return rsp, nil
	}
	log.Debug(fmt.Sprintf("DesiredComposed resources: %d", len(desired)))
	in.Spec.Params["dcds"], err = pkgresource.ObjToRawExtension(desired)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// The composed resources desired by any previous Functions in the pipeline.
	observed, err := request.GetObservedComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get observed composed resources from %T", req))
		return rsp, nil
	}
	log.Debug(fmt.Sprintf("ObservedComposed resources: %d", len(observed)))
	in.Spec.Params["ocds"], err = pkgresource.ObjToRawExtension(desired)
	if err != nil {
		response.Fatal(rsp, err)
		return rsp, nil
	}

	// Input Example: https://github.com/kcl-lang/krm-kcl/blob/main/examples/mutation/set-annotations/suite/good.yaml
	inputBytes, outputBytes := bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{})
	kclRunBytes, err := yaml.Marshal(in)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot marshal input to yaml"))
		return rsp, nil
	}
	inputBytes.Write(kclRunBytes)
	// Run pipeline to get the result mutated or validated by the KCL source.
	pipeline := kio.NewPipeline(inputBytes, outputBytes, false)
	if err := pipeline.Execute(); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "failed to run kcl function pipelines"))
		return rsp, nil
	}

	data, err := pkgresource.DataResourcesFromYaml(outputBytes.Bytes())
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot parse data resources from the pipeline output in %T", rsp))
		return rsp, nil
	}

	var resources pkgresource.ResourceList
	for _, r := range in.Spec.Resources {
		base, err := pkgresource.JsonByteToUnstructured(r.Base.Raw)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot parse data resources from the pipeline output in %T", rsp))
			return rsp, nil
		}
		resources = append(resources, pkgresource.Resource{
			Name: r.Name,
			Base: *base,
		})
	}
	result, err := pkgresource.ProcessResources(dxr, oxr, desired, observed, in.Spec.Target, resources, &pkgresource.AddResourcesOptions{
		Basename:  in.Name,
		Data:      data,
		Overwrite: true,
	})
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot process xr and state with the pipeline output in %T", rsp))
		return rsp, nil
	}

	// Set dxr and desired state
	log.Debug(fmt.Sprintf("Setting desired XR state to %+v", dxr.Resource))
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return rsp, nil
	}
	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources in %T", rsp))
		return rsp, nil
	}

	log.Debug(fmt.Sprintf("Set %d resource(s) to the desired state", result.MsgCount))
	for _, msg := range result.Msgs {
		rsp.Results = append(rsp.Results, &fnv1beta1.Result{
			Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
			Message:  msg,
		})
	}

	log.Info("Successfully processed crossplane KCL function resources", "input", in.Name)

	return rsp, nil
}
