package templates

import (
	"context"
	"fmt"
	reflect "reflect"
	"slices"

	"github.com/dominikbraun/graph"
	templates "github.com/htemuri/azure-pulumi-service-broker/gen/go/templates/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Template interface {
	Hash() templates.TemplateOptions
	GetDefaultParams() *DefaultParams
	Deploy(
		ctx context.Context,
		cm map[string]any,
		autonamingConfig map[string]string,
	) (map[string]any, error)
	GetProjectName() string
	GetStackName() string
	GetProviders() []*ProviderVersion
	GetDependsOn() []TemplateOptions
	PulumiRunFunc() pulumi.RunFunc
	Validate() error
}

func createOrSelectStack(t Template, ctx context.Context, autonamingConfig map[string]string) (auto.Stack, error) {
	err := t.Validate()
	if err != nil {
		return auto.Stack{}, fmt.Errorf("%T template validation failed: %s", t, err)
	}

	projectName := fmt.Sprintf("client-project-%s", t.GetProjectName())
	s, err := auto.UpsertStackInlineSource(ctx, t.GetStackName(), projectName, t.PulumiRunFunc())
	if err != nil {
		return auto.Stack{}, fmt.Errorf("failed to create/update stack with error: %s", err)
	}
	workspace := s.Workspace()
	for _, p := range t.GetProviders() {
		err := workspace.InstallPlugin(ctx, p.GetProviderName(), p.GetVersion())
		if err != nil {
			return auto.Stack{}, fmt.Errorf("failed to install program plugins: %v\n", err)
		}
	}
	s.SetConfig(ctx, "azure-native:location", auto.ConfigValue{Value: t.GetDefaultParams().Region.ShortString()})
	for k, v := range autonamingConfig {
		c := fmt.Sprintf("pulumi:autonaming.providers.azure-native.resources.azure-native:%s.pattern", k)
		err = s.SetConfigWithOptions(ctx, c, auto.ConfigValue{Value: v}, &auto.ConfigOptions{
			Path: true,
		})
		if err != nil {
			return auto.Stack{}, fmt.Errorf("failed to set autonaming config: %v\n", err)
		}
	}
	_, err = s.Refresh(ctx)
	if err != nil {
		return auto.Stack{}, fmt.Errorf("failed to refresh stack: %v\n", err)
	}

	return s, nil
}

func GetValidDefaultParams(t Template) (*DefaultParams, error) {
	d := t.GetDefaultParams()
	if d == nil {
		return &DefaultParams{}, fmt.Errorf("default params can't be nil")
	}
	if d.ProjectName == "" {
		return &DefaultParams{}, fmt.Errorf("projectName cannot be an empty string")
	}
	if d.Environment == Environment_ENVIRONMENT_UNSPECIFIED {
		return &DefaultParams{}, fmt.Errorf("environment must be specified")
	}
	cred := d.GetPulumiProviderCredential()
	if cred == nil {
		return &DefaultParams{}, fmt.Errorf("pulumi provider credentials can't be nil")
	}
	if _, err := cred.Validate(); err != nil {
		return &DefaultParams{}, err
	}
	return d, nil
}

func getEnabledTemplates(templates []*templates.TemplateArgs) ([]Template, error) {
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
	convertedTemplates, err := getEnabledTemplates(t)
	if err != nil {
		return []Template{}, fmt.Errorf("failed to convert project templates input to template interface: %s", err)
	}
	g, err := buildTemplateDAG(convertedTemplates)
	if err != nil {
		return []Template{}, fmt.Errorf("failed to build directed acyclic graph of template dependencies: %s", err)
	}
	sortedTemplateOptions, err := graph.TopologicalSort(g)
	slices.Reverse(sortedTemplateOptions) // topological sort does it in the opposite order we need
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
