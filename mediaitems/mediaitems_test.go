package mediaitems

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"golang.org/x/exp/slog"
)

var _ http.RoundTripper = mockRoundTripper{}

type mockRoundTripper struct {
	roundTripperFn func(*http.Request) (*http.Response, error)
}

// RoundTrip implements http.RoundTripper.
func (mock mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return mock.roundTripperFn(req)
}

func TestList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	listReq := ListMediaItemsRequest{
		PageSize:                 0,
		PageToken:                "",
		ExcludeNonAppCreatedData: false,
	}

	client := &http.Client{
		Transport: mockRoundTripper{
			roundTripperFn: func(req *http.Request) (*http.Response, error) {
				albumsResponse := ListMediaItemsResponse{
					MediaItems: []MediaItem{
						{
							ID:              "test-a",
							Description:     "",
							ProductURL:      "",
							BaseURL:         "",
							MimeType:        "",
							MediaMetadata:   &MediaMetadata{},
							ContributorInfo: &ContributorInfo{},
							Filename:        "",
						},
					},
					NextPageToken: "1",
				}

				data, err := json.Marshal(&albumsResponse)
				if err != nil {
					// TODO: respond badly?
					return nil, err
				}
				buf := bytes.NewBuffer(data)
				r := bufio.NewReader(buf)

				return &http.Response{
					Status:           http.StatusText(http.StatusOK),
					StatusCode:       http.StatusOK,
					Proto:            "http/1.0",
					ProtoMajor:       1,
					ProtoMinor:       0,
					Header:           map[string][]string{},
					Body:             io.NopCloser(r),
					ContentLength:    int64(buf.Len()),
					TransferEncoding: []string{},
					Close:            false,
					Uncompressed:     false,
					Trailer:          map[string][]string{},
					Request:          req,
					TLS:              &tls.ConnectionState{},
				}, nil
			},
		},
	}

	mediaItemCh, errCh := List(ctx, client, listReq)
LOOP:
	select {
	case err := <-errCh:
		t.Log(err)
	case mediaItem, closed := <-mediaItemCh:
		t.Logf("mediaItem ID: %s\n", mediaItem.ID)
		if !closed {
			goto LOOP
		}
	}

	// success?
}

func TestSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	searchReq := SearchMediaItemRequest{
		PageSize:  0,
		PageToken: "",
	}
	searchResponses := map[string]SearchMediaItemResponse{
		"": SearchMediaItemResponse{
			MediaItems: []MediaItem{
				MediaItem{
					ID:              "test-a",
					Description:     "",
					ProductURL:      "",
					BaseURL:         "",
					MimeType:        "",
					MediaMetadata:   &MediaMetadata{},
					ContributorInfo: &ContributorInfo{},
					Filename:        "",
				},
			},
			NextPageToken: "a",
		},
		"a": SearchMediaItemResponse{
			MediaItems: []MediaItem{
				MediaItem{
					ID:              "test-b",
					Description:     "",
					ProductURL:      "",
					BaseURL:         "",
					MimeType:        "",
					MediaMetadata:   &MediaMetadata{},
					ContributorInfo: &ContributorInfo{},
					Filename:        "",
				},
			},
			NextPageToken: "",
		},
	}

	client := &http.Client{
		Transport: mockRoundTripper{
			roundTripperFn: func(req *http.Request) (*http.Response, error) {
				var rtSearchReq SearchMediaItemRequest
				if err := json.NewDecoder(req.Body).Decode(&rtSearchReq); err != nil {
					return nil, err
				}
				if err := req.Body.Close(); err != nil {
					return nil, err
				}

				searchResp, ok := searchResponses[rtSearchReq.PageToken]
				if !ok {
					return nil, fmt.Errorf("searchResponse not found for %s", rtSearchReq.PageToken)
				}

				data, err := json.Marshal(&searchResp)
				if err != nil {
					// TODO: respond badly?
					return nil, err
				}
				buf := bytes.NewBuffer(data)
				r := bufio.NewReader(buf)

				return &http.Response{
					Status:           http.StatusText(http.StatusOK),
					StatusCode:       http.StatusOK,
					Proto:            "http/1.0",
					ProtoMajor:       1,
					ProtoMinor:       0,
					Header:           map[string][]string{},
					Body:             io.NopCloser(r),
					ContentLength:    int64(buf.Len()),
					TransferEncoding: []string{},
					Close:            false,
					Uncompressed:     false,
					Trailer:          map[string][]string{},
					Request:          req,
					TLS:              &tls.ConnectionState{},
				}, nil
			},
		},
	}

	mediaItems := make([]MediaItem, 0)
	mediaItemCh, errCh := Search(ctx, client, searchReq)

	for mediaItem := range mediaItemCh {
		select {
		case err := <-errCh:
			t.Log(err)
		default:
			t.Logf("mediaItem ID: %s\n", mediaItem.ID)
			mediaItems = append(mediaItems, mediaItem)
		}
	}

	if len(mediaItems) != 2 {
		t.Errorf("incorrect number of MediaItems have %d want %d", len(mediaItems), 2)
	}
}
