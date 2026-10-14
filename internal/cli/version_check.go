package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lintsinghua/oops/internal/version"
)

func checkLatest(stdout, stderr io.Writer, url string) error {
	if url == "" {
		url = os.Getenv("OOPS_VERSION_URL")
	}
	if url == "" {
		url = "https://api.github.com/repos/lintsinghua/oops/releases/latest"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fail(1, "could not check for updates: "+err.Error())
	}
	req.Header.Set("User-Agent", "oops")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail(1, "could not check for updates: "+err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintln(stdout, "no release published")
		return nil
	}
	if resp.StatusCode >= 300 {
		return fail(1, "could not check for updates: "+resp.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil || rel.TagName == "" {
		return fail(1, "could not check for updates: unexpected response")
	}
	installed := version.Current().Version
	if sameVersion(installed, rel.TagName) {
		fmt.Fprintf(stdout, "oops is up to date (%s)\n", installed)
		return nil
	}
	fmt.Fprintf(stdout, "oops update available: %s (installed %s)\n", rel.TagName, installed)
	_ = stderr
	return nil
}

func sameVersion(installed, tag string) bool {
	return strings.TrimPrefix(installed, "v") == strings.TrimPrefix(tag, "v")
}
