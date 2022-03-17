package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	promPatternMessageDeletedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_pattern_messages_deleted_total",
			Help: "Number of messages deleted automatically by pattern matching",
		},
	)
	promPatternKickBannedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "opbot_pattern_kick_bans_total",
			Help: "Number of users kicked or banned by message pattern matching",
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
		promPatternMessageDeletedCount,
		promPatternKickBannedCount,
	)

	// Add handlers.
	http.Handle("/metrics", promhttp.Handler())
}
