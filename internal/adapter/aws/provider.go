// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package aws collects AWS metadata using SDK v2 and never reads parameter values.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/appsync"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/raywall/aws-architecture-discovery/internal/domain"
)

// Provider scans the configured account and region. Each invocation owns its graph.
type Provider struct { // Config provides SDK credentials, region and HTTP transport.
	Config sdk.Config
	// AccountName is the display name for the scanned account.
	AccountName string
	// AccountNames maps referenced account IDs to display names.
	AccountNames map[string]string
}
type scan struct {
	cfg                sdk.Config
	graph              domain.Graph
	account, partition string
	refs               map[string][]string
}

// Collect lists resources with pagination and fails on incomplete discovery.
func (p Provider) Collect(ctx context.Context) (domain.Graph, error) {
	identity, err := sts.NewFromConfig(p.Config).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return domain.Graph{}, err
	}
	a, err := arn.Parse(sdk.ToString(identity.Arn))
	if err != nil {
		return domain.Graph{}, err
	}
	s := scan{cfg: p.Config, account: sdk.ToString(identity.Account), partition: a.Partition, refs: map[string][]string{}}
	for _, step := range []struct {
		name string
		fn   func(context.Context) error
	}{{"tags", s.tags}, {"lambda", s.functions}, {"ecs", s.containers}, {"api-gateway", s.apis}, {"appsync", s.graphql}, {"sqs", s.queues}, {"sns", s.topics}, {"msk", s.kafka}, {"dynamodb", s.tables}, {"ssm", s.parameters}} {
		if err := step.fn(ctx); err != nil {
			return domain.Graph{}, fmt.Errorf("%s: %w", step.name, err)
		}
	}
	// Only exact known resource IDs, URLs or embedded ARNs become environment-reference edges.
	for from, values := range s.refs {
		for _, value := range values {
			for _, n := range s.graph.Nodes {
				if n.ID != from && (value == n.ID || containsARN(value, n.ID)) {
					s.edge(from, n.ID, "references")
				}
			}
		}
	}
	for i := range s.graph.Nodes {
		n := &s.graph.Nodes[i]
		n.AccountID = s.account
		if parsed, err := arn.Parse(n.ID); err == nil && parsed.AccountID != "" {
			n.AccountID = parsed.AccountID
		}
		n.AccountName = p.AccountNames[n.AccountID]
		if n.AccountID == s.account && p.AccountName != "" {
			n.AccountName = p.AccountName
		}
	}
	return s.graph, nil
}
func (s *scan) id(service, resource string) string {
	return "arn:" + s.partition + ":" + service + ":" + s.cfg.Region + ":" + s.account + ":" + resource
}
func (s *scan) add(id, name, kind string) {
	if id == "" {
		return
	}
	for i := range s.graph.Nodes {
		if s.graph.Nodes[i].ID == id {
			if kind != "event-source" && kind != "subscription-target" {
				s.graph.Nodes[i].Name = name
				s.graph.Nodes[i].Kind = kind
			}
			return
		}
	}
	s.graph.Nodes = append(s.graph.Nodes, domain.Node{ID: id, Name: name, Kind: kind})
}
func (s *scan) edge(from, to, kind string) {
	if from != "" && to != "" {
		s.graph.Edges = append(s.graph.Edges, domain.Edge{From: from, To: to, Kind: kind, Source: "aws"})
	}
}
func (s *scan) identity(id, service, env string) {
	for i := range s.graph.Nodes {
		if s.graph.Nodes[i].ID == id {
			if service != "" {
				s.graph.Nodes[i].Aliases = append(s.graph.Nodes[i].Aliases, service)
			}
			if env != "" {
				s.graph.Nodes[i].Environment = env
			}
		}
	}
}

var arnPattern = regexp.MustCompile(`arn:[a-z0-9-]+:[a-z0-9-]+:[a-z0-9-]*:[0-9]*:[A-Za-z0-9_./:@+=,-]+`)

func containsARN(value, id string) bool {
	for offset := 0; offset < len(value); {
		pos := strings.Index(value[offset:], "arn:")
		if pos < 0 {
			break
		}
		offset += pos
		a := arnPattern.FindString(value[offset:])
		if a == id || strings.TrimSuffix(a, "/invocations") == id {
			return true
		}
		offset += 4
	}
	return false
}
func (s *scan) tags(ctx context.Context) error {
	c := resourcegroupstaggingapi.NewFromConfig(s.cfg)
	p := resourcegroupstaggingapi.NewGetResourcesPaginator(c, &resourcegroupstaggingapi.GetResourcesInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, r := range o.ResourceTagMappingList {
			id := sdk.ToString(r.ResourceARN)
			a, e := arn.Parse(id)
			if e != nil {
				continue
			}
			s.add(id, a.Resource, a.Service)
			tags := map[string]string{}
			for _, t := range r.Tags {
				tags[sdk.ToString(t.Key)] = sdk.ToString(t.Value)
			}
			s.identity(id, tags["service"], tags["env"])
		}
	}
	return nil
}
func (s *scan) functions(ctx context.Context) error {
	c := lambda.NewFromConfig(s.cfg)
	p := lambda.NewListFunctionsPaginator(c, &lambda.ListFunctionsInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, f := range o.Functions {
			id := sdk.ToString(f.FunctionArn)
			s.add(id, sdk.ToString(f.FunctionName), "lambda")
			detail, e := c.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{FunctionName: f.FunctionArn})
			if e != nil {
				return e
			}
			if detail.Environment != nil {
				v := detail.Environment.Variables
				s.identity(id, v["DD_SERVICE"], v["DD_ENV"])
				for _, value := range v {
					s.refs[id] = append(s.refs[id], value)
				}
			}
		}
	}
	mappings := lambda.NewListEventSourceMappingsPaginator(c, &lambda.ListEventSourceMappingsInput{})
	for mappings.HasMorePages() {
		o, e := mappings.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, m := range o.EventSourceMappings {
			source := sdk.ToString(m.EventSourceArn)
			if source != "" {
				s.add(source, source, "event-source")
				s.edge(source, sdk.ToString(m.FunctionArn), "triggers")
			}
			for _, topic := range m.Topics {
				id := source + "/topic/" + topic
				s.add(id, topic, "kafka-topic")
				s.edge(id, sdk.ToString(m.FunctionArn), "triggers")
			}
		}
	}
	return nil
}
func (s *scan) containers(ctx context.Context) error {
	c := ecs.NewFromConfig(s.cfg)
	p := ecs.NewListClustersPaginator(c, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, cluster := range o.ClusterArns {
			s.add(cluster, cluster, "ecs-cluster")
			sp := ecs.NewListServicesPaginator(c, &ecs.ListServicesInput{Cluster: sdk.String(cluster)})
			for sp.HasMorePages() {
				services, e := sp.NextPage(ctx)
				if e != nil {
					return e
				}
				for _, id := range services.ServiceArns {
					d, e := c.DescribeServices(ctx, &ecs.DescribeServicesInput{Cluster: sdk.String(cluster), Services: []string{id}})
					if e != nil {
						return e
					}
					if len(d.Failures) > 0 {
						return fmt.Errorf("ECS DescribeServices returned failures")
					}
					for _, v := range d.Services {
						s.add(id, sdk.ToString(v.ServiceName), "ecs-service")
						s.edge(cluster, id, "contains")
						if e = s.taskDefinition(ctx, c, id, v.TaskDefinition); e != nil {
							return e
						}
					}
				}
			}
			tp := ecs.NewListTasksPaginator(c, &ecs.ListTasksInput{Cluster: sdk.String(cluster)})
			for tp.HasMorePages() {
				tasks, e := tp.NextPage(ctx)
				if e != nil {
					return e
				}
				for _, id := range tasks.TaskArns {
					d, e := c.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: sdk.String(cluster), Tasks: []string{id}})
					if e != nil {
						return e
					}
					if len(d.Failures) > 0 {
						return fmt.Errorf("ECS DescribeTasks returned failures")
					}
					for _, v := range d.Tasks {
						s.add(id, id, "ecs-task")
						s.edge(cluster, id, "contains")
						if e = s.taskDefinition(ctx, c, id, v.TaskDefinitionArn); e != nil {
							return e
						}
					}
				}
			}
			ip := ecs.NewListContainerInstancesPaginator(c, &ecs.ListContainerInstancesInput{Cluster: sdk.String(cluster)})
			for ip.HasMorePages() {
				instances, e := ip.NextPage(ctx)
				if e != nil {
					return e
				}
				for _, id := range instances.ContainerInstanceArns {
					s.add(id, id, "ecs-container-instance")
					s.edge(cluster, id, "contains")
				}
			}
		}
	}
	return nil
}
func (s *scan) taskDefinition(ctx context.Context, c *ecs.Client, id string, definition *string) error {
	if definition == nil {
		return nil
	}
	d, e := c.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: definition})
	if e != nil {
		return e
	}
	if d.TaskDefinition == nil {
		return nil
	}
	for _, container := range d.TaskDefinition.ContainerDefinitions {
		v := map[string]string{}
		for _, pair := range container.Environment {
			v[sdk.ToString(pair.Name)] = sdk.ToString(pair.Value)
			s.refs[id] = append(s.refs[id], sdk.ToString(pair.Value))
		}
		for _, secret := range container.Secrets {
			s.refs[id] = append(s.refs[id], sdk.ToString(secret.ValueFrom))
		}
		if !strings.Contains(id, ":task/") {
			s.identity(id, v["DD_SERVICE"], v["DD_ENV"])
		}
	}
	return nil
}
func (s *scan) apis(ctx context.Context) error {
	c := apigateway.NewFromConfig(s.cfg)
	p := apigateway.NewGetRestApisPaginator(c, &apigateway.GetRestApisInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, a := range o.Items {
			id := "arn:" + s.partition + ":apigateway:" + s.cfg.Region + "::/restapis/" + sdk.ToString(a.Id)
			s.add(id, sdk.ToString(a.Name), "api-gateway-rest")
			rp := apigateway.NewGetResourcesPaginator(c, &apigateway.GetResourcesInput{RestApiId: a.Id, Embed: []string{"methods"}})
			for rp.HasMorePages() {
				resources, e := rp.NextPage(ctx)
				if e != nil {
					return e
				}
				for _, r := range resources.Items {
					for _, m := range r.ResourceMethods {
						if m.MethodIntegration != nil {
							s.refs[id] = append(s.refs[id], sdk.ToString(m.MethodIntegration.Uri))
						}
					}
				}
			}
		}
	}
	v := apigatewayv2.NewFromConfig(s.cfg)
	var token *string
	for {
		o, e := v.GetApis(ctx, &apigatewayv2.GetApisInput{NextToken: token})
		if e != nil {
			return e
		}
		for _, a := range o.Items {
			id := "arn:" + s.partition + ":apigateway:" + s.cfg.Region + "::/apis/" + sdk.ToString(a.ApiId)
			s.add(id, sdk.ToString(a.Name), "api-gateway-v2")
			var next *string
			for {
				d, e := v.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{ApiId: a.ApiId, NextToken: next})
				if e != nil {
					return e
				}
				for _, i := range d.Items {
					s.refs[id] = append(s.refs[id], sdk.ToString(i.IntegrationUri))
				}
				next = d.NextToken
				if sdk.ToString(next) == "" {
					break
				}
			}
		}
		token = o.NextToken
		if sdk.ToString(token) == "" {
			break
		}
	}
	return nil
}
func (s *scan) graphql(ctx context.Context) error {
	c := appsync.NewFromConfig(s.cfg)
	var token *string
	for {
		o, e := c.ListGraphqlApis(ctx, &appsync.ListGraphqlApisInput{NextToken: token})
		if e != nil {
			return e
		}
		for _, a := range o.GraphqlApis {
			id := sdk.ToString(a.Arn)
			s.add(id, sdk.ToString(a.Name), "appsync")
			var next *string
			for {
				d, e := c.ListDataSources(ctx, &appsync.ListDataSourcesInput{ApiId: a.ApiId, NextToken: next})
				if e != nil {
					return e
				}
				for _, v := range d.DataSources {
					if v.LambdaConfig != nil {
						s.refs[id] = append(s.refs[id], sdk.ToString(v.LambdaConfig.LambdaFunctionArn))
					}
					if v.DynamodbConfig != nil {
						s.refs[id] = append(s.refs[id], "arn:"+s.partition+":dynamodb:"+sdk.ToString(v.DynamodbConfig.AwsRegion)+":"+s.account+":table/"+sdk.ToString(v.DynamodbConfig.TableName))
					}
				}
				next = d.NextToken
				if sdk.ToString(next) == "" {
					break
				}
			}
		}
		token = o.NextToken
		if sdk.ToString(token) == "" {
			break
		}
	}
	return nil
}
func (s *scan) queues(ctx context.Context) error {
	c := sqs.NewFromConfig(s.cfg)
	p := sqs.NewListQueuesPaginator(c, &sqs.ListQueuesInput{MaxResults: sdk.Int32(1000)})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, u := range o.QueueUrls {
			a, e := c.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{QueueUrl: sdk.String(u), AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn}})
			if e != nil {
				return e
			}
			id := a.Attributes["QueueArn"]
			s.add(id, u[strings.LastIndex(u, "/")+1:], "sqs")
			for from, values := range s.refs {
				for _, v := range values {
					if v == u {
						s.edge(from, id, "references")
					}
				}
			}
		}
	}
	return nil
}
func (s *scan) topics(ctx context.Context) error {
	c := sns.NewFromConfig(s.cfg)
	p := sns.NewListTopicsPaginator(c, &sns.ListTopicsInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, t := range o.Topics {
			id := sdk.ToString(t.TopicArn)
			s.add(id, id[strings.LastIndex(id, ":")+1:], "sns")
		}
	}
	sp := sns.NewListSubscriptionsPaginator(c, &sns.ListSubscriptionsInput{})
	for sp.HasMorePages() {
		o, e := sp.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, v := range o.Subscriptions {
			endpoint := sdk.ToString(v.Endpoint)
			if strings.HasPrefix(endpoint, "arn:") {
				s.add(endpoint, endpoint, "subscription-target")
				s.edge(sdk.ToString(v.TopicArn), endpoint, "publishes")
			}
		}
	}
	return nil
}
func (s *scan) kafka(ctx context.Context) error {
	c := kafka.NewFromConfig(s.cfg)
	p := kafka.NewListClustersV2Paginator(c, &kafka.ListClustersV2Input{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, v := range o.ClusterInfoList {
			id := sdk.ToString(v.ClusterArn)
			s.add(id, sdk.ToString(v.ClusterName), "msk")
			if v.Provisioned == nil || v.Provisioned.CurrentBrokerSoftwareInfo == nil || !supportsTopics(sdk.ToString(v.Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion)) {
				slog.WarnContext(ctx, "MSK topic inventory unavailable for this cluster type or version", "cluster", id)
				continue
			}
			var token *string
			for {
				d, e := c.ListTopics(ctx, &kafka.ListTopicsInput{ClusterArn: v.ClusterArn, NextToken: token})
				if e != nil {
					return e
				}
				for _, t := range d.Topics {
					tid := id + "/topic/" + sdk.ToString(t.TopicName)
					s.add(tid, sdk.ToString(t.TopicName), "kafka-topic")
					s.edge(id, tid, "contains")
				}
				token = d.NextToken
				if sdk.ToString(token) == "" {
					break
				}
			}
		}
	}
	return nil
}
func (s *scan) tables(ctx context.Context) error {
	c := dynamodb.NewFromConfig(s.cfg)
	p := dynamodb.NewListTablesPaginator(c, &dynamodb.ListTablesInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, n := range o.TableNames {
			s.add(s.id("dynamodb", "table/"+n), n, "dynamodb")
		}
	}
	return nil
}
func (s *scan) parameters(ctx context.Context) error {
	c := ssm.NewFromConfig(s.cfg)
	p := ssm.NewDescribeParametersPaginator(c, &ssm.DescribeParametersInput{})
	for p.HasMorePages() {
		o, e := p.NextPage(ctx)
		if e != nil {
			return e
		}
		for _, v := range o.Parameters {
			n := sdk.ToString(v.Name)
			s.add(s.id("ssm", "parameter/"+strings.TrimPrefix(n, "/")), n, "ssm-parameter")
		}
	}
	return nil
}

func supportsTopics(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, e := strconv.Atoi(parts[0])
	if e != nil {
		return false
	}
	minor, e := strconv.Atoi(parts[1])
	return e == nil && (major > 3 || (major == 3 && minor >= 6))
}
