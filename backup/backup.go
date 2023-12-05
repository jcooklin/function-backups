package backup

import (
	"encoding/json"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type BackupTemplate struct {
	StorageLocation         string   `json:"storageLocation"`
	IncludedResources       []string `json:"includedResources"`
	IncludeClusterResources bool     `json:"includeClusterResources"`
	// IncludedNamespaces      []string `json:"includedNamespaces"`
	LabelSelector struct {
		MatchLabels map[string]string `json:"matchLabels"`
	} `json:"labelSelector"`
}

type Backup struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		BackupTemplate
	} `json:"spec"`
}

func NewBackup(name, claimNamespace, storageLocation string, resources []string) (*Object, error) {
	backup := &Backup{
		APIVersion: "velero.io/v1",
		Kind:       "Backup",
		Metadata: struct {
			Name      string "json:\"name\""
			Namespace string "json:\"namespace\""
		}{
			Name:      name,
			Namespace: "velero-system",
		},
		Spec: struct {
			BackupTemplate
		}{
			BackupTemplate{
				StorageLocation:         storageLocation,
				IncludeClusterResources: true,
				IncludedResources:       resources,
				LabelSelector: struct {
					MatchLabels map[string]string `json:"matchLabels"`
				}{
					MatchLabels: map[string]string{
						"service-platform.io/part-of-xr": name,
					},
				},
			},
		},
	}
	manifests, err := json.Marshal(backup)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal backup schedule")
	}
	return wrap(name, strings.Trim(string(manifests), "{}")), nil
}

type BackupSchedule struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Schedule                   string         `json:"schedule"`
		UseOwnerReferencesInBackup bool           `json:"useOwnerReferencesInBackup"`
		Template                   BackupTemplate `json:"template"`
	} `json:"spec"`
}

func NewBackupSchedule(name, claimNamespace, storageLocation, cronSchedule string, resources []string) (*Object, error) {
	backup := &BackupSchedule{
		APIVersion: "velero.io/v1",
		Kind:       "Schedule",
		Metadata: struct {
			Name      string "json:\"name\""
			Namespace string "json:\"namespace\""
		}{
			Name:      name,
			Namespace: "velero-system",
		},
		Spec: struct {
			Schedule                   string         `json:"schedule"`
			UseOwnerReferencesInBackup bool           `json:"useOwnerReferencesInBackup"`
			Template                   BackupTemplate `json:"template"`
		}{
			Schedule:                   cronSchedule,
			UseOwnerReferencesInBackup: true,
			Template: BackupTemplate{
				StorageLocation:         storageLocation,
				IncludeClusterResources: true,
				IncludedResources:       resources,
				LabelSelector: struct {
					MatchLabels map[string]string `json:"matchLabels"`
				}{
					MatchLabels: map[string]string{
						"service-platform.io/part-of-xr": name,
						// "service-platform.io/claim-namespace": claimNamespace,
					},
				},
			},
		},
	}
	manifests, err := json.Marshal(backup)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal backup schedule")
	}
	return wrap(name, strings.Trim(string(manifests), "{}")), nil
}

type Object struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations,omitempty"`
	} `json:"metadata"`
	Spec struct {
		ForProvider struct {
			Manifest string `json:"manifest"`
		} `json:"forProvider"`
	} `json:"spec"`
}

func wrap(name, manifest string) *Object {
	return &Object{
		APIVersion: "kubernetes.crossplane.io/v1alpha1",
		Kind:       "Object",
		Metadata: struct {
			Name        string            "json:\"name\""
			Annotations map[string]string `json:"annotations,omitempty"`
		}{
			Name:        name,
			Annotations: map[string]string{"service-platform.io/exclude-from-backup": "true"},
		},
		Spec: struct {
			ForProvider struct {
				Manifest string `json:"manifest"`
			} `json:"forProvider"`
		}{
			ForProvider: struct {
				Manifest string `json:"manifest"`
			}{
				Manifest: manifest,
			},
		},
	}
}
