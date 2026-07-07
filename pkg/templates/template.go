package templates

import (
	"context"
	"fmt"
	reflect "reflect"
	"slices"

	"github.com/dominikbraun/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Template interface {
	hash() TemplateOptions
	getDefaultParams() *DefaultParams
	GetStackName() string
	Deploy(
		ctx context.Context,
		templateResponses []*TemplatesResponse,
		autonamingConfig map[string]string,
		debugOptions optup.Option,
		streamer optup.Option,
	) (isTemplatesResponse_Response, error)
	GetProviderVersions() []*ProviderVersion
	GetDependsOn() []TemplateOptions
	pulumiRunFunc() pulumi.RunFunc
	validate() error
}

type TemplateRequest interface {
	newTemplate() (Template, error)
}

func (tr *TemplatesRequest) NewTemplate() (Template, error) {
	wrapper := tr.GetRequest()
	if wrapper == nil {
		return nil, fmt.Errorf("template request entry has no oneof case set")
	}
	inner, err := unwrapTemplateRequestOneof(wrapper)
	if err != nil {
		return nil, err
	}
	req, ok := inner.(TemplateRequest)
	if !ok {
		return nil, fmt.Errorf("type %T does not implement TemplateRequest", inner)
	}
	return req.newTemplate()
}

func createOrSelectStack(ctx context.Context, t Template, autonamingConfig map[string]string) (auto.Stack, error) {
	err := t.validate()
	if err != nil {
		return auto.Stack{}, fmt.Errorf("%T template validation failed: %s", t, err)
	}

	projectName := fmt.Sprintf("client-project-%s", t.getDefaultParams().GetProjectName())
	s, err := auto.UpsertStackInlineSource(ctx, t.GetStackName(), projectName, t.pulumiRunFunc())
	if err != nil {
		return auto.Stack{}, fmt.Errorf("failed to create/update stack with error: %s", err)
	}
	workspace := s.Workspace()
	for _, p := range t.GetProviderVersions() {
		err := workspace.InstallPlugin(ctx, p.GetProviderName(), p.GetVersion())
		if err != nil {
			return auto.Stack{}, fmt.Errorf("failed to install program plugins: %v\n", err)
		}
	}
	s.SetConfig(ctx, "azure-native:location", auto.ConfigValue{Value: t.getDefaultParams().Region.ShortString()})
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

func getValidDefaultParams(t Template) (*DefaultParams, error) {
	d := t.getDefaultParams()
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

func unwrapTemplateRequestOneof(wrapper isTemplatesRequest_Request) (any, error) {
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
	g := graph.New(Template.hash, graph.Directed(), graph.Acyclic())

	for _, t := range templates {
		err := g.AddVertex(t)
		if err != nil {
			return nil, fmt.Errorf("failed to add vertex: %s", err)
		}
		for _, d := range t.GetDependsOn() {
			err := g.AddEdge(t.hash(), d)
			if err != nil {
				return nil, fmt.Errorf("failed to add edge %s", err.Error())
			}
		}
	}

	return g, nil
}

func GetTemplateInstallOrder(t []Template) ([]Template, error) {
	g, err := buildTemplateDAG(t)
	if err != nil {
		return []Template{}, fmt.Errorf("failed to build directed acyclic graph of template dependencies: %s", err)
	}
	fmt.Println(g)
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
