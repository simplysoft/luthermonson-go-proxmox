package proxmox

import (
	"context"
	"fmt"
)

func (cl *Cluster) SdnZones(ctx context.Context) (zones []*SDNZone, err error) {
	err = cl.client.Get(ctx, "/cluster/sdn/zones", &zones)

	if nil == err {
		for _, n := range zones {
			n.client = cl.client
		}
	}
	return
}

func (cl *Cluster) SdnVNets(ctx context.Context) (vnets SDNVNets, err error) {
	err = cl.client.Get(ctx, "/cluster/sdn/vnets", &vnets)

	if nil == err {
		for _, n := range vnets {
			n.client = cl.client
		}
	}
	return
}

func (n *Node) SdnZones(ctx context.Context) (zones NodeSDNZones, err error) {
	err = n.client.Get(ctx, fmt.Sprintf("/nodes/%s/sdn/zones", n.Name), &zones)

	if nil == err {
		for _, z := range zones {
			z.client = n.client
			z.node = n
		}
	}
	return
}

func (z *NodeSDNZone) SdnZoneContent(ctx context.Context) (vnets NodeSDNZoneVNets, err error) {
	err = z.client.Get(ctx, fmt.Sprintf("/nodes/%s/sdn/zones/%s/content", z.node.Name, z.Zone), &vnets)

	if nil == err {
		for _, v := range vnets {
			v.client = z.client
			v.zone = z
		}
	}
	return
}
