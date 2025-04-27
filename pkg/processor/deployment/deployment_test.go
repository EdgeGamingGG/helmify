package deployment

import (
	"testing"

	"github.com/EdgeGamingGG/helmify/pkg/config"
	"github.com/EdgeGamingGG/helmify/pkg/metadata"
	"github.com/EdgeGamingGG/helmify/pkg/processor/pod"

	"github.com/EdgeGamingGG/helmify/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	strDepl = `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: my-operator-controller-manager
  namespace: my-operator-system
spec:
  revisionHistoryLimit: 5
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - args:
        - --secure-listen-address=0.0.0.0:8443
        - --upstream=http://127.0.0.1:8080/
        - --logtostderr=true
        - --v=10
        image: gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0
        name: kube-rbac-proxy
        ports:
        - containerPort: 8443
          name: https
      - args:
        - --health-probe-bind-address=:8081
        - --metrics-bind-address=127.0.0.1:8080
        - --leader-elect
        command:
        - /manager
        volumeMounts:
        - mountPath: /controller_manager_config.yaml
          name: manager-config
          subPath: controller_manager_config.yaml
        - name: secret-volume
          mountPath: /my.ca
        - name: sample-pv-storage
          mountPath: "/usr/share/nginx/html"
        env:
        - name: VAR1
          valueFrom:
            secretKeyRef:
              name: my-operator-secret-vars
              key: VAR1
        - name: VAR2
          valueFrom:
            configMapKeyRef:
              name: my-operator-configmap-vars
              key: VAR2
        - name: VAR3
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: VAR4
          valueFrom:
            resourceFieldRef:
              resource: limits.cpu
        - name: VAR5
          value: "123"
        - name: VAR6
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['app.kubernetes.io/something']
        image: controller:latest
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        name: manager
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 100m
            memory: 30Mi
          requests:
            cpu: 100m
            memory: 20Mi
        securityContext:
          allowPrivilegeEscalation: false
      securityContext:
        runAsNonRoot: true
      serviceAccountName: my-operator-controller-manager
      terminationGracePeriodSeconds: 10
      volumes:
      - configMap:
          name: my-operator-manager-config
        name: manager-config
      - name: secret-volume
        secret:
          secretName: my-operator-secret-ca
      - name: sample-pv-storage
        persistentVolumeClaim:
          claimName: my-sample-pv-claim
`
)

func Test_deployment_Process(t *testing.T) {
	var testInstance deployment

	t.Run("processed", func(t *testing.T) {
		obj := internal.GenerateObj(strDepl)
		processed, _, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, true, processed)
	})
	t.Run("skipped", func(t *testing.T) {
		obj := internal.TestNs
		processed, _, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, false, processed)
	})
}

var singleQuotesTest = []struct {
	input    string
	expected string
}{
	{
		"{{ .Values.x }}",
		"{{ .Values.x }}",
	},
	{
		"'{{ .Values.x }}'",
		"{{ .Values.x }}",
	},
	{
		"'{{ .Values.x }}:{{ .Values.y }}'",
		"{{ .Values.x }}:{{ .Values.y }}",
	},
	{
		"'{{ .Values.x }}:{{ .Values.y \n\t| default .Chart.AppVersion}}'",
		"{{ .Values.x }}:{{ .Values.y \n\t| default .Chart.AppVersion}}",
	},
	{
		"echo 'x'",
		"echo 'x'",
	},
	{
		"abcd: x.y['x/y']",
		"abcd: x.y['x/y']",
	},
	{
		"abcd: x.y[\"'{{}}'\"]",
		"abcd: x.y[\"{{}}\"]",
	},
	{
		"image: '{{ .Values.x }}'",
		"image: {{ .Values.x }}",
	},
	{
		"'{{ .Values.x }} y'",
		"{{ .Values.x }} y",
	},
	{
		"\t\t- mountPath: './x.y'",
		"\t\t- mountPath: './x.y'",
	},
	{
		"'{{}}'",
		"{{}}",
	},
	{
		"'{{ {nested} }}'",
		"{{ {nested} }}",
	},
	{
		"'{{ '{{nested}}' }}'",
		"{{ '{{nested}}' }}",
	},
	{
		"'{{ unbalanced }'",
		"'{{ unbalanced }'",
	},
	{
		"'{{\nincomplete content'",
		"'{{\nincomplete content'",
	},
	{
		"'{{ @#$%^&*() }}'",
		"{{ @#$%^&*() }}",
	},
}

func Test_replaceSingleQuotes(t *testing.T) {
	for _, tt := range singleQuotesTest {
		t.Run(tt.input, func(t *testing.T) {
			s := replaceSingleQuotes(tt.input)
			if s != tt.expected {
				t.Errorf("got %q, want %q", s, tt.expected)
			}
		})
	}
}

func TestProcessSpec_VolumeMounts(t *testing.T) {
	tests := []struct {
		name              string
		spec              appsv1.DeploymentSpec
		expectedSpec      map[string]interface{}
		expectedPodValues map[string]interface{}
	}{
		{
			name: "hostpath volume mount",
			spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:1.14.2",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "host-data",
										MountPath: "/data",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "host-data",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/mnt/data",
										Type: ptr(corev1.HostPathDirectory),
									},
								},
							},
						},
					},
				},
			},
			expectedSpec: map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"env": []interface{}{
							map[string]interface{}{
								"name":  "KUBERNETES_CLUSTER_DOMAIN",
								"value": "{{ quote .Values.kubernetesClusterDomain }}",
							},
						},
						"image": "{{ .Values.test.testContainer.image.repository }}:{{ .Values.test.testContainer.image.tag | default .Chart.AppVersion }}",
						"name":  "test-container",
						"volumeMounts": []interface{}{
							"{{- toYaml .Values.test.testContainer.volumeMounts.hostData | nindent 10 }}",
						},
						"resources": map[string]interface{}{},
					},
				},
				"volumes": []interface{}{
					"{{- toYaml .Values.test.volumes.hostData | nindent 8 }}",
				},
			},
			expectedPodValues: map[string]interface{}{
				"test": map[string]interface{}{
					"testContainer": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "nginx",
							"tag":        "1.14.2",
						},
						"volumeMounts": map[string]interface{}{
							"hostData": map[string]interface{}{
								"mountPath": "/data",
								"name":      "host-data",
							},
						},
					},
					"volumes": map[string]interface{}{
						"hostData": map[string]interface{}{
							"name": "host-data",
							"hostPath": map[string]interface{}{
								"path": "/mnt/data",
								"type": "Directory",
							},
						},
					},
				},
			},
		},
		{
			name: "csi volume mount",
			spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:1.14.2",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "csi-volume",
										MountPath: "/mnt/csi",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "csi-volume",
								VolumeSource: corev1.VolumeSource{
									CSI: &corev1.CSIVolumeSource{
										Driver: "csi.example.com",
										VolumeAttributes: map[string]string{
											"type": "ssd",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedSpec: map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"env": []interface{}{
							map[string]interface{}{
								"name":  "KUBERNETES_CLUSTER_DOMAIN",
								"value": "{{ quote .Values.kubernetesClusterDomain }}",
							},
						},
						"image": "{{ .Values.test.testContainer.image.repository }}:{{ .Values.test.testContainer.image.tag | default .Chart.AppVersion }}",
						"name":  "test-container",
						"volumeMounts": []interface{}{
							"{{- toYaml .Values.test.testContainer.volumeMounts.csiVolume | nindent 10 }}",
						},
						"resources": map[string]interface{}{},
					},
				},
				"volumes": []interface{}{
					"{{- toYaml .Values.test.volumes.csiVolume | nindent 8 }}",
				},
			},
			expectedPodValues: map[string]interface{}{
				"test": map[string]interface{}{
					"testContainer": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "nginx",
							"tag":        "1.14.2",
						},
						"volumeMounts": map[string]interface{}{
							"csiVolume": map[string]interface{}{
								"mountPath": "/mnt/csi",
								"name":      "csi-volume",
							},
						},
					},
					"volumes": map[string]interface{}{
						"csiVolume": map[string]interface{}{
							"name": "csi-volume",
							"csi": map[string]interface{}{
								"driver": "csi.example.com",
								"volumeAttributes": map[string]interface{}{
									"type": "ssd",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ChartName: "test-chart",
			}
			appMeta := metadata.New(*cfg)

			got, gotPodValues, err := pod.ProcessSpec("test", appMeta, tt.spec.Template.Spec)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSpec, map[string]interface{}(got))
			assert.Equal(t, tt.expectedPodValues, map[string]interface{}(gotPodValues))
		})
	}
}

func ptr(v corev1.HostPathType) *corev1.HostPathType {
	return &v
}
