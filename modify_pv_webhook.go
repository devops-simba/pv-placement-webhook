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

type PvPlacementModificationWebhook struct {
	mutatePvInSystemNamespaces            bool
	defaultStorageClassName               string
	storageClassNameToZoneMapping         map[string]string
	zoneToPreferedStorageClassNameMapping map[string]string
}

func NewPvPlacementModifier() *PvPlacementModificationWebhook {
	return &PvPlacementModificationWebhook{
		defaultStorageClassName:               helpers.ReadEnv(defaultStorageClassName_ENV, defaultDefaultStorageClassName),
		mutatePvInSystemNamespaces:            toBool(helpers.ReadEnv(mutateInSystemNS_Env, "no")),
		storageClassNameToZoneMapping:         loadStorageClassNameToZoneMapping(),
		zoneToPreferedStorageClassNameMapping: loadZoneToPreferredStorageClassNameMapping(),
	}
}

func (this *PvPlacementModificationWebhook) Name() string {
	return "pv-placement-modifier"
}
func (this *PvPlacementModificationWebhook) Type() webhookCore.AdmissionWebhookType {
	return webhookCore.MutatingAdmissionWebhook
}
func (this *PvPlacementModificationWebhook) Rules() []admissionRegistration.RuleWithOperations {
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
func (this *PvPlacementModificationWebhook) Configurations() []webhookCore.WebhookConfiguration {
	return []webhookCore.WebhookConfiguration{
		webhookCore.CreateConfig(defaultStorageClassName_ENV, defaultDefaultStorageClassName,
			"Default storage class of the system"),
		webhookCore.CreateConfig(mutateInSystemNS_Env, "false",
			"Should we modify PVs that defined in system namespaces?"),
		webhookCore.CreateConfig(storageClassNameToZoneMapping_Env, defaultStorageClassNameToZoneMapping,
			"Mapping that assign storageClassName to a zone, so we can add nodeAffinity for that"),
		webhookCore.CreateConfig(zoneToPreferedStorageClassNameMapping_Env, defaultZoneToPreferredStorageClassNameMapping,
			"Mapping that indicate what is the preferred storageClassName for a zone"),
	}
}
func (this *PvPlacementModificationWebhook) TimeoutInSeconds() int {
	return webhookCore.DefaultTimeoutInSeconds
}
func (this *PvPlacementModificationWebhook) SupportedAdmissionVersions() []string {
	return webhookCore.SupportedAdmissionVersions
}
func (this *PvPlacementModificationWebhook) SideEffects() admissionRegistration.SideEffectClass {
	return admissionRegistration.SideEffectClassNone
}
func (this *PvPlacementModificationWebhook) Initialize() {}
func (this *PvPlacementModificationWebhook) HandleAdmission(
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
	if !this.mutatePvInSystemNamespaces &&
		webhookCore.IsObjectInNamespaces(&pv.ObjectMeta, webhookCore.IgnoredNamespaces) {
		log.Infof("PV is in a system namespace. Ignoring modification")
		return noChangeResponse, nil
	}

	var patches []webhookCore.PatchOperation
	storageClassName := pv.Spec.StorageClassName

	// If object missing a storageClass, we lookup its namespace and try to guess storageClassName
	if storageClassName == "" || storageClassName == this.defaultStorageClassName {
		log.V(10).Infof("PV have no storageClassName or its storageClassName is default, trying to read Zone from the namespace(%s)", pv.Spec.ClaimRef.Namespace)
		namespacePreferredZone, err := getNamespacePreferredZone(pv.Spec.ClaimRef.Namespace)
		if err != nil {
			log.Warningf("Failed to read namespace of the PV: %v", err)
		} else if namespacePreferredZone != "" {
			if preferredStorageClassName, ok := this.zoneToPreferedStorageClassNameMapping[namespacePreferredZone]; ok {
				log.V(10).Infof("Default StorageClassName for zone(%s) is %s", namespacePreferredZone, preferredStorageClassName)
				storageClassName = preferredStorageClassName
				if storageClassName == "" {
					patches = append(patches, webhookCore.NewAddPatch("/spec/storageClassName", storageClassName))
				} else {
					patches = append(patches, webhookCore.NewReplacePatch("/spec/storageClassName", storageClassName))
				}
			} else {
				log.Errorf("There is no preferred storageClassName for zone %s", namespacePreferredZone)
			}
		}
		if storageClassName == "" {
			return noChangeResponse, nil // this must rejected by validator
		}
	}

	// If PV already have a nodeAffinity and there is a matchExpression for topology.kubernetes.io/zone
	// we don't want to update it
	zoneKey, err := findZoneAffinity(&pv)
	if err != nil {
		return nil, err
	}

	// if we have a zone for this storageClass and user does not provide any zone mapping, we set it on PV
	if mappedZone, ok := this.storageClassNameToZoneMapping[storageClassName]; zoneKey == "" && ok {
		// we have a mapping for this storage class
		terms := []corev1.NodeSelectorTerm{}
		zoneTerm := corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				corev1.NodeSelectorRequirement{
					Key:      ZoneKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{mappedZone},
				},
			},
		}

		if pv.Spec.NodeAffinity == nil {
			log.V(10).Info("nodeAffinity is nil")
			nodeAffinity := corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{zoneTerm},
				},
			}
			patches = append(patches,
				webhookCore.NewAddPatch("/spec/nodeAffinity", nodeAffinity))
		} else if pv.Spec.NodeAffinity.Required == nil {
			log.V(10).Info("nodeAffinity.required is nil")
			required := &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{zoneTerm},
			}

			patches = append(patches,
				webhookCore.NewAddPatch("/spec/nodeAffinity/required", required))
		} else if len(pv.Spec.NodeAffinity.Required.NodeSelectorTerms) != 0 {
			log.V(10).Infof("old value of nodeAffinity.required.nodeSelectorTerm: %#v", pv.Spec.NodeAffinity.Required.NodeSelectorTerms)
			terms = append(terms, pv.Spec.NodeAffinity.Required.NodeSelectorTerms...)
			terms = append(terms, zoneTerm)
			patches = append(patches,
				webhookCore.NewReplacePatch("/spec/nodeAffinity/required/nodeSelectorTerms", terms))
		} else {
			log.V(10).Info("nodeAffinity.required.nodeSelectorTerms is nil")
			terms = append(terms, zoneTerm)
			patches = append(patches,
				webhookCore.NewAddPatch("/spec/nodeAffinity/required/nodeSelectorTerms", terms))
		}
	}

	if len(patches) != 0 {
		return webhookCore.CreatePatchResponse(patches)
	}
	return noChangeResponse, nil
}
