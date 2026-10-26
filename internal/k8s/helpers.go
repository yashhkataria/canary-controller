package k8s

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func StableName(app string) string { return app + stableSuffix }

func CanaryName(app string) string { return app + canarySuffix }

func (s DeploymentStatus) ReadyAt(desired int32) bool {
	return s.ObservedGeneration >= s.Generation &&
		s.Desired == desired &&
		s.Replicas == desired &&
		s.Updated == desired &&
		s.Ready == desired &&
		s.Available == desired
}

func mapError(err error, op string) error {
	switch {
	case apierrors.IsNotFound(err):
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	case apierrors.IsForbidden(err):
		return fmt.Errorf("%s: %w: %v", op, ErrForbidden, err)
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}
