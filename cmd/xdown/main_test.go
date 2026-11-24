package main

import (
	"encoding/json"
	"testing"
)

func TestBestVariant(t *testing.T) {
	// fxtwitter shape: variants sit next to type/url, so xVariants must decode as an embedded field.
	var m struct {
		Type string `json:"type"`
		xVariants
	}
	js := `{"type":"video","variants":[
		{"url":"m3u8","bitrate":0,"content_type":"application/x-mpegURL"},
		{"url":"hi","bitrate":2176000,"content_type":"video/mp4"},
		{"url":"lo","bitrate":632000,"content_type":"video/mp4"}]}`
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatal(err)
	}
	if got := m.best(); got != "hi" {
		t.Errorf("best() = %q, want hi", got)
	}
}

func TestParse(t *testing.T) {
	// expected tokens computed with node: ((id/1e15)*Math.PI).toString(36).replace(/(0+|\.)/g,"")
	for id, want := range map[string]string{
		"1234567890123456789": "2zqic77uqyk",
		"20":                  "6dq1a2xwd93",
		"1858000000000000000": "4i52uku86y2",
	} {
		if got := xToken(id); got != want {
			t.Errorf("xToken(%s) = %s, want %s", id, got, want)
		}
	}
	for u, want := range map[string]string{
		"https://x.com/foo/status/123?s=20":           "123",
		"https://twitter.com/foo/status/456/video/1":  "456",
		"https://mobile.twitter.com/foo/statuses/789": "789",
	} {
		if m := xRe.FindStringSubmatch(u); m == nil || m[1] != want {
			t.Errorf("xRe(%s) = %v", u, m)
		}
	}
	for u, want := range map[string]string{
		"https://www.instagram.com/p/DAx0X2ESsbk/":          "DAx0X2ESsbk",
		"https://www.instagram.com/reel/C_abc-12/?igsh=xyz": "C_abc-12",
		"https://instagram.com/someuser/p/Xyz_1/":           "Xyz_1",
	} {
		if m := igRe.FindStringSubmatch(u); m == nil || m[1] != want {
			t.Errorf("igRe(%s) = %v", u, m)
		}
	}
}
