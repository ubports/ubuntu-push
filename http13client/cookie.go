// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// This implementation is done according to RFC 6265:
//
//    http://tools.ietf.org/html/rfc6265

// readSetCookies parses all "Set-Cookie" values from
// the header h and returns the successfully parsed Cookies.
func readSetCookies(h http.Header) []*http.Cookie {
	cookies := []*http.Cookie{}
	for _, line := range h["Set-Cookie"] {
		parts := strings.Split(strings.TrimSpace(line), ";")
		if len(parts) == 1 && parts[0] == "" {
			continue
		}
		parts[0] = strings.TrimSpace(parts[0])
		j := strings.Index(parts[0], "=")
		if j < 0 {
			continue
		}
		name, value := parts[0][:j], parts[0][j+1:]
		if !isCookieNameValid(name) {
			continue
		}
		value, success := parseCookieValue(value)
		if !success {
			continue
		}
		c := &http.Cookie{
			Name:  name,
			Value: value,
			Raw:   line,
		}
		for i := 1; i < len(parts); i++ {
			parts[i] = strings.TrimSpace(parts[i])
			if len(parts[i]) == 0 {
				continue
			}

			attr, val := parts[i], ""
			if j := strings.Index(attr, "="); j >= 0 {
				attr, val = attr[:j], attr[j+1:]
			}
			lowerAttr := strings.ToLower(attr)
			parseCookieValueFn := parseCookieValue
			if lowerAttr == "expires" {
				parseCookieValueFn = parseCookieExpiresValue
			}
			val, success = parseCookieValueFn(val)
			if !success {
				c.Unparsed = append(c.Unparsed, parts[i])
				continue
			}
			switch lowerAttr {
			case "secure":
				c.Secure = true
				continue
			case "httponly":
				c.HttpOnly = true
				continue
			case "domain":
				c.Domain = val
				continue
			case "max-age":
				secs, err := strconv.Atoi(val)
				if err != nil || secs != 0 && val[0] == '0' {
					break
				}
				if secs <= 0 {
					c.MaxAge = -1
				} else {
					c.MaxAge = secs
				}
				continue
			case "expires":
				c.RawExpires = val
				exptime, err := time.Parse(time.RFC1123, val)
				if err != nil {
					exptime, err = time.Parse("Mon, 02-Jan-2006 15:04:05 MST", val)
					if err != nil {
						c.Expires = time.Time{}
						break
					}
				}
				c.Expires = exptime.UTC()
				continue
			case "path":
				c.Path = val
				continue
			}
			c.Unparsed = append(c.Unparsed, parts[i])
		}
		cookies = append(cookies, c)
	}
	return cookies
}

// readCookies parses all "Cookie" values from the header h and
// returns the successfully parsed Cookies.
//
// if filter isn't empty, only cookies of that name are returned
func readCookies(h http.Header, filter string) []*http.Cookie {
	cookies := []*http.Cookie{}
	lines, ok := h["Cookie"]
	if !ok {
		return cookies
	}

	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), ";")
		if len(parts) == 1 && parts[0] == "" {
			continue
		}
		// Per-line attributes
		parsedPairs := 0
		for i := 0; i < len(parts); i++ {
			parts[i] = strings.TrimSpace(parts[i])
			if len(parts[i]) == 0 {
				continue
			}
			name, val := parts[i], ""
			if j := strings.Index(name, "="); j >= 0 {
				name, val = name[:j], name[j+1:]
			}
			if !isCookieNameValid(name) {
				continue
			}
			if filter != "" && filter != name {
				continue
			}
			val, success := parseCookieValue(val)
			if !success {
				continue
			}
			cookies = append(cookies, &http.Cookie{Name: name, Value: val})
			parsedPairs++
		}
	}
	return cookies
}

// validCookieDomain returns wheter v is a valid cookie domain-value.
func validCookieDomain(v string) bool {
	if isCookieDomainName(v) {
		return true
	}
	if net.ParseIP(v) != nil && !strings.Contains(v, ":") {
		return true
	}
	return false
}

// isCookieDomainName returns whether s is a valid domain name or a valid
// domain name with a leading dot '.'.  It is almost a direct copy of
// package net's isDomainName.
func isCookieDomainName(s string) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) > 255 {
		return false
	}

	if s[0] == '.' {
		// A cookie a domain attribute may start with a leading dot.
		s = s[1:]
	}
	last := byte('.')
	ok := false // Ok once we've seen a letter.
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			// No '_' allowed here (in contrast to package net).
			ok = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return ok
}

var cookieNameSanitizer = strings.NewReplacer("\n", "-", "\r", "-")

func sanitizeCookieName(n string) string {
	return cookieNameSanitizer.Replace(n)
}

// http://tools.ietf.org/html/rfc6265#section-4.1.1
// cookie-value      = *cookie-octet / ( DQUOTE *cookie-octet DQUOTE )
// cookie-octet      = %x21 / %x23-2B / %x2D-3A / %x3C-5B / %x5D-7E
//           ; US-ASCII characters excluding CTLs,
//           ; whitespace DQUOTE, comma, semicolon,
//           ; and backslash
func sanitizeCookieValue(v string) string {
	return sanitizeOrWarn("Cookie.Value", validCookieValueByte, v)
}

func validCookieValueByte(b byte) bool {
	return 0x20 < b && b < 0x7f && b != '"' && b != ',' && b != ';' && b != '\\'
}

// path-av           = "Path=" path-value
// path-value        = <any CHAR except CTLs or ";">
func sanitizeCookiePath(v string) string {
	return sanitizeOrWarn("Cookie.Path", validCookiePathByte, v)
}

func validCookiePathByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != ';'
}

func sanitizeOrWarn(fieldName string, valid func(byte) bool, v string) string {
	ok := true
	for i := 0; i < len(v); i++ {
		if valid(v[i]) {
			continue
		}
		log.Printf("net/http: invalid byte %q in %s; dropping invalid bytes", v[i], fieldName)
		ok = false
		break
	}
	if ok {
		return v
	}
	buf := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		if b := v[i]; valid(b) {
			buf = append(buf, b)
		}
	}
	return string(buf)
}

func unquoteCookieValue(v string) string {
	if len(v) > 1 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

func isCookieByte(c byte) bool {
	switch {
	case c == 0x21, 0x23 <= c && c <= 0x2b, 0x2d <= c && c <= 0x3a,
		0x3c <= c && c <= 0x5b, 0x5d <= c && c <= 0x7e:
		return true
	}
	return false
}

func isCookieExpiresByte(c byte) (ok bool) {
	return isCookieByte(c) || c == ',' || c == ' '
}

func parseCookieValue(raw string) (string, bool) {
	return parseCookieValueUsing(raw, isCookieByte)
}

func parseCookieExpiresValue(raw string) (string, bool) {
	return parseCookieValueUsing(raw, isCookieExpiresByte)
}

func parseCookieValueUsing(raw string, validByte func(byte) bool) (string, bool) {
	raw = unquoteCookieValue(raw)
	for i := 0; i < len(raw); i++ {
		if !validByte(raw[i]) {
			return "", false
		}
	}
	return raw, true
}

func isCookieNameValid(raw string) bool {
	return strings.IndexFunc(raw, isNotToken) < 0
}
