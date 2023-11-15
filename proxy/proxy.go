package proxy

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	jsoniter "github.com/json-iterator/go"
	"github.com/node-real/go-pkg/log"
	"github.com/node-real/go-pkg/units"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/valyala/fasthttp"

	"github.com/reneecok/puissant-proxy/node"
)

var (
	namespace = "puissant_proxy_validator"

	apiLatencyHist = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "api",
		Name:      "latency",
		Buckets:   prometheus.ExponentialBuckets(0.01, 3, 15),
	}, []string{"method"})
)

type Proxy struct {
	timeout           units.Duration
	nodes             node.Nodes
	puissantReportUrl string
}

type Config struct {
	HTTPListenAddr    string
	Concurrency       int64
	Timeout           units.Duration
	PuissantReportURL string
}

func NewValidatorProxy(cfg *Config, nodes node.Nodes) *Proxy {
	return &Proxy{
		timeout:           cfg.Timeout,
		nodes:             nodes,
		puissantReportUrl: cfg.PuissantReportURL,
	}
}

func (s *Proxy) SendPuissant(ctx context.Context, args node.SendPuissantArgs) error {
	method := "eth_sendPuissant"
	start := time.Now()
	defer recordLatency(method, start)
	defer timeoutCancel(&ctx, s.timeout)()

	return s.nodes.SendPuissant(ctx, args)
}

// TODO refer geth code
type puissantStatusCode uint8
type puissantInfoCode uint8
type puissantTxStatusCode uint8

type tUploadTransaction struct {
	TxHash    string               `json:"tx_hash"`
	GasUsed   uint64               `json:"gas_used"`
	Status    puissantTxStatusCode `json:"status"`
	RevertMsg string               `json:"revert_msg"`
}

type tUploadPuissant struct {
	UUID   string                `json:"uuid"`
	Status puissantStatusCode    `json:"status"`
	Info   puissantInfoCode      `json:"info"`
	Txs    []*tUploadTransaction `json:"txs"`
}

type tUploadDataWithText struct {
	BlockNumber string             `json:"block"`
	Text        string             `json:"text"`
	Result      []*tUploadPuissant `json:"result"`
}

func (s *Proxy) ReportPuissant(ctx context.Context, report tUploadDataWithText) {
	method := "eth_reportPuissant"
	start := time.Now()
	defer recordLatency(method, start)
	defer timeoutCancel(&ctx, s.timeout)()

	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	log.Infow("report packing result", "report", report.Text)

	//// TODO get header
	//var msgSigner func([]byte) []byte
	//
	//if err := doRequest(s.puissantReportUrl, report, req, resp, msgSigner); err != nil {
	//	log.Errorw("❌ report packing result failed", "err", err)
	//}
}

func recordLatency(method string, start time.Time) {
	apiLatencyHist.WithLabelValues(method).Observe(float64(time.Since(start).Milliseconds()))
}

func nilCancel() {
}

func timeoutCancel(ctx *context.Context, timeout units.Duration) func() {
	if timeout > 0 {
		var cancel func()
		*ctx, cancel = context.WithTimeout(*ctx, time.Duration(timeout))
		return cancel
	}

	return nilCancel
}

func doRequest(url string, data interface{}, req *fasthttp.Request, resp *fasthttp.Response, msgSigner func([]byte) []byte) error {
	req.SetRequestURI(url)
	req.Header.Set("Content-Type", "application/json")
	req.Header.SetMethod(http.MethodPost)

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", hexutil.Encode(msgSigner([]byte(timestamp))))

	b, _ := jsoniter.Marshal(data)
	req.SetBodyRaw(b)

	return fasthttp.DoTimeout(req, resp, 2*time.Second)
}
