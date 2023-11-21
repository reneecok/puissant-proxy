package ethclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"github.com/node-real/go-ethlibs/jsonrpc"
	"github.com/node-real/go-pkg/log"
)

func NewClient(maxConnsPerHost int) *Client {
	dialer := &net.Dialer{
		Timeout:   time.Second,
		KeepAlive: 60 * time.Second,
	}

	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		MaxIdleConnsPerHost: 2000,
		MaxConnsPerHost:     maxConnsPerHost,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{
		Timeout:   6 * time.Minute,
		Transport: transport,
	}

	return &Client{
		httpClient: httpClient,
	}
}

type Client struct {
	httpClient *http.Client
}

// JSONRPCCall param url is a specified url
func (c *Client) JSONRPCCall(ctx context.Context, url string, req *jsonrpc.Request) (*jsonrpc.RawResponse, error) {
	reqByte, err := jsoniter.Marshal(req)
	if err != nil {
		log.Errorw("fail to marshal json rpc req", "req", req, "url", url, "err", err)
		return nil, err
	}

	httpReq, httpErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqByte))
	if httpErr != nil {
		log.Errorw("fail to new http request", "url", url, "err", httpErr)
		return nil, httpErr
	}

	httpReq.Header.Set("Content-Type", gin.MIMEJSON)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		log.Errorw("fail to do json rpc call", "method", req.Method, "url", url, "err", err)
		return nil, fmt.Errorf("json rpc call failed, method:%v, url:%v", req.Method, url)
	}

	defer httpResp.Body.Close()

	if !HttpCode(httpResp.StatusCode).Success() {
		log.Errorw("fail to do json rpc call", "method", req.Method, "code", httpResp.StatusCode, "url", url)
		return nil, fmt.Errorf("json rpc call failed, method:%v, url:%v, code:%v",
			req.Method, url, httpResp.StatusCode)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Errorw("fail to read json rpc resp body", "method", req.Method, "url", url, "err", err)
		return nil, err
	}

	resp := jsonrpc.RawResponse{}
	err = jsoniter.Unmarshal(body, &resp)
	if err != nil {
		log.Errorw("fail to unmarshal json rpc resp body", "method", req.Method, "url", url, "err", err)
		return nil, err
	}

	return &resp, nil
}

type HttpCode int

func (code HttpCode) Success() bool {
	return 200 <= code && code <= 299
}
