package utils

import (
	"crypto/tls"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

// GetTLSOpts returns the TLS configuration slice for both Webhooks and Metrics.
func GetTLSOpts(enableHTTP2 bool) []func(*tls.Config) {

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}
	return tlsOpts
}

// GetMetricsOptions now uses the helper above.
func GetMetricsOptions(addr string, secure bool, enableHTTP2 bool) metricsserver.Options {
	return metricsserver.Options{
		BindAddress:    addr,
		SecureServing:  secure,
		TLSOpts:        GetTLSOpts(enableHTTP2),
		FilterProvider: filters.WithAuthenticationAndAuthorization,
	}
}
