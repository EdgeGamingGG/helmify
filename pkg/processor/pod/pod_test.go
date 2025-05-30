package pod

import (
	"testing"

	"github.com/EdgeGamingGG/helmify/pkg/helmify"
	"github.com/EdgeGamingGG/helmify/pkg/metadata"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/EdgeGamingGG/helmify/internal"
	"github.com/stretchr/testify/assert"
)

const (
	strDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        args:
        - --test
        - --arg
        ports:
        - containerPort: 80
`

	strDeploymentWithTagAndDigest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2@sha256:cb5c1bddd1b5665e1867a7fa1b5fa843a47ee433bbb75d4293888b71def53229
        ports:
        - containerPort: 80
`

	strDeploymentWithNoArgs = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`

	strDeploymentWithPort = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: localhost:6001/my_project:latest
        ports:
        - containerPort: 80
`
	strDeploymentWithPodSecurityContext = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: localhost:6001/my_project:latest
      securityContext:
        fsGroup: 20000
        runAsGroup: 30000
        runAsNonRoot: true
        runAsUser: 65532

`
	strDeploymentWithTolerations = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
      tolerations:
      - key: "key1"
        operator: "Equal"
        value: "value1"
        effect: "NoSchedule"
      - key: "key2"
        operator: "Exists"
        effect: "NoExecute"
`
	strDeploymentWithNodeSelector = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nginx-daemonset
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
      nodeSelector:
        disktype: ssd
        region: us-east-1
`
)

func Test_pod_Process(t *testing.T) {
	t.Run("deployment with args", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeployment)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"args": "{{- toYaml .Values.nginx.nginx.args | nindent 8 }}",
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image": "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]interface{}{},
				},
			},
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "nginx",
						"tag":        "1.14.2",
					},
					"args": []interface{}{
						"--test",
						"--arg",
					},
				},
			},
		}, tmpl)
	})

	t.Run("deployment with no args", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithNoArgs)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image": "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]interface{}{},
				},
			},
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "nginx",
						"tag":        "1.14.2",
					},
				},
			},
		}, tmpl)
	})

	t.Run("deployment with image tag and digest", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithTagAndDigest)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image": "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]interface{}{},
				},
			},
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "nginx",
						"tag":        "1.14.2@sha256:cb5c1bddd1b5665e1867a7fa1b5fa843a47ee433bbb75d4293888b71def53229",
					},
				},
			},
		}, tmpl)
	})

	t.Run("deployment with image tag and port", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithPort)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image": "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx", "ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]interface{}{},
				},
			},
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "localhost:6001/my_project",
						"tag":        "latest",
					},
				},
			},
		}, tmpl)
	})
	t.Run("deployment with securityContext", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithPodSecurityContext)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image":     "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":      "nginx",
					"resources": map[string]interface{}{},
				},
			},
			"securityContext": "{{- toYaml .Values.nginx.podSecurityContext | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"podSecurityContext": map[string]interface{}{
					"fsGroup":      int64(20000),
					"runAsGroup":   int64(30000),
					"runAsNonRoot": true,
					"runAsUser":    int64(65532),
				},
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "localhost:6001/my_project",
						"tag":        "latest",
					},
				},
			},
		}, tmpl)
	})

	t.Run("deployment with tolerations", func(t *testing.T) {
		var deploy appsv1.Deployment
		obj := internal.GenerateObj(strDeploymentWithTolerations)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deploy)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, deploy.Spec.Template.Spec)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image": "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]interface{}{},
				},
			},
			"tolerations": "{{- toYaml .Values.nginx.tolerations | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "nginx",
						"tag":        "1.14.2",
					},
				},
				"tolerations": []interface{}{
					map[string]interface{}{
						"key":      "key1",
						"operator": "Equal",
						"value":    "value1",
						"effect":   "NoSchedule",
					},
					map[string]interface{}{
						"key":      "key2",
						"operator": "Exists",
						"effect":   "NoExecute",
					},
				},
			},
		}, tmpl)
	})

	t.Run("daemonset with nodeSelector", func(t *testing.T) {
		var dae appsv1.DaemonSet
		obj := internal.GenerateObj(strDeploymentWithNodeSelector)
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dae)
		specMap, tmpl, err := ProcessSpec("nginx", &metadata.Service{}, dae.Spec.Template.Spec)
		assert.NoError(t, err)

		assert.Equal(t, map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{
							"name":  "KUBERNETES_CLUSTER_DOMAIN",
							"value": "{{ quote .Values.kubernetesClusterDomain }}",
						},
					},
					"image": "{{ .Values.nginx.nginx.image.repository }}:{{ .Values.nginx.nginx.image.tag | default .Chart.AppVersion }}",
					"name":  "nginx",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": int64(80),
						},
					},
					"resources": map[string]interface{}{},
				},
			},
			"nodeSelector": "{{- toYaml .Values.nginx.nodeSelector | nindent 8 }}",
		}, specMap)

		assert.Equal(t, helmify.Values{
			"nginx": map[string]interface{}{
				"nginx": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": "nginx",
						"tag":        "1.14.2",
					},
				},
				"nodeSelector": map[string]interface{}{
					"disktype": "ssd",
					"region":   "us-east-1",
				},
			},
		}, tmpl)
	})

}
