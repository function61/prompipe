package prompipeclient

import (
	"bytes"
	"context"
	"github.com/function61/gokit/ezhttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"io"
)

const (
	promContentType = "text/plain; version=0.0.4; charset=utf-8"
)

type Client struct {
	endpoint    string
	bearerToken string
}

func New(endpoint string, bearerToken string) *Client {
	return &Client{endpoint, bearerToken}
}

func (c *Client) Send(ctx context.Context, registry *prometheus.Registry) error {
	wireBytes := &bytes.Buffer{}

	if err := GatherToTextExport(registry, wireBytes); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, ezhttp.DefaultTimeout10s)
	defer cancel()

	_, err := ezhttp.Put(
		ctx,
		c.endpoint,
		ezhttp.AuthBearer(c.bearerToken),
		ezhttp.SendBody(wireBytes, promContentType),
	)

	return err
}

func GatherToTextExport(registry *prometheus.Registry, output io.Writer) error {
	metricFamilies, err := registry.Gather()
	if err != nil {
		return err
	}

	wireEncoder := expfmt.NewEncoder(output, expfmt.FmtText)

	for _, metricFamily := range metricFamilies {
		if err := wireEncoder.Encode(metricFamily); err != nil {
			return err
		}
	}

	return nil
}
