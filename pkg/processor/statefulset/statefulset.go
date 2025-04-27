package statefulset

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/EdgeGamingGG/helmify/pkg/processor/pod"

	"github.com/EdgeGamingGG/helmify/pkg/helmify"
	"github.com/EdgeGamingGG/helmify/pkg/processor"
	yamlformat "github.com/EdgeGamingGG/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var statefulsetGVC = schema.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "StatefulSet",
}

var statefulsetTempl, _ = template.New("statefulset").Parse(
	`{{- .Meta }}
spec:
{{ .Spec }}`)

// New creates processor for k8s StatefulSet resource.
func New() helmify.Processor {
	return &statefulset{}
}

type statefulset struct{}

// Process k8s StatefulSet object into template. Returns false if not capable of processing given resource type.
func (d statefulset) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != statefulsetGVC {
		return false, nil, nil
	}
	ss := appsv1.StatefulSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ss)
	if err != nil {
		return true, nil, fmt.Errorf("%w: unable to cast to StatefulSet", err)
	}
	meta, err := processor.ProcessObjMeta(appMeta, obj)
	if err != nil {
		return true, nil, err
	}

	ssSpec := ss.Spec
	ssSpecMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ssSpec)
	if err != nil {
		return true, nil, err
	}
	delete((ssSpecMap["template"].(map[string]interface{}))["metadata"].(map[string]interface{}), "creationTimestamp")

	values := helmify.Values{}

	name := appMeta.TrimName(obj.GetName())
	nameCamel := strcase.ToLowerCamel(name)

	if ssSpec.ServiceName != "" {
		servName := appMeta.TemplatedName(ssSpec.ServiceName)
		ssSpecMap["serviceName"] = servName
	}

	if ssSpec.Replicas != nil {
		repl, err := values.Add(*ssSpec.Replicas, nameCamel, "replicas")
		if err != nil {
			return true, nil, err
		}
		ssSpecMap["replicas"] = repl
	}

	for i, claim := range ssSpec.VolumeClaimTemplates {
		volName := claim.ObjectMeta.Name
		delete(((ssSpecMap["volumeClaimTemplates"].([]interface{}))[i]).(map[string]interface{}), "status")
		if claim.Spec.StorageClassName != nil {
			scName := appMeta.TemplatedName(*claim.Spec.StorageClassName)
			err = unstructured.SetNestedField(((ssSpecMap["volumeClaimTemplates"].([]interface{}))[i]).(map[string]interface{}), scName, "spec", "storageClassName")
			if err != nil {
				return true, nil, err
			}
		}
		if claim.Spec.VolumeName != "" {
			vName := appMeta.TemplatedName(claim.Spec.VolumeName)
			err = unstructured.SetNestedField(((ssSpecMap["volumeClaimTemplates"].([]interface{}))[i]).(map[string]interface{}), vName, "spec", "volumeName")
			if err != nil {
				return true, nil, err
			}
		}

		resMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&claim.Spec.Resources)
		if err != nil {
			return true, nil, err
		}
		resName, err := values.AddYaml(resMap, 8, true, nameCamel, "volumeClaims", volName)
		if err != nil {
			return true, nil, err
		}
		err = unstructured.SetNestedField(((ssSpecMap["volumeClaimTemplates"].([]interface{}))[i]).(map[string]interface{}), resName, "spec", "resources")
		if err != nil {
			return true, nil, err
		}
	}

	// process pod spec:
	podSpecMap, podValues, err := pod.ProcessSpec(nameCamel, appMeta, ssSpec.Template.Spec)
	if err != nil {
		return true, nil, err
	}
	err = values.Merge(podValues)
	if err != nil {
		return true, nil, err
	}
	err = unstructured.SetNestedMap(ssSpecMap, podSpecMap, "template", "spec")
	if err != nil {
		return true, nil, err
	}

	spec, err := yamlformat.Marshal(ssSpecMap, 2)
	if err != nil {
		return true, nil, err
	}
	spec = strings.ReplaceAll(spec, "'", "")

	return true, &result{
		values: values,
		data: struct {
			Meta string
			Spec string
		}{
			Meta: meta,
			Spec: spec,
		},
	}, nil
}

type result struct {
	data struct {
		Meta string
		Spec string
	}
	values helmify.Values
}

func (r *result) Filename() string {
	return "statefulset.yaml"
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	return statefulsetTempl.Execute(writer, r.data)
}

func ProcessSpec(objName string, appMeta helmify.AppMetadata, spec appsv1.StatefulSetSpec) (map[string]interface{}, helmify.Values, error) {
	podSpecMap, podValues, err := pod.ProcessSpec(objName, appMeta, spec.Template.Spec)
	if err != nil {
		return nil, nil, err
	}

	// Process volume claim templates
	if len(spec.VolumeClaimTemplates) > 0 {
		for i := range spec.VolumeClaimTemplates {
			pvc := spec.VolumeClaimTemplates[i]

			// Validate required fields
			if pvc.Spec.Resources.Requests == nil || len(pvc.Spec.Resources.Requests) == 0 {
				return nil, nil, fmt.Errorf("volume claim template %q must specify resources.requests", pvc.Name)
			}
			if len(pvc.Spec.AccessModes) == 0 {
				return nil, nil, fmt.Errorf("volume claim template %q must specify at least one access mode", pvc.Name)
			}

			pvcName := strcase.ToLowerCamel(pvc.Name)

			// Add PVC template to values
			pvcMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pvc)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to convert PVC template to map", err)
			}

			// Clean up metadata and status
			delete(pvcMap, "status")
			if metadata, ok := pvcMap["metadata"].(map[string]interface{}); ok {
				delete(metadata, "creationTimestamp")
				if len(metadata) == 0 {
					delete(pvcMap, "metadata")
				}
			}

			// Template storage class name if present
			if spec, ok := pvcMap["spec"].(map[string]interface{}); ok {
				if storageClassName, ok := spec["storageClassName"].(string); ok {
					spec["storageClassName"] = appMeta.TemplatedName(storageClassName)
				}
			}

			err = unstructured.SetNestedField(podValues, pvcMap, objName, "volumeClaimTemplates", pvcName)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to set PVC template value", err)
			}

			// Replace PVC template with template
			spec.VolumeClaimTemplates[i].Name = appMeta.TemplatedName(pvc.Name)
		}
	}

	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: unable to convert StatefulSetSpec to map", err)
	}

	// Process volume claim templates for templating
	if vcts, ok := specMap["volumeClaimTemplates"].([]interface{}); ok {
		templatedVcts := make([]interface{}, len(vcts))
		for i, vct := range vcts {
			vctMap := vct.(map[string]interface{})
			vctName := vctMap["metadata"].(map[string]interface{})["name"].(string)
			vctNameCamel := strcase.ToLowerCamel(vctName)

			// Replace volume claim template with template
			templatedVcts[i] = fmt.Sprintf(`{{- toYaml .Values.%s.volumeClaimTemplates.%s | nindent 8 }}`, objName, vctNameCamel)
		}
		specMap["volumeClaimTemplates"] = templatedVcts
	}

	// Set pod spec in the template
	err = unstructured.SetNestedMap(specMap, podSpecMap, "template", "spec")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: unable to set pod spec in template", err)
	}

	return specMap, podValues, nil
}
