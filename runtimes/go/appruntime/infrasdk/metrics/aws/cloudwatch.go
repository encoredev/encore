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

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/system"
	"encore.dev/appruntime/shared/nativehist"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.AWSCloudWatchMetricsProvider, meta *metadata.ContainerMetadata, rootLogger zerolog.Logger) *Exporter {
	// Precompute container metadata dimensions.
	exporter := &Exporter{
		svcs:       svcs,
		cfg:        cfg,
		rootLogger: rootLogger,
		containerMetadataDims: metadata.MapMetadataLabels(meta, func(key, value string) types.Dimension {
			return types.Dimension{
				Name:  aws.String(key),
				Value: aws.String(value),
			}
		}),
	}

	return exporter
}

type Exporter struct {
	svcs                  []string
	cfg                   *config.AWSCloudWatchMetricsProvider
	containerMetadataDims []types.Dimension
	rootLogger            zerolog.Logger

	clientMu sync.Mutex
	client   *cloudwatch.Client
}

func (x *Exporter) Shutdown(p *shutdown.Process) error {
	return nil
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now()
	data := x.getMetricData(now, collected)
	data = append(data, x.getSysMetrics(now)...)

	// CloudWatch has a maximum of 1000 metrics per PutMetricData request
	const maxMetricsPerRequest = 1000
	client := x.getClient()

	for i := 0; i < len(data); i += maxMetricsPerRequest {
		end := min(i+maxMetricsPerRequest, len(data))

		batch := data[i:end]
		_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
			MetricData: batch,
			Namespace:  aws.String(x.cfg.Namespace),
		})
		if err != nil {
			return fmt.Errorf("unable to send metrics to AWS CloudWatch: %v", err)
		}
	}

	return nil
}

func (x *Exporter) getMetricData(now time.Time, collected []metrics.CollectedMetric) []types.MetricDatum {
	data := make([]types.MetricDatum, 0, len(collected))

	doAdd := func(val float64, metricName string, baseDims []types.Dimension, svcIdx uint16) {
		dims := make([]types.Dimension, len(baseDims)+1)
		copy(dims, baseDims)
		dims[len(baseDims)] = types.Dimension{
			Name:  aws.String("service"),
			Value: aws.String(x.svcs[svcIdx]),
		}
		data = append(data, types.MetricDatum{
			MetricName: aws.String(metricName),
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(val),
			Dimensions: dims,
		})
	}

	for _, m := range collected {
		dims := make([]types.Dimension, len(x.containerMetadataDims), len(x.containerMetadataDims)+len(m.Labels))
		copy(dims, x.containerMetadataDims)
		for _, label := range m.Labels {
			if label.Value == "" {
				x.rootLogger.Warn().Str("label", label.Key).Msg("metrics: aws cloudwatch does not support empty label values, skipping")
				continue
			}
			dims = append(dims, types.Dimension{
				Name:  aws.String(label.Key),
				Value: aws.String(label.Value),
			})
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

func (x *Exporter) getSysMetrics(now time.Time) []types.MetricDatum {
	sysMetrics := system.ReadSysMetrics(x.rootLogger)
	return []types.MetricDatum{
		{
			MetricName: aws.String(system.MetricNameHeapObjectsBytes),
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(float64(sysMetrics[system.MetricNameHeapObjectsBytes])),
			Dimensions: x.containerMetadataDims,
		},
		{
			MetricName: aws.String(system.MetricNameGoroutines),
			Timestamp:  aws.Time(now),
			Value:      aws.Float64(float64(sysMetrics[system.MetricNameGoroutines])),
			Dimensions: x.containerMetadataDims,
		},
	}
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
