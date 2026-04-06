//go:build !encore_no_azure

package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/system"
	"encore.dev/appruntime/shared/nativehist"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/metrics"
)

// New creates a new Azure Monitor metrics exporter.
func New(svcs []string, cfg *config.AzureMonitorMetricsProvider, meta *metadata.ContainerMetadata, rootLogger zerolog.Logger) *Exporter {
	return &Exporter{
		svcs:       svcs,
		cfg:        cfg,
		rootLogger: rootLogger,
		containerMetaDims: metadata.MapMetadataLabels(meta, func(key, value string) dimKV {
			return dimKV{key: key, value: value}
		}),
	}
}

type dimKV struct {
	key, value string
}

// metricBatch groups series that share the same dimension names for a single
// Azure Monitor custom-metrics POST request.
type metricBatch struct {
	dimNames []string
	series   []azureCustomMetricSeries
}

// Exporter sends Encore metrics to Azure Monitor using the Custom Metrics REST API.
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-custom-overview
type Exporter struct {
	svcs              []string
	cfg               *config.AzureMonitorMetricsProvider
	containerMetaDims []dimKV
	rootLogger        zerolog.Logger

	credMu sync.Mutex
	cred   *azidentity.DefaultAzureCredential
}

func (x *Exporter) Shutdown(p *shutdown.Process) error {
	return nil
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now().UTC()

	batches := x.getMetricBatches(now, collected)
	for name, b := range x.getSysBatches(now) {
		batches[name] = b
	}

	token, err := x.getToken(ctx)
	if err != nil {
		return fmt.Errorf("azure monitor: get auth token: %w", err)
	}

	for metricName, batch := range batches {
		if err := x.sendBatch(ctx, token, now, metricName, batch); err != nil {
			return err
		}
	}
	return nil
}

// getMetricBatches converts collected Encore metrics into per-name batches ready for posting.
func (x *Exporter) getMetricBatches(now time.Time, collected []metrics.CollectedMetric) map[string]metricBatch {
	result := make(map[string]metricBatch)

	for _, m := range collected {
		// Build base dimension list: container metadata dims + metric label dims.
		baseDims := make([]dimKV, 0, len(x.containerMetaDims)+len(m.Labels))
		baseDims = append(baseDims, x.containerMetaDims...)
		for _, l := range m.Labels {
			baseDims = append(baseDims, dimKV{key: l.Key, value: l.Value})
		}

		svcNum := m.Info.SvcNum()

		doAdd := func(s azureCustomMetricSeries, svcIdx uint16) {
			dims := append(baseDims, dimKV{key: "service", value: x.svcs[svcIdx]})

			dimNames := make([]string, len(dims))
			dimValues := make([]string, len(dims))
			for i, kv := range dims {
				dimNames[i] = kv.key
				dimValues[i] = kv.value
			}
			s.DimValues = dimValues

			b := result[m.Info.Name()]
			if b.dimNames == nil {
				b.dimNames = dimNames
			}
			b.series = append(b.series, s)
			result[m.Info.Name()] = b
		}

		scalarSeries := func(val float64) azureCustomMetricSeries {
			return azureCustomMetricSeries{Sum: val, Count: 1, Min: val, Max: val}
		}

		switch vals := m.Val.(type) {
		case []float64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(scalarSeries(vals[0]), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(scalarSeries(val), uint16(i))
					}
				}
			}
		case []int64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(scalarSeries(float64(vals[0])), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(scalarSeries(float64(val)), uint16(i))
					}
				}
			}
		case []uint64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(scalarSeries(float64(vals[0])), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(scalarSeries(float64(val)), uint16(i))
					}
				}
			}
		case []time.Duration:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(scalarSeries(float64(vals[0]/time.Second)), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(scalarSeries(float64(val/time.Second)), uint16(i))
					}
				}
			}
		case []*nativehist.Histogram:
			if svcNum > 0 {
				if m.Valid[0].Load() && vals[0] != nil {
					st := vals[0].Stats()
					doAdd(azureCustomMetricSeries{
						Sum:   st.Sum,
						Count: int(st.Count),
						Min:   st.Min,
						Max:   st.Max,
					}, svcNum-1)
				}
			} else {
				for i, h := range vals {
					if m.Valid[i].Load() && h != nil {
						st := h.Stats()
						doAdd(azureCustomMetricSeries{
							Sum:   st.Sum,
							Count: int(st.Count),
							Min:   st.Min,
							Max:   st.Max,
						}, uint16(i))
					}
				}
			}
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s", m.Val, m.Info.Name())
		}
	}
	return result
}

// getSysBatches returns batches for Go runtime system metrics.
func (x *Exporter) getSysBatches(now time.Time) map[string]metricBatch {
	sysMetrics := system.ReadSysMetrics(x.rootLogger)

	dimNames := make([]string, len(x.containerMetaDims))
	dimValues := make([]string, len(x.containerMetaDims))
	for i, kv := range x.containerMetaDims {
		dimNames[i] = kv.key
		dimValues[i] = kv.value
	}

	makeBatch := func(val uint64) metricBatch {
		f := float64(val)
		return metricBatch{
			dimNames: dimNames,
			series: []azureCustomMetricSeries{
				{DimValues: dimValues, Sum: f, Count: 1, Min: f, Max: f},
			},
		}
	}

	return map[string]metricBatch{
		system.MetricNameHeapObjectsBytes: makeBatch(sysMetrics[system.MetricNameHeapObjectsBytes]),
		system.MetricNameGoroutines:       makeBatch(sysMetrics[system.MetricNameGoroutines]),
	}
}

// azureCustomMetricPayload is the JSON body for the Azure Monitor custom metrics REST API.
type azureCustomMetricPayload struct {
	Time string                     `json:"time"`
	Data azureCustomMetricData      `json:"data"`
}

type azureCustomMetricData struct {
	BaseData azureCustomMetricBaseData `json:"baseData"`
}

type azureCustomMetricBaseData struct {
	Metric   string                   `json:"metric"`
	Namespace string                  `json:"namespace"`
	DimNames []string                 `json:"dimNames,omitempty"`
	Series   []azureCustomMetricSeries `json:"series"`
}

type azureCustomMetricSeries struct {
	DimValues []string `json:"dimValues,omitempty"`
	Sum       float64  `json:"sum"`
	Count     int      `json:"count"`
	Min       float64  `json:"min"`
	Max       float64  `json:"max"`
}

func (x *Exporter) sendBatch(ctx context.Context, token string, now time.Time, metricName string, batch metricBatch) error {
	if len(batch.series) == 0 {
		return nil
	}

	payload := azureCustomMetricPayload{
		Time: now.Format(time.RFC3339),
		Data: azureCustomMetricData{
			BaseData: azureCustomMetricBaseData{
				Metric:    metricName,
				Namespace: x.cfg.Namespace,
				DimNames:  batch.dimNames,
				Series:    batch.series,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("azure monitor: marshal payload for metric %s: %w", metricName, err)
	}

	url := fmt.Sprintf(
		"https://%s.monitoring.azure.com/subscriptions/%s/resourceGroups/%s/providers/%s/%s/metrics",
		x.cfg.Location,
		x.cfg.SubscriptionID,
		x.cfg.ResourceGroup,
		x.cfg.ResourceNamespace,
		x.cfg.ResourceName,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("azure monitor: create request for metric %s: %w", metricName, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("azure monitor: send metric %s: %w", metricName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("azure monitor: unexpected status %d for metric %s", resp.StatusCode, metricName)
	}
	return nil
}

// getToken returns a fresh bearer token for the Azure Monitor scope.
// The azidentity credential caches the token and refreshes it automatically.
func (x *Exporter) getToken(ctx context.Context) (string, error) {
	cred, err := x.getCred()
	if err != nil {
		return "", err
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://monitoring.azure.com/.default"},
	})
	if err != nil {
		return "", err
	}
	return tok.Token, nil
}

func (x *Exporter) getCred() (*azidentity.DefaultAzureCredential, error) {
	x.credMu.Lock()
	defer x.credMu.Unlock()
	if x.cred == nil {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("create Azure credential: %w", err)
		}
		x.cred = cred
	}
	return x.cred, nil
}
