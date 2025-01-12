package cluster

import (
	"sync"

	"github.com/changlongH/srpc/client"
)

type Cluster struct {
	sync.RWMutex
	nodes map[string]*client.Client
}

var (
	inst *Cluster
	once sync.Once
)

func GetCluster() *Cluster {
	if inst == nil {
		once.Do(func() {
			inst = &Cluster{
				nodes: map[string]*client.Client{},
			}
		})
	}
	return inst
}

func (cs *Cluster) register(name, address string, opts ...client.Option) (*client.Client, error) {
	cs.Lock()
	defer cs.Unlock()
	if c, ok := cs.nodes[name]; ok {
		if c.Address != address {
			c.Close()
			delete(cs.nodes, name)
		} else {
			return nil, nil
		}
	}

	var c, err = client.NewClient(address, opts...)
	if err != nil {
		return nil, err
	}
	cs.nodes[name] = c
	return c, nil
}

func (cs *Cluster) remove(name string) {
	cs.Lock()
	defer cs.Unlock()
	var c, ok = cs.nodes[name]
	if !ok {
		return
	}
	delete(cs.nodes, name)
	c.Close()
}

func (cs *Cluster) query(name string) *client.Client {
	cs.RLock()
	defer cs.RUnlock()
	if c, ok := cs.nodes[name]; ok && !c.IsClosing() {
		return c
	}
	return nil
}

/*
Register register skynet cluster node
default withArgsCodec(json) and Timeout(5*second)

Examples:

Register("node1", "192.0.2.1:6789", client.WithArgsCodec(&client.ArgsCodecJson{})

If you want to reload node address to port:6790, old client will close after 30s
Register("node1", "192.0.2.1:6790")
*/
func Register(node string, address string, opts ...client.Option) (*client.Client, error) {
	return GetCluster().register(node, address, opts...)
}

/*
Remove remove cluster registed node

Client connecting will break afeter 30s
*/
func Remove(node string) {
	GetCluster().remove(node)
}

/*
Query query a registed node in cluster

Returns a [*client.Client]
*/
func Query(node string) *client.Client {
	return GetCluster().query(node)
}

/*
ReloadCluster register multi skynet cluster node

Default withArgsCodec(json) and Timeout(5*second)

# It's safely to Reload multi times with same config
# If client connecting. It will closed after 30s

Examples:

ReloadCluster("node1", map[string]string{"node":"127.0.0.1:8080"}, client.WithArgsCodec(&client.ArgsCodecJson{})

Returns register fail nodesErr
*/
func ReloadCluster(nodes map[string]string, opts ...client.Option) map[string]error {
	var errNodes = map[string]error{}
	var cst = GetCluster()
	for name, address := range nodes {
		if _, err := cst.register(name, address, opts...); err != nil {
			errNodes[name] = err
		}
	}
	if len(errNodes) == 0 {
		return nil
	}
	return errNodes
}
