package main

import (
	"context"
	"fmt"
	"maps"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/jcookl_nike/function-backups/backup"
	"github.com/jcookl_nike/function-backups/input/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	backupStorageLocation = "default"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Backup{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	// set the backup storage location if it was provided
	if in.BackupStorageLocation != nil {
		backupStorageLocation = *in.BackupStorageLocation
	}

	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get observed composite resource from %T", req))
		return rsp, nil
	}
	claimNamespace := ""
	if oxr.Resource.GetClaimReference() != nil {
		claimNamespace = oxr.Resource.GetClaimReference().Namespace
	}
	log := f.log.WithValues(
		"xr-apiversion", oxr.Resource.GetAPIVersion(),
		"xr-kind", oxr.Resource.GetKind(),
		"xr-name", oxr.Resource.GetName(),
	)

	// If the composite resource is not ready and we haven't created a backup MR yet, return nothing to do
	c := oxr.Resource.GetCondition(xpv1.TypeReady)
	xrAnnotations := oxr.Resource.GetAnnotations()
	if xrAnnotations == nil {
		xrAnnotations = map[string]string{}
	}
	if _, ok := xrAnnotations["service-platform.io/initial-backup-created"]; c.Status == corev1.ConditionFalse && !ok {
		log.Debug("Initial backup already created, skipping")
		return rsp, nil
	}
	if c.Status == corev1.ConditionFalse && oxr.Resource.GetAnnotations()["service-platform.io/initial-backup-created"] != "true" {
		log.Debug("Composite resource is not ready, skipping", "status", c.Status)
		return rsp, nil
	}

	// If the composite resource is opting out of backups, skip
	if val, ok := oxr.Resource.GetAnnotations()["service-platform.io/backup-disabled"]; ok && strings.ToLower(val) == "true" {
		log.Debug("Composite resource is opting out of backups, skipping")
		return rsp, nil
	}

	// The desired composed resources.
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired composed resources from %T", req))
		return rsp, nil
	}

	// Add backup labels to the desired composed resources
	for name, dr := range desired {
		//skip the backup MR and backup schedule MR
		if name == "composition-backup" || name == "composition-backup-schedule" {
			continue
		}
		labels := dr.Resource.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		maps.Copy(labels, map[string]string{
			"service-platform.io/part-of-xr": oxr.Resource.GetName(),
		})
		dr.Resource.SetLabels(labels)
	}

	resourcesToBackup := []string{}
	tmpMap := map[string]interface{}{}
	for _, dr := range desired {
		gvk := schema.FromAPIVersionAndKind(dr.Resource.GetAPIVersion(), dr.Resource.GetKind())
		if dr.Resource.GetAnnotations()["service-platform.io/exclude-from-backup"] == "true" {
			continue
		}
		tmpMap[fmt.Sprintf("%s.%s", gvk.Kind, gvk.Group)] = nil
	}
	for k := range tmpMap {
		resourcesToBackup = append(resourcesToBackup, k)
	}

	b := backup.NewBackup(oxr.Resource.GetName(),
		claimNamespace,
		backupStorageLocation,
		resourcesToBackup,
	)
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(b)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot convert backup to unstructured"))
		return rsp, nil
	}
	desiredBackup := resource.NewDesiredComposed()
	desiredBackup.Resource.Object = obj
	desired[resource.Name("composition-backup")] = desiredBackup
	// are we configured to create a backup schedule?
	if in.BackupSchedule != nil {
		bs := backup.NewBackupSchedule(oxr.Resource.GetName(),
			oxr.Resource.GetClaimReference().Namespace,
			backupStorageLocation,
			*in.BackupSchedule,
			resourcesToBackup,
		)
		bsObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(bs)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "converting backup schedule to unstructured"))
			return rsp, nil
		}
		desiredBackupSchedule := resource.NewDesiredComposed()
		desiredBackupSchedule.Resource.Object = bsObj
		desired[resource.Name("composition-backup-schedule")] = desiredBackupSchedule

	}

	xrAnnotations["service-platform.io/initial-backup-created"] = "true"
	if in.BackupSchedule != nil {
		xrAnnotations["service-platform.io/initial-backup-schedule-created"] = "true"
	}
	oxr.Resource.SetAnnotations(xrAnnotations)
	response.SetDesiredCompositeResource(rsp, oxr)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources"))
		return rsp, nil
	}

	err = response.SetDesiredComposedResources(rsp, desired)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources"))
		return rsp, nil
	}

	return rsp, nil
}
