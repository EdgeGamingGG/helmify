package service

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/EdgeGamingGG/helmify/pkg/processor"

	"github.com/EdgeGamingGG/helmify/pkg/helmify"
	yamlformat "github.com/EdgeGamingGG/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

const (
	svcTempSpec = `
spec:
  type: {{ .Values.%[1]s.type }}
  selector:
%[2]s
    {{- include "%[3]s.selectorLabels" . | nindent 4 }}
  ports:
  {{- .Values.%[1]s.ports | toYaml | nindent 2 }}`
)

const (
	lbSourceRangesTempSpec = `
  loadBalancerSourceRanges:
  {{- .Values.%[1]s.loadBalancerSourceRanges | toYaml | nindent 2 }}`
)

var svcGVC = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "Service",
}

// New creates processor for k8s Service resource.
func New() helmify.Processor {
	return &svc{}
}

type svc struct{}

// Process k8s Service object into template. Returns false if not capable of processing given resource type.
func (r svc) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != svcGVC {
		return false, nil, nil
	}
	service := corev1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &service)
	if err != nil {
		return true, nil, fmt.Errorf("%w: unable to cast to service", err)
	}

	meta, err := processor.ProcessObjMeta(appMeta, obj)
	if err != nil {
		return true, nil, err
	}

	name := appMeta.TrimName(obj.GetName())
	shortName := strings.TrimPrefix(name, "controller-manager-")
	shortNameCamel := strcase.ToLowerCamel(shortName)

	selector, _ := yaml.Marshal(service.Spec.Selector)
	selector = yamlformat.Indent(selector, 4)
	selector = bytes.TrimRight(selector, "\n ")

	values := helmify.Values{}
	svcType := service.Spec.Type
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}
	_ = unstructured.SetNestedField(values, string(svcType), shortNameCamel, "type")
	ports := make([]interface{}, len(service.Spec.Ports))
	for i, p := range service.Spec.Ports {
		pMap := map[string]interface{}{
			"port": int64(p.Port),
		}
		if p.Name != "" {
			pMap["name"] = p.Name
		}
		if p.NodePort != 0 {
			pMap["nodePort"] = int64(p.NodePort)
		}
		if p.Protocol != "" {
			pMap["protocol"] = string(p.Protocol)
		}
		if p.TargetPort.Type == intstr.Int {
			pMap["targetPort"] = int64(p.TargetPort.IntVal)
		} else {
			pMap["targetPort"] = p.TargetPort.StrVal
		}
		ports[i] = pMap
	}

	_ = unstructured.SetNestedSlice(values, ports, shortNameCamel, "ports")
	res := meta + fmt.Sprintf(svcTempSpec, shortNameCamel, selector, appMeta.ChartName())

	res += parseLoadBalancerSourceRanges(values, service, shortNameCamel)

	if shortNameCamel == "webhookService" && appMeta.Config().AddWebhookOption {
		res = fmt.Sprintf("{{- if .Values.webhook.enabled }}\n%s\n{{- end }}", res)
	}
	return true, &result{
		name:   shortName,
		data:   res,
		values: values,
	}, nil
}

func parseLoadBalancerSourceRanges(values helmify.Values, service corev1.Service, shortNameCamel string) string {
	if len(service.Spec.LoadBalancerSourceRanges) < 1 {
		return ""
	}
	lbSourceRanges := make([]interface{}, len(service.Spec.LoadBalancerSourceRanges))
	for i, ip := range service.Spec.LoadBalancerSourceRanges {
		lbSourceRanges[i] = ip
	}
	_ = unstructured.SetNestedSlice(values, lbSourceRanges, shortNameCamel, "loadBalancerSourceRanges")
	return fmt.Sprintf(lbSourceRangesTempSpec, shortNameCamel)
}

type result struct {
	name   string
	data   string
	values helmify.Values
}

func (r *result) Filename() string {
	return r.name + ".yaml"
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	_, err := writer.Write([]byte(r.data))
	return err
}
