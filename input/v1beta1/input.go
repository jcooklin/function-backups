// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=fn.service-platform.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// TODO: Add your input type here! It doesn't need to be called 'Input', you can
// rename it to anything you like.

// Input can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// BackupStorageLocation is the name of the storage location to use for backups
	BackupStorageLocation *string `json:"backupStorageLocation,omitempty"`
	// BackupSchedule is the cron schedule for the backup job
	BackupSchedule *string `json:"backupSchedule,omitempty"`
}
