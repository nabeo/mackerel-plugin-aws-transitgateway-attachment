package mpawstgwattch

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	mp "github.com/mackerelio/go-mackerel-plugin"
)

// AwsTgwAttchPlugin struct
type AwsTgwAttchPlugin struct {
	Prefix      string
	AccessKeyID string
	SecretKeyID string
	Region      string
	RoleArn     string
	TgwAttach   string
	Tgw         string
	CloudWatch  *cloudwatch.Client
}

const (
	namespace = "AWS/TransitGateway"
)

type metrics struct {
	Name string
	Type types.Statistic
}

// GraphDefinition : return graph definition
func (p AwsTgwAttchPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := cases.Title(language.Und, cases.NoLower).String(p.Prefix)
	labelPrefix = strings.ReplaceAll(labelPrefix, "-", " ")

	// https://docs.aws.amazon.com/vpc/latest/tgw/transit-gateway-cloudwatch-metrics.html#transit-gateway-metrics
	return map[string]mp.Graphs{
		"Bytes": {
			Label: labelPrefix + " Bytes",
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "BytesIn", Label: "Bytes In"},
				{Name: "BytesOut", Label: "Bytes Out"},
			},
		},
		"Packets": {
			Label: labelPrefix + " Packets",
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "PacketsIn", Label: "Packets In"},
				{Name: "PacketsOut", Label: "Packets Out"},
			},
		},
		"PacketDrop": {
			Label: labelPrefix + "Packet Drop",
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "PacketDropCountBlackhole", Label: "Blackhole"},
				{Name: "PacketDropCountNoRoute", Label: "No Route"},
			},
		},
		"BytesDrop": {
			Label: labelPrefix + "Bytes Drop",
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "BytesDropCountBlackhole", Label: "Blackhole"},
				{Name: "BytesDropCountNoRoute", Label: "No Route"},
			},
		},
	}
}

// MetricKeyPrefix : interface for PluginWithPrefix
func (p AwsTgwAttchPlugin) MetricKeyPrefix() string {
	if p.Prefix == "" {
		p.Prefix = "TgwAttach"
	}
	return p.Prefix
}

// FetchMetrics : fetch metrics
func (p AwsTgwAttchPlugin) FetchMetrics() (map[string]float64, error) {
	stat := make(map[string]float64)
	for _, met := range []metrics{
		{Name: "BytesIn", Type: types.StatisticSum},
		{Name: "BytesOut", Type: types.StatisticSum},
		{Name: "PacketsIn", Type: types.StatisticSum},
		{Name: "PacketsOut", Type: types.StatisticSum},
		{Name: "PacketDropCountBlackhole", Type: types.StatisticSum},
		{Name: "PacketDropCountNoRoute", Type: types.StatisticSum},
		{Name: "BytesDropCountBlackhole", Type: types.StatisticSum},
		{Name: "BytesDropCountNoRoute", Type: types.StatisticSum},
	} {
		v, err := p.getLastPoint(met)
		if err != nil {
			log.Printf("%s : %s", met, err)
		}
		stat[met.Name] = v
	}
	return stat, nil
}

func (p AwsTgwAttchPlugin) getLastPoint(metric metrics) (float64, error) {
	now := time.Now()
	dimensions := []types.Dimension{
		{
			Name:  aws.String("TransitGateway"),
			Value: aws.String(p.Tgw),
		},
		{
			Name:  aws.String("TransitGatewayAttachment"),
			Value: aws.String(p.TgwAttach),
		},
	}

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		Dimensions: dimensions,
		StartTime:  aws.Time(now.Add(time.Duration(180) * time.Second * -1)), // 3 min (to fetch at least 1 data-point)
		EndTime:    aws.Time(now),
		Period:     aws.Int32(60),
		MetricName: aws.String(metric.Name),
		Statistics: []types.Statistic{metric.Type},
	}

	response, err := p.CloudWatch.GetMetricStatistics(context.Background(), input)
	if err != nil {
		return 0, err
	}

	datapoints := response.Datapoints
	if len(datapoints) == 0 {
		return 0, errors.New("fetch no datapoints : " + p.TgwAttach)
	}

	// get least recently datapoint.
	// because a most recently datapoint is not stable.
	least := time.Now()
	var latestVal float64
	for _, dp := range datapoints {
		if dp.Timestamp.Before(least) {
			least = *dp.Timestamp
			if metric.Type == types.StatisticSum {
				latestVal = *dp.Sum
			}
		}
	}

	return latestVal, nil
}

func (p *AwsTgwAttchPlugin) prepare() error {
	var opts []func(*config.LoadOptions) error

	if p.RoleArn != "" {
		cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			return err
		}
		stsclient := sts.NewFromConfig(cfg)

		appCreds := stscreds.NewAssumeRoleProvider(stsclient, p.RoleArn)
		opts = append(opts, config.WithCredentialsProvider(appCreds))
	} else if p.AccessKeyID != "" && p.SecretKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.AccessKeyID, p.SecretKeyID, "")))
	}

	if p.Region != "" {
		opts = append(opts, config.WithRegion(p.Region))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return err
	}

	p.CloudWatch = cloudwatch.NewFromConfig(cfg)

	return nil
}

// Do : Do plugin
func Do() {
	optPrefix := flag.String("metric-key-prefix", "", "Metric Key Prefix")
	optAccessKeyID := flag.String("access-key-id", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS Access Key ID")
	optSecretKeyID := flag.String("secret-key-id", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS Secret Access Key ID")
	optRegion := flag.String("region", os.Getenv("AWS_DEFAULT_REGION"), "AWS Region")
	optRoleArn := flag.String("role-arn", "", "IAM Role ARN for assume role")
	optTgwAttch := flag.String("tgw-attch", "", "Resource ID of Transit Gateway Attachement")
	optTgw := flag.String("tgw", "", "Resource ID of Transit Gateway")
	flag.Parse()

	var AwsTgwAttch AwsTgwAttchPlugin

	AwsTgwAttch.Prefix = *optPrefix
	AwsTgwAttch.AccessKeyID = *optAccessKeyID
	AwsTgwAttch.SecretKeyID = *optSecretKeyID
	AwsTgwAttch.Region = *optRegion
	AwsTgwAttch.RoleArn = *optRoleArn
	AwsTgwAttch.TgwAttach = *optTgwAttch
	AwsTgwAttch.Tgw = *optTgw

	err := AwsTgwAttch.prepare()
	if err != nil {
		log.Fatalln(err)
	}

	helper := mp.NewMackerelPlugin(AwsTgwAttch)
	helper.Run()
}
