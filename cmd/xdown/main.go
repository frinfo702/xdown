// xdown downloads videos (and optionally photos) from X/Twitter and Instagram posts.
//
//	xdown [-d dir] [-p] <url>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36"

var (
	xRe  = regexp.MustCompile(`(?:twitter|x)\.com/[^/]+/status(?:es)?/(\d+)`)
	igRe = regexp.MustCompile(`instagram\.com/(?:[^/]+/)?(?:p|reels?|tv)/([A-Za-z0-9_-]+)`)
)

type media struct {
	url   string
	photo bool
}

func main() {
	dir := flag.String("d", ".", "output directory")
	photos := flag.Bool("p", false, "also download photos")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: xdown [-d dir] [-p] <url>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	link := flag.Arg(0)
	flag.CommandLine.Parse(flag.Args()[1:]) // allow flags after the URL too

	if err := run(link, *dir, *photos); err != nil {
		fmt.Fprintln(os.Stderr, "xdown:", err)
		os.Exit(1)
	}
}

func run(link, dir string, photos bool) error {
	var (
		prefix string
		items  []media
		err    error
	)
	if m := xRe.FindStringSubmatch(link); m != nil {
		prefix = "x_" + m[1]
		items, err = fetchX(m[1])
	} else if m := igRe.FindStringSubmatch(link); m != nil {
		prefix = "ig_" + m[1]
		items, err = fetchIG(m[1])
	} else {
		return fmt.Errorf("unsupported URL: %s", link)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	n := 0
	for _, it := range items {
		if it.photo && !photos {
			continue
		}
		n++
		u, _ := url.Parse(it.url)
		name := filepath.Join(dir, fmt.Sprintf("%s_%d%s", prefix, n, path.Ext(u.Path)))
		if err := download(it.url, name); err != nil {
			return err
		}
		fmt.Println(name)
	}
	if n == 0 && len(items) > 0 {
		return fmt.Errorf("post has only photos (use -p to download them)")
	}
	if n == 0 {
		return fmt.Errorf("post has no photos or videos")
	}
	return nil
}

func getJSON(req *http.Request, v any) error {
	req.Header.Set("User-Agent", ua)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("%s: HTTP %d", req.URL.Host, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		return fmt.Errorf("%s: unexpected non-JSON response", req.URL.Host)
	}
	return nil
}

// --- X / Twitter ---

type xVariants struct {
	Variants []struct {
		Bitrate     int    `json:"bitrate"`
		ContentType string `json:"content_type"`
		URL         string `json:"url"`
	} `json:"variants"`
}

// best returns the highest-bitrate mp4 variant, or "" if there is none.
func (x xVariants) best() string {
	best, br := "", -1
	for _, v := range x.Variants {
		if v.ContentType == "video/mp4" && v.Bitrate > br {
			best, br = v.URL, v.Bitrate
		}
	}
	return best
}

func fetchX(id string) ([]media, error) {
	if out, err := fetchXSyndication(id); err == nil {
		return out, nil
	}
	// Syndication refuses sensitive/age-restricted posts (and breaks occasionally); fxtwitter serves them.
	out, err := fetchXFx(id)
	if err != nil {
		return nil, fmt.Errorf("x: post unavailable (deleted, protected, or age-restricted): %w", err)
	}
	return out, nil
}

// fetchXSyndication uses the public API embedded tweets use.
func fetchXSyndication(id string) ([]media, error) {
	var t struct {
		Typename     string `json:"__typename"`
		MediaDetails []struct {
			Type      string    `json:"type"`
			URL       string    `json:"media_url_https"`
			VideoInfo xVariants `json:"video_info"`
		} `json:"mediaDetails"`
	}
	req, _ := http.NewRequest("GET", "https://cdn.syndication.twimg.com/tweet-result?id="+id+"&token="+xToken(id), nil)
	if err := getJSON(req, &t); err != nil {
		return nil, err
	}
	if t.Typename == "TweetTombstone" {
		return nil, fmt.Errorf("syndication: tombstone")
	}
	var out []media
	for _, m := range t.MediaDetails {
		if m.Type == "photo" {
			out = append(out, media{m.URL + "?name=orig", true})
		} else if u := m.VideoInfo.best(); u != "" {
			out = append(out, media{u, false})
		}
	}
	return out, nil
}

// fetchXFx uses the public fxtwitter API (github.com/FxEmbed/FxEmbed).
func fetchXFx(id string) ([]media, error) {
	var r struct {
		Tweet *struct {
			Media struct {
				All []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
					xVariants
				} `json:"all"`
			} `json:"media"`
		} `json:"tweet"`
	}
	req, _ := http.NewRequest("GET", "https://api.fxtwitter.com/status/"+id, nil)
	if err := getJSON(req, &r); err != nil {
		return nil, err
	}
	if r.Tweet == nil {
		return nil, fmt.Errorf("fxtwitter: no tweet in response")
	}
	var out []media
	for _, m := range r.Tweet.Media.All {
		if m.Type == "photo" {
			out = append(out, media{m.URL, true})
		} else if u := m.best(); u != "" {
			out = append(out, media{u, false})
		}
	}
	return out, nil
}

// xToken mirrors ((id / 1e15) * Math.PI).toString(36).replace(/(0+|\.)/g, "").
func xToken(id string) string {
	n, _ := strconv.ParseFloat(id, 64)
	return strings.NewReplacer("0", "", ".", "").Replace(toBase36(n / 1e15 * math.Pi))
}

// toBase36 ports V8's DoubleToRadixCString for positive values with a small integer part.
func toBase36(v float64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	ip := math.Floor(v)
	frac := v - ip
	delta := math.Max(0.5*(math.Nextafter(v, math.Inf(1))-v), math.Nextafter(0, 1))
	var buf []byte
	if frac >= delta {
		for {
			frac *= 36
			delta *= 36
			d := int(frac)
			buf = append(buf, digits[d])
			frac -= float64(d)
			if (frac > 0.5 || (frac == 0.5 && d&1 == 1)) && frac+delta > 1 {
				for { // round up, carrying leftwards
					i := len(buf) - 1
					if i < 0 {
						ip++
						break
					}
					d := strings.IndexByte(digits, buf[i])
					buf = buf[:i]
					if d+1 < 36 {
						buf = append(buf, digits[d+1])
						break
					}
				}
				break
			}
			if frac < delta {
				break
			}
		}
	}
	s := strconv.FormatInt(int64(ip), 36)
	if len(buf) > 0 {
		s += "." + string(buf)
	}
	return s
}

// --- Instagram: anonymous GraphQL query the web app uses for logged-out post pages ---

type igNode struct {
	IsVideo    bool   `json:"is_video"`
	VideoURL   string `json:"video_url"`
	DisplayURL string `json:"display_url"`
	Children   struct {
		Edges []struct {
			Node igNode `json:"node"`
		} `json:"edges"`
	} `json:"edge_sidecar_to_children"`
}

func fetchIG(code string) ([]media, error) {
	form := url.Values{
		"variables":                {`{"shortcode":"` + code + `"}`},
		"doc_id":                   {"8845758582119845"},
		"fb_api_caller_class":      {"RelayModern"},
		"fb_api_req_friendly_name": {"PolarisPostActionLoadPostQueryQuery"},
		"server_timestamps":        {"true"},
	}
	req, _ := http.NewRequest("POST", "https://www.instagram.com/graphql/query", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	var r struct {
		Data struct {
			Media *igNode `json:"xdt_shortcode_media"`
		} `json:"data"`
	}
	if err := getJSON(req, &r); err != nil {
		return nil, err
	}
	if r.Data.Media == nil {
		return nil, fmt.Errorf("instagram: post not found (private, deleted, or rate-limited)")
	}
	nodes := []igNode{*r.Data.Media}
	if c := r.Data.Media.Children.Edges; len(c) > 0 {
		nodes = nodes[:0]
		for _, e := range c {
			nodes = append(nodes, e.Node)
		}
	}
	var out []media
	for _, n := range nodes {
		if n.IsVideo {
			out = append(out, media{n.VideoURL, false})
		} else {
			out = append(out, media{n.DisplayURL, true})
		}
	}
	return out, nil
}

func download(src, dst string) error {
	req, _ := http.NewRequest("GET", src, nil)
	req.Header.Set("User-Agent", ua)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("download %s: HTTP %d", src, res.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		os.Remove(dst) // don't leave a truncated file behind
		return err
	}
	return f.Close()
}
