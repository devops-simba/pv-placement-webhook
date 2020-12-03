package main

import (
	"encoding/json"
	"strings"

	"github.com/devops-simba/helpers"
	webhookCore "github.com/devops-simba/webhook_core"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/golang/glog"
)

func toBool(value string) bool {
	return helpers.ContainsString(trueStrings, strings.ToLower(value))
}
func loadStorageClassNameToZoneMapping() (storageClassNameToZoneMapping map[string]string) {
	storageClassNameToZoneMappingDefinition := helpers.ReadEnv(storageClassNameToZoneMapping_Env, defaultStorageClassNameToZoneMapping)
	if err := json.Unmarshal([]byte(storageClassNameToZoneMappingDefinition), &storageClassNameToZoneMapping); err != nil {
		panic(err)
	}

	return
}
func loadZoneToPreferredStorageClassNameMapping() (zoneToPreferredStorageClassName map[string]string) {
	zonePreferredStorageClassNameDefinition := helpers.ReadEnv(zoneToPreferedStorageClassNameMapping_Env, defaultZoneToPreferredStorageClassNameMapping)
	if err := json.Unmarshal([]byte(zonePreferredStorageClassNameDefinition), &zoneToPreferredStorageClassName); err != nil {
		panic(err)
	}

	return
}

func getNamespacePreferredZone(ns string) (string, error) {
	namespace, err := webhookCore.GetNamespace(ns, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if nodeSelector, ok := namespace.Annotations["openshift.io/node-selector"]; ok && nodeSelector != "" {
		specs := strings.Split(nodeSelector, ",")
		for _, spec := range specs {
			keyValue := strings.Split(spec, "=")
			if len(keyValue) == 1 {
				keyValue = strings.Split(spec, ":")
			}

			if len(keyValue) == 2 {
				if keyValue[0] == ZoneKey && keyValue[1] != "" {
					log.V(10).Infof("Preferred zone of namespace(%s) is %s", ns, keyValue[1])
					return keyValue[1], nil
				}
			}
		}
	}
	return "", nil
}
func findZoneAffinity(pv *corev1.PersistentVolume) (string, error) {
	if pv.Spec.NodeAffinity == nil {
		return "", nil
	}
	if pv.Spec.NodeAffinity.Required == nil {
		return "", nil
	}

	for i := 0; i < len(pv.Spec.NodeAffinity.Required.NodeSelectorTerms); i++ {
		term := &pv.Spec.NodeAffinity.Required.NodeSelectorTerms[i]
		for j := 0; j < len(term.MatchExpressions); j++ {
			expr := &term.MatchExpressions[j]
			if expr.Key == ZoneKey {
				if expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) == 1 {
					return expr.Values[0], nil
				}
				return "", helpers.StringError("Invalid ZoneKey affinity")
			}
		}
	}
	return "", nil
}
