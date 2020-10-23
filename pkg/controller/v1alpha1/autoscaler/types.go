package autoscalers

import (
	"github.com/oam-dev/kubevela/api/v1alpha1"
	"k8s.io/api/autoscaling/v2beta2"
)

const (
	CPUType              v1alpha1.TriggerType = "cpu"
	MemoryType           v1alpha1.TriggerType = "memory"
	StorageType          v1alpha1.TriggerType = "storage"
	EphemeralStorageType v1alpha1.TriggerType = "ephemeral-storage"
	CronType             v1alpha1.TriggerType = "cron"

	CPUUtilization v2beta2.MetricTargetType = "Utilization"
)