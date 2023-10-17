package proxmox

import "fmt"

func (cl *Cluster) SdnZones() (zones []*SDNZone, err error) {
	err = cl.client.Get("/cluster/sdn/zones", &zones)

	if nil == err {
		for _, n := range zones {
			n.client = cl.client
		}
	}
	return
}

func (cl *Cluster) SdnVNets() (vnets SDNVNets, err error) {
	err = cl.client.Get("/cluster/sdn/vnets", &vnets)

	if nil == err {
		for _, n := range vnets {
			n.client = cl.client
		}
	}
	return
}

func (n *Node) SdnZones() (zones NodeSDNZones, err error) {
	err = n.client.Get(fmt.Sprintf("/nodes/%s/sdn/zones", n.Name), &zones)

	if nil == err {
		for _, z := range zones {
			z.client = n.client
			z.node = n
		}
	}
	return
}

func (z *NodeSDNZone) SdnZoneContent() (vnets NodeSDNZoneVNets, err error) {
	err = z.client.Get(fmt.Sprintf("/nodes/%s/sdn/zones/%s/content", z.node.Name, z.Zone), &vnets)

	if nil == err {
		for _, v := range vnets {
			v.client = z.client
			v.zone = z
		}
	}
	return
}
