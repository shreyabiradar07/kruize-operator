/*
Copyright (c) 2026, 2026 Red Hat, IBM Corporation and others.

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

package common

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AddFinalizer adds the provided finalizer to the object and updates it in the cluster.
// This function ensures idempotency - if the finalizer already exists, no update is performed.
func AddFinalizer(ctx context.Context, client client.Client, obj client.Object, finalizer string) error {
	log := log.FromContext(ctx)
	
	// Check if finalizer already exists to avoid unnecessary updates
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		log.V(1).Info("finalizer already present", "namespace", obj.GetNamespace(), "name", obj.GetName())
		return nil
	}
	
	log.Info("adding finalizer to object", "namespace", obj.GetNamespace(), "name", obj.GetName(), "finalizer", finalizer)
	controllerutil.AddFinalizer(obj, finalizer)
	
	err := client.Update(ctx, obj)
	if err != nil {
		log.Error(err, "failed to add finalizer to object", "namespace", obj.GetNamespace(),
			"name", obj.GetName())
		return fmt.Errorf("failed to add finalizer to object: %w", err)
	}
	
	log.Info("successfully added finalizer", "namespace", obj.GetNamespace(), "name", obj.GetName())
	return nil
}

// RemoveFinalizer removes the provided finalizer from the object and updates it in the cluster.
// This function ensures idempotency - if the finalizer doesn't exist, no update is performed.
func RemoveFinalizer(ctx context.Context, client client.Client, obj client.Object, finalizer string) error {
	log := log.FromContext(ctx)
	
	// Check if finalizer exists before attempting removal
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		log.V(1).Info("finalizer not present, nothing to remove", "namespace", obj.GetNamespace(), "name", obj.GetName())
		return nil
	}
	
	log.Info("removing finalizer from object", "namespace", obj.GetNamespace(), "name", obj.GetName(), "finalizer", finalizer)
	controllerutil.RemoveFinalizer(obj, finalizer)
	
	err := client.Update(ctx, obj)
	if err != nil {
		log.Error(err, "failed to remove finalizer from object", "namespace", obj.GetNamespace(),
			"name", obj.GetName())
		return fmt.Errorf("failed to remove finalizer from object: %w", err)
	}
	
	log.Info("successfully removed finalizer", "namespace", obj.GetNamespace(), "name", obj.GetName())
	return nil
}

// HasFinalizer checks if the object has the specified finalizer.
func HasFinalizer(obj client.Object, finalizer string) bool {
	return controllerutil.ContainsFinalizer(obj, finalizer)
}

// IsBeingDeleted checks if the object is marked for deletion by checking if DeletionTimestamp is set.
func IsBeingDeleted(obj client.Object) bool {
	return obj.GetDeletionTimestamp() != nil
}

// HandleFinalizer is a convenience function that manages the complete finalizer lifecycle.
// It adds the finalizer if the object is not being deleted, or runs finalization logic
// and removes the finalizer if the object is being deleted.
//
// Parameters:
//   - ctx: The context for the operation
//   - client: The Kubernetes client
//   - obj: The object to manage finalizers for
//   - finalizer: The finalizer string to use
//   - finalizeFn: The function to call when finalizing (cleaning up resources)
//
// Returns:
//   - needsRequeue: true if the reconciliation should be requeued
//   - err: any error that occurred
func HandleFinalizer(ctx context.Context, client client.Client, obj client.Object, 
	finalizer string, finalizeFn func(context.Context) error) (needsRequeue bool, err error) {
	
	log := log.FromContext(ctx)
	
	// Check if the object is being deleted
	if IsBeingDeleted(obj) {
		if HasFinalizer(obj, finalizer) {
			// Run finalization logic
			log.Info("object is being deleted, running finalization logic", 
				"namespace", obj.GetNamespace(), "name", obj.GetName())
			
			if err := finalizeFn(ctx); err != nil {
				log.Error(err, "failed to finalize object", 
					"namespace", obj.GetNamespace(), "name", obj.GetName())
				// Return error to retry finalization
				return false, fmt.Errorf("failed to finalize object: %w", err)
			}
			
			// Remove finalizer to allow deletion
			if err := RemoveFinalizer(ctx, client, obj, finalizer); err != nil {
				log.Error(err, "failed to remove finalizer after finalization", 
					"namespace", obj.GetNamespace(), "name", obj.GetName())
				return false, err
			}
			
			log.Info("finalization complete, object can now be deleted", 
				"namespace", obj.GetNamespace(), "name", obj.GetName())
		}
		// Object is being deleted and finalizer is removed (or was never present)
		// No need to requeue
		return false, nil
	}
	
	// Object is not being deleted, ensure finalizer is present
	if !HasFinalizer(obj, finalizer) {
		if err := AddFinalizer(ctx, client, obj, finalizer); err != nil {
			log.Error(err, "failed to add finalizer", 
				"namespace", obj.GetNamespace(), "name", obj.GetName())
			return false, err
		}
		// Finalizer was just added, requeue to ensure it's persisted
		return true, nil
	}
	
	// Finalizer is present and object is not being deleted
	// Continue with normal reconciliation
	return false, nil
}
