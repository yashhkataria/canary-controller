package k8s

import "time"

const (
	requestTimeout   = 10 * time.Second
	userAgent        = "canary-controller"
	fieldManager     = "canary-controller"
	subresourceScale = "scale"
)

const (
	stableSuffix = "-stable"
	canarySuffix = "-canary"
)
