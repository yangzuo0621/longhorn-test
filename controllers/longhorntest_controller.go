/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	k8sbatchv1 "k8s.io/api/batch/v1"
	k8scorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	testzuyadevv1 "zuya.dev/longhorn-test/api/v1"
)

const (
	clientCredentialsSecretName = "aks-e2e-client-credentials"
)

var (
	nameLetters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	nameRand    *rand.Rand
)

func init() {
	nameRand = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// LonghornTestReconciler reconciles a LonghornTest object
type LonghornTestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=test.zuya.dev.zuya.dev,resources=longhorntests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=test.zuya.dev.zuya.dev,resources=longhorntests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=test.zuya.dev.zuya.dev,resources=longhorntests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LonghornTest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *LonghornTestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.V(1).Info(fmt.Sprintf("updating %s", req))

	var longhornTest testzuyadevv1.LonghornTest
	err := r.Client.Get(ctx, req.NamespacedName, &longhornTest)
	if err != nil {
		logger.Error(err, "failed to fetch longhorn test")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info(fmt.Sprintf("image: %s", longhornTest.Spec.Image))

	goalJob := k8sbatchv1.Job{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", "longhorntest", kubenizeName(generateSessionID())),
			Namespace: longhornTest.Namespace,
		},
		Spec: k8sbatchv1.JobSpec{
			Template: k8scorev1.PodTemplateSpec{
				Spec: k8scorev1.PodSpec{
					RestartPolicy: k8scorev1.RestartPolicyNever,
					Containers: []k8scorev1.Container{
						{
							Name:  "longhorn-test",
							Image: longhornTest.Spec.Image,
							Resources: k8scorev1.ResourceRequirements{
								Requests: k8scorev1.ResourceList{
									k8scorev1.ResourceCPU:    resource.MustParse("50m"),
									k8scorev1.ResourceMemory: resource.MustParse("250Mi"),
								},
							},
							EnvFrom: []k8scorev1.EnvFromSource{
								{
									SecretRef: &k8scorev1.SecretEnvSource{
										LocalObjectReference: k8scorev1.LocalObjectReference{
											Name: clientCredentialsSecretName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	job := &k8sbatchv1.Job{}
	goalJob.ObjectMeta.DeepCopyInto(&job.ObjectMeta)
	goalJob.Spec.DeepCopyInto(&job.Spec)
	job.Namespace = longhornTest.Namespace

	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "unable to create Job for LonghornTest", "job", job)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LonghornTestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testzuyadevv1.LonghornTest{}).
		Complete(r)
}

func kubenizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")

	return name
}

func generateSessionID() string {
	return fmt.Sprintf(
		"%s-%s",
		time.Now().Format("200601020304"),
		generateRandomName(5),
	)
}

func generateRandomName(length int) string {
	if length < 1 {
		return ""
	}

	b := make([]rune, length, length)
	for i := range b {
		b[i] = nameLetters[nameRand.Intn(len(nameLetters))]
	}
	return string(b)
}
