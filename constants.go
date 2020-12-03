package main

import (
	admissionApi "k8s.io/api/admission/v1"
)

const (
	defaultStorageClassNameToZoneMapping = `{
	"irancell-standard-block-storage": "irancell",
	"irancell-standard-sharedfs-storage": "irancell",
	"afranet-standard-block-storage": "afranet",
	"afranet-standard-sharedfs-storage": "afranet"
}`
	defaultZoneToPreferredStorageClassNameMapping = `{
	"irancell": "irancell-standard-block-storage",
	"afranet": "afranet-standard-block-storage"
}`
	defaultDefaultStorageClassName            = "irancell-standard-block-storage"
	storageClassNameToZoneMapping_Env         = "STORAGECLASSNAME_TO_ZONE_MAP"
	zoneToPreferedStorageClassNameMapping_Env = "ZONE_TO_PREFERRED_STORAGECLASSNAME_MAP"
	mutateInSystemNS_Env                      = "MUTATE_IN_SYSTEM_NS"
	defaultStorageClassName_ENV               = "DEFAULT_STORAGECLASSNAME"
	ZoneKey                                   = "topology.kubernetes.io/zone"
)

var (
	trueStrings      = []string{"true", "yes", "1", "ok"}
	noChangeResponse = &admissionApi.AdmissionResponse{Allowed: true}
)
