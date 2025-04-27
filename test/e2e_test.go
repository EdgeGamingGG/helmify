package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/EdgeGamingGG/helmify/pkg/app"
	"github.com/EdgeGamingGG/helmify/pkg/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHelmifyE2E(t *testing.T) {
	// Setup test directories
	testDataDir := filepath.Join("testdata")
	outputDir := filepath.Join(testDataDir, "output")

	// Clean and recreate output directory
	err := os.RemoveAll(outputDir)
	require.NoError(t, err)
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	// Create sample kustomize files
	kustomizeDir := filepath.Join(testDataDir, "kustomize")
	err = os.MkdirAll(kustomizeDir, 0755)
	require.NoError(t, err)

	// Create a complex deployment with various edge cases
	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
  labels:
    app: test-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: main
        image: nginx:1.19
        resources:
          requests:
            memory: "64Mi"
            cpu: "250m"
          limits:
            memory: "128Mi"
            cpu: "500m"
        env:
        - name: ENV_VAR
          value: "test"
        - name: SECRET_VAR
          valueFrom:
            secretKeyRef:
              name: test-secret
              key: secret-key
        volumeMounts:
        - name: config-volume
          mountPath: /etc/config
      volumes:
      - name: config-volume
        configMap:
          name: test-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  config.yaml: |
    key: value
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: default
type: Opaque
stringData:
  secret-key: secret-value
---
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP`

	err = os.WriteFile(filepath.Join(kustomizeDir, "deployment.yaml"), []byte(deploymentYAML), 0644)
	require.NoError(t, err)

	// Create kustomization.yaml
	kustomizationYAML := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deployment.yaml`

	err = os.WriteFile(filepath.Join(kustomizeDir, "kustomization.yaml"), []byte(kustomizationYAML), 0644)
	require.NoError(t, err)

	// Run kustomize build
	kustomizeOutput := &bytes.Buffer{}
	cmd := exec.Command("kubectl", "kustomize", kustomizeDir)
	cmd.Stdout = kustomizeOutput
	err = cmd.Run()
	require.NoError(t, err)

	// Create a pipe for stdin
	stdin := bytes.NewReader(kustomizeOutput.Bytes())

	// Configure helmify
	cfg := config.Config{
		ChartName: "test-chart",
		ChartDir:  outputDir,
		Verbose:   true,
	}

	// Run helmify
	err = app.Start(stdin, cfg)
	require.NoError(t, err)

	// Verify output
	chartDir := filepath.Join(outputDir, "test-chart")
	require.DirExists(t, chartDir)

	// Check if values.yaml exists and is valid YAML
	valuesPath := filepath.Join(chartDir, "values.yaml")
	valuesData, err := os.ReadFile(valuesPath)
	require.NoError(t, err)

	var values map[string]interface{}
	err = yaml.Unmarshal(valuesData, &values)
	require.NoError(t, err, "values.yaml should be valid YAML")

	// Check if templates exist and contain valid Go templates
	templatesDir := filepath.Join(chartDir, "templates")
	require.DirExists(t, templatesDir)

	// Verify deployment template
	deploymentTemplate := filepath.Join(templatesDir, "deployment.yaml")
	templateData, err := os.ReadFile(deploymentTemplate)
	require.NoError(t, err)

	// Check for Go template syntax
	templateStr := string(templateData)
	require.Contains(t, templateStr, "{{")
	require.Contains(t, templateStr, "}}")

	// Verify the template has the expected structure
	require.Contains(t, templateStr, "apiVersion:")
	require.Contains(t, templateStr, "kind: Deployment")
	require.Contains(t, templateStr, "test-app")
	require.Contains(t, templateStr, ".Values.app.main.image")
	require.Contains(t, templateStr, "resources:")

	// Verify other expected templates exist
	require.FileExists(t, filepath.Join(templatesDir, "config.yaml"))
	require.FileExists(t, filepath.Join(templatesDir, "secret.yaml"))
	require.FileExists(t, filepath.Join(templatesDir, "service.yaml"))

	// Verify values.yaml has the expected structure
	appValues, ok := values["app"].(map[string]interface{})
	require.True(t, ok, "values.yaml should have app section")
	mainValues, ok := appValues["main"].(map[string]interface{})
	require.True(t, ok, "values.yaml should have app.main section")
	imageValues, ok := mainValues["image"].(map[string]interface{})
	require.True(t, ok, "values.yaml should have app.main.image section")
	require.Equal(t, "nginx", imageValues["repository"], "image repository should be nginx")
	require.Equal(t, "1.19", imageValues["tag"], "image tag should be 1.19")
}
