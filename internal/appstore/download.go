package appstore

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ipatool "github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/schollz/progressbar/v3"
	"howett.net/plist"
)

// DownloadTicket is an App-Store-authorized download: the signed CDN URL to
// fetch, the per-file sinf licensing blobs, and the response metadata. The
// version listing and metadata lookups read the release details from it.
type DownloadTicket struct {
	URL       string
	Sinfs     []Sinf
	Metadata  map[string]any
	AssetInfo map[string]any
}

// Version returns CFBundleShortVersionString ("1.54.0") - the human-readable
// version of the specific release this ticket describes. Empty if absent.
func (t DownloadTicket) Version() string {
	return metaString(t.Metadata, "bundleShortVersionString")
}

// ExternalVersionID returns the stable App Store identifier for this
// specific release (e.g. "847134900"). Useful as a cache key since it can't
// collide across releases the way human version strings can.
func (t DownloadTicket) ExternalVersionID() string {
	return metaString(t.Metadata, "softwareVersionExternalIdentifier")
}

// BundleID returns softwareVersionBundleId from the metadata - handy when
// the caller doesn't already know it.
func (t DownloadTicket) BundleID() string {
	return metaString(t.Metadata, "softwareVersionBundleId")
}

// FileSize returns the IPA download size in bytes as reported by the
// response (asset-info.file-size). 0 if unknown.
func (t DownloadTicket) FileSize() int64 {
	if v, ok := t.AssetInfo["file-size"]; ok {
		switch n := v.(type) {
		case int64:
			return n
		case uint64:
			return int64(n)
		case int:
			return int64(n)
		}
	}

	return 0
}

func metaString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}

	return ""
}

// metaIntSlice reads a []any of numeric values from the metadata map and
// returns them as []int. plist decoding yields int64/uint64/float64, hence
// the type switch.
func metaIntSlice(m map[string]any, key string) []int {
	v, ok := m[key]
	if !ok {
		return nil
	}

	arr, ok := v.([]any)
	if !ok {
		return nil
	}

	out := make([]int, 0, len(arr))
	for _, e := range arr {
		switch n := e.(type) {
		case int:
			out = append(out, n)
		case int64:
			out = append(out, int(n))
		case uint64:
			out = append(out, int(n))
		case float64:
			out = append(out, int(n))
		}
	}

	return out
}

type downloadItem struct {
	URL       string         `plist:"URL,omitempty"`
	Sinfs     []Sinf         `plist:"sinfs,omitempty"`
	Metadata  map[string]any `plist:"metadata,omitempty"`
	AssetInfo map[string]any `plist:"asset-info,omitempty"`
}

type downloadResult struct {
	FailureType     string         `plist:"failureType,omitempty"`
	CustomerMessage string         `plist:"customerMessage,omitempty"`
	Items           []downloadItem `plist:"songList,omitempty"`
}

// Download fetches app into outPath, with its sinfs and iTunesMetadata.plist
// in place, and returns the version it fetched. It fetches the latest
// version, or the one given by externalVersionID. onProgress, if not nil, is
// called about every 100ms during the transfer.
//
// It uses ipatool's download, which since Apple's August 2026 changes tries
// ent/download with a generated kbsync first and then the volumeStore,
// redownload and update endpoints in turn. volumeStoreDownloadProduct alone
// now returns an empty songList for many apps.
//
// On ErrPasswordTokenExpired the caller must re-Login and retry.
// On ErrLicenseRequired the caller must Purchase and retry.
func (c *Client) Download(acc *Account, app App, externalVersionID, outPath string, onProgress func(cur, total int64)) (string, error) {
	if outPath == "" {
		return "", errors.New("download: outPath is required (must be a file path)")
	}

	var progress *progressbar.ProgressBar
	if onProgress != nil {
		progress = progressbar.NewOptions64(-1, progressbar.OptionSetWriter(io.Discard))
		done := make(chan struct{})
		defer close(done)

		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if total := progress.GetMax64(); total > 0 {
						onProgress(int64(progress.State().CurrentBytes), total)
					}
				}
			}
		}()
	}

	_, err := c.ipatool().Download(ipatool.DownloadInput{
		Account:           acc.ipatool(),
		App:               ipatool.App{ID: app.ID, BundleID: app.BundleID, Name: app.Name, Version: app.Version, Price: app.Price},
		OutputPath:        outPath,
		Progress:          progress,
		ExternalVersionID: externalVersionID,
	})

	switch {
	case errors.Is(err, ipatool.ErrPasswordTokenExpired):
		return "", ErrPasswordTokenExpired
	case errors.Is(err, ipatool.ErrLicenseRequired):
		return "", ErrLicenseRequired
	case err != nil:
		return "", fmt.Errorf("download: %w", err)
	}

	if onProgress != nil {
		if total := progress.GetMax64(); total > 0 {
			onProgress(total, total)
		}
	}

	version, err := ipaVersion(outPath)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	return version, nil
}

// volumeDownload is the shared POST to the volumeStoreDownloadProduct
// endpoint used for both authorized downloads and metadata-only lookups
// (list versions, get per-version metadata). The endpoint is the same;
// whether externalVersionID is set decides what is returned.
func (c *Client) volumeDownload(acc *Account, app App, externalVersionID string) (downloadItem, error) {
	g, err := guid()
	if err != nil {
		return downloadItem{}, err
	}

	podPrefix := ""
	if acc.Pod != "" {
		podPrefix = "p" + acc.Pod + "-"
	}

	url := fmt.Sprintf("https://%s%s%s?guid=%s", podPrefix, storeDomain, downloadPath, g)

	payload := map[string]any{
		"creditDisplay": "",
		"guid":          g,
		"salableAdamId": app.ID,
		// Without it Apple answers 5002 (majd/ipatool#500).
		"serialNumber": "0",
	}
	if externalVersionID != "" {
		payload["externalVersionId"] = externalVersionID
	}

	body, err := plistBody(payload)
	if err != nil {
		return downloadItem{}, err
	}

	headers := map[string]string{
		"Content-Type": "application/x-apple-plist",
		"iCloud-DSID":  acc.DirectoryServicesID,
		"X-Dsid":       acc.DirectoryServicesID,
	}

	var out downloadResult
	if _, err := c.send(http.MethodPost, url, headers, body, formatXML, &out); err != nil {
		return downloadItem{}, fmt.Errorf("download: %w", err)
	}

	switch {
	case out.FailureType == failurePasswordTokenExpired,
		out.FailureType == failureSignInRequired,
		out.FailureType == failureDeviceVerificationFailed,
		out.FailureType == failureLicenseAlreadyExists:
		return downloadItem{}, ErrPasswordTokenExpired
	case out.FailureType == failureLicenseNotFound:
		return downloadItem{}, ErrLicenseRequired
	case out.FailureType != "" && out.CustomerMessage != "":
		return downloadItem{}, errors.New(out.CustomerMessage)
	case out.FailureType != "":
		return downloadItem{}, fmt.Errorf("download: %s", out.FailureType)
	case len(out.Items) == 0:
		return downloadItem{}, errors.New("download: empty songList")
	}

	return out.Items[0], nil
}

// ipaVersion returns CFBundleShortVersionString of the app in the IPA at path.
func ipaVersion(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}

	defer zr.Close()

	for _, f := range zr.File {
		parts := strings.Split(f.Name, "/")
		if len(parts) != 3 || parts[0] != "Payload" || !strings.HasSuffix(parts[1], ".app") || parts[2] != "Info.plist" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open %s: %w", f.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("read %s: %w", f.Name, err)
		}

		var info struct {
			Version string `plist:"CFBundleShortVersionString"`
		}
		if _, err := plist.Unmarshal(data, &info); err != nil {
			return "", fmt.Errorf("parse %s: %w", f.Name, err)
		}

		if info.Version == "" {
			return "", fmt.Errorf("%s has no CFBundleShortVersionString", f.Name)
		}

		return info.Version, nil
	}

	return "", fmt.Errorf("no Payload/*.app/Info.plist in %s", path)
}
