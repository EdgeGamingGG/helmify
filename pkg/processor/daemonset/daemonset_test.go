package daemonset

import (
	"testing"

	"github.com/EdgeGamingGG/helmify/internal"
	"github.com/EdgeGamingGG/helmify/pkg/config"
	"github.com/EdgeGamingGG/helmify/pkg/metadata"
	"github.com/EdgeGamingGG/helmify/pkg/processor/pod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	strDepl = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentd-elasticsearch
  namespace: kube-system
  labels:
    k8s-app: fluentd-logging
spec:
  selector:
    matchLabels:
      name: fluentd-elasticsearch
  template:
    metadata:
      labels:
        name: fluentd-elasticsearch
    spec:
      tolerations:
      # this toleration is to have the daemonset runnable on master nodes
      # remove it if your masters can't run pods
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      containers:
      - name: fluentd-elasticsearch
        image: quay.io/fluentd_elasticsearch/fluentd:v2.5.2
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 200Mi
        volumeMounts:
        - name: varlog
          mountPath: /var/log
        - name: varlibdockercontainers
          mountPath: /var/lib/docker/containers
          readOnly: true
      terminationGracePeriodSeconds: 30
      volumes:
      - name: varlog
        hostPath:
          path: /var/log
      - name: varlibdockercontainers
        hostPath:
          path: /var/lib/docker/containers
`
)

func Test_daemonset_Process(t *testing.T) {
	var testInstance daemonset

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

func TestProcessSpec_VolumeMounts(t *testing.T) {
	tests := []struct {
		name              string
		spec              appsv1.DaemonSetSpec
		expectedSpec      map[string]interface{}
		expectedPodValues map[string]interface{}
	}{
		{
			name: "hostpath volume mount",
			spec: appsv1.DaemonSetSpec{
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
			spec: appsv1.DaemonSetSpec{
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
