package auth

import (
	"net"
	"net/http"
	"strings"
)

type IPExtractorMode string

const (
	IPModeDirect        IPExtractorMode = "direct"
	IPModeXRealIP       IPExtractorMode = "x-real-ip"
	IPModeXForwardedFor IPExtractorMode = "x-forwarded-for"
)

type ClientIPExtractor struct {
	mode    IPExtractorMode
	trusted []*net.IPNet
}

func NewClientIPExtractor(mode string, trustedCIDRs []string) ClientIPExtractor {
	extractor := ClientIPExtractor{mode: IPModeDirect}
	switch IPExtractorMode(strings.TrimSpace(strings.ToLower(mode))) {
	case IPModeXRealIP:
		extractor.mode = IPModeXRealIP
	case IPModeXForwardedFor:
		extractor.mode = IPModeXForwardedFor
	}

	for _, cidr := range trustedCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err == nil {
			extractor.trusted = append(extractor.trusted, network)
		}
	}
	if extractor.mode != IPModeDirect && len(extractor.trusted) == 0 {
		extractor.mode = IPModeDirect
	}
	return extractor
}

func (e ClientIPExtractor) ClientIP(r *http.Request) string {
	direct := directIP(r.RemoteAddr)
	if e.mode == IPModeDirect || !e.trusts(direct) {
		return fallbackIP(direct)
	}

	switch e.mode {
	case IPModeXRealIP:
		if ip := parseIP(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	case IPModeXForwardedFor:
		return e.forwardedForIP(r.Header.Get("X-Forwarded-For"), direct)
	}
	return fallbackIP(direct)
}

func (e ClientIPExtractor) forwardedForIP(header string, direct string) string {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := parseIP(parts[i])
		if ip == "" {
			continue
		}
		if !e.trusts(ip) {
			return ip
		}
	}
	return fallbackIP(direct)
}

func (e ClientIPExtractor) trusts(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range e.trusted {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

func directIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		return parseIP(host)
	}
	return parseIP(remoteAddr)
}

func parseIP(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func fallbackIP(ip string) string {
	if ip == "" {
		return "unknown"
	}
	return ip
}
