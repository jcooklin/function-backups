package backup

import ()

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

func NewBackup(name, claimNamespace, storageLocation string, resources []string) *Backup {
	return &Backup{
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
						// "service-platform.io/claim-namespace": claimNamespace,
					},
				},
			},
		},
	}
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

func NewBackupSchedule(name, claimNamespace, storageLocation, cronSchedule string, resources []string) *BackupSchedule {
	return &BackupSchedule{
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
}
