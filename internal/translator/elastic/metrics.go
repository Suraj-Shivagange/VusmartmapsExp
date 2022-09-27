package elastic

import (
	"sort"
	"strings"
	"time"

	"go.elastic.co/apm/model"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// EncodeMetrics encodes an OpenTelemetry metrics slice, and instrumentation
// library information, as one or more metricset lines, writing to w.
//
// TODO(axw) otlpLibrary is currently not used. We should consider recording
// it as metadata.
func EncodeMetrics(otlpMetrics pmetric.MetricSlice, otlpLibrary pcommon.InstrumentationScope, w *fastjson.Writer) (dropped int, _ error) {
	var metricsets metricsets
	for i := 0; i < otlpMetrics.Len(); i++ {
		metric := otlpMetrics.At(i)

		name := metric.Name()
		switch metric.DataType() {
		case pmetric.MetricDataTypeGauge:
			doubleGauge := metric.Gauge()
			dps := doubleGauge.DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				var val float64
				switch dp.ValueType() {
				case pmetric.NumberDataPointValueTypeDouble:
					val = dp.DoubleVal()
				case pmetric.NumberDataPointValueTypeInt:
					val = float64(dp.IntVal())
				}
				metricsets.upsert(model.Metrics{
					Timestamp: asTime(dp.Timestamp()),
					Labels:    asStringMap(dp.Attributes()),
					Samples: map[string]model.Metric{name: {
						Value: val,
					}},
				})
			}
		case pmetric.MetricDataTypeSum:
			doubleSum := metric.Sum()
			dps := doubleSum.DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				var val float64
				switch dp.ValueType() {
				case pmetric.NumberDataPointValueTypeDouble:
					val = dp.DoubleVal()
				case pmetric.NumberDataPointValueTypeInt:
					val = float64(dp.IntVal())
				}
				metricsets.upsert(model.Metrics{
					Timestamp: asTime(dp.Timestamp()),
					Labels:    asStringMap(dp.Attributes()),
					Samples: map[string]model.Metric{name: {
						Value: val,
					}},
				})
			}
		case pmetric.MetricDataTypeHistogram:
			// TODO(axw) requires https://github.com/elastic/apm-server/issues/3195
			doubleHistogram := metric.Histogram()
			dropped += doubleHistogram.DataPoints().Len()
		default:
			// Unknown type, so just increment dropped by 1 as a best effort.
			dropped++
		}
	}
	for _, metricset := range metricsets {
		w.RawString(`{"metricset":`)
		if err := metricset.MarshalFastJSON(w); err != nil {
			return dropped, err
		}
		w.RawString("}\n")
	}
	return dropped, nil
}

func asTime(in pcommon.Timestamp) model.Time {
	return model.Time(time.Unix(0, int64(in)))
}

func asStringMap(in pcommon.Map) model.StringMap {
	var out model.StringMap
	in.Sort()
	in.Range(func(k string, v pcommon.Value) bool {
		out = append(out, model.StringMapItem{
			Key:   k,
			Value: v.AsString(),
		})
		return true
	})
	return out
}

type metricsets []model.Metrics

func (ms *metricsets) upsert(m model.Metrics) {
	i := ms.search(m)
	if i < len(*ms) && compareMetricsets((*ms)[i], m) == 0 {
		existing := (*ms)[i]
		for k, v := range m.Samples {
			existing.Samples[k] = v
		}
	} else {
		head := (*ms)[:i]
		tail := append([]model.Metrics{m}, (*ms)[i:]...)
		head = append(head, tail...)
		*ms = head
	}
}

func (ms *metricsets) search(m model.Metrics) int {
	return sort.Search(len(*ms), func(i int) bool {
		return compareMetricsets((*ms)[i], m) >= 0
	})
}

func compareMetricsets(a, b model.Metrics) int {
	atime, btime := time.Time(a.Timestamp), time.Time(b.Timestamp)
	if atime.Before(btime) {
		return -1
	} else if atime.After(btime) {
		return 1
	}
	n := len(a.Labels) - len(b.Labels)
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	for i, la := range a.Labels {
		lb := b.Labels[i]
		if n := strings.Compare(la.Key, lb.Key); n != 0 {
			return n
		}
		if n := strings.Compare(la.Value, lb.Value); n != 0 {
			return n
		}
	}
	return 0
}
