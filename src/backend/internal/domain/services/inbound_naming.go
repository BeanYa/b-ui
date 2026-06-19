package service

import (
	"strings"
	"unicode"
)

// NamingRule toggles which segments appear in the inbound name.
type NamingRule struct {
	IncludeProtocol bool
	IncludeSecurity bool
	IncludeFlag     bool
}

// ProtocolLabel maps an inbound type to its title-case label. hysteria2 -> "Hy2".
func ProtocolLabel(inboundType string) string {
	t := strings.ToLower(strings.TrimSpace(inboundType))
	switch t {
	case "":
		return ""
	case "hysteria2":
		return "Hy2"
	default:
		r := []rune(t)
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	}
}

// SecurityLabel maps tls template + reality flag to a label. Empty when none.
func SecurityLabel(tlsTemplate string, realityEnabled bool) string {
	switch strings.ToLower(strings.TrimSpace(tlsTemplate)) {
	case "reality":
		return "Reality"
	case "hysteria2":
		return "Hy2"
	case "standard", "standard-cert":
		return "TLS"
	default:
		if realityEnabled {
			return "Reality"
		}
		return ""
	}
}

// CountryFlagEmoji converts a 2-letter ISO code to a regional-indicator emoji.
func CountryFlagEmoji(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return ""
	}
	var b strings.Builder
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return ""
		}
		b.WriteRune(0x1F1E6 + (r - 'A'))
	}
	return b.String()
}

// slugAllowed reports whether r may appear in a slug segment.
func slugAllowed(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '_' || r == '.' || r == '-':
		return true
	}
	return false
}

// SanitizeSlugSegment lowercases and sanitizes a segment for the machine-safe slug.
// Any run of disallowed characters collapses to a single '-'.
func SanitizeSlugSegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prev := false
	for _, r := range s {
		if slugAllowed(r) {
			b.WriteRune(r)
			prev = false
			continue
		}
		if !prev {
			b.WriteByte('-')
			prev = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// showSecurity decides whether the security segment is emitted (Hy2 dedup).
func showSecurity(protoLabel, secLabel string) bool {
	if secLabel == "" {
		return false
	}
	if protoLabel == "Hy2" && secLabel == "Hy2" {
		return false
	}
	return true
}

// BuildInboundRemark builds the pretty visible name (emoji flag + space-joined labels).
func BuildInboundRemark(rule NamingRule, protoLabel, secLabel, countryCode, displayName string) string {
	flag := CountryFlagEmoji(countryCode)
	segs := make([]string, 0, 4)
	if rule.IncludeFlag && flag != "" {
		segs = append(segs, flag)
	}
	if rule.IncludeProtocol && protoLabel != "" {
		segs = append(segs, protoLabel)
	}
	if rule.IncludeSecurity && showSecurity(protoLabel, secLabel) {
		segs = append(segs, secLabel)
	}
	segs = append(segs, displayName)
	return strings.Join(segs, " ")
}

// BuildInboundSlug builds the machine-safe tag (lowercased, sanitized, dash-joined).
func BuildInboundSlug(rule NamingRule, protoLabel, secLabel, countryCode, displayName string) string {
	segs := make([]string, 0, 4)
	if rule.IncludeFlag && strings.TrimSpace(countryCode) != "" {
		segs = append(segs, SanitizeSlugSegment(countryCode))
	}
	if rule.IncludeProtocol && protoLabel != "" {
		segs = append(segs, SanitizeSlugSegment(protoLabel))
	}
	if rule.IncludeSecurity && showSecurity(protoLabel, secLabel) {
		segs = append(segs, SanitizeSlugSegment(secLabel))
	}
	segs = append(segs, SanitizeSlugSegment(displayName))
	out := strings.Join(segs, "-")
	out = strings.Trim(out, "-")
	if out == "" {
		out = "inbound"
	}
	return out
}
