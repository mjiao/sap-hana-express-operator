/*
Copyright 2023.

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
	"k8s.io/apimachinery/pkg/util/intstr"

	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbv1alpha1 "github.com/redhat-sap/sap-hana-express-operator/api/v1alpha1"
)

const hanaExpressFinalizer = "db.sap-redhat.io/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableHanaExpress represents the status of the StatefulSet reconciliation
	typeAvailableHanaExpress = "Available"
	// typeDegradedHanaExpress represents the status used when the custom resource is deleted and the finalizer operations must occur.
	typeDegradedHanaExpress = "Degraded"
)

// HanaExpressReconciler reconciles a HanaExpress object
type HanaExpressReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=db.sap-redhat.io,resources=hanaexpresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.sap-redhat.io,resources=hanaexpresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.sap-redhat.io,resources=hanaexpresses/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HanaExpressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	hanaExpress := &dbv1alpha1.HanaExpress{}
	err := r.Get(ctx, req.NamespacedName, hanaExpress)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("HanaExpress resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get HanaExpress")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if hanaExpress.Status.Conditions == nil || len(hanaExpress.Status.Conditions) == 0 {
		meta.SetStatusCondition(&hanaExpress.Status.Conditions, metav1.Condition{Type: typeAvailableHanaExpress, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, hanaExpress); err != nil {
			log.Error(err, "Failed to update HanaExpress status")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, hanaExpress); err != nil {
			log.Error(err, "Failed to re-fetch HanaExpress")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	if !controllerutil.ContainsFinalizer(hanaExpress, hanaExpressFinalizer) {
		log.Info("Adding Finalizer for HanaExpress")
		if ok := controllerutil.AddFinalizer(hanaExpress, hanaExpressFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Get(ctx, req.NamespacedName, hanaExpress); err != nil {
			log.Error(err, "Failed to re-fetch HanaExpress")
			return ctrl.Result{}, err
		}

		if err = r.Update(ctx, hanaExpress); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the HanaExpress instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isHanaExpressMarkedToBeDeleted := hanaExpress.GetDeletionTimestamp() != nil
	if isHanaExpressMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(hanaExpress, hanaExpressFinalizer) {
			log.Info("Performing Finalizer Operations for HanaExpress before delete CR")

			meta.SetStatusCondition(&hanaExpress.Status.Conditions, metav1.Condition{Type: typeDegradedHanaExpress,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", hanaExpress.Name)})

			if err := r.Status().Update(ctx, hanaExpress); err != nil {
				log.Error(err, "Failed to update HanaExpress status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForHanaExpress(hanaExpress)

			// TODO(user): If you add operations to the doFinalizerOperationsForHanaExpress method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			if err := r.Get(ctx, req.NamespacedName, hanaExpress); err != nil {
				log.Error(err, "Failed to re-fetch HanaExpress")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&hanaExpress.Status.Conditions, metav1.Condition{Type: typeDegradedHanaExpress,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", hanaExpress.Name)})

			if err := r.Status().Update(ctx, hanaExpress); err != nil {
				log.Error(err, "Failed to update HanaExpress status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for HanaExpress after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(hanaExpress, hanaExpressFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for HanaExpress")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, hanaExpress); err != nil {
				log.Error(err, "Failed to remove finalizer for HanaExpress")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if the statefulset already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: hanaExpress.Name, Namespace: hanaExpress.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new statefulset
		sts, err := r.statefulSetForHanaExpress(hanaExpress)
		if err != nil {
			log.Error(err, "Failed to define new StatefulSet resource for HanaExpress")

			// The following implementation will update the status
			meta.SetStatusCondition(&hanaExpress.Status.Conditions, metav1.Condition{Type: typeAvailableHanaExpress,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", hanaExpress.Name, err)})

			if err := r.Status().Update(ctx, hanaExpress); err != nil {
				log.Error(err, "Failed to update HanaExpress status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new StatefulSet",
			"StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
		if err = r.Create(ctx, sts); err != nil {
			log.Error(err, "Failed to create new StatefulSet",
				"StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			return ctrl.Result{}, err
		}

		// StatefulSet created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get StatefulSet")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	size := int32(1)
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		if err = r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update StatefulSet",
				"StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)

			if err := r.Get(ctx, req.NamespacedName, hanaExpress); err != nil {
				log.Error(err, "Failed to re-fetch HanaExpress")
				return ctrl.Result{}, err
			}

			// The following implementation will update the status
			meta.SetStatusCondition(&hanaExpress.Status.Conditions, metav1.Condition{Type: typeAvailableHanaExpress,
				Status: metav1.ConditionFalse, Reason: "Resizing",
				Message: fmt.Sprintf("Failed to update the size for the custom resource (%s): (%s)", hanaExpress.Name, err)})

			if err := r.Status().Update(ctx, hanaExpress); err != nil {
				log.Error(err, "Failed to update HanaExpress status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// TODO: allow users to resize pvc

	// The following implementation will update the status
	meta.SetStatusCondition(&hanaExpress.Status.Conditions, metav1.Condition{Type: typeAvailableHanaExpress,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("StatefulSet for custom resource (%s) with %d replicas created successfully", hanaExpress.Name, size)})

	if err := r.Status().Update(ctx, hanaExpress); err != nil {
		log.Error(err, "Failed to update StatefulSet status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// doFinalizerOperationsForHanaExpress will perform the required operations before delete the CR.
func (r *HanaExpressReconciler) doFinalizerOperationsForHanaExpress(cr *dbv1alpha1.HanaExpress) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended on the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the StatefulSet will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))
}

// statefulSetForHanaExpress returns a HanaExpress StatefulSet object
func (r *HanaExpressReconciler) statefulSetForHanaExpress(
	hanaExpress *dbv1alpha1.HanaExpress) (*appsv1.StatefulSet, error) {
	ls := labelsForHanaExpress(hanaExpress.Name)
	replicas := int32(1)

	// Get the Operand image
	image, err := imageForHanaExpress()
	if err != nil {
		return nil, err
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hanaExpress.Name,
			Namespace: hanaExpress.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resourceQuantity(hanaExpress.Spec.PVCSize),
							},
						},
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					// TODO(user): Uncomment the following code to configure the nodeAffinity expression
					// according to the platforms which are supported by your solution. It is considered
					// best practice to support multiple architectures. build your manager image using the
					// makefile target docker-buildx. Also, you can use docker manifest inspect <image>
					// to check what are the platforms supported.
					// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity
					//Affinity: &corev1.Affinity{
					//	NodeAffinity: &corev1.NodeAffinity{
					//		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					//			NodeSelectorTerms: []corev1.NodeSelectorTerm{
					//				{
					//					MatchExpressions: []corev1.NodeSelectorRequirement{
					//						{
					//							Key:      "kubernetes.io/arch",
					//							Operator: "In",
					//							Values:   []string{"amd64", "arm64", "ppc64le", "s390x"},
					//						},
					//						{
					//							Key:      "kubernetes.io/os",
					//							Operator: "In",
					//							Values:   []string{"linux"},
					//						},
					//					},
					//				},
					//			},
					//		},
					//	},
					//},
					//SecurityContext: &corev1.PodSecurityContext{
					//	// RunAsNonRoot: &[]bool{true}[0],
					//	// IMPORTANT: seccomProfile was introduced with Kubernetes 1.19
					//	// If you are looking for to produce solutions to be supported
					//	// on lower versions you must remove this option.
					//	SeccompProfile: &corev1.SeccompProfile{
					//		Type: corev1.SeccompProfileTypeRuntimeDefault,
					//	},
					//},

					Volumes: []corev1.Volume{
						{
							Name: "hxepasswd",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: hanaExpress.Spec.Credential.Name,
									DefaultMode: func() *int32 {
										mode := int32(0511)
										return &mode
									}(),
								},
							},
						},
					},

					InitContainers: []corev1.Container{
						{
							Image:   "registry.access.redhat.com/ubi8/ubi:8.5-239.1651231664",
							Name:    "set-data-dir-ownership",
							Command: []string{"sh", "-c", "cp /tmp/mounts/* /hana/mounts && chown -R 12000:79 /hana/mounts"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "hxepasswd",
									MountPath: "/tmp/mounts",
								},
								{
									Name:      "data",
									MountPath: "/hana/mounts",
								},
							},
						},
					},

					Containers: []corev1.Container{
						{
							Image:           image,
							Name:            "hana-express",
							ImagePullPolicy: corev1.PullIfNotPresent,
							// Ensure restrictive context for the container
							// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
							SecurityContext: &corev1.SecurityContext{
								// WARNING: Ensure that the image used defines an UserID in the Dockerfile
								// otherwise the Pod will not run and will fail with "container has runAsNonRoot and image has non-numeric user"".
								// If you want your workloads admitted in namespaces enforced with the restricted mode in OpenShift/OKD vendors
								// then, you MUST ensure that the Dockerfile defines a User ID OR you MUST leave the "RunAsNonRoot" and
								// "RunAsUser" fields empty.
								RunAsNonRoot: &[]bool{true}[0],

								// The hanaExpress image does not use a non-zero numeric user as the default user.
								// Due to RunAsNonRoot field being set to true, we need to force the user in the
								// container to a non-zero numeric user. We do this using the RunAsUser field.
								// However, if you are looking to provide solution for K8s vendors like OpenShift
								// be aware that you cannot run under its restricted-v2 SCC if you set this value.
								RunAsUser:  &[]int64{12000}[0],
								RunAsGroup: &[]int64{79}[0],
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(39017),
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: int32(39041),
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: int32(59031),
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: int32(8090),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Command: []string{"/run_hana", "--passwords-url", "file:///hana/mounts/hxepasswd.json", "--agree-to-sap-license"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "hxepasswd",
									MountPath: "/tmp/mounts",
								},
								{
									Name:      "data",
									MountPath: "/hana/mounts",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(39017),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(hanaExpress, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}

// labelsForHanaExpress returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForHanaExpress(name string) map[string]string {
	var imageTag string
	image, err := imageForHanaExpress()
	if err == nil {
		imageTag = strings.Split(image, ":")[1]
	}
	return map[string]string{"app.kubernetes.io/name": "HanaExpress",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "hanaexpress-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// imageForHanaExpress gets the Operand image which is managed by this controller
// from the HANAEXPRESS_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForHanaExpress() (string, error) {
	var imageEnvVar = "HANAEXPRESS_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("Unable to find %s environment variable with the image", imageEnvVar)
	}
	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HanaExpressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.HanaExpress{}).
		Complete(r)
}

// Helper function to create resource quantity
func resourceQuantity(quantity string) resource.Quantity {
	q, _ := resource.ParseQuantity(quantity)
	return q
}
