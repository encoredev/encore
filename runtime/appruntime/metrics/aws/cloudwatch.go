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
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.AWSCloudWatchMetricsProvider, rootLogger zerolog.Logger) *Exporter {
	return &Exporter{
		svcs:       svcs,
		cfg:        cfg,
		rootLogger: rootLogger,
	}
}

type Exporter struct {
	svcs       []string
	cfg        *config.AWSCloudWatchMetricsProvider
	rootLogger zerolog.Logger

	clientMu sync.Mutex
	client   *cloudwatch.Client
}

func (x *Exporter) Shutdown(force context.Context) {
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now()
	data := x.getMetricData(now, collected)
	_, err := x.getClient().PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		MetricData: data,
		Namespace:  aws.String(x.cfg.Namespace),
	})
	if err != nil {
		return fmt.Errorf("unable to send metrics to GCP Cloud Monitoring: %v", err)
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
				doAdd(vals[0], m.Info.Name(), dims, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(val, m.Info.Name(), dims, uint16(i))
				}
			}
		case []int64:
			if svcNum > 0 {
				doAdd(float64(vals[0]), m.Info.Name(), dims, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(float64(val), m.Info.Name(), dims, uint16(i))
				}
			}
		case []uint64:
			if svcNum > 0 {
				doAdd(float64(vals[0]), m.Info.Name(), dims, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(float64(val), m.Info.Name(), dims, uint16(i))
				}
			}
		case []time.Duration:
			if svcNum > 0 {
				doAdd(float64(vals[0]/time.Second), m.Info.Name(), dims, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(float64(val/time.Second), m.Info.Name(), dims, uint16(i))
				}
			}
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
