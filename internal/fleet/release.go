package fleet

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/router-for-me/CLIProxyAPI/releases/latest"
const releasePrefix = "https://github.com/router-for-me/CLIProxyAPI/releases/download/"

// FetchStableRelease verifies the official archive and derives its binary hash without executing it.
func FetchStableRelease(ctx context.Context) (Release, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	read := func(url string, limit int64) ([]byte, error) {
		request, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if errRequest != nil {
			return nil, errRequest
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		response, errResponse := client.Do(request)
		if errResponse != nil {
			return nil, errResponse
		}
		defer func() {
			if errClose := response.Body.Close(); errClose != nil {
				log.WithError(errClose).Debug("fleet release response close failed")
			}
		}()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("release HTTP %d", response.StatusCode)
		}
		data, errRead := io.ReadAll(io.LimitReader(response.Body, limit+1))
		if errRead != nil {
			return nil, errRead
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("release response too large")
		}
		return data, nil
	}
	raw, errRead := read(releaseAPI, 4<<20)
	if errRead != nil {
		return Release{}, errRead
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
		Assets     []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if errDecode := json.Unmarshal(raw, &release); errDecode != nil {
		return Release{}, errDecode
	}
	version := strings.TrimPrefix(release.Tag, "v")
	if release.Draft || release.Prerelease || !versionPattern.MatchString(version) {
		return Release{}, fmt.Errorf("release is not stable")
	}
	name := "CLIProxyAPI_" + version + "_darwin_aarch64.tar.gz"
	archiveURL, checksURL, assetDigest := "", "", ""
	for _, asset := range release.Assets {
		if asset.Name == name {
			archiveURL = asset.URL
			assetDigest = asset.Digest
		}
		if asset.Name == "checksums.txt" {
			checksURL = asset.URL
		}
	}
	expectedPrefix := releasePrefix + release.Tag + "/"
	if !strings.HasPrefix(archiveURL, expectedPrefix) || !strings.HasPrefix(checksURL, expectedPrefix) {
		return Release{}, fmt.Errorf("unexpected release asset origin")
	}
	checks, errChecks := read(checksURL, 1<<20)
	if errChecks != nil {
		return Release{}, errChecks
	}
	expected := ""
	for _, line := range strings.Split(string(checks), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == name {
			expected = parts[0]
		}
	}
	if !hashPattern.MatchString(expected) {
		return Release{}, fmt.Errorf("archive checksum missing")
	}
	archive, errArchive := read(archiveURL, 256<<20)
	if errArchive != nil {
		return Release{}, errArchive
	}
	sum := sha256.Sum256(archive)
	actual := hex.EncodeToString(sum[:])
	if actual != expected || (assetDigest != "" && assetDigest != "sha256:"+actual) {
		return Release{}, fmt.Errorf("archive checksum mismatch")
	}
	zipped, errZip := gzip.NewReader(bytes.NewReader(archive))
	if errZip != nil {
		return Release{}, errZip
	}
	defer func() {
		if errClose := zipped.Close(); errClose != nil {
			log.WithError(errClose).Debug("fleet release archive close failed")
		}
	}()
	reader := tar.NewReader(zipped)
	for {
		header, errNext := reader.Next()
		if errNext == io.EOF {
			break
		}
		if errNext != nil {
			return Release{}, errNext
		}
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != "cli-proxy-api" {
			continue
		}
		if header.Size <= 0 || header.Size > 256<<20 {
			return Release{}, fmt.Errorf("invalid binary size")
		}
		hash := sha256.New()
		size, errCopy := io.Copy(hash, io.LimitReader(reader, header.Size+1))
		if errCopy != nil {
			return Release{}, errCopy
		}
		if size != header.Size {
			return Release{}, fmt.Errorf("binary size mismatch")
		}
		return Release{Version: version, URL: archiveURL, ArchiveSHA256: actual, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: size}, nil
	}
	return Release{}, fmt.Errorf("binary missing from release archive")
}
