package pod

import (
	"fmt"
	"strings"

	"github.com/EdgeGamingGG/helmify/pkg/cluster"
	"github.com/EdgeGamingGG/helmify/pkg/helmify"
	securityContext "github.com/EdgeGamingGG/helmify/pkg/processor/security-context"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const imagePullPolicyTemplate = "{{ .Values.%[1]s.%[2]s.imagePullPolicy }}"
const envValue = "{{ quote .Values.%[1]s.%[2]s.%[3]s.%[4]s }}"

func ProcessSpec(objName string, appMeta helmify.AppMetadata, spec corev1.PodSpec) (map[string]interface{}, helmify.Values, error) {
	values, err := processPodSpec(objName, appMeta, &spec)
	if err != nil {
		return nil, nil, err
	}

	// replace PVC to templated name
	for i := 0; i < len(spec.Volumes); i++ {
		vol := spec.Volumes[i]
		if vol.PersistentVolumeClaim != nil {
			tempPVCName := appMeta.TemplatedName(vol.PersistentVolumeClaim.ClaimName)
			spec.Volumes[i].PersistentVolumeClaim.ClaimName = tempPVCName
		}
		if vol.ConfigMap != nil {
			vol.ConfigMap.Name = appMeta.TemplatedName(vol.ConfigMap.Name)
		}
		if vol.Secret != nil {
			vol.Secret.SecretName = appMeta.TemplatedName(vol.Secret.SecretName)
		}
		if vol.Projected != nil {
			for j := range vol.Projected.Sources {
				if vol.Projected.Sources[j].ConfigMap != nil {
					vol.Projected.Sources[j].ConfigMap.Name = appMeta.TemplatedName(vol.Projected.Sources[j].ConfigMap.Name)
				}
				if vol.Projected.Sources[j].Secret != nil {
					vol.Projected.Sources[j].Secret.Name = appMeta.TemplatedName(vol.Projected.Sources[j].Secret.Name)
				}
			}
		}
	}

	// replace container resources with template to values.
	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: unable to convert podSpec to map", err)
	}

	// Process volumes for templating
	if volumes, ok := specMap["volumes"].([]interface{}); ok {
		for i, vol := range volumes {
			volMap := vol.(map[string]interface{})
			volName := volMap["name"].(string)
			volNameCamel := strcase.ToLowerCamel(volName)

			// Add volume to values
			err = unstructured.SetNestedField(values, volMap, objName, "volumes", volNameCamel)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to set volume value", err)
			}

			// Replace volume with template
			specMap["volumes"].([]interface{})[i] = fmt.Sprintf(`{{- toYaml .Values.%s.volumes.%s | nindent 8 }}`, objName, volNameCamel)
		}
	}

	specMap, values, err = processNestedContainers(specMap, objName, values, "containers")
	if err != nil {
		return nil, nil, err
	}

	specMap, values, err = processNestedContainers(specMap, objName, values, "initContainers")
	if err != nil {
		return nil, nil, err
	}

	if appMeta.Config().ImagePullSecrets {
		if _, defined := specMap["imagePullSecrets"]; !defined {
			specMap["imagePullSecrets"] = "{{ .Values.imagePullSecrets | default list | toJson }}"
			values["imagePullSecrets"] = []string{}
		}
	}

	err = securityContext.ProcessContainerSecurityContext(objName, specMap, &values)
	if err != nil {
		return nil, nil, err
	}
	if spec.SecurityContext != nil {
		securityContextMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec.SecurityContext)
		if err != nil {
			return nil, nil, err
		}
		if len(securityContextMap) > 0 {
			err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%[1]s.podSecurityContext | nindent 8 }}`, objName), "securityContext")
			if err != nil {
				return nil, nil, err
			}

			err = unstructured.SetNestedField(values, securityContextMap, objName, "podSecurityContext")
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to set deployment value field", err)
			}
		}
	}

	// process nodeSelector if presented:
	if spec.NodeSelector != nil {
		err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%s.nodeSelector | nindent 8 }}`, objName), "nodeSelector")
		if err != nil {
			return nil, nil, err
		}
		err = unstructured.SetNestedStringMap(values, spec.NodeSelector, objName, "nodeSelector")
		if err != nil {
			return nil, nil, err
		}
	}

	// process tolerations if presented:
	if spec.Tolerations != nil {
		err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%s.tolerations | nindent 8 }}`, objName), "tolerations")
		if err != nil {
			return nil, nil, err
		}

		tolerationsUnstr := make([]interface{}, len(spec.Tolerations))
		for i, t := range spec.Tolerations {
			tCopy := t
			unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&tCopy)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to convert toleration to unstructured", err)
			}
			tolerationsUnstr[i] = unstr
		}
		err = unstructured.SetNestedSlice(values, tolerationsUnstr, objName, "tolerations")
		if err != nil {
			return nil, nil, err
		}
	}

	return specMap, values, nil
}

func processNestedContainers(specMap map[string]interface{}, objName string, values map[string]interface{}, containerKey string) (map[string]interface{}, map[string]interface{}, error) {
	containers, _, err := unstructured.NestedSlice(specMap, containerKey)
	if err != nil {
		return nil, nil, err
	}

	if len(containers) > 0 {
		containers, values, err = processContainers(objName, values, containerKey, containers)
		if err != nil {
			return nil, nil, err
		}

		err = unstructured.SetNestedSlice(specMap, containers, containerKey)
		if err != nil {
			return nil, nil, err
		}
	}

	return specMap, values, nil
}

func processContainers(objName string, values helmify.Values, containerType string, containers []interface{}) ([]interface{}, helmify.Values, error) {
	for i := range containers {
		containerName := strcase.ToLowerCamel((containers[i].(map[string]interface{})["name"]).(string))

		// Process volumeMounts
		if volumeMounts, ok := containers[i].(map[string]interface{})["volumeMounts"].([]interface{}); ok {
			for j, vm := range volumeMounts {
				vmMap := vm.(map[string]interface{})
				vmName := vmMap["name"].(string)
				vmNameCamel := strcase.ToLowerCamel(vmName)

				// Add volumeMount to values
				err := unstructured.SetNestedField(values, vmMap, objName, containerName, "volumeMounts", vmNameCamel)
				if err != nil {
					return nil, nil, fmt.Errorf("%w: unable to set volumeMount value", err)
				}

				// Replace volumeMount with template
				containers[i].(map[string]interface{})["volumeMounts"].([]interface{})[j] = fmt.Sprintf(`{{- toYaml .Values.%s.%s.volumeMounts.%s | nindent 10 }}`, objName, containerName, vmNameCamel)
			}
		}

		res, exists, err := unstructured.NestedMap(values, objName, containerName, "resources")
		if err != nil {
			return nil, nil, err
		}
		if exists && len(res) > 0 {
			err = unstructured.SetNestedField(containers[i].(map[string]interface{}), fmt.Sprintf(`{{- toYaml .Values.%s.%s.resources | nindent 10 }}`, objName, containerName), "resources")
			if err != nil {
				return nil, nil, err
			}
		}

		args, exists, err := unstructured.NestedStringSlice(containers[i].(map[string]interface{}), "args")
		if err != nil {
			return nil, nil, err
		}
		if exists && len(args) > 0 {
			err = unstructured.SetNestedField(containers[i].(map[string]interface{}), fmt.Sprintf(`{{- toYaml .Values.%[1]s.%[2]s.args | nindent 8 }}`, objName, containerName), "args")
			if err != nil {
				return nil, nil, err
			}

			err = unstructured.SetNestedStringSlice(values, args, objName, containerName, "args")
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to set deployment value field", err)
			}
		}
	}
	return containers, values, nil
}

func processPodSpec(name string, appMeta helmify.AppMetadata, pod *corev1.PodSpec) (helmify.Values, error) {
	values := helmify.Values{}
	for i, c := range pod.Containers {
		processed, err := processPodContainer(name, appMeta, c, &values)
		if err != nil {
			return nil, err
		}
		pod.Containers[i] = processed
	}

	for i, c := range pod.InitContainers {
		processed, err := processPodContainer(name, appMeta, c, &values)
		if err != nil {
			return nil, err
		}
		pod.InitContainers[i] = processed
	}

	for _, v := range pod.Volumes {
		if v.ConfigMap != nil {
			v.ConfigMap.Name = appMeta.TemplatedName(v.ConfigMap.Name)
		}
		if v.Secret != nil {
			v.Secret.SecretName = appMeta.TemplatedName(v.Secret.SecretName)
		}
	}
	pod.ServiceAccountName = appMeta.TemplatedName(pod.ServiceAccountName)

	for i, s := range pod.ImagePullSecrets {
		pod.ImagePullSecrets[i].Name = appMeta.TemplatedName(s.Name)
	}

	return values, nil
}

func processPodContainer(name string, appMeta helmify.AppMetadata, c corev1.Container, values *helmify.Values) (corev1.Container, error) {
	index := strings.LastIndex(c.Image, ":")
	if strings.Contains(c.Image, "@") && strings.Count(c.Image, ":") >= 2 {
		last := strings.LastIndex(c.Image, ":")
		index = strings.LastIndex(c.Image[:last], ":")
	}
	if index < 0 {
		return c, fmt.Errorf("wrong image format: %q", c.Image)
	}
	repo, tag := c.Image[:index], c.Image[index+1:]
	containerName := strcase.ToLowerCamel(c.Name)
	c.Image = fmt.Sprintf("{{ .Values.%[1]s.%[2]s.image.repository }}:{{ .Values.%[1]s.%[2]s.image.tag | default .Chart.AppVersion }}", name, containerName)

	err := unstructured.SetNestedField(*values, repo, name, containerName, "image", "repository")
	if err != nil {
		return c, fmt.Errorf("%w: unable to set deployment value field", err)
	}
	err = unstructured.SetNestedField(*values, tag, name, containerName, "image", "tag")
	if err != nil {
		return c, fmt.Errorf("%w: unable to set deployment value field", err)
	}

	c, err = processEnv(name, appMeta, c, values)
	if err != nil {
		return c, err
	}

	for _, e := range c.EnvFrom {
		if e.SecretRef != nil {
			e.SecretRef.Name = appMeta.TemplatedName(e.SecretRef.Name)
		}
		if e.ConfigMapRef != nil {
			e.ConfigMapRef.Name = appMeta.TemplatedName(e.ConfigMapRef.Name)
		}
	}
	c.Env = append(c.Env, corev1.EnvVar{
		Name:  cluster.DomainEnv,
		Value: fmt.Sprintf("{{ quote .Values.%s }}", cluster.DomainKey),
	})
	for k, v := range c.Resources.Requests {
		err = unstructured.SetNestedField(*values, v.ToUnstructured(), name, containerName, "resources", "requests", k.String())
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container resources value", err)
		}
	}
	for k, v := range c.Resources.Limits {
		err = unstructured.SetNestedField(*values, v.ToUnstructured(), name, containerName, "resources", "limits", k.String())
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container resources value", err)
		}
	}

	if c.ImagePullPolicy != "" {
		err = unstructured.SetNestedField(*values, string(c.ImagePullPolicy), name, containerName, "imagePullPolicy")
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container imagePullPolicy", err)
		}
		c.ImagePullPolicy = corev1.PullPolicy(fmt.Sprintf(imagePullPolicyTemplate, name, containerName))
	}
	return c, nil
}

func processEnv(name string, appMeta helmify.AppMetadata, c corev1.Container, values *helmify.Values) (corev1.Container, error) {
	containerName := strcase.ToLowerCamel(c.Name)
	for i := 0; i < len(c.Env); i++ {
		if c.Env[i].ValueFrom != nil {
			switch {
			case c.Env[i].ValueFrom.SecretKeyRef != nil:
				c.Env[i].ValueFrom.SecretKeyRef.Name = appMeta.TemplatedName(c.Env[i].ValueFrom.SecretKeyRef.Name)
			case c.Env[i].ValueFrom.ConfigMapKeyRef != nil:
				c.Env[i].ValueFrom.ConfigMapKeyRef.Name = appMeta.TemplatedName(c.Env[i].ValueFrom.ConfigMapKeyRef.Name)
			case c.Env[i].ValueFrom.FieldRef != nil, c.Env[i].ValueFrom.ResourceFieldRef != nil:
				// nothing to change here, keep the original value
			}
			continue
		}

		err := unstructured.SetNestedField(*values, c.Env[i].Value, name, containerName, "env", strcase.ToLowerCamel(strings.ToLower(c.Env[i].Name)))
		if err != nil {
			return c, fmt.Errorf("%w: unable to set deployment value field", err)
		}
		c.Env[i].Value = fmt.Sprintf(envValue, name, containerName, "env", strcase.ToLowerCamel(strings.ToLower(c.Env[i].Name)))
	}
	return c, nil
}
