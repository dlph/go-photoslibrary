package mediaitems

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dlph/go-photoslibrary/api"

	"golang.org/x/exp/slog"
)

const (
	ListMediaItemsPath   = "mediaItems"
	GetmediaItemPath     = "mediaItems"
	SearchMediaItemsPath = "mediaItems:search"
)

const (
	UnspecifiedVideoProcessingStatus VideoProcessingStatus = "UNSPECIFIED"
	ProcessingVideoProcessingStatus  VideoProcessingStatus = "PROCESSING"
	ReadyVideoProcessingStatus       VideoProcessingStatus = "READY"
	FailedVideoProcessingStatus      VideoProcessingStatus = "FAILED"
)

type MediaItem struct {
	ID              string           `json:"id,omitempty"`
	Description     string           `json:"description,omitempty"`
	ProductURL      string           `json:"productUrl,omitempty"`
	BaseURL         string           `json:"baseUrl,omitempty"`
	MimeType        string           `json:"mimeType,omitempty"`
	MediaMetadata   *MediaMetadata   `json:"mediaMetadata,omitempty"`
	ContributorInfo *ContributorInfo `json:"contributorInfo,omitempty"`
	Filename        string           `json:"filename,omitempty"`
}

type MediaMetadata struct {
	CreationTime time.Time `json:"creationTime,omitempty"`
	Width        int64     `json:"width,omitempty"`
	Height       int64     `json:"height,omitempty"`
	Photo        *Photo    `json:"photo,omitempty"`
	Video        *Video    `json:"video,omitempty"`
}
type Photo struct {
	CameraMake       string  `json:"cameraMake"`
	CameraModel      string  `json:"cameraModel"`
	FocalLength      float64 `json:"focalLength"`
	AperatureFNumber float64 `json:"apertureFNumber"`
	ISOEquivalent    int64   `json:"isoEquivalent"`
	ExposureTime     string  `json:"exposureTime"`
}

type Video struct {
	CameraMake  string                `json:"cameraMake"`
	CameraModel string                `json:"cameraModel"`
	FPS         float64               `json:"fps"`
	Status      VideoProcessingStatus `json:"status"`
}

type VideoProcessingStatus string

type ContributorInfo struct {
	ProfilePictureBaseUrl string `json:"profilePictureBaseUrl"`
	DisplayName           string `json:"displayName"`
}

type ListMediaItemsRequest struct {
	PageSize                 int    `json:"pageSize,omitempty"`
	PageToken                string `json:"pageToken,omitempty"`
	ExcludeNonAppCreatedData bool   `json:"excludeNonAppCreatedData,omitempty"`
}

type ListMediaItemsResponse struct {
	MediaItems    []MediaItem `json:"mediaItems"`
	NextPageToken string      `json:"nextPageToken"`
}

// List https://developers.google.com/photos/library/reference/rest/v1/mediaItems/list
func List(ctx context.Context, client *http.Client, listRequest ListMediaItemsRequest) (<-chan MediaItem, <-chan error) {
	respCh := make(chan MediaItem)
	errCh := make(chan error)

	go func(req ListMediaItemsRequest) {
		defer close(respCh)
		for {
			// handle context cancel
			select {
			case <-ctx.Done():
				slog.DebugContext(ctx, "list context done")
				return
			default:
			}

			resp, err := list(ctx, client, req)
			if err != nil {
				errCh <- err
				return // TODO continue?
			}

			for _, album := range resp.MediaItems {
				select {
				case <-ctx.Done():
					slog.DebugContext(ctx, "AlbumsResponse list context done")
					return
				case respCh <- album:
				}
			}

			if resp.NextPageToken == "" {
				return // exit
			}

			req.PageToken = resp.NextPageToken
		}
	}(listRequest)

	return respCh, errCh
}

// List https://developers.google.com/photos/library/guides/list
func list(ctx context.Context, client *http.Client, listAlbumsRequest ListMediaItemsRequest) (ListMediaItemsResponse, error) {
	var listResponse ListMediaItemsResponse

	urlPath, err := url.JoinPath(api.PhotosLibraryVersion, ListMediaItemsPath)
	if err != nil {
		return listResponse, err
	}

	urlValues := make(url.Values)
	// could use something like [go-querystring](https://github.com/google/go-querystring) but it uses refection
	logAttrs := make([]slog.Attr, 0, 5)
	if listAlbumsRequest.PageSize > 0 {
		urlValues.Add(api.PageSizeQueryKey, strconv.Itoa(listAlbumsRequest.PageSize))
		logAttrs = append(logAttrs, slog.Int(api.PageSizeQueryKey, listAlbumsRequest.PageSize))
	}
	if listAlbumsRequest.PageToken != "" {
		urlValues.Add(api.PageTokenQueryKey, listAlbumsRequest.PageToken)
		logAttrs = append(logAttrs, slog.String(api.PageTokenQueryKey, listAlbumsRequest.PageToken))
	}
	urlValues.Add(api.ExcludeNonAppCreatedDataQueryKey, strconv.FormatBool(listAlbumsRequest.ExcludeNonAppCreatedData))
	logAttrs = append(logAttrs, slog.Bool(api.ExcludeNonAppCreatedDataQueryKey, listAlbumsRequest.ExcludeNonAppCreatedData))

	rawURL := url.URL{
		Scheme:   api.PhotosLibraryScheme,
		Host:     api.PhotosLibraryHost,
		Path:     urlPath,
		RawQuery: urlValues.Encode(),
	}

	logAttrs = append(logAttrs, slog.String("url", rawURL.String()))
	slog.DebugContext(ctx, "listing albums for request", "url_values", logAttrs)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL.String(), nil)
	if err != nil {
		return listResponse, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return listResponse, err
	}

	slog.DebugContext(ctx, "received response from client", "response_headers", resp.Header, "request_headers", resp.Request.Header)

	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return listResponse, err
	}

	if err := resp.Body.Close(); err != nil {
		return listResponse, err
	}

	slog.DebugContext(ctx, "decoded json response body", "mediaItems", len(listResponse.MediaItems), "nextPageToken", listResponse.NextPageToken)

	return listResponse, nil
}

type GetMediaItemRequest struct {
	MediaItemID string `json:"mediaItemId"`
}

type GetMediaItemResponse struct {
	MediaItem MediaItem
}

// Get https://developers.google.com/photos/library/reference/rest/v1/albums/get
func Get(ctx context.Context, client *http.Client, getRequest GetMediaItemRequest) (MediaItem, error) {
	var getResponse GetMediaItemResponse

	urlPath, err := url.JoinPath(api.PhotosLibraryVersion, ListMediaItemsPath, getRequest.MediaItemID)
	if err != nil {
		return getResponse.MediaItem, err
	}

	rawURL := url.URL{
		Scheme: api.PhotosLibraryScheme,
		Host:   api.PhotosLibraryHost,
		Path:   urlPath,
	}

	req, err := http.NewRequest(http.MethodGet, rawURL.String(), nil)
	if err != nil {
		return getResponse.MediaItem, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return getResponse.MediaItem, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&getResponse); err != nil {
		return getResponse.MediaItem, err
	}

	return getResponse.MediaItem, resp.Body.Close()
}

type SearchMediaItemRequest struct {
	AlbumID   string  `json:"albumId,omitempty"`
	PageSize  int64   `json:"pageSize,omitempty"`
	PageToken string  `json:"pageToken,omitempty`
	Filters   Filters `json:"filters,omitempty"`
	OrderBy   string  `json:"orderBy,omitempty"`
}
type Filters struct {
	// TODO https://developers.google.com/photos/library/guides/apply-filters
}

type SearchMediaItemResponse struct {
	MediaItems    []MediaItem `json:"mediaItems"`
	NextPageToken string      `json:"nextPageToken"`
}

func Search(ctx context.Context, client *http.Client, searchRequest SearchMediaItemRequest) (<-chan MediaItem, <-chan error) {
	mediaItemCh := make(chan MediaItem)
	errCh := make(chan error)

	go func(req SearchMediaItemRequest) {
		defer close(mediaItemCh)
		for {
			// handle context cancel
			select {
			case <-ctx.Done():
				slog.DebugContext(ctx, "search context done")
				return
			default:
			}

			resp, err := search(ctx, client, req)
			if err != nil {
				errCh <- err
				return // TODO continue?
			}

			for _, mediaItem := range resp.MediaItems {
				select {
				case <-ctx.Done():
					slog.DebugContext(ctx, "search context done")
					return
				case mediaItemCh <- mediaItem:
				}
			}

			if resp.NextPageToken == "" {
				return // exit
			}

			req.PageToken = resp.NextPageToken
		}
	}(searchRequest)

	return mediaItemCh, errCh
}

func search(ctx context.Context, client *http.Client, searchRequest SearchMediaItemRequest) (SearchMediaItemResponse, error) {
	var searchResponse SearchMediaItemResponse

	urlPath, err := url.JoinPath(api.PhotosLibraryVersion, SearchMediaItemsPath)
	if err != nil {
		return searchResponse, err
	}

	rawURL := url.URL{
		Scheme: api.PhotosLibraryScheme,
		Host:   api.PhotosLibraryHost,
		Path:   urlPath,
	}

	b := bytes.NewBuffer(nil)
	if err := json.NewEncoder(b).Encode(&searchRequest); err != nil {
		return searchResponse, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL.String(), bufio.NewReader(b))
	if err != nil {
		return searchResponse, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return searchResponse, err
	}

	slog.DebugContext(ctx, "received response from client", "response_headers", resp.Header, "request_headers", resp.Request.Header)

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return searchResponse, err
	}

	if err := resp.Body.Close(); err != nil {
		return searchResponse, err
	}

	slog.DebugContext(ctx, "decoded json response body", "mediaItems", len(searchResponse.MediaItems), "nextPageToken", searchResponse.NextPageToken)

	return searchResponse, nil
}
