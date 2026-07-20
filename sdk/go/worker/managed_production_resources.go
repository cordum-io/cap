package worker

import (
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func validateManagedProductionResources(
	request *agentv1.JobRequest, resolvers []string, now time.Time,
) error {
	if err := validateManagedProductionResource(
		"context", request.GetContextPtr(), request.GetContextRef(), resolvers, now,
	); err != nil {
		return err
	}
	compensation := request.GetCompensation()
	if compensation == nil {
		return nil
	}
	return validateManagedProductionResource(
		"compensation context", compensation.GetContextPtr(), compensation.GetContextRef(), resolvers, now,
	)
}

func validateManagedProductionResource(
	name, legacy string, reference *agentv1.ResourceRef, resolvers []string, now time.Time,
) error {
	if reference == nil {
		if legacy != "" {
			return fmt.Errorf("%w: %s requires ResourceRef", capsdk.ErrInvalidResourceRef, name)
		}
		return nil
	}
	if err := capsdk.ValidateResourceRefAt(reference, resolvers, now); err != nil {
		return err
	}
	return capsdk.ValidateResourceRefCompatibility(legacy, reference)
}

func validateManagedProductionResultResources(
	result *agentv1.JobResult, resolvers []string, now time.Time,
) error {
	return validateManagedProductionOutputResources(
		result.GetResultPtr(), result.GetResultRef(), result.GetArtifactPtrs(), result.GetArtifactRefs(), resolvers, now,
	)
}

func validateManagedProductionProgressResources(
	progress *agentv1.JobProgress, resolvers []string, now time.Time,
) error {
	return validateManagedProductionOutputResources(
		progress.GetResultPtr(), progress.GetResultRef(), progress.GetArtifactPtrs(), progress.GetArtifactRefs(), resolvers, now,
	)
}

func validateManagedProductionOutputResources(
	legacyResult string, result *agentv1.ResourceRef, legacyArtifacts []string,
	artifacts []*agentv1.ResourceRef, resolvers []string, now time.Time,
) error {
	if err := validateManagedProductionResource("result", legacyResult, result, resolvers, now); err != nil {
		return err
	}
	if len(legacyArtifacts) > 0 && len(legacyArtifacts) != len(artifacts) {
		return fmt.Errorf("%w: artifact references are ambiguous", capsdk.ErrInvalidResourceRef)
	}
	for index, artifact := range artifacts {
		legacy := ""
		if index < len(legacyArtifacts) {
			legacy = legacyArtifacts[index]
		}
		if err := validateManagedProductionResource("artifact", legacy, artifact, resolvers, now); err != nil {
			return err
		}
	}
	return nil
}
