package albums

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/dlph/go-photoslibrary/api"

	"golang.org/x/exp/slog"
)

const (
	ListAlbumsPath  = "albums"
	GetAlbumPath    = "albums"
	CreateAlbumPath = "albums"
)

type Album struct {
	ID                    string     `json:"id,omitempty"`
	Title                 string     `json:"title,omitempty"`
	ProductURL            string     `json:"productUrl,omitempty"`
	IsWriteable           bool       `json:"isWriteable,omitempty"`
	ShareInfo             *ShareInfo `json:"shareInfo,omitempty"`
	MediaItemsCount       string     `json:"mediaItemsCount,omitempty"`
	CoverPhotoBaseURL     string     `json:"coverPhotoBaseUrl,omitempty"`
	CoverPhotoMediaItemID string     `json:"coverPhotoMediaItemId,omitempty"`
}

type ShareInfo struct {
	SharedAlbumOptions SharedAlbumOptions `json:"sharedAlbumOptions"`
	ShareableURL       string             `json:"shareableUrl"`
	ShareToken         string             `json:"shareToken"`
	IsJoined           bool               `json:"isJoined"`
	IsOwned            bool               `json:"isOwned"`
	IsJoinable         bool               `json:"isJoinable"`
}

type SharedAlbumOptions struct {
	IsCollaborative bool `json:"isCollaborative"`
	IsCommentable   bool `json:"isCommentable"`
}

type ListAlbumsRequest struct {
	PageSize                 int    `json:"pageSize,omitempty"`
	PageToken                string `json:"pageToken,omitempty"`
	ExcludeNonAppCreatedData bool   `json:"excludeNonAppCreatedData,omitempty"`
}

type ListAlbumsResponse struct {
	Albums        []Album `json:"albums"`
	NextPageToken string  `json:"nextPageToken"`
}

// List https://developers.google.com/photos/library/reference/rest/v1/albums/list
func List(ctx context.Context, client *http.Client, listAlbumsRequest ListAlbumsRequest) (<-chan Album, <-chan error) {
	albumCh := make(chan Album)
	errCh := make(chan error)

	go func(req ListAlbumsRequest) {
		defer close(albumCh)
		for {
			// handle context cancel
			select {
			case <-ctx.Done():
				slog.DebugContext(ctx, "list context done")
				return
			default:
			}

			albumsResp, err := list(ctx, client, req)
			if err != nil {
				errCh <- err
				return // TODO continue?
			}

			for _, album := range albumsResp.Albums {
				select {
				case <-ctx.Done():
					slog.DebugContext(ctx, "AlbumsResponse list context done")
					return
				case albumCh <- album:
				}
			}

			if albumsResp.NextPageToken == "" {
				return // exit
			}

			req.PageToken = albumsResp.NextPageToken
		}
	}(listAlbumsRequest)

	return albumCh, errCh
}

// list https://developers.google.com/photos/library/guides/list
func list(ctx context.Context, client *http.Client, listAlbumsRequest ListAlbumsRequest) (ListAlbumsResponse, error) {
	var listAlbumsResponse ListAlbumsResponse

	urlPath, err := url.JoinPath(api.PhotosLibraryVersion, ListAlbumsPath)
	if err != nil {
		return listAlbumsResponse, err
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
		return listAlbumsResponse, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return listAlbumsResponse, err
	}

	slog.DebugContext(ctx, "received response from client", "response_headers", resp.Header, "request_headers", resp.Request.Header)

	if err := json.NewDecoder(resp.Body).Decode(&listAlbumsResponse); err != nil {
		return listAlbumsResponse, err
	}

	if err := resp.Body.Close(); err != nil {
		return listAlbumsResponse, err
	}

	slog.DebugContext(ctx, "decoded json response body", "albums", len(listAlbumsResponse.Albums), "nextPageToken", listAlbumsResponse.NextPageToken)

	return listAlbumsResponse, nil
}

type GetAlbumRequest struct {
	AlbumID string
}

type GetAlbumResponse struct {
	Album Album `json:"album"`
}

// Get https://developers.google.com/photos/library/reference/rest/v1/albums/get
func Get(ctx context.Context, client *http.Client, getAlbumRequest GetAlbumRequest) (Album, error) {
	var getAlbumResponse GetAlbumResponse

	urlPath, err := url.JoinPath(api.PhotosLibraryVersion, GetAlbumPath, getAlbumRequest.AlbumID)
	if err != nil {
		return getAlbumResponse.Album, err
	}

	rawURL := url.URL{
		Scheme: api.PhotosLibraryScheme,
		Host:   api.PhotosLibraryHost,
		Path:   urlPath,
	}

	req, err := http.NewRequest(http.MethodGet, rawURL.String(), nil)
	if err != nil {
		return getAlbumResponse.Album, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return getAlbumResponse.Album, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&getAlbumResponse); err != nil {
		return getAlbumResponse.Album, err
	}

	return getAlbumResponse.Album, resp.Body.Close()
}

type CreateAlbumRequest struct {
	Album Album `json:"album"`
}

type CreateAlbumResponse struct {
	Album Album `json:"album"`
}

// Create https://developers.google.com/photos/library/reference/rest/v1/albums/create
func Create(ctx context.Context, client *http.Client, createAlbumRequest CreateAlbumRequest) (Album, error) {
	var createAlbumResponse CreateAlbumResponse

	urlPath, err := url.JoinPath(api.PhotosLibraryVersion, CreateAlbumPath)
	if err != nil {
		return createAlbumResponse.Album, err
	}

	rawURL := url.URL{
		Scheme: api.PhotosLibraryScheme,
		Host:   api.PhotosLibraryHost,
		Path:   urlPath,
	}

	buf := bytes.NewBuffer(nil)
	w := bufio.NewWriter(buf)
	json.NewEncoder(w).Encode(&createAlbumRequest)

	req, err := http.NewRequest(http.MethodPost, rawURL.String(), buf)
	if err != nil {
		return createAlbumResponse.Album, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return createAlbumResponse.Album, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&createAlbumResponse); err != nil {
		return createAlbumResponse.Album, err
	}

	return createAlbumResponse.Album, resp.Body.Close()
}
