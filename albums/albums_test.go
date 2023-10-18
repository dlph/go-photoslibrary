package albums

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"testing"
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

	albumsReq := ListAlbumsRequest{
		PageSize:                 0,
		PageToken:                "",
		ExcludeNonAppCreatedData: false,
	}

	client := &http.Client{
		Transport: mockRoundTripper{
			roundTripperFn: func(req *http.Request) (*http.Response, error) {
				albumsResponse := ListAlbumsResponse{
					Albums: []Album{
						{
							ID:                    "test-a",
							Title:                 "test-Aaaa",
							ProductURL:            "123",
							IsWriteable:           false,
							ShareInfo:             nil,
							MediaItemsCount:       "1",
							CoverPhotoBaseURL:     "localhost",
							CoverPhotoMediaItemID: "123",
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

	albumCh, errCh := List(ctx, client, albumsReq)
LOOP:
	select {
	case err := <-errCh:
		t.Log(err)
	case album, closed := <-albumCh:
		t.Logf("album: %s\n", album.Title)
		if !closed {
			goto LOOP
		}
	}

	// success?
}

func TestGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &http.Client{
		Transport: mockRoundTripper{
			roundTripperFn: func(req *http.Request) (*http.Response, error) {
				resp := GetAlbumResponse{
					Album: Album{ID: "1"},
				}

				data, err := json.Marshal(&resp)
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

	album, err := Get(ctx, client, GetAlbumRequest{AlbumID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if album.ID != "1" {
		t.Errorf("album id %s not expected %s", album.ID, "1")
	}
}

func TestCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &http.Client{
		Transport: mockRoundTripper{
			roundTripperFn: func(req *http.Request) (*http.Response, error) {
				resp := CreateAlbumResponse{
					Album: Album{ID: "1"},
				}

				data, err := json.Marshal(&resp)
				if err != nil {
					// TODO: respond badly?
					return nil, err
				}
				buf := bytes.NewBuffer(data)
				r := bufio.NewReader(buf) // reader for streaming

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

	album, err := Create(ctx, client, CreateAlbumRequest{Album: Album{ID: "12"}})
	if err != nil {
		t.Fatal(err)
	}
	if album.ID != "1" {
		t.Errorf("album id %s not expected %s", album.ID, "1")
	}
}
