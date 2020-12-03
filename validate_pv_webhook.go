package main

import (
	"encoding/json"
	"net/http"

	"github.com/devops-simba/helpers"
	webhookCore "github.com/devops-simba/webhook_core"
	admissionApi "k8s.io/api/admission/v1"
	admissionRegistration "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"

	log "github.com/golang/glog"
)

type PvPlacementValidationWebhook struct {
	verifyPvInSystemNamespaces    bool
	storageClassNameToZoneMapping map[string]string
}

func NewPvPlacementValidator() *PvPlacementValidationWebhook {
	return &PvPlacementValidationWebhook{
		verifyPvInSystemNamespaces:    toBool(helpers.ReadEnv(mutateInSystemNS_Env, "no")),
		storageClassNameToZoneMapping: loadStorageClassNameToZoneMapping(),
	}
}

func (this *PvPlacementValidationWebhook) Name() string {
	return "pv-placement-validator"
}
func (this *PvPlacementValidationWebhook) Type() webhookCore.AdmissionWebhookType {
	return webhookCore.ValidatingAdmissionWebhook
}
func (this *PvPlacementValidationWebhook) Rules() []admissionRegistration.RuleWithOperations {
	return []admissionRegistration.RuleWithOperations{
		admissionRegistration.RuleWithOperations{
			Rule: admissionRegistration.Rule{
				APIGroups:   []string{""},
				Resources:   []string{"persistentvolumes"},
				APIVersions: []string{"*"},
				Scope:       nil, // any scope
			},
			Operations: []admissionRegistration.OperationType{
				admissionRegistration.Create,
				admissionRegistration.Update,
			},
		},
	}
}
func (this *PvPlacementValidationWebhook) Configurations() []webhookCore.WebhookConfiguration {
	return []webhookCore.WebhookConfiguration{
		// webhookCore.CreateConfig(mutateInSystemNS_Env, "false",
		// 	"Should we verifiy PVs that defined in system namespaces?"),
		// webhookCore.CreateConfig(storageClassNameToZoneMapping_Env, defaultStorageClassNameToZoneMapping,
		// 	"Mapping that assign storageClassName to a zone, so we can verify nodeAffinity for that"),
	}
}
func (this *PvPlacementValidationWebhook) TimeoutInSeconds() int {
	return webhookCore.DefaultTimeoutInSeconds
}
func (this *PvPlacementValidationWebhook) SupportedAdmissionVersions() []string {
	return webhookCore.SupportedAdmissionVersions
}
func (this *PvPlacementValidationWebhook) SideEffects() admissionRegistration.SideEffectClass {
	return admissionRegistration.SideEffectClassNone
}
func (this *PvPlacementValidationWebhook) Initialize() {}
func (this *PvPlacementValidationWebhook) HandleAdmission(
	request *http.Request,
	ar *admissionApi.AdmissionReview,
) (*admissionApi.AdmissionResponse, error) {
	// Read PV from AdmissionRequest
	var pv corev1.PersistentVolume
	if err := json.Unmarshal(ar.Request.Object.Raw, &pv); err != nil {
		log.Errorf("Could not unmarshal pv from %v: %v", string(ar.Request.Object.Raw), err)
		return nil, err
	}

	// Check if PV is in a system NS?
	if !this.verifyPvInSystemNamespaces &&
		webhookCore.IsObjectInNamespaces(&pv.ObjectMeta, webhookCore.IgnoredNamespaces) {
		log.Infof("PV is in a system namespace. Ignoring modification")
		return noChangeResponse, nil
	}

	// All PVs must have a storageClassName(or our mutating must already added it)
	if pv.Spec.StorageClassName == "" {
		return nil, helpers.StringError("Using default storageClass is not allowed, please select an specific storageClass for the PV")
	}

	if zone, ok := this.storageClassNameToZoneMapping[pv.Spec.StorageClassName]; ok {
		zoneKey, err := findZoneAffinity(&pv)
		if err != nil {
			return nil, err
		}
		if zoneKey == "" {
			return nil, helpers.StringError("Missing nodeAffinity for the zone")
		}
		if zoneKey != zone {
			return nil, helpers.StringError("Invalid nodeAffinity")
		}

		return noChangeResponse, nil
	} else {
		return nil, helpers.StringError("Invalid storageClassName")
	}
}
