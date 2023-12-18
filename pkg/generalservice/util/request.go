package util

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coredns/coredns/plugin/pkg/log"
	"moul.io/http2curl/v2"
)

func DoRequest[Result any](ctx context.Context, url string, method string, header http.Header, data interface{}, debug bool) (result Result, err error) {
	var transport *http.Transport
	var httpClient *http.Client

	transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
	}
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient = &http.Client{Transport: transport}

	body, err := json.Marshal(data)
	if err != nil {
		return result, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header = header

	if debug {
		command, _ := http2curl.GetCurlCommand(req)
		log.Debug(command.String())
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if len(respBody) > 0 {
			err = json.Unmarshal(respBody, &result)
		}
		return result, err
	}

	return result, fmt.Errorf("status: %v, result :%s", resp.StatusCode, respBody)
}
