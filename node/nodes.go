package node

import (
	"context"
	"errors"
	"github.com/node-real/go-pkg/log"
	"github.com/node-real/go-pkg/utils/syncutils"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	"github.com/reneecok/puissant-proxy/ethclient"
)

type Nodes interface {
	SendPuissant(ctx context.Context, args SendPuissantArgs) error
}

type Config struct {
	URLs            []string
	ExpectedVersion string
	MaxConnPerNode  int
}

type nodes struct {
	expectedVersion string
	nodes           []Node
}

func NewNodes(ctx context.Context, cfg *Config) Nodes {
	n := &nodes{
		expectedVersion: cfg.ExpectedVersion,
		nodes:           make([]Node, len(cfg.URLs)),
	}

	client := ethclient.NewClient(cfg.MaxConnPerNode)

	for i, v := range cfg.URLs {
		n.nodes[i] = newNode(v, client)
	}

	statusProbe := func() {
		n.statusProbe(ctx)
	}

	go wait.Until(statusProbe, 5*time.Minute, ctx.Done())

	return n
}

// SendPuissant will send puissant to active nodes,
// return nil when all nodes return nil, return error when any node return error
func (n *nodes) SendPuissant(ctx context.Context, args SendPuissantArgs) error {
	br := syncutils.NewBatchRunner()

	for _, v := range n.nodes {
		v := v

		// skip inactive validator
		if !v.Active() {
			continue
		}

		br.AddTasks(func() error {
			er := v.SendPuissant(ctx, args)
			if er != nil {
				log.Errorw("fail to send puissant", "err", er)
				return er
			}

			return nil
		})
	}

	brDone := make(chan error, 1)

	go func() {
		brDone <- br.Exec()

		close(brDone)
	}()

	select {
	case err, ok := <-brDone:
		if !ok {
			log.Errorw("Receive data channel close msg!")
			return nil
		}

		if err != nil {
			log.Errorw("fail to batch send puissant ", "err", err)
			return err
		}
		return nil
	case <-ctx.Done():
		return errors.New("request timeout")
	}
}

func (n *nodes) statusProbe(ctx context.Context) {
	for _, v := range n.nodes {
		version, err := v.ClientVersion(ctx)
		if err != nil {
			log.Errorw("fail to maintain client version", "err", err)
			v.SetActive(false)

			continue
		}

		if version >= n.expectedVersion {
			v.SetActive(true)
		} else {
			v.SetActive(false)
		}
	}
}
