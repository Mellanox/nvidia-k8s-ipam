/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package v1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var logger = logPkg.Log.WithName("IPPool-validator")

// SetupWebhookWithManager registers webhook handler in the manager
func (r *IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.CustomValidator = &IPPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *IPPool) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	logger.V(1).Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *IPPool) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	logger.V(1).Info("validate update", "name", r.Name)
	return r.validate()
}

func (r *IPPool) validate() (admission.Warnings, error) {
	errList := r.Validate()
	if len(errList) == 0 {
		logger.V(1).Info("validation succeed")
		return nil, nil
	}
	err := errList.ToAggregate()
	logger.V(1).Info("validation failed", "reason", err.Error())
	return nil, err
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *IPPool) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
