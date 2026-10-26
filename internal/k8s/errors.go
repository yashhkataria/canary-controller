package k8s

import "errors"

var (
	ErrNotFound  = errors.New("deployment not found")
	ErrForbidden = errors.New("not allowed by RBAC")
)
