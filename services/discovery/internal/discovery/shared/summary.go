package shared

import "github.com/telark/telark/internal/data/resources/application"

func ResourceSummaryFromKindCounts(kindCounts map[string]int) application.ResourceSummary {
	return application.ResourceSummary{
		Deployment:              kindCounts[KindDeployment],
		StatefulSet:             kindCounts[KindStatefulSet],
		DaemonSet:               kindCounts[KindDaemonSet],
		Job:                     kindCounts[KindJob],
		CronJob:                 kindCounts[KindCronJob],
		Service:                 kindCounts[KindService],
		NetworkPolicy:           kindCounts[KindNetworkPolicy],
		Ingress:                 kindCounts[KindIngress],
		ServiceAccount:          kindCounts[KindServiceAccount],
		ConfigMap:               kindCounts[KindConfigMap],
		Secret:                  kindCounts[KindSecret],
		PersistentVolumeClaim:   kindCounts[KindPersistentVolumeClaim],
		HorizontalPodAutoscaler: kindCounts[KindHorizontalPodAutoscaler],
		VerticalPodAutoscaler:   kindCounts[KindVerticalPodAutoscaler],
	}
}
