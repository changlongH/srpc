package cluster

import (
	"sync"
)

type Cluster struct {
	sync.RWMutex
	nodes map[string]*Client
}

var (
	inst *Cluster
	once sync.Once
)

func GetCluster() *Cluster {
	if inst == nil {
		once.Do(func() {
			inst = &Cluster{
				nodes: map[string]*Client{},
			}
		})
	}
	return inst
}

func (cs *Cluster) register(name, address string, opts ...ClientOption) (*Client, error) {
	cs.Lock()
	defer cs.Unlock()
	if c, ok := cs.nodes[name]; ok {
		if c.Address != address {
			c.Close()
			delete(cs.nodes, name)
		} else {
			//errors.New("register duplicate name: " + name)
			return nil, nil
		}
	}

	var c, err = NewClient(address, opts...)
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

func (cs *Cluster) query(name string) *Client {
	cs.RLock()
	defer cs.RUnlock()
	if c, ok := cs.nodes[name]; ok && !c.closing {
		return c
	}
	return nil
}

// Register register skynet cluster node
// Examples:
//
//	Register("node1", "192.0.2.1:6789")
//	Register("node2", "192.0.2.1:6790")
func Register(node string, address string, opts ...ClientOption) (*Client, error) {
	return GetCluster().register(node, address, opts...)
}

func Remove(node string) {
	GetCluster().remove(node)
}

func Query(node string) *Client {
	return GetCluster().query(node)
}

// ReloadCluster Register multi address
// returns register err nodes
func ReloadCluster(nodes map[string]string, opts ...ClientOption) map[string]error {
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
