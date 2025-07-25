/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	// runtime for finding directorys
	DirRuntime "runtime"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mydomainv1alpha1 "github.com/ncau/kruize-operator/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KruizeReconciler reconciles a Kruize object
type KruizeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=my.domain,resources=kruizes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=my.domain,resources=kruizes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=my.domain,resources=kruizes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete;use

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kruize object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *KruizeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("Reconciling Kruize %v", req.NamespacedName))

	kruize := &mydomainv1alpha1.Kruize{}
	err := r.Get(ctx, req.NamespacedName, kruize)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	fmt.Println("kruize object base: ", kruize.Spec)

	// Call your deployment function with proper error handling
	err = r.deployKruize(ctx, kruize)
	if err != nil {
		logger.Error(err, "Failed to deploy Kruize")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	fmt.Println("Deployment initiated, waiting for pods to be ready")

	// Determine the target namespace based on cluster type
	var targetNamespace string
	switch kruize.Spec.Cluster_type {
	case "openshift":
		targetNamespace = kruize.Spec.Namespace
	case "minikube":
		targetNamespace = "monitoring"
	default:
		targetNamespace = "test-operator"
	}

	// Wait for Kruize pods to be ready
	err = r.waitForKruizePods(ctx, targetNamespace, 5*time.Minute)
	if err != nil {
		logger.Error(err, "Kruize pods not ready yet")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("All Kruize pods are ready!", "namespace", targetNamespace)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *KruizeReconciler) waitForKruizePods(ctx context.Context, namespace string, timeout time.Duration) error {
	logger := log.FromContext(ctx)

	requiredPods := []string{"kruize", "kruize-ui", "kruize-db"}
	logger.Info("Waiting for Kruize pods to be ready", "namespace", namespace, "pods", requiredPods)

	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for Kruize pods to be ready in namespace %s", namespace)

		case <-ticker.C:
			readyPods, totalPods, podStatus, err := r.checkKruizePodsStatus(ctx, namespace)
			if err != nil {
				logger.Error(err, "Failed to check pod status")
				continue
			}

			logger.Info("Pod status check", "ready", readyPods, "total", totalPods, "namespace", namespace)
			fmt.Printf("Pod status: %v\n", podStatus)

			// Check if we have all required pods running
			if readyPods >= 3 && totalPods >= 3 {
				logger.Info("All Kruize pods are ready", "readyPods", readyPods)
				return nil
			}

			logger.Info("Waiting for more pods to be ready", "ready", readyPods, "total", totalPods)
		}
	}
}

func (r *KruizeReconciler) checkKruizePodsStatus(ctx context.Context, namespace string) (int, int, map[string]string, error) {
	podList := &corev1.PodList{}
	err := r.Client.List(ctx, podList, client.InNamespace(namespace))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var kruizePods []corev1.Pod
	kruizeKeywords := []string{"kruize"}
	podStatus := make(map[string]string)

	// Filter for Kruize-related pods
	for _, pod := range podList.Items {
		for _, keyword := range kruizeKeywords {
			if strings.Contains(strings.ToLower(pod.Name), keyword) {
				kruizePods = append(kruizePods, pod)
				break
			}
		}
	}

	readyCount := 0
	for _, pod := range kruizePods {
		podStatus[pod.Name] = string(pod.Status.Phase)
		fmt.Printf("Pod %s status: %s\n", pod.Name, pod.Status.Phase)

		// Check if pod is ready
		if pod.Status.Phase == corev1.PodRunning {
			// Additional check for readiness conditions
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					readyCount++
					fmt.Printf("Pod %s is ready\n", pod.Name)
					break
				}
			}
		}
	}

	return readyCount, len(kruizePods), podStatus, nil
}

// Add this function to filter for Kruize-specific pods
func (r *KruizeReconciler) filterKruizePods(allPods []string) []string {
	var kruizePods []string

	// Look for pods that contain kruize-related names
	kruizeKeywords := []string{"kruize", "autotune", "kruize-ui", "kruize-db"}

	for _, pod := range allPods {
		for _, keyword := range kruizeKeywords {
			if strings.Contains(strings.ToLower(pod), keyword) {
				kruizePods = append(kruizePods, pod)
				fmt.Printf("Found Kruize-related pod: %s\n", pod)
				break
			}
		}
	}

	return kruizePods
}

func (r *KruizeReconciler) deployKruize(ctx context.Context, kruize *mydomainv1alpha1.Kruize) error {
	cluster_type := kruize.Spec.Cluster_type
	fmt.Println("Deploying Kruize for cluster type:", cluster_type)

	var autotune_ns string

	switch cluster_type {
	case "openshift":
		autotune_ns = kruize.Spec.Namespace
	case "minikube":
		autotune_ns = "monitoring"
	default:
		return fmt.Errorf("unsupported cluster type: %s", cluster_type)
	}

	// Deploy the Kruize components directly
	err := r.deployKruizeComponents(ctx, autotune_ns, cluster_type)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize components: %v", err)
	}

	fmt.Printf("Successfully deployed Kruize components to namespace: %s\n", autotune_ns)
	return nil
}

func (r *KruizeReconciler) deployKruizeComponents(ctx context.Context, namespace string, clusterType string) error {
	// Deploy Kruize DB first
	kruizeDBManifest := r.generateKruizeDBManifest(namespace)
	err := r.applyYAMLString(ctx, kruizeDBManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize DB: %v", err)
	}

	// Wait a bit for DB to initialize
	fmt.Println("Waiting for database to initialize...")
	time.Sleep(30 * time.Second)

	// Deploy Kruize main component
	kruizeManifest := r.generateKruizeManifest(namespace, clusterType)
	err = r.applyYAMLString(ctx, kruizeManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize main component: %v", err)
	}

	// Deploy Kruize UI last
	kruizeUIManifest := r.generateKruizeUIManifest(namespace)
	err = r.applyYAMLString(ctx, kruizeUIManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize UI: %v", err)
	}

	return nil
}

// TODO: clean up manifests to generate from files rather than strings
func (r *KruizeReconciler) generateKruizeManifest(namespace string, clusterType string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kruize-config
  namespace: %s
data:
  kruize.properties: |
    # Kruize Configuration
    clustertype=%s
    k8stype=openshift
    authtype=bearer
    monitoringagent=prometheus
    monitoringservice=prometheus
    savetodb=true
    dbdriver=postgresql
    hibernate_dialect=org.hibernate.dialect.PostgreSQLDialect
    hibernate_driver=org.postgresql.Driver
    hibernate_c3p0minsize=5
    hibernate_c3p0maxsize=20
    hibernate_c3p0timeout=300
    hibernate_c3p0maxstatements=50
    hibernate_hbm2ddlauto=update
    hibernate_showsql=false
    hibernate_timezone=UTC
    bulkresultslimit=1000
    deletepartitionsthreshold=30
    plots=true
    log_recommendation_metrics_level=info
    autotunemode=monitor
    emonly=false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruize
  namespace: %s
  labels:
    app: kruize
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kruize
  template:
    metadata:
      labels:
        app: kruize
    spec:
      containers:
      - name: kruize
        image: quay.io/kruize/autotune_operator:latest
        ports:
        - containerPort: 8080
        env:
        - name: CLUSTER_TYPE
          value: "%s"
        - name: LOGGING_LEVEL
          value: "info"
        - name: ROOT_LOGGING_LEVEL
          value: "INFO"
        - name: ENV_ROOT_LOGGING_LEVEL
          value: "INFO"
        - name: LOG_LEVEL
          value: "INFO"
        - name: JAVA_OPTS
          value: "-Xms256m -Xmx512m -Dlog4j2.level=INFO -Droot.logging.level=INFO"
        # Database configuration
        - name: DB_DRIVER
          value: "postgresql"
        - name: DB_URL
          value: "jdbc:postgresql://kruize-db-service:5432/kruizedb"
        - name: DB_USERNAME
          value: "kruize"
        - name: DB_PASSWORD
          value: "kruize123"
        # Kruize specific configuration
        - name: K8S_TYPE
          value: "%s"
        - name: AUTH_TYPE
          value: "bearer"
        - name: MONITORING_AGENT
          value: "prometheus"
        - name: MONITORING_SERVICE
          value: "prometheus"
        - name: SAVE_TO_DB
          value: "true"
        - name: AUTOTUNE_MODE
          value: "monitor"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        volumeMounts:
        - name: config-volume
          mountPath: /opt/app/config
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 45
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 90
          periodSeconds: 20
          timeoutSeconds: 5
          failureThreshold: 3
      volumes:
      - name: config-volume
        configMap:
          name: kruize-config
---
apiVersion: v1
kind: Service
metadata:
  name: kruize-service
  namespace: %s
spec:
  selector:
    app: kruize
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  type: ClusterIP
`, namespace, namespace, clusterType, namespace, clusterType, clusterType, namespace)
}

func (r *KruizeReconciler) generateKruizeUIManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruize-ui
  namespace: %s
  labels:
    app: kruize-ui
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kruize-ui
  template:
    metadata:
      labels:
        app: kruize-ui
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 101
        runAsGroup: 101
        fsGroup: 101
      containers:
      - name: kruize-ui
        image: quay.io/kruize/kruize-ui:latest
        ports:
        - containerPort: 8080
        env:
        - name: KRUIZE_API_URL
          value: "http://kruize-service:8080"
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          runAsUser: 101
          runAsGroup: 101
          capabilities:
            drop:
            - ALL
          seccompProfile:
            type: RuntimeDefault
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
        readinessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        volumeMounts:
        - name: nginx-cache
          mountPath: /var/cache/nginx
        - name: nginx-pid
          mountPath: /var/run
        - name: nginx-tmp
          mountPath: /tmp
      volumes:
      - name: nginx-cache
        emptyDir: {}
      - name: nginx-pid
        emptyDir: {}
      - name: nginx-tmp
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: kruize-ui-service
  namespace: %s
spec:
  selector:
    app: kruize-ui
  ports:
  - name: http
    port: 3000
    targetPort: 8080
  type: ClusterIP
`, namespace, namespace)
}

func (r *KruizeReconciler) generateKruizeDBManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-init
  namespace: %s
data:
  init.sql: |
    -- Initialize Kruize database
    CREATE DATABASE kruizedb;
    CREATE USER kruize WITH PASSWORD 'kruize123';
    GRANT ALL PRIVILEGES ON DATABASE kruizedb TO kruize;
    \c kruizedb;
    GRANT ALL ON SCHEMA public TO kruize;
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruize-db
  namespace: %s
  labels:
    app: kruize-db
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kruize-db
  template:
    metadata:
      labels:
        app: kruize-db
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 999
        runAsGroup: 999
        fsGroup: 999
      containers:
      - name: kruize-db
        image: postgres:13
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: "kruizedb"
        - name: POSTGRES_USER
          value: "kruize"
        - name: POSTGRES_PASSWORD
          value: "kruize123"
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          runAsUser: 999
          runAsGroup: 999
          capabilities:
            drop:
            - ALL
          seccompProfile:
            type: RuntimeDefault
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "300m"
        readinessProbe:
          exec:
            command:
            - pg_isready
            - -U
            - kruize
            - -d
            - kruizedb
          initialDelaySeconds: 15
          periodSeconds: 5
        livenessProbe:
          exec:
            command:
            - pg_isready
            - -U
            - kruize
            - -d
            - kruizedb
          initialDelaySeconds: 30
          periodSeconds: 10
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
        - name: postgres-init
          mountPath: /docker-entrypoint-initdb.d
      volumes:
      - name: postgres-data
        emptyDir: {}
      - name: postgres-init
        configMap:
          name: postgres-init
---
apiVersion: v1
kind: Service
metadata:
  name: kruize-db-service
  namespace: %s
spec:
  selector:
    app: kruize-db
  ports:
  - name: postgres
    port: 5432
    targetPort: 5432
  type: ClusterIP
`, namespace, namespace, namespace)
}

func (r *KruizeReconciler) applyYAMLString(ctx context.Context, yamlContent string, namespace string) error {
	fmt.Printf("Applying YAML content (size: %d bytes) to namespace: %s\n", len(yamlContent), namespace)

	docs := strings.Split(yamlContent, "---")

	// Create namespace first if it doesn't exist
	if namespace != "" {
		err := r.ensureNamespace(ctx, namespace)
		if err != nil {
			fmt.Printf("Warning: failed to ensure namespace %s: %v\n", namespace, err)
		}
	}

	var successCount, failCount int

	for i, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := dec.Decode([]byte(doc), nil, obj)
		if err != nil {
			fmt.Printf("Warning: failed to decode YAML document %d: %v\n", i, err)
			failCount++
			continue
		}

		// Handle namespace setting
		objNamespace := obj.GetNamespace()
		if objNamespace == "" && namespace != "" && !r.isClusterScopedResource(obj.GetKind()) {
			obj.SetNamespace(namespace)
		}

		// Apply security context fixes for Pod Security Standards
		if obj.GetKind() == "Deployment" || obj.GetKind() == "StatefulSet" {
			r.applySecurityContext(obj)
		}

		// Apply the object
		err = r.Client.Patch(ctx, obj, client.Apply, &client.PatchOptions{
			FieldManager: "kruize-operator",
			Force:        &[]bool{true}[0],
		})

		if err != nil {
			fmt.Printf("Warning: failed to apply %s/%s: %v\n", obj.GetKind(), obj.GetName(), err)
			failCount++
		} else {
			fmt.Printf("Successfully applied %s/%s\n", obj.GetKind(), obj.GetName())
			successCount++
		}
	}

	fmt.Printf("Applied %d resources successfully, %d failed\n", successCount, failCount)
	return nil
}

func (r *KruizeReconciler) getRunningPods(ctx context.Context, namespace string) ([]string, error) {
	var podNames []string

	podList := &corev1.PodList{}
	err := r.Client.List(ctx, podList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
		fmt.Printf("Pod name: %s\n", pod.Name)
	}

	return podNames, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KruizeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fmt.Println("Setting up the controller with the Manager")
	return ctrl.NewControllerManagedBy(mgr).
		For(&mydomainv1alpha1.Kruize{}).
		Complete(r)
}

// RootDir returns the root directory of the project
func RootDir() string {
	_, b, _, _ := DirRuntime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func (r *KruizeReconciler) applyYAMLFile(ctx context.Context, filePath string, namespace string) error {
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	fmt.Printf("Applying manifest file: %s (size: %d bytes)\n", filePath, len(yamlFile))
	docs := strings.Split(string(yamlFile), "---")

	// Create namespace first if it doesn't exist
	if namespace != "" {
		err := r.ensureNamespace(ctx, namespace)
		if err != nil {
			fmt.Printf("Warning: failed to ensure namespace %s: %v\n", namespace, err)
		}
	}

	var successCount, failCount int

	for i, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := dec.Decode([]byte(doc), nil, obj)
		if err != nil {
			fmt.Printf("Warning: failed to decode YAML document %d: %v\n", i, err)
			failCount++
			continue
		}

		// Handle namespace setting
		objNamespace := obj.GetNamespace()
		if objNamespace == "" && namespace != "" && !r.isClusterScopedResource(obj.GetKind()) {
			obj.SetNamespace(namespace)
		}

		// Apply security context fixes for Pod Security Standards
		if obj.GetKind() == "Deployment" || obj.GetKind() == "StatefulSet" {
			r.applySecurityContext(obj)
		}

		// Apply the object
		err = r.Client.Patch(ctx, obj, client.Apply, &client.PatchOptions{
			FieldManager: "kruize-operator",
			Force:        &[]bool{true}[0],
		})

		if err != nil {
			fmt.Printf("Warning: failed to apply %s/%s: %v\n", obj.GetKind(), obj.GetName(), err)
			failCount++
		} else {
			fmt.Printf("Successfully applied %s/%s\n", obj.GetKind(), obj.GetName())
			successCount++
		}
	}

	fmt.Printf("Applied %d resources successfully, %d failed\n", successCount, failCount)
	return nil
}

func (r *KruizeReconciler) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: name}, ns)
	if err != nil && errors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: name,
			},
		}
		return r.Client.Create(ctx, ns)
	}
	return err
}

func (r *KruizeReconciler) isClusterScopedResource(kind string) bool {
	clusterScopedResources := []string{
		"ClusterRole", "ClusterRoleBinding", "PersistentVolume", "StorageClass",
		"CustomResourceDefinition", "Namespace", "SecurityContextConstraints",
	}
	for _, resource := range clusterScopedResources {
		if kind == resource {
			return true
		}
	}
	return false
}

// TODO: this is a fix to RBAC but need to identify if this is the correct way to do it
func (r *KruizeReconciler) applySecurityContext(obj *unstructured.Unstructured) {
	// Get the spec.template.spec path
	spec, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "spec")
	if !found || err != nil {
		return
	}

	// Apply pod-level security context
	podSecurityContext := map[string]interface{}{
		"runAsNonRoot": true,
		"runAsUser":    int64(65534),
		"fsGroup":      int64(65534),
		"seccompProfile": map[string]interface{}{
			"type": "RuntimeDefault",
		},
	}

	err = unstructured.SetNestedMap(spec, podSecurityContext, "securityContext")
	if err != nil {
		fmt.Printf("Warning: failed to set pod security context: %v\n", err)
	}

	// Define security context for containers
	containerSecurityContext := map[string]interface{}{
		"allowPrivilegeEscalation": false,
		"runAsNonRoot":             true,
		"runAsUser":                int64(65534),
		"capabilities": map[string]interface{}{
			"drop": []interface{}{"ALL"},
		},
		"seccompProfile": map[string]interface{}{
			"type": "RuntimeDefault",
		},
	}

	// Apply to regular containers
	containers, found, err := unstructured.NestedSlice(spec, "containers")
	if found && err == nil {
		for i, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				containerMap["securityContext"] = containerSecurityContext
				containers[i] = containerMap
			}
		}
		err = unstructured.SetNestedSlice(spec, containers, "containers")
		if err != nil {
			fmt.Printf("Warning: failed to set container security contexts: %v\n", err)
		}
	}

	// Apply to init containers as well
	initContainers, found, err := unstructured.NestedSlice(spec, "initContainers")
	if found && err == nil {
		for i, container := range initContainers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				containerMap["securityContext"] = containerSecurityContext
				initContainers[i] = containerMap
			}
		}
		err = unstructured.SetNestedSlice(spec, initContainers, "initContainers")
		if err != nil {
			fmt.Printf("Warning: failed to set init container security contexts: %v\n", err)
		}
	}

	// Update the object
	err = unstructured.SetNestedMap(obj.Object, spec, "spec", "template", "spec")
	if err != nil {
		fmt.Printf("Warning: failed to update object spec: %v\n", err)
	}
}
