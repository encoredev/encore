//go:build !encore_no_aws

package aws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/system"
	"encore.dev/internal/nativehist"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.AWSCloudWatchMetricsProvider, meta *metadata.ContainerMetadata, rootLogger zerolog.Logger) *Exporter {
	return &Exporter{
		svcs:              svcs,
		cfg:               cfg,
		containerMetadata: meta,
		rootLogger:        rootLogger,
	}
}

type Exporter struct {
	svcs              []string
	cfg               *config.AWSCloudWatchMetricsProvider
	containerMetadata *metadata.ContainerMetadata
	rootLogger        zerolog.Logger

	clientMu sync.Mutex
	client   *cloudwatch.Client
}

func (x *Exporter) Shutdown(force context.Context) {
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now()
	data := x.getMetricData(now, collected)
	data = append(data, x.getSysMetrics(now)...)
	_, err := x.getClient().PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		MetricData: data,
		Namespace:  aws.String(x.cfg.Namespace),
	})
	if err != nil {
		return fmt.Errorf("unable to send metrics to AWS CloudWatch: %v", err)
	}
	return nil
}

func (x *Exporter) getMetricData(now time.Time, collected []metrics.CollectedMetric) []types.MetricDatum {
	data := make([]types.MetricDatum, 0, len(collected))

	doAdd := func(val float64, metricName string, baseDims []types.Dimension, svcIdx uint16) {
		containerMetadataDims := containerMetadataDimensions(x.containerMetadata)
		dims := make([]types.Dimension, 0, len(baseDims)+len(containerMetadataDims)+1)
		copy(dims, baseDims)
		dims = append(dims, append(containerMetadataDims, types.Dimension{
			Name:  aws.String("service"),
			Value: aws.String(x.svcs[svcIdx]),
		})...)
	}

	for _, m := range collected {
		var dims []types.Dimension
		if n := len(m.Labels); n > 0 {
			dims = make([]types.Dimension, 0, n)
			for _, label := range m.Labels {
				dims = append(dims, types.Dimension{
					Name:  aws.String(label.Key),
					Value: aws.String(label.Value),
				})
			}
		}

		svcNum := m.Info.SvcNum()
		switch vals := m.Val.(type) {
		case []float64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(vals[0], m.Info.Name(), dims, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(val, m.Info.Name(), dims, uint16(i))
					}
				}
			}
		case []int64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]), m.Info.Name(), dims, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val), m.Info.Name(), dims, uint16(i))
					}
				}
			}
		case []uint64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]), m.Info.Name(), dims, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val), m.Info.Name(), dims, uint16(i))
					}
				}
			}
		case []time.Duration:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]/time.Second), m.Info.Name(), dims, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val/time.Second), m.Info.Name(), dims, uint16(i))
					}
				}
			}
		case []*nativehist.Histogram:
			// TODO implement support
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s",
				m.Val, m.Info.Name())
		}
	}

	return data
}

func (x *Exporter) getClient() *cloudwatch.Client {
	x.clientMu.Lock()
	defer x.clientMu.Unlock()
	if x.client == nil {
		cfg, err := awsconfig.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(fmt.Sprintf("unable to load AWS config: %v", err))
		}
		cl := cloudwatch.NewFromConfig(cfg)
		x.client = cl
	}
	return x.client
}

func (x *Exporter) getSysMetrics(now time.Time) []types.MetricDatum {
	sysMetrics := system.ReadSysMetrics()
	containerMetadataDims := containerMetadataDimensions(x.containerMetadata)
	return []types.MetricDatum{
		{
			MetricName: aws.String(system.MetricNameMemUsageBytes),
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(float64(sysMetrics[system.MetricNameMemUsageBytes])),
			Dimensions: containerMetadataDims,
		},
		{
			MetricName: aws.String(system.MetricNameNumGoroutines),
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(float64(sysMetrics[system.MetricNameNumGoroutines])),
			Dimensions: containerMetadataDims,
		},
	}
}

func containerMetadataDimensions(meta *metadata.ContainerMetadata) []types.Dimension {
	return []types.Dimension{
		{
			Name:  aws.String("service_id"),
			Value: aws.String(meta.ServiceID),
		},
		{
			Name:  aws.String("revision_id"),
			Value: aws.String(meta.RevisionID),
		},
		{
			Name:  aws.String("instance_id"),
			Value: aws.String(meta.InstanceID),
		},
	}
}
