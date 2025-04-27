package statefulset

import (
	"testing"

	"github.com/EdgeGamingGG/helmify/pkg/config"
	"github.com/EdgeGamingGG/helmify/pkg/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProcessSpec_VolumeClaimTemplates(t *testing.T) {
	tests := []struct {
		name              string
		spec              appsv1.StatefulSetSpec
		expectedSpec      map[string]interface{}
		expectedPodValues map[string]interface{}
		expectedError     bool
	}{
		{
			name: "basic volume claim template",
			spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:1.14.2",
							},
						},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			expectedSpec: map[string]interface{}{
				"selector":    nil,
				"serviceName": "",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"env": []interface{}{
									map[string]interface{}{
										"name":  "KUBERNETES_CLUSTER_DOMAIN",
										"value": "{{ quote .Values.kubernetesClusterDomain }}",
									},
								},
								"image":     "{{ .Values.test.testContainer.image.repository }}:{{ .Values.test.testContainer.image.tag | default .Chart.AppVersion }}",
								"name":      "test-container",
								"resources": map[string]interface{}{},
							},
						},
					},
				},
				"updateStrategy":       map[string]interface{}{},
				"volumeClaimTemplates": []interface{}{"{{- toYaml .Values.test.volumeClaimTemplates.data | nindent 8 }}"},
			},
			expectedPodValues: map[string]interface{}{
				"test": map[string]interface{}{
					"testContainer": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "nginx",
							"tag":        "1.14.2",
						},
					},
					"volumeClaimTemplates": map[string]interface{}{
						"data": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "data",
							},
							"spec": map[string]interface{}{
								"accessModes": []interface{}{"ReadWriteOnce"},
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"storage": "1Gi",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "volume claim template with storage class",
			spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:1.14.2",
							},
						},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							StorageClassName: ptr("standard"),
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			expectedSpec: map[string]interface{}{
				"selector":    nil,
				"serviceName": "",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"env": []interface{}{
									map[string]interface{}{
										"name":  "KUBERNETES_CLUSTER_DOMAIN",
										"value": "{{ quote .Values.kubernetesClusterDomain }}",
									},
								},
								"image":     "{{ .Values.test.testContainer.image.repository }}:{{ .Values.test.testContainer.image.tag | default .Chart.AppVersion }}",
								"name":      "test-container",
								"resources": map[string]interface{}{},
							},
						},
					},
				},
				"updateStrategy":       map[string]interface{}{},
				"volumeClaimTemplates": []interface{}{"{{- toYaml .Values.test.volumeClaimTemplates.data | nindent 8 }}"},
			},
			expectedPodValues: map[string]interface{}{
				"test": map[string]interface{}{
					"testContainer": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "nginx",
							"tag":        "1.14.2",
						},
					},
					"volumeClaimTemplates": map[string]interface{}{
						"data": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "data",
							},
							"spec": map[string]interface{}{
								"accessModes": []interface{}{"ReadWriteOnce"},
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"storage": "1Gi",
									},
								},
								"storageClassName": "standard",
							},
						},
					},
				},
			},
		},
		{
			name: "multiple volume claim templates",
			spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:1.14.2",
							},
						},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "logs",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							StorageClassName: ptr("fast"),
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("2Gi"),
								},
							},
						},
					},
				},
			},
			expectedSpec: map[string]interface{}{
				"selector":    nil,
				"serviceName": "",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"env": []interface{}{
									map[string]interface{}{
										"name":  "KUBERNETES_CLUSTER_DOMAIN",
										"value": "{{ quote .Values.kubernetesClusterDomain }}",
									},
								},
								"image":     "{{ .Values.test.testContainer.image.repository }}:{{ .Values.test.testContainer.image.tag | default .Chart.AppVersion }}",
								"name":      "test-container",
								"resources": map[string]interface{}{},
							},
						},
					},
				},
				"updateStrategy": map[string]interface{}{},
				"volumeClaimTemplates": []interface{}{
					"{{- toYaml .Values.test.volumeClaimTemplates.data | nindent 8 }}",
					"{{- toYaml .Values.test.volumeClaimTemplates.logs | nindent 8 }}",
				},
			},
			expectedPodValues: map[string]interface{}{
				"test": map[string]interface{}{
					"testContainer": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "nginx",
							"tag":        "1.14.2",
						},
					},
					"volumeClaimTemplates": map[string]interface{}{
						"data": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "data",
							},
							"spec": map[string]interface{}{
								"accessModes": []interface{}{"ReadWriteOnce"},
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"storage": "1Gi",
									},
								},
							},
						},
						"logs": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "logs",
							},
							"spec": map[string]interface{}{
								"accessModes": []interface{}{"ReadWriteOnce"},
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"storage": "2Gi",
									},
								},
								"storageClassName": "fast",
							},
						},
					},
				},
			},
		},
		{
			name: "invalid volume claim template - missing resources",
			spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:1.14.2",
							},
						},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						},
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ChartName: "test-chart",
			}
			appMeta := metadata.New(*cfg)

			got, gotPodValues, err := ProcessSpec("test", appMeta, tt.spec)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSpec, map[string]interface{}(got))
			assert.Equal(t, tt.expectedPodValues, map[string]interface{}(gotPodValues))
		})
	}
}

func ptr(s string) *string {
	return &s
}
