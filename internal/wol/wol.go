package wol

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
)

// MagicPacket is a 102-byte Wake-on-LAN payload:
// 6 × 0xFF followed by 16 repetitions of the 6-byte target MAC.
type MagicPacket [102]byte

// NewMagicPacket builds a magic packet for the given MAC address string.
// mac may be colon-separated ("AA:BB:CC:DD:EE:FF") or hyphen-separated.
func NewMagicPacket(mac string) (MagicPacket, error) {
	mac = strings.ReplaceAll(mac, "-", ":")
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return MagicPacket{}, fmt.Errorf("invalid MAC address %q: expected 6 hex octets", mac)
	}

	var hw [6]byte
	for i, p := range parts {
		b, err := hex.DecodeString(p)
		if err != nil || len(b) != 1 {
			return MagicPacket{}, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		hw[i] = b[0]
	}

	var pkt MagicPacket
	// Header: 6 bytes of 0xFF.
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	// Payload: 16 repetitions of the MAC address.
	for i := 0; i < 16; i++ {
		copy(pkt[6+i*6:], hw[:])
	}
	return pkt, nil
}

// SendOptions configures how the magic packet is sent.
type SendOptions struct {
	// Interface is the network interface name to bind the socket to (e.g. "br-lan").
	// An empty string sends via the default route.
	Interface string
	// BroadcastIP is the UDP destination address (default "255.255.255.255").
	BroadcastIP string
	// Port is the UDP destination port (default 9).
	Port int
}

// Send transmits a WoL magic packet for each MAC address in macs.
// It binds to opts.Interface if specified so the packet is sent out the correct LAN port.
func Send(macs []string, opts SendOptions) error {
	if len(macs) == 0 {
		return errors.New("no MAC addresses configured")
	}
	if opts.BroadcastIP == "" {
		opts.BroadcastIP = "255.255.255.255"
	}
	if opts.Port == 0 {
		opts.Port = 9
	}

	var errs []string
	for _, mac := range macs {
		if err := sendOne(mac, opts); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", mac, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func sendOne(mac string, opts SendOptions) error {
	pkt, err := NewMagicPacket(mac)
	if err != nil {
		return err
	}

	targetAddr, err := net.ResolveUDPAddr("udp4",
		fmt.Sprintf("%s:%d", opts.BroadcastIP, opts.Port))
	if err != nil {
		return fmt.Errorf("resolve broadcast addr: %w", err)
	}

	// Use ListenUDP + WriteTo instead of DialUDP so that the OS sets
	// SO_BROADCAST automatically (required for 255.255.255.255 on Linux).
	localAddr := &net.UDPAddr{}
	if opts.Interface != "" {
		if ip, err := interfaceIP(opts.Interface); err == nil {
			localAddr.IP = ip
		}
		// Ignore interface lookup errors: fall back to OS routing.
	}

	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}
	defer conn.Close()

	if _, err := conn.WriteTo(pkt[:], targetAddr); err != nil {
		return fmt.Errorf("send packet: %w", err)
	}
	return nil
}

// interfaceIP returns the first IPv4 address assigned to iface.
func interfaceIP(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if v4 := ip.To4(); v4 != nil {
			return v4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address on interface %q", name)
}
