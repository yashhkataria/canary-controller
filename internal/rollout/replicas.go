package rollout

import "math"

func Replicas(weight int) (canary, stable int32) {
	c := int32(math.Round(float64(TotalReplicas*weight) / 100))
	if c < 0 {
		c = 0
	}
	if c > TotalReplicas {
		c = TotalReplicas
	}
	return c, TotalReplicas - c
}
