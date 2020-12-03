package main

import (
	"encoding/json"
	"strings"

	"github.com/devops-simba/helpers"

	corev1 "k8s.io/api/core/v1"
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
