package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"kcl-lang.io/krm-kcl/pkg/kio"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane/function-template-go/input/v1beta1"

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

	// The composed resources desired by any previous Functions in the pipeline.
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired composed resources from %T", req))
		return rsp, nil
	}
	log.Debug(fmt.Sprintf("DesiredComposed resources: %d", len(desired)))
	// The composed resources desired by any previous Functions in the pipeline.
	observed, err := request.GetObservedComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired composed resources from %T", req))
		return rsp, nil
	}
	log.Debug(fmt.Sprintf("ObservedComposed resources: %d", len(observed)))

	// Input Example: https://github.com/kcl-lang/krm-kcl/blob/main/examples/mutation/set-annotations/suite/good.yaml
	inputBytes, outputBytes := bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{})
	kclRunBytes, err := yaml.Marshal(in)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}
	inputBytes.Write(kclRunBytes)
	// Run pipeline to get the result mutated or validated by the KCL source.
	pipeline := kio.NewPipeline(inputBytes, outputBytes, false)
	if err := pipeline.Execute(); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}

	output := successOutput{
		target: in.Spec.Target,
	}
	conf := addResourcesConf{
		overwrite: true,
	}

	data, err := buildData(outputBytes.Bytes())

	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot match resources to desired"))
		return rsp, nil
	}

	switch in.Spec.Target {
	case v1beta1.XR:
		conf.data = data
		if err := addResourcesTo(dxr, conf); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot add resources to XR"))
			return rsp, nil
		}
		output.object = dxr
		output.msgCount = 1
	case v1beta1.PatchDesired:
		log.Debug("Matching PatchDesired Resources")
		desiredMatches, err := matchResources(desired, data)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot match resources to desired"))
			return rsp, nil
		}
		log.Debug(fmt.Sprintf("Matched %+v", desiredMatches))

		if err := addResourcesTo(desiredMatches, conf); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot update existing DesiredComposed"))
			return rsp, nil
		}
		output.object = data
		output.msgCount = len(data)
	case v1beta1.PatchResources:
		// Render the List of DesiredComposed resources from the input
		// Update the existing desired map to be created as a base
		for _, r := range in.Spec.Resources {
			tmp := &resource.DesiredComposed{Resource: composed.New()}

			if err := renderFromJSON(tmp.Resource, r.Base.Raw); err != nil {
				response.Fatal(rsp, errors.Wrapf(err, "cannot parse base template of composed resource %q", r.Name))
				return rsp, nil
			}

			desired[resource.Name(r.Name)] = tmp
		}
		// Match the data to the desired resources
		desiredMatches, err := matchResources(desired, data)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot match resources to input resources"))
			return rsp, nil
		}

		if err := addResourcesTo(desiredMatches, conf); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot add resources to DesiredComposed"))
			return rsp, nil
		}
		output.object = data
		output.msgCount = len(data)
	case v1beta1.Resources:
		conf.basename = in.Name
		conf.data = data
		if err := addResourcesTo(desired, conf); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot add resources to DesiredComposed"))
			return rsp, nil
		}
		// Pass data here instead of desired
		// This is because there already may be desired objects
		output.object = data
		output.msgCount = len(data)
	}

	// Set dxr and desired state
	log.Debug(fmt.Sprintf("Setting desired XR state to %+v", dxr.Resource))
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return rsp, nil
	}

	for _, d := range desired {
		log.Debug(fmt.Sprintf("Setting DesiredComposed state to %+v", d.Resource))
	}
	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources in %T", rsp))
		return rsp, nil
	}
	log.Debug(fmt.Sprintf("Set %d resource(s) to the desired state", output.msgCount))

	// Output success
	output.setSuccessMsgs()
	for _, msg := range output.msgs {
		rsp.Results = append(rsp.Results, &fnv1beta1.Result{
			Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
			Message:  msg,
		})
	}

	log.Info("Successfully processed crossplane KCL function resources",
		"input", in.Name)

	return rsp, nil
}
