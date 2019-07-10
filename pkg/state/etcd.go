package state

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	etcdcliv3 "github.com/coreos/etcd/clientv3"

	"github.com/eleme/lindb/pkg/logger"
)

// etcdRepository is repository based on etc storage
type etcdRepository struct {
	namespace string
	client    *etcdcliv3.Client
}

// newEtedRepository creates a new repository based on etcd storage
func newEtedRepository(config Config) (Repository, error) {
	cfg := etcdcliv3.Config{
		Endpoints: config.Endpoints,
		// DialTimeout: config.DialTimeout * time.Second,
	}
	cli, err := etcdcliv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create etc client error:%s", err)
	}
	logger.GetLogger().Info("new etcd client successfully", zap.Any("endpoints", config.Endpoints))
	return &etcdRepository{
		namespace: config.Namespace,
		client:    cli,
	}, nil
}

// Get retrieves value for given key from etcd
func (r *etcdRepository) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := r.get(ctx, key)
	if err != nil {
		return nil, err
	}
	return r.getValue(key, resp)
}

// List retrieves list for given prefix from etcd
func (r *etcdRepository) List(ctx context.Context, prefix string) ([][]byte, error) {
	resp, err := r.client.Get(ctx, prefix, etcdcliv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var result [][]byte

	if len(resp.Kvs) > 0 {
		for _, kv := range resp.Kvs {
			if len(kv.Value) > 0 {
				result = append(result, kv.Value)
			}
		}
	}
	return result, nil
}

// Put puts a key-value pair into etcd
func (r *etcdRepository) Put(ctx context.Context, key string, val []byte) error {
	_, err := r.client.Put(ctx, key, string(val))
	return err
}

// Delete deletes value for given key from etcd
func (r *etcdRepository) Delete(ctx context.Context, key string) error {
	_, err := r.client.Delete(ctx, key)
	return err
}

// Close closes etcd client
func (r *etcdRepository) Close() error {
	return r.client.Close()
}

// Heartbeat does heartbeat on the key with a value and ttl based on etcd
func (r *etcdRepository) Heartbeat(ctx context.Context, key string, value []byte, ttl int64) (<-chan Closed, error) {
	h := newHeartbeat(r.client, key, value, ttl)
	err := h.grantKeepAliveLease(ctx)
	if err != nil {
		return nil, err
	}
	ch := make(chan Closed)
	// do keepalive/retry background
	go func() {
		// close closed channel, if keep alive stopped
		defer close(ch)
		h.keepAlive(ctx, false)
	}()
	return ch, nil
}

// PutIfNotExitAndKeepLease  puts a key with a value.it will be success
// if the key does not exist,otherwise it will be failed.When this
// operation success,it will do keepalive background
func (r *etcdRepository) PutIfNotExist(ctx context.Context, key string,
	value []byte, ttl int64) (bool, <-chan Closed, error) {
	h := newHeartbeat(r.client, key, value, ttl)
	success, err := h.PutIfNotExist(ctx)
	if err != nil {
		return false, nil, err
	}
	// when put success,do keep alive
	if success {
		ch := make(chan Closed)
		// do keepalive/retry background
		go func() {
			// close closed channel, if keep alive stopped
			defer close(ch)
			h.keepAlive(ctx, true)
		}()
		return success, ch, nil
	}
	return success, nil, nil
}

// get returns response of get operator
func (r *etcdRepository) get(ctx context.Context, key string) (*etcdcliv3.GetResponse, error) {
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get value failure for key[%s], error:%s", key, err)
	}
	return resp, nil
}

// getValue returns value of get's response
func (r *etcdRepository) getValue(key string, resp *etcdcliv3.GetResponse) ([]byte, error) {
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("key[%s] not exist", key)
	}

	firstkv := resp.Kvs[0]
	if len(firstkv.Value) == 0 {
		return nil, fmt.Errorf("key[%s]'s value is empty", key)
	}
	return firstkv.Value, nil
}

// Watch watches on a key. The watched events will be returned through the returned channel.
//
// NOTE: when caller meets EventTypeAll, it must clean all previous values, since it may contains
// deleted values we do not know.
func (r *etcdRepository) Watch(ctx context.Context, key string) WatchEventChan {
	watcher := newWatcher(ctx, r, key)
	return watcher.EventC
}

// WatchPrefix watches on a prefix.All of the changes who has the prefix
// will be notified through the WatchEventChan channel.
//
// NOTE: when caller meets EventTypeAll, it must clean all previous values, since it may contains
// deleted values we do not know.
func (r *etcdRepository) WatchPrefix(ctx context.Context, prefixKey string) WatchEventChan {
	watcher := newWatcher(ctx, r, prefixKey, etcdcliv3.WithPrefix())
	return watcher.EventC
}

// Txn returns a etcdcliv3.Txn.
func (r *etcdRepository) Txn(ctx context.Context) etcdcliv3.Txn {
	return r.client.Txn(ctx)
}
