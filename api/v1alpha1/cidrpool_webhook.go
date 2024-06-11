/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES
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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var cidrPoolLogger = logPkg.Log.WithName("CIDRPool-validator")

// SetupWebhookWithManager registers webhook handler in the manager
func (r *CIDRPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Validator = &CIDRPool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CIDRPool) ValidateCreate() (admission.Warnings, error) {
	cidrPoolLogger.V(1).Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CIDRPool) ValidateUpdate(_ runtime.Object) (admission.Warnings, error) {
	cidrPoolLogger.V(1).Info("validate update", "name", r.Name)
	return r.validate()
}

func (r *CIDRPool) validate() (admission.Warnings, error) {
	errList := r.Validate()
	if len(errList) == 0 {
		cidrPoolLogger.V(1).Info("validation succeed")
		return nil, nil
	}
	err := errList.ToAggregate()
	cidrPoolLogger.V(1).Info("validation failed", "reason", err.Error())
	return nil, err
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CIDRPool) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
