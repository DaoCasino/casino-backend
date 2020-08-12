package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const prometheusPrefix = "casino_"

var (
	registry   *prometheus.Registry
	registerer prometheus.Registerer

	SigniDiceProcessingTimeMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "signidice_part_2_event_ms",
			Help:    "signidice part 2 event processing time in ms",
			Buckets: []float64{20, 50, 100, 200, 500},
		})

	SignTransactionProcessingTimeMs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "http_sign_transaction_ms",
			Help:    "HTTP /sign_transaction query processing time in ms",
			Buckets: []float64{20, 50, 100, 200, 500},
		})
)

func init() {
	registry = prometheus.NewRegistry()
	registerer = prometheus.WrapRegistererWithPrefix(prometheusPrefix, registry)
	registerer.MustRegister(prometheus.NewGoCollector())
	registerer.MustRegister(SigniDiceProcessingTimeMs)
	registerer.MustRegister(SignTransactionProcessingTimeMs)
}

func GetHandler() http.Handler {
	return promhttp.InstrumentMetricHandler(registerer, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
}
