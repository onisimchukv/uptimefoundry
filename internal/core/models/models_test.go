// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResourceSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	res := Resource{
		ID:            "res-1234",
		EnvironmentID: "env-prod",
		Kind:          KindK8sPod,
		Provider:      "kubernetes",
		ExternalID:    "pod/billing-app-1",
		Name:          "billing-app-1",
		Namespace:     "production",
		Status:        "Running",
		Source:        "k8s_informer",
		HierarchyPath: "prod.cluster1.production.billing_app_1",
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("failed to marshal resource: %v", err)
	}

	var decoded Resource
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal resource: %v", err)
	}

	if decoded.Kind != KindK8sPod {
		t.Fatalf("expected kind %s, got %s", KindK8sPod, decoded.Kind)
	}
	if decoded.HierarchyPath != "prod.cluster1.production.billing_app_1" {
		t.Fatalf("hierarchy path mismatch, got %s", decoded.HierarchyPath)
	}
}

func TestRelationshipSerialization(t *testing.T) {
	rel := Relationship{
		ID:               "rel-99",
		EnvironmentID:    "env-prod",
		SourceResourceID: "pod-1",
		TargetResourceID: "host-1",
		RelationshipType: RelRunsOn,
		Confidence:       1.0,
		EvidenceSource:   "k8s_api",
		Weight:           1.2, // RTT in ms
		ValidFrom:        time.Now().UTC(),
	}

	data, err := json.Marshal(rel)
	if err != nil {
		t.Fatalf("failed to marshal relationship: %v", err)
	}

	var decoded Relationship
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal relationship: %v", err)
	}

	if decoded.RelationshipType != RelRunsOn {
		t.Fatalf("expected relationship type %s, got %s", RelRunsOn, decoded.RelationshipType)
	}
	if decoded.Weight != 1.2 {
		t.Fatalf("expected weight 1.2, got %f", decoded.Weight)
	}
}
