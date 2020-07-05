package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	// Application level metrics.
	promMessageCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_messages_total",
			Help: "Total count of messages",
		},
	)
	promJoinCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_joins_total",
			Help: "Total count of new user joins",
		},
	)
	promCaptchaCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_captchas_sent_total",
			Help: "Total count of captchas sent",
		},
	)
	promCaptchaValidatedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_captchas_validated_total",
			Help: "Total count of captchas validated",
		},
	)
	promCaptchaFailedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_captchas_failed_total",
			Help: "Total count of captchas failed",
		},
	)
	promRichMessageDeletedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_rich_messages_deleted_total",
			Help: "Number of rich messages deleted from new users",
		},
	)
)

func init() {
	prometheus.MustRegister(
		promMessageCount,
		promJoinCount,
		promCaptchaCount,
		promCaptchaValidatedCount,
		promCaptchaFailedCount,
		promRichMessageDeletedCount,
	)

	// Add handlers.
	http.Handle("/metrics", promhttp.Handler())
}
