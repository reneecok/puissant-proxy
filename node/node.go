package node

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	jsoniter "github.com/json-iterator/go"
	"github.com/node-real/go-ethlibs/jsonrpc"
	"github.com/node-real/go-pkg/log"

	"github.com/reneecok/puissant-proxy/ethclient"
)

const usualErr = "the method eth_sendPuissant does not exist/is not available"

type Node interface {
	Active() bool

	SetActive(bool)

	SendPuissant(context.Context, SendPuissantArgs) error

	ClientVersion(context.Context) (string, error)
}

func newNode(url string, client *ethclient.Client) Node {
	return &node{
		url:    url,
		client: client,
	}
}

type node struct {
	url    string
	active bool

	client *ethclient.Client
}

type SendPuissantArgs struct {
	Txs            []hexutil.Bytes `json:"txs"`
	MaxTimestamp   uint64          `json:"maxTimestamp"`
	Revertible     []common.Hash   `json:"revertible"`
	RelaySignature hexutil.Bytes   `json:"relaySignature"`
}

func (n *node) Active() bool {
	return n.active
}

func (n *node) SetActive(active bool) {
	n.active = active
}

func (n *node) SendPuissant(ctx context.Context, args SendPuissantArgs) error {
	if !n.Active() {
		return errors.New(usualErr)
	}

	param, _ := jsoniter.Marshal(args)

	req := jsonrpc.NewRequest()
	req.Method = "eth_sendPuissant"
	req.Params = []jsonrpc.Param{param}

	resp, err := n.client.JSONRPCCall(ctx, n.url, req)
	if err != nil {
		log.Errorw("fail to do json rpc call", "url", n.url, "err", err)
		return err
	}

	if resp.Error != nil {
		log.Errorw("json rpc call return error", "url", n.url, "err", err)

		jrError := jsonrpc.Error{}
		err = jsoniter.Unmarshal(*resp.Error, &jrError)
		if err != nil {
			log.Warnw("unmarshal resp.Error of %v failed", req.Method)
			return errors.New(usualErr)
		}

		return errors.New(jrError.Message)
	}

	return nil
}

func (n *node) ClientVersion(ctx context.Context) (string, error) {
	req := jsonrpc.NewRequest()
	req.Method = "web3_clientVersion"

	resp, err := n.client.JSONRPCCall(ctx, n.url, req)
	if err != nil {
		log.Errorw("fail to do json rpc call", "url", n.url, "err", err)
		return "", err
	}

	if resp.Error != nil {
		log.Errorw("json rpc call return error", "url", n.url, "err", err)
		return "", errors.New(string(*resp.Error))
	}

	var result string

	err = jsoniter.Unmarshal(resp.Result, &result)
	if err != nil {
		log.Errorw("fail to unmarshal json rpc result", "err", err)
		return "", err
	}

	return result, nil
}
