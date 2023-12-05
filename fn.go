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
		if name == "composition-backup" || name == "composition-backup-schedule" {
			continue
		}
		// if dr.Resource.GetLabels() == nil {
		// 	dr.Resource.SetLabels(map[string]string{})
		// }
		// dr.Resource.GetLabels()["service-platform.io/part-of-xr"] = oxr.Resource.GetName()
		labels := dr.Resource.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		maps.Copy(labels, map[string]string{
			"service-platform.io/part-of-xr": oxr.Resource.GetName(),
		})
		dr.Resource.SetLabels(labels)
	}

	backupExists := false
	if _, ok := desired["composition-backup"]; ok {
		backupExists = true
	}

	// If the composite resource is ready and we haven't already created a backup
	// or if the backup/backupSchedule resource already exists we should calculate
	// the backup CR
	c := oxr.Resource.GetCondition(xpv1.TypeReady)
	if c.Status == corev1.ConditionTrue || backupExists {
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

		b, err := backup.NewBackup(oxr.Resource.GetName(),
			claimNamespace,
			backupStorageLocation,
			resourcesToBackup,
		)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "creating backup"))
			return rsp, nil
		}
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(b)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot convert backup to unstructured"))
			return rsp, nil
		}
		desiredBackup := resource.NewDesiredComposed()
		desiredBackup.Resource.Object = obj
		desired["composition-backup"] = desiredBackup
		// are we configured to create a backup schedule?
		if in.BackupSchedule != nil {
			bs, err := backup.NewBackupSchedule(oxr.Resource.GetName(),
				oxr.Resource.GetClaimReference().Namespace,
				backupStorageLocation,
				*in.BackupSchedule,
				resourcesToBackup,
			)
			if err != nil {
				response.Fatal(rsp, errors.Wrapf(err, "creating backup schedule"))
				return rsp, nil
			}
			bsObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(bs)
			if err != nil {
				response.Fatal(rsp, errors.Wrapf(err, "converting backup schedule to unstructured"))
				return rsp, nil
			}
			desiredBackup := resource.NewDesiredComposed()
			desiredBackup.Resource.Object = bsObj
			desired["composition-backup-schedule"] = desiredBackup
		}

	} else {
		log.Debug("Composite resource is  not ready and/or backup doesn't yet exist, skipping")
	}
	err = response.SetDesiredComposedResources(rsp, desired)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources"))
		return rsp, nil
	}

	return rsp, nil
}
