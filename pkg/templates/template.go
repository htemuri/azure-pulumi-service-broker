package templates

import (
	"fmt"
	reflect "reflect"

	"github.com/dominikbraun/graph"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Template interface {
	Hash() TemplateOptions
	GetDefaultParams() *DefaultParams
	PulumiRunFunc() pulumi.RunFunc
	GetStackName() string
	GetProviders() []ProviderVersion
	GetDependsOn() []TemplateOptions
	Validate() error
}

func GetEnabledTemplates(templates []*Templates) ([]Template, error) {
	var out []Template
	for _, t := range templates {
		wrapper := t.GetTemplate()
		if wrapper == nil {
			return nil, fmt.Errorf("template entry has no oneof case set")
		}
		inner, err := unwrapOneof(wrapper)
		if err != nil {
			return nil, err
		}
		tmpl, ok := inner.(Template)
		if !ok {
			return nil, fmt.Errorf("type %T does not implement Template", inner)
		}
		out = append(out, tmpl)
	}
	return out, nil
}

// extracts the single field out of Templates oneof wrapper struct
func unwrapOneof(wrapper isTemplates_Template) (any, error) {
	v := reflect.ValueOf(wrapper)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("unexpected oneof wrapper type %T", wrapper)
	}
	elem := v.Elem()
	if elem.NumField() != 1 {
		return nil, fmt.Errorf("oneof wrapper %T does not have exactly one field", wrapper)
	}
	return elem.Field(0).Interface(), nil
}

func buildTemplateDAG(templates []Template) (graph.Graph[TemplateOptions, Template], error) {
	g := graph.New(Template.Hash, graph.Directed(), graph.Acyclic())

	for _, t := range templates {
		err := g.AddVertex(t)
		if err != nil {
			return nil, fmt.Errorf("failed to add vertex: %s", err)
		}
		for _, d := range t.GetDependsOn() {
			err := g.AddEdge(t.Hash(), d)
			if err != nil {
				return nil, fmt.Errorf("failed to add edge %s", err.Error())
			}
		}
	}

	return g, nil
}

func GetTemplateInstallOrder(t []*Templates) ([]Template, error) {
	convertedTemplates, err := GetEnabledTemplates(t)
	if err != nil {
		return []Template{}, fmt.Errorf("failed to convert project templates input to template interface: %s", err)
	}
	g, err := buildTemplateDAG(convertedTemplates)
	if err != nil {
		return []Template{}, fmt.Errorf("failed to build directed acyclic graph of template dependencies: %s", err)
	}
	sortedTemplateOptions, err := graph.TopologicalSort(g)
	if err != nil {
		return []Template{}, fmt.Errorf("failed to sort dag: %s", err)
	}
	var sortedTemplates []Template
	for _, x := range sortedTemplateOptions {
		t, err := g.Vertex(x)
		if err != nil {
			return []Template{}, fmt.Errorf("failed to get vertex of templateoption: %s", err)
		}
		sortedTemplates = append(sortedTemplates, t)
	}
	return sortedTemplates, nil
}
